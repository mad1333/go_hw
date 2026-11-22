package usecase

import (
	"context"
	"errors"
	"go_hw/internal/domain"
	"reflect"
	"testing"
)

type searchRepoFake struct {
	movies []domain.Movie
	ids    []int
	reads  int
	err    error
}

func (r *searchRepoFake) AllMovies(context.Context) ([]domain.Movie, error) { return r.movies, r.err }
func (r *searchRepoFake) MoviesByIDs(_ context.Context, ids []int) ([]domain.Movie, error) {
	r.reads++
	r.ids = ids
	return r.movies, r.err
}

type indexFake struct {
	words []string
	index map[string][]int
	ids   []int
}

func (i *indexFake) Replace(_ context.Context, index map[string][]int) error {
	i.index = index
	return nil
}
func (i *indexFake) Find(_ context.Context, words []string) ([]int, error) {
	i.words = words
	return i.ids, nil
}

type cacheFake struct {
	values map[string][]domain.Movie
	writes int
}

func (c *cacheFake) Get(_ context.Context, q string) ([]domain.Movie, bool, error) {
	m, ok := c.values[q]
	return m, ok, nil
}
func (c *cacheFake) Set(_ context.Context, q string, m []domain.Movie) error {
	c.values[q] = m
	c.writes++
	return nil
}

func TestWords(t *testing.T) {
	actual := Words("  СОЛЯРИС — Andrei, Tarkovsky! 2001/a2 ")
	expected := []string{"солярис", "andrei", "tarkovsky", "2001", "a2"}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("%v", actual)
	}
}
func TestIndexAndCachedSearch(t *testing.T) {
	ctx := context.Background()
	repo := &searchRepoFake{movies: []domain.Movie{{ID: 1, Title: "Solaris Solaris", Director: "Andrei Tarkovsky", Year: 1972}}}
	index := &indexFake{ids: []int{1}}
	cache := &cacheFake{values: map[string][]domain.Movie{}}
	service := NewSearch(repo, index, cache)
	if err := service.BuildIndex(ctx); err != nil {
		t.Fatal(err)
	}
	expected := map[string][]int{"solaris": {1}, "andrei": {1}, "tarkovsky": {1}}
	if !reflect.DeepEqual(index.index, expected) {
		t.Fatalf("%v", index.index)
	}
	first, err := service.Search(ctx, " SOLARIS Andrei ")
	if err != nil || len(first) != 1 || !reflect.DeepEqual(index.words, []string{"solaris", "andrei"}) || !reflect.DeepEqual(repo.ids, []int{1}) {
		t.Fatalf("%v %v", first, err)
	}
	repo.err = errors.New("database unavailable")
	second, err := service.Search(ctx, "SOLARIS Andrei")
	if err != nil || !reflect.DeepEqual(first, second) || repo.reads != 1 || cache.writes != 1 {
		t.Fatalf("%v %v reads=%d writes=%d", second, err, repo.reads, cache.writes)
	}
	if _, err = service.Search(ctx, "Tarkovsky"); err == nil || cache.writes != 1 {
		t.Fatal("database error was cached")
	}
}
func TestInvalidAndEmptySearch(t *testing.T) {
	repo := &searchRepoFake{}
	index := &indexFake{}
	cache := &cacheFake{values: map[string][]domain.Movie{}}
	service := NewSearch(repo, index, cache)
	for _, q := range []string{"", "  ", "\t\n"} {
		if _, err := service.Search(context.Background(), q); domain.Code(err) != "invalid_argument" {
			t.Fatal(err)
		}
	}
	for _, q := range []string{"missing", "!!!"} {
		m, err := service.Search(context.Background(), q)
		if err != nil || m == nil || len(m) != 0 {
			t.Fatalf("%v %v", m, err)
		}
	}
	if repo.reads != 0 || cache.writes != 2 {
		t.Fatalf("reads=%d writes=%d", repo.reads, cache.writes)
	}
}
