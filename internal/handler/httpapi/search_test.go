package httpapi

import (
	"context"
	"errors"
	"go_hw/internal/domain"
	"net/http/httptest"
	"testing"
)

type searchFunc func(context.Context, string) ([]domain.Movie, error)

func (f searchFunc) Search(ctx context.Context, q string) ([]domain.Movie, error) { return f(ctx, q) }
func TestSearchResponses(t *testing.T) {
	tests := []struct {
		path   string
		result []domain.Movie
		err    error
		status int
	}{
		{"/search?q=Solaris", []domain.Movie{{ID: 1, Title: "Solaris", Year: 1972, Director: "Andrei Tarkovsky"}}, nil, 200},
		{"/search?q=missing", []domain.Movie{}, nil, 200},
		{"/search", nil, &domain.Error{Code: "invalid_argument", Message: "q must not be empty"}, 400},
		{"/search?q=Solaris", nil, errors.New("database unavailable"), 500},
	}
	for _, tt := range tests {
		handler := New(nil, searchFunc(func(context.Context, string) ([]domain.Movie, error) { return tt.result, tt.err }))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", tt.path, nil))
		if w.Code != tt.status {
			t.Fatalf("%s: %d %s", tt.path, w.Code, w.Body.String())
		}
		if tt.status == 200 && len(tt.result) == 0 && w.Body.String() != "[]\n" {
			t.Fatal(w.Body.String())
		}
		if tt.status >= 400 && w.Body.Len() == 0 {
			t.Fatal("empty error")
		}
	}
}
