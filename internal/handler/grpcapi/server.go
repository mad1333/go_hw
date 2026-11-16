package grpcapi

import (
	"context"
	"errors"
	pb "go_hw/api/movies/v1"
	"go_hw/internal/domain"
	"go_hw/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
)

type Server struct {
	pb.UnimplementedMovieServiceServer
	service usecase.MovieUseCase
}

func grpcError(err error) error {
	code := codes.Unavailable
	switch domain.Code(err) {
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
func convert(m domain.Movie) *pb.Movie {
	return &pb.Movie{Id: int64(m.ID), Title: m.Title, Year: m.Year, Director: m.Director}
}
func (s *Server) LoadMovies(ctx context.Context, r *pb.LoadMoviesRequest) (*pb.LoadMoviesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	movies, err := s.service.LoadMovies(ctx, int(r.GetFrom()), int(r.GetTo()))
	var partial *domain.PartialError
	failures := []domain.Failure{}
	if errors.As(err, &partial) {
		failures = partial.Failures
		err = nil
	}
	if err != nil {
		return nil, grpcError(err)
	}
	out := &pb.LoadMoviesResponse{}
	for _, m := range movies {
		out.Movies = append(out.Movies, convert(m))
	}
	for _, e := range failures {
		out.Errors = append(out.Errors, &pb.MovieError{Id: int64(e.ID), Code: e.Code, Message: e.Message})
	}

	return out, nil
}
func (s *Server) GetLastMovie(ctx context.Context, _ *pb.GetLastMovieRequest) (*pb.Movie, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	m, err := s.service.GetLastMovie(ctx)
	if err != nil {
		return nil, grpcError(err)
	}
	return convert(m), nil
}

func New(service usecase.MovieUseCase) *Server { return &Server{service: service} }
