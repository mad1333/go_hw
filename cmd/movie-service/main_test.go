package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"go_hw/internal/domain"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestStartupAndRestart(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("startup_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	t.Setenv("DATABASE_URL", parsed.String())
	var last atomic.Int32
	last.Store(3)
	var offline atomic.Bool
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if offline.Load() {
			w.WriteHeader(503)
			return
		}
		var id int
		fmt.Sscanf(r.URL.Path, "/%d/info.0.json", &id)
		if id < 1 || id > int(last.Load()) {
			w.WriteHeader(404)
			return
		}
		json.NewEncoder(w).Encode(domain.Movie{ID: id, Title: "Film", Year: 2020, Director: "Director"})
	}))
	defer upstream.Close()
	t.Setenv("MOVIE_BASE_URL", upstream.URL)
	for iteration := 0; iteration < 2; iteration++ {
		if iteration == 1 {
			if _, err = pool.Exec(ctx, "DELETE FROM "+schema+".movies WHERE id=2"); err != nil {
				t.Fatal(err)
			}
			last.Store(4)
			offline.Store(false)
		}
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := listener.Addr().String()
		listener.Close()
		t.Setenv("HTTP_ADDR", addr)
		t.Setenv("GRPC_ADDR", "127.0.0.1:0")
		runCtx, stop := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- serve(runCtx) }()
		func() {
			defer func() {
				stop()
				select {
				case err := <-done:
					if err != nil {
						t.Error(err)
					}
				case <-time.After(7 * time.Second):
					t.Error("shutdown timed out")
				}
			}()
			client := &http.Client{Timeout: time.Second}
			ready := false
			for !ready {
				response, err := client.Get("http://" + addr + "/last-movie")
				if err == nil {
					response.Body.Close()
					ready = response.StatusCode == 200
				}
				if ready {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatal("startup timed out")
				case <-time.After(20 * time.Millisecond):
				}
			}
			offline.Store(true)
			before := requests.Load()
			response, err := client.Get("http://" + addr + "/load-movies?from=1&to=" + fmt.Sprint(last.Load()))
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			var movies []domain.Movie
			if err = json.NewDecoder(response.Body).Decode(&movies); err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != 200 || len(movies) != int(last.Load()) {
				t.Fatalf("status=%d movies=%+v", response.StatusCode, movies)
			}
			for i, m := range movies {
				if m.ID != i+1 || m.Title != "Film" || m.Year != 2020 || m.Director != "Director" {
					t.Fatalf("%+v", m)
				}
			}
			if requests.Load() != before {
				t.Fatal("API called external service")
			}
			var count int
			if err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+schema+".movies").Scan(&count); err != nil || count != int(last.Load()) {
				t.Fatalf("count=%d err=%v", count, err)
			}
		}()
	}
}
