package moviehttp

import (
	"context"
	"encoding/json"
	"fmt"
	"go_hw/internal/domain"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	client  *http.Client
	timeout time.Duration
}

func New(baseURL string, client *http.Client, timeout time.Duration) *Client {
	return &Client{baseURL, client, timeout}
}
func (s *Client) GetMovie(ctx context.Context, id int64) (domain.Movie, error) {
	var m domain.Movie
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%d/info.0.json", s.baseURL, id), nil)
	if err != nil {
		return m, &domain.Error{"upstream", "cannot create upstream request"}
	}
	resp, err := s.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return m, ctx.Err()
		}
		return m, &domain.Error{"upstream", "upstream connection failed"}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		code := "upstream"
		if resp.StatusCode == 404 {
			code = "not_found"
		}
		if resp.StatusCode == 429 {
			code = "rate_limited"
		}
		return m, &domain.Error{code, fmt.Sprintf("upstream returned HTTP %d", resp.StatusCode)}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		if ctx.Err() != nil {
			return m, ctx.Err()
		}
		return m, &domain.Error{"upstream", "cannot read upstream response"}
	}
	if len(data) > 1<<20 {
		return m, &domain.Error{"upstream", "upstream response too large"}
	}
	if json.Unmarshal(data, &m) != nil {
		return m, &domain.Error{"upstream", "invalid upstream JSON"}
	}
	if m.ID != id || m.Title == "" || m.Director == "" || m.Year == 0 {
		return m, &domain.Error{"upstream", "invalid upstream movie fields"}
	}
	return m, nil
}
