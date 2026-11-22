package usecase

import (
	"context"
	"go_hw/internal/domain"
	"strings"
	"unicode"
)

type SearchRepository interface {
	AllMovies(context.Context) ([]domain.Movie, error)
	MoviesByIDs(context.Context, []int) ([]domain.Movie, error)
}
type SearchIndex interface {
	Replace(context.Context, map[string][]int) error
	Find(context.Context, []string) ([]int, error)
}
type SearchCache interface {
	Get(context.Context, string) ([]domain.Movie, bool, error)
	Set(context.Context, string, []domain.Movie) error
}
type MovieSearch interface {
	Search(context.Context, string) ([]domain.Movie, error)
}
type SearchService struct {
	repository SearchRepository
	index      SearchIndex
	cache      SearchCache
}

func NewSearch(repository SearchRepository, index SearchIndex, cache SearchCache) *SearchService {
	return &SearchService{repository: repository, index: index, cache: cache}
}
func Words(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}
func (s *SearchService) BuildIndex(ctx context.Context) error {
	movies, err := s.repository.AllMovies(ctx)
	if err != nil {
		return err
	}
	index := make(map[string][]int)
	for _, movie := range movies {
		if err := ctx.Err(); err != nil {
			return err
		}
		seen := make(map[string]bool)
		for _, word := range Words(movie.Title + " " + movie.Director) {
			if !seen[word] {
				index[word] = append(index[word], movie.ID)
				seen[word] = true
			}
		}
	}
	return s.index.Replace(ctx, index)
}
func (s *SearchService) Search(ctx context.Context, query string) ([]domain.Movie, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, &domain.Error{Code: "invalid_argument", Message: "q must not be empty"}
	}
	movies, hit, err := s.cache.Get(ctx, query)
	if err != nil {
		return nil, err
	}
	if hit {
		return movies, nil
	}
	ids, err := s.index.Find(ctx, Words(query))
	if err != nil {
		return nil, err
	}
	movies = []domain.Movie{}
	if len(ids) > 0 {
		movies, err = s.repository.MoviesByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		if movies == nil {
			movies = []domain.Movie{}
		}
	}
	if err = s.cache.Set(ctx, query, movies); err != nil {
		return nil, err
	}
	return movies, nil
}
