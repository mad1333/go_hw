package grpcapi

import (
	"context"
	pb "go_hw/api/movies/v1"
	"go_hw/internal/movies"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
)

type Server struct {
	pb.UnimplementedMovieServiceServer
	Service *movies.Service
}

func grpcError(err error) error {
	code := codes.Unavailable
	switch movies.Code(err) {
	case "invalid_argument":
		code = codes.InvalidArgument
	case "not_found":
		code = codes.NotFound
	case "rate_limited":
		code = codes.ResourceExhausted
	case "timeout":
		code = codes.DeadlineExceeded
	case "canceled":
		code = codes.Canceled
	}
	return status.Error(code, err.Error())
}
func convert(m movies.Movie) *pb.Movie {
	return &pb.Movie{Id: m.ID, Title: m.Title, Year: m.Year, Director: m.Director}
}
func (s *Server) LoadMovies(ctx context.Context, r *pb.LoadMoviesRequest) (*pb.LoadMoviesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	b, err := s.Service.Load(ctx, r.GetFrom(), r.GetTo())
	if err != nil {
		return nil, grpcError(err)
	}
	out := &pb.LoadMoviesResponse{}
	for _, m := range b.Movies {
		out.Movies = append(out.Movies, convert(m))
	}
	for _, e := range b.Errors {
		out.Errors = append(out.Errors, &pb.MovieError{Id: e.ID, Code: e.Code, Message: e.Message})
	}
	// Per-ID failures are part of the response, including an entirely failed range.
	return out, nil
}
func (s *Server) GetLastMovie(ctx context.Context, _ *pb.GetLastMovieRequest) (*pb.Movie, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	m, err := s.Service.Last(ctx)
	if err != nil {
		return nil, grpcError(err)
	}
	return convert(m), nil
}
