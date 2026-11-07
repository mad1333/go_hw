package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	pb "go_hw/api/movies/v1"
	"go_hw/internal/adapter/moviehttp"
	"go_hw/internal/domain"
	"go_hw/internal/handler/grpcapi"
	"go_hw/internal/handler/httpapi"
	"go_hw/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestSyncAndStoredAPI(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repository, err := New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	if _, err = repository.pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer repository.pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	cfg := repository.pool.Config()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	isolated := &Repository{pool: pool}
	defer isolated.Close()
	if err = isolated.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = isolated.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int64
	var fail atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if fail.Load() {
			w.WriteHeader(500)
			return
		}
		var id int
		fmt.Sscanf(r.URL.Path, "/%d/info.0.json", &id)
		if id < 1 || id > 5 {
			w.WriteHeader(404)
			return
		}
		json.NewEncoder(w).Encode(domain.Movie{ID: id, Title: fmt.Sprintf("Film %d", id), Year: 2020, Director: "Director"})
	}))
	defer upstream.Close()
	provider := moviehttp.New(upstream.URL, upstream.Client(), time.Second)
	syncer := usecase.NewSynchronizer(provider, isolated)
	if err = syncer.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if err = syncer.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	ids, err := isolated.ExistingIDs(ctx)
	if err != nil || len(ids) != 5 {
		t.Fatalf("ids=%v err=%v", ids, err)
	}
	if _, err = isolated.pool.Exec(ctx, "DELETE FROM movies WHERE id=2"); err != nil {
		t.Fatal(err)
	}
	if err = syncer.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	m, err := isolated.GetMovie(ctx, 2)
	if err != nil || m.Title != "Film 2" || m.Year != 2020 || m.Director != "Director" {
		t.Fatalf("%+v %v", m, err)
	}
	fail.Store(true)
	before := requests.Load()
	service := usecase.NewStored(isolated)
	handler := httpapi.New(service)
	for _, path := range []string{"/load-movies?from=1&to=5", "/last-movie"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	listener := bufconn.Listen(1 << 20)
	defer listener.Close()
	server := grpc.NewServer()
	pb.RegisterMovieServiceServer(server, grpcapi.New(service))
	go server.Serve(listener)
	defer server.Stop()
	conn, err := grpc.NewClient("passthrough:///test", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewMovieServiceClient(conn)
	batch, err := client.LoadMovies(ctx, &pb.LoadMoviesRequest{From: 1, To: 5})
	if err != nil || len(batch.GetMovies()) != 5 {
		t.Fatalf("%+v %v", batch, err)
	}
	last, err := client.GetLastMovie(ctx, &pb.GetLastMovieRequest{})
	if err != nil || last.GetId() != 5 {
		t.Fatalf("%+v %v", last, err)
	}
	if requests.Load() != before {
		t.Fatal("API requested external movies")
	}
}
