package redis

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"go_hw/internal/adapter/postgres"
	"go_hw/internal/domain"
	"go_hw/internal/usecase"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestRedisSearchIntegration(t *testing.T) {
	addr, dsn := os.Getenv("TEST_REDIS_ADDR"), os.Getenv("TEST_DATABASE_URL")
	if addr == "" || dsn == "" {
		t.Skip("TEST_REDIS_ADDR and TEST_DATABASE_URL are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("search_%d", time.Now().UnixNano())
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
	repo, err := postgres.New(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	if err = repo.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	for _, m := range []domain.Movie{
		{ID: 1, Title: "Solaris", Year: 1972, Director: "Andrei Tarkovsky"},
		{ID: 2, Title: "Солярис 2", Year: 2002, Director: "Другой Режиссёр"},
		{ID: 3, Title: "Stalker", Year: 1979, Director: "Andrei Tarkovsky"},
	} {
		if err = repo.Save(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	store, err := New(ctx, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := usecase.NewSearch(repo, store, store)
	if err = service.BuildIndex(ctx); err != nil {
		t.Fatal(err)
	}
	check := func(q string, expected []int) {
		t.Helper()
		movies, err := service.Search(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		actual := make([]int, 0, len(movies))
		for _, m := range movies {
			actual = append(actual, m.ID)
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("%q: %v want %v", q, actual, expected)
		}
	}
	check("Solaris", []int{1})
	check("Andrei", []int{1, 3})
	check("Tarkovsky", []int{1, 3})
	check("SOLARIS", []int{1})
	check("  Solaris Andrei  ", []int{1})
	check("солЯрис", []int{2})
	check("сол", []int{})
	check("missing", []int{})
	check("!!!", []int{})
	check("1972", []int{})
	cached, _, err := store.Get(ctx, "Solaris")
	if err != nil || len(cached) != 1 || cached[0].Year != 1972 || cached[0].Director != "Andrei Tarkovsky" {
		t.Fatalf("%v %v", cached, err)
	}
	ttl, err := store.client.TTL(ctx, cacheKey("Solaris")).Result()
	if err != nil || ttl > time.Hour || ttl < 3595*time.Second {
		t.Fatalf("ttl=%v err=%v", ttl, err)
	}
	indexTTL, err := store.client.TTL(ctx, indexPrefix+"solaris").Result()
	if err != nil || indexTTL != -1 {
		t.Fatalf("index ttl=%v err=%v", indexTTL, err)
	}
	if err = store.client.PExpire(ctx, cacheKey("Solaris"), 90*time.Second).Err(); err != nil {
		t.Fatal(err)
	}
	check("Solaris", []int{1})
	ttl, err = store.client.PTTL(ctx, cacheKey("Solaris")).Result()
	if err != nil || ttl > 90*time.Second {
		t.Fatalf("cache read extended ttl: %v %v", ttl, err)
	}
	if err = repo.Save(ctx, domain.Movie{ID: 4, Title: "Solaris Returns", Year: 2020, Director: "New Director"}); err != nil {
		t.Fatal(err)
	}
	if err = service.BuildIndex(ctx); err != nil {
		t.Fatal(err)
	}
	if _, hit, err := store.Get(ctx, "Solaris"); err != nil || hit {
		t.Fatalf("old cache survived: %v %v", hit, err)
	}
	check("Solaris", []int{1, 4})
	if err = store.Replace(ctx, map[string][]int{}); err != nil {
		t.Fatal(err)
	}
	if err = service.BuildIndex(ctx); err != nil {
		t.Fatal(err)
	}
	if err = service.BuildIndex(ctx); err != nil {
		t.Fatal(err)
	}
	check("Solaris", []int{1, 4})
	if err = store.client.SRem(ctx, indexPrefix+"solaris", 1, 4).Err(); err != nil {
		t.Fatal(err)
	}
	check("solaris", []int{})
	if err = service.BuildIndex(ctx); err != nil {
		t.Fatal(err)
	}
	check("Solaris", []int{1, 4})
	repo.Close()
	check("Solaris", []int{1, 4})
	if _, err = service.Search(ctx, "Tarkovsky"); err == nil {
		t.Fatal("expected uncached database failure")
	}
	if _, hit, err := store.Get(ctx, "Tarkovsky"); err != nil || hit {
		t.Fatalf("error was cached: %v %v", hit, err)
	}
	if _, err = service.Search(ctx, " "); domain.Code(err) != "invalid_argument" {
		t.Fatal(err)
	}
	if _, hit, err := store.Get(ctx, ""); err != nil || hit {
		t.Fatalf("validation error was cached: %v %v", hit, err)
	}
}
