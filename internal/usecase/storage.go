package usecase

import (
	"context"
	"fmt"
	"go_hw/internal/domain"
)

type MovieRepository interface {
	MovieProvider
	GetLastMovie(context.Context) (domain.Movie, error)
	ExistingIDs(context.Context) (map[int64]bool, error)
	Save(context.Context, domain.Movie) error
}

type StoredService struct {
	loader     *Service
	repository MovieRepository
}

func NewStored(repository MovieRepository) *StoredService {
	return &StoredService{loader: New(repository), repository: repository}
}
func (s *StoredService) LoadMovies(ctx context.Context, from, to int64) (domain.Batch, error) {
	return s.loader.LoadMovies(ctx, from, to)
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
	for id := int64(1); id <= last.ID; id++ {
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
