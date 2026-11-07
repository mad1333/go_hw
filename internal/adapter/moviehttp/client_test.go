package moviehttp_test

import (
	"context"
	"errors"
	"fmt"
	"go_hw/internal/adapter/moviehttp"
	"go_hw/internal/domain"
	"go_hw/internal/usecase"
	"math"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestLast(t *testing.T) {
	for _, last := range []int{0, 1, 2, 3, 1000, math.MaxInt} {
		t.Run(fmt.Sprint(last), func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				var id int
				fmt.Sscanf(r.URL.Path, "/%d/info.0.json", &id)
				if id > last {
					w.WriteHeader(404)
					return
				}
				fmt.Fprintf(w, `{"id":%d,"title":"Film","year":2020,"director":"Director"}`, id)
			}))
			defer srv.Close()
			m, err := newService(srv.URL, srv.Client(), time.Second).GetLastMovie(context.Background())
			if last == 0 {
				if domain.Code(err) != "not_found" {
					t.Fatalf("%v", err)
				}
				return
			}
			if err != nil || m.ID != last {
				t.Fatalf("%+v %v", m, err)
			}
			if calls.Load() > 128 {
				t.Fatalf("too many requests: %d", calls.Load())
			}
		})
	}
}
func TestLastDoesNotTreatServerErrorsAsMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/1/info.0.json" {
			w.WriteHeader(500)
			return
		}
		fmt.Fprint(w, `{"id":1,"title":"Film","year":2020,"director":"Director"}`)
	}))
	defer srv.Close()
	if _, err := newService(srv.URL, srv.Client(), time.Second).GetLastMovie(context.Background()); domain.Code(err) != "upstream" {
		t.Fatalf("%v", err)
	}
}
func TestFetchFailures(t *testing.T) {
	tests := []struct {
		name, body, code string
		status           int
	}{
		{"missing", "", "not_found", 404}, {"rate limit", "", "rate_limited", 429}, {"server error", "", "upstream", 500},
		{"bad JSON", "{", "upstream", 200}, {"null", "null", "upstream", 200}, {"wrong ID", `{"id":2,"title":"x","year":2020,"director":"y"}`, "upstream", 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tt.status); fmt.Fprint(w, tt.body) }))
			defer srv.Close()
			_, err := newService(srv.URL, srv.Client(), time.Second).GetMovie(context.Background(), 1)
			if domain.Code(err) != tt.code {
				t.Fatalf("%v", err)
			}
		})
	}
}
func TestTimeoutAndCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer srv.Close()
	s := newService(srv.URL, srv.Client(), 20*time.Millisecond)
	if _, err := s.GetMovie(context.Background(), 1); domain.Code(err) != "timeout" {
		t.Fatalf("%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.LoadMovies(ctx, 1, 100); domain.Code(err) != "canceled" {
		t.Fatalf("%v", err)
	}
	if _, err := s.GetLastMovie(ctx); domain.Code(err) != "canceled" {
		t.Fatalf("%v", err)
	}
}
func TestLoadMixed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var id int
		fmt.Sscanf(r.URL.Path, "/%d/info.0.json", &id)
		if id == 2 {
			w.WriteHeader(500)
			return
		}
		fmt.Fprintf(w, `{"id":%d,"title":"Film","year":2020,"director":"Director"}`, id)
	}))
	defer srv.Close()
	b, err := newService(srv.URL, srv.Client(), time.Second).LoadMovies(context.Background(), 1, 3)
	var partial *domain.PartialError
	if !errors.As(err, &partial) || len(b) != 2 || len(partial.Failures) != 1 || partial.Failures[0].ID != 2 || b[0].ID != 1 || b[1].ID != 3 {
		t.Fatalf("%+v %v", b, err)
	}
}

type testService struct {
	*usecase.Service
	*moviehttp.Client
}

func newService(url string, client *http.Client, timeout time.Duration) *testService {
	p := moviehttp.New(url, client, timeout)
	return &testService{usecase.New(p), p}
}
