package movies

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"
)

type Movie struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Year     int32  `json:"year"`
	Director string `json:"director"`
}
type Failure struct {
	ID      int64  `json:"id"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
type Batch struct {
	Movies []Movie   `json:"movies"`
	Errors []Failure `json:"errors"`
}
type Error struct{ Code, Message string }

func (e *Error) Error() string { return e.Message }
func Code(err error) string {
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return "upstream"
}
func Validate(from, to int64) error {
	if from < 1 || to < from {
		return &Error{"invalid_argument", "from and to must be positive, and from must not exceed to"}
	}
	return nil
}

type Service struct {
	baseURL string
	client  *http.Client
	timeout time.Duration
}

func New(baseURL string, client *http.Client, timeout time.Duration) *Service {
	return &Service{baseURL, client, timeout}
}
func (s *Service) Fetch(ctx context.Context, id int64) (Movie, error) {
	var m Movie
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%d/info.0.json", s.baseURL, id), nil)
	if err != nil {
		return m, &Error{"upstream", "cannot create upstream request"}
	}
	resp, err := s.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return m, ctx.Err()
		}
		return m, &Error{"upstream", "upstream connection failed"}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		code := "upstream"
		if resp.StatusCode == 404 {
			code = "not_found"
		}
		if resp.StatusCode == 429 {
			code = "rate_limited"
		}
		return m, &Error{code, fmt.Sprintf("upstream returned HTTP %d", resp.StatusCode)}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		if ctx.Err() != nil {
			return m, ctx.Err()
		}
		return m, &Error{"upstream", "cannot read upstream response"}
	}
	if len(data) > 1<<20 {
		return m, &Error{"upstream", "upstream response too large"}
	}
	if json.Unmarshal(data, &m) != nil {
		return m, &Error{"upstream", "invalid upstream JSON"}
	}
	if m.ID != id || m.Title == "" || m.Director == "" || m.Year == 0 {
		return m, &Error{"upstream", "invalid upstream movie fields"}
	}
	return m, nil
}
func (s *Service) Load(ctx context.Context, from, to int64) (Batch, error) {
	b := Batch{Movies: []Movie{}, Errors: []Failure{}}
	if err := Validate(from, to); err != nil {
		return b, err
	}
	type result struct {
		id  int64
		m   Movie
		err error
	}
	jobs := make(chan int64)
	results := make(chan result)
	var wg sync.WaitGroup
	workers := int64(10)
	if to-from+1 < workers {
		workers = to - from + 1
	}
	for i := int64(0); i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				if ctx.Err() != nil {
					return
				}
				m, err := s.Fetch(ctx, id)
				select {
				case results <- result{id, m, err}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for id := from; ; id++ {
			select {
			case jobs <- id:
			case <-ctx.Done():
				return
			}
			if id == to {
				return
			}
		}
	}()
	go func() { wg.Wait(); close(results) }()
	for r := range results {
		if r.err != nil {
			b.Errors = append(b.Errors, Failure{r.id, Code(r.err), r.err.Error()})
		} else {
			b.Movies = append(b.Movies, r.m)
		}
	}
	sort.Slice(b.Movies, func(i, j int) bool { return b.Movies[i].ID < b.Movies[j].ID })
	sort.Slice(b.Errors, func(i, j int) bool { return b.Errors[i].ID < b.Errors[j].ID })
	return b, ctx.Err()
}
func (s *Service) Last(ctx context.Context) (Movie, error) {
	last, err := s.Fetch(ctx, 1)
	if err != nil {
		return Movie{}, err
	}
	low, high := int64(1), int64(2)
	// First bracket the boundary, then binary-search it. Only 404 means absent.
	for {
		m, err := s.Fetch(ctx, high)
		if err != nil {
			if Code(err) == "not_found" {
				break
			}
			return Movie{}, err
		}
		low, last = high, m
		if high == math.MaxInt64 {
			return last, nil
		}
		if high > math.MaxInt64/2 {
			high = math.MaxInt64
		} else {
			high *= 2
		}
	}
	for high-low > 1 {
		mid := low + (high-low)/2
		m, err := s.Fetch(ctx, mid)
		if err == nil {
			low, last = mid, m
			continue
		}
		if Code(err) != "not_found" {
			return Movie{}, err
		}
		high = mid
	}
	return last, nil
}
