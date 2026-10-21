package movies

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestLast(t *testing.T) {
	for _, last := range []int64{0, 1, 2, 3, 1000, math.MaxInt64} {
		t.Run(fmt.Sprint(last), func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				var id int64
				fmt.Sscanf(r.URL.Path, "/%d/info.0.json", &id)
				if id > last {
					w.WriteHeader(404)
					return
				}
				fmt.Fprintf(w, `{"id":%d,"title":"Film","year":2020,"director":"Director"}`, id)
			}))
			defer srv.Close()
			m, err := New(srv.URL, srv.Client(), time.Second).Last(context.Background())
			if last == 0 {
				if Code(err) != "not_found" {
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
	if _, err := New(srv.URL, srv.Client(), time.Second).Last(context.Background()); Code(err) != "upstream" {
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
			_, err := New(srv.URL, srv.Client(), time.Second).Fetch(context.Background(), 1)
			if Code(err) != tt.code {
				t.Fatalf("%v", err)
			}
		})
	}
}
func TestTimeoutAndCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer srv.Close()
	s := New(srv.URL, srv.Client(), 20*time.Millisecond)
	if _, err := s.Fetch(context.Background(), 1); Code(err) != "timeout" {
		t.Fatalf("%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Load(ctx, 1, 100); Code(err) != "canceled" {
		t.Fatalf("%v", err)
	}
	if _, err := s.Last(ctx); Code(err) != "canceled" {
		t.Fatalf("%v", err)
	}
}
func TestLoadMixed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var id int64
		fmt.Sscanf(r.URL.Path, "/%d/info.0.json", &id)
		if id == 2 {
			w.WriteHeader(500)
			return
		}
		fmt.Fprintf(w, `{"id":%d,"title":"Film","year":2020,"director":"Director"}`, id)
	}))
	defer srv.Close()
	b, err := New(srv.URL, srv.Client(), time.Second).Load(context.Background(), 1, 3)
	if err != nil || len(b.Movies) != 2 || len(b.Errors) != 1 || b.Errors[0].ID != 2 || b.Movies[0].ID != 1 || b.Movies[1].ID != 3 {
		t.Fatalf("%+v %v", b, err)
	}
}
