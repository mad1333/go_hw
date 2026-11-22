package redis

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	redisclient "github.com/redis/go-redis/v9"
	"go_hw/internal/domain"
	"strconv"
	"time"
)

const indexPrefix = "go_hw:index:"
const cachePrefix = "go_hw:cache:"
const cacheTTL = time.Hour

type Store struct{ client *redisclient.Client }

func New(ctx context.Context, addr string) (*Store, error) {
	client := redisclient.NewClient(&redisclient.Options{Addr: addr, ContextTimeoutEnabled: true, DialTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second})
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("redis connection: %w", err)
	}
	return &Store{client: client}, nil
}
func (s *Store) Close() error { return s.client.Close() }
func (s *Store) Replace(ctx context.Context, index map[string][]int) error {
	keys := []string{}
	for _, prefix := range []string{indexPrefix, cachePrefix} {
		var cursor uint64
		for {
			batch, next, err := s.client.Scan(ctx, cursor, prefix+"*", 1000).Result()
			if err != nil {
				return err
			}
			keys = append(keys, batch...)
			cursor = next
			if cursor == 0 {
				break
			}
		}
	}
	_, err := s.client.TxPipelined(ctx, func(pipe redisclient.Pipeliner) error {
		for start := 0; start < len(keys); start += 1000 {
			end := start + 1000
			if end > len(keys) {
				end = len(keys)
			}
			pipe.Del(ctx, keys[start:end]...)
		}
		for word, ids := range index {
			members := make([]interface{}, len(ids))
			for i, id := range ids {
				members[i] = id
			}
			if len(members) > 0 {
				pipe.SAdd(ctx, indexPrefix+word, members...)
			}
		}
		return nil
	})
	return err
}
func (s *Store) Find(ctx context.Context, words []string) ([]int, error) {
	if len(words) == 0 {
		return []int{}, nil
	}
	keys := make([]string, len(words))
	for i, word := range words {
		keys[i] = indexPrefix + word
	}
	values, err := s.client.SInter(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(values))
	for _, value := range values {
		id, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("invalid indexed movie id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
func cacheKey(query string) string {
	return fmt.Sprintf("%s%x", cachePrefix, sha256.Sum256([]byte(query)))
}
func (s *Store) Get(ctx context.Context, query string) ([]domain.Movie, bool, error) {
	data, err := s.client.Get(ctx, cacheKey(query)).Bytes()
	if errors.Is(err, redisclient.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var movies []domain.Movie
	if err = json.Unmarshal(data, &movies); err != nil {
		return nil, false, err
	}
	if movies == nil {
		movies = []domain.Movie{}
	}
	return movies, true, nil
}
func (s *Store) Set(ctx context.Context, query string, movies []domain.Movie) error {
	data, err := json.Marshal(movies)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, cacheKey(query), data, cacheTTL).Err()
}
