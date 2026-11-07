package usecase

import (
	"context"
	"errors"
	"go_hw/internal/domain"
	"math"
	"testing"
)

type providerFunc func(context.Context, int) (domain.Movie, error)

func (f providerFunc) GetMovie(ctx context.Context, id int) (domain.Movie, error) {
	return f(ctx, id)
}

func TestLastBoundary(t *testing.T) {
	for _, last := range []int{1, 2, 3, 1000, math.MaxInt} {
		calls := 0
		s := New(providerFunc(func(ctx context.Context, id int) (domain.Movie, error) {
			calls++
			if id > last {
				return domain.Movie{}, &domain.Error{Code: "not_found", Message: "missing"}
			}
			return domain.Movie{ID: id}, nil
		}))
		m, err := s.GetLastMovie(context.Background())
		if err != nil || m.ID != last || calls > 128 {
			t.Fatalf("last=%d movie=%+v calls=%d err=%v", last, m, calls, err)
		}
	}
}

func TestLoadValidationAndPartialFailure(t *testing.T) {
	s := New(providerFunc(func(ctx context.Context, id int) (domain.Movie, error) {
		if id == 2 {
			return domain.Movie{}, errors.New("unavailable")
		}
		return domain.Movie{ID: id}, nil
	}))
	if _, err := s.LoadMovies(context.Background(), 0, 2); domain.Code(err) != "invalid_argument" {
		t.Fatal(err)
	}
	b, err := s.LoadMovies(context.Background(), 1, 3)
	var partial *domain.PartialError
	if !errors.As(err, &partial) || len(b) != 2 || len(partial.Failures) != 1 || partial.Failures[0].ID != 2 || b[0].ID != 1 || b[1].ID != 3 {
		t.Fatalf("%+v %v", b, err)
	}
}

func TestLastPropagatesFailure(t *testing.T) {
	expected := errors.New("unavailable")
	s := New(providerFunc(func(ctx context.Context, id int) (domain.Movie, error) {
		if id == 1 {
			return domain.Movie{ID: 1}, nil
		}
		return domain.Movie{}, expected
	}))
	if _, err := s.GetLastMovie(context.Background()); !errors.Is(err, expected) {
		t.Fatal(err)
	}
}

func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := New(providerFunc(func(ctx context.Context, id int) (domain.Movie, error) { return domain.Movie{}, ctx.Err() }))
	if _, err := s.LoadMovies(ctx, 1, 100); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := s.GetLastMovie(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
