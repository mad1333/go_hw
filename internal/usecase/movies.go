package usecase

import (
	"context"
	"go_hw/internal/domain"
	"math"
	"sort"
	"sync"
)

type MovieProvider interface {
	GetMovie(context.Context, int) (domain.Movie, error)
}
type MovieUseCase interface {
	LoadMovies(context.Context, int, int) ([]domain.Movie, error)
	GetLastMovie(context.Context) (domain.Movie, error)
}
type Service struct{ provider MovieProvider }

func New(provider MovieProvider) *Service { return &Service{provider: provider} }
func Validate(from, to int) error {
	if from < 1 || to < from {
		return &domain.Error{"invalid_argument", "from and to must be positive, and from must not exceed to"}
	}
	return nil
}

func (s *Service) LoadMovies(ctx context.Context, from, to int) ([]domain.Movie, error) {
	b := domain.Batch{Movies: []domain.Movie{}, Errors: []domain.Failure{}}
	if err := Validate(from, to); err != nil {
		return nil, err
	}
	type result struct {
		id  int
		m   domain.Movie
		err error
	}
	jobs := make(chan int)
	results := make(chan result)
	var wg sync.WaitGroup
	workers := int(10)
	if to-from+1 < workers {
		workers = to - from + 1
	}
	for i := int(0); i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				if ctx.Err() != nil {
					return
				}
				m, err := s.provider.GetMovie(ctx, id)
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
			b.Errors = append(b.Errors, domain.Failure{r.id, domain.Code(r.err), r.err.Error()})
		} else {
			b.Movies = append(b.Movies, r.m)
		}
	}
	sort.Slice(b.Movies, func(i, j int) bool { return b.Movies[i].ID < b.Movies[j].ID })
	sort.Slice(b.Errors, func(i, j int) bool { return b.Errors[i].ID < b.Errors[j].ID })
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(b.Errors) > 0 {
		return b.Movies, &domain.PartialError{Failures: b.Errors}
	}
	return b.Movies, nil
}
func (s *Service) GetLastMovie(ctx context.Context) (domain.Movie, error) {
	last, err := s.provider.GetMovie(ctx, 1)
	if err != nil {
		return domain.Movie{}, err
	}
	low, high := int(1), int(2)

	for {
		m, err := s.provider.GetMovie(ctx, high)
		if err != nil {
			if domain.Code(err) == "not_found" {
				break
			}
			return domain.Movie{}, err
		}
		low, last = high, m
		if high == math.MaxInt {
			return last, nil
		}
		if high > math.MaxInt/2 {
			high = math.MaxInt
		} else {
			high *= 2
		}
	}
	for high-low > 1 {
		mid := low + (high-low)/2
		m, err := s.provider.GetMovie(ctx, mid)
		if err == nil {
			low, last = mid, m
			continue
		}
		if domain.Code(err) != "not_found" {
			return domain.Movie{}, err
		}
		high = mid
	}
	return last, nil
}
