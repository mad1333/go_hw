package grpcapi

import (
	"context"
	"fmt"
	pb "go_hw/api/movies/v1"
	"go_hw/internal/movies"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func client(t *testing.T, handler http.HandlerFunc) pb.MovieServiceClient {
	t.Helper()
	upstream := httptest.NewServer(handler)
	t.Cleanup(upstream.Close)
	listener := bufconn.Listen(1 << 20)
	t.Cleanup(func() { listener.Close() })
	server := grpc.NewServer()
	t.Cleanup(server.Stop)
	pb.RegisterMovieServiceServer(server, &Server{Service: movies.New(upstream.URL, upstream.Client(), 30*time.Millisecond)})
	go server.Serve(listener)
	conn, err := grpc.NewClient("passthrough:///test", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return pb.NewMovieServiceClient(conn)
}
func TestGRPC(t *testing.T) {
	c := client(t, func(w http.ResponseWriter, r *http.Request) {
		var id int64
		fmt.Sscanf(r.URL.Path, "/%d/info.0.json", &id)
		if id > 3 {
			w.WriteHeader(404)
			return
		}
		fmt.Fprintf(w, `{"id":%d,"title":"Film","year":2020,"director":"Director"}`, id)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	b, err := c.LoadMovies(ctx, &pb.LoadMoviesRequest{From: 1, To: 3})
	if err != nil || len(b.GetMovies()) != 3 || len(b.GetErrors()) != 0 {
		t.Fatalf("%v %v", b, err)
	}
	b, err = c.LoadMovies(ctx, &pb.LoadMoviesRequest{From: 2, To: 4})
	if err != nil || len(b.GetMovies()) != 2 || len(b.GetErrors()) != 1 || b.Errors[0].Id != 4 || b.Errors[0].Code != "not_found" {
		t.Fatalf("%v %v", b, err)
	}
	for _, r := range []*pb.LoadMoviesRequest{{}, {From: -1, To: 2}, {From: 3, To: 2}} {
		if _, err = c.LoadMovies(ctx, r); status.Code(err) != codes.InvalidArgument {
			t.Fatal(err)
		}
	}
	m, err := c.GetLastMovie(ctx, &pb.GetLastMovieRequest{})
	if err != nil || m.GetId() != 3 {
		t.Fatalf("%v %v", m, err)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err = c.GetLastMovie(canceled, &pb.GetLastMovieRequest{}); status.Code(err) != codes.Canceled {
		t.Fatal(err)
	}
}
func TestGRPCErrors(t *testing.T) {
	for _, tt := range []struct {
		up   int
		want codes.Code
	}{{404, codes.NotFound}, {429, codes.ResourceExhausted}, {500, codes.Unavailable}, {0, codes.DeadlineExceeded}} {
		t.Run(fmt.Sprint(tt.up), func(t *testing.T) {
			c := client(t, func(w http.ResponseWriter, r *http.Request) {
				if tt.up == 0 {
					<-r.Context().Done()
					return
				}
				w.WriteHeader(tt.up)
			})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if _, err := c.GetLastMovie(ctx, &pb.GetLastMovieRequest{}); status.Code(err) != tt.want {
				t.Fatalf("%v", err)
			}
			b, err := c.LoadMovies(ctx, &pb.LoadMoviesRequest{From: 1, To: 2})
			if err != nil || len(b.GetErrors()) != 2 {
				t.Fatalf("%v %v", b, err)
			}
		})
	}
}
