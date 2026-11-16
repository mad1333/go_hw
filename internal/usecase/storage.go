package usecase

import (
	"context"
	"fmt"
	"go_hw/internal/domain"
)

type MovieRepository interface {
	GetMovies(context.Context, int, int) ([]domain.Movie, error)
	GetLastMovie(context.Context) (domain.Movie, error)
	ExistingIDs(context.Context) (map[int]bool, error)
	Save(context.Context, domain.Movie) error
}

type StoredService struct {
	repository MovieRepository
}

func NewStored(repository MovieRepository) *StoredService {
	return &StoredService{repository: repository}
}
func (s *StoredService) LoadMovies(ctx context.Context, from, to int) ([]domain.Movie, error) {
	if err := Validate(from, to); err != nil {
		return nil, err
	}
	movies, err := s.repository.GetMovies(ctx, from, to)
	if err != nil {
		return nil, err
	}
	found := make(map[int]bool, len(movies))
	for _, m := range movies {
		found[m.ID] = true
	}
	failures := []domain.Failure{}
	for id := from; ; id++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !found[id] {
			failures = append(failures, domain.Failure{ID: id, Code: "not_found", Message: "movie not found"})
		}
		if id == to {
			break
		}
	}
	if len(failures) > 0 {
		return movies, &domain.PartialError{Failures: failures}
	}
	return movies, nil
}
func (s *StoredService) GetLastMovie(ctx context.Context) (domain.Movie, error) {
	return s.repository.GetLastMovie(ctx)
}

type Synchronizer struct {
	provider   MovieProvider
	repository MovieRepository
}

func NewSynchronizer(provider MovieProvider, repository MovieRepository) *Synchronizer {
	return &Synchronizer{provider: provider, repository: repository}
}
func (s *Synchronizer) Sync(ctx context.Context) error {
	last, err := New(s.provider).GetLastMovie(ctx)
	if err != nil {
		if domain.Code(err) == "not_found" {
			return nil
		}
		return fmt.Errorf("find last movie: %w", err)
	}
	ids, err := s.repository.ExistingIDs(ctx)
	if err != nil {
		return fmt.Errorf("read existing movies: %w", err)
	}
	for id := int(1); id <= last.ID; id++ {
		if err = ctx.Err(); err != nil {
			return err
		}
		if !ids[id] {
			m := last
			if id != last.ID {
				m, err = s.provider.GetMovie(ctx, id)
				if err != nil {
					return fmt.Errorf("load movie %d: %w", id, err)
				}
			}
			if err = s.repository.Save(ctx, m); err != nil {
				return fmt.Errorf("save movie %d: %w", id, err)
			}
		}
		if id == last.ID {
			break
		}
	}
	return nil
}
