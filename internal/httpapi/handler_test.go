package httpapi

import (
	"encoding/json"
	"fmt"
	"go_hw/internal/movies"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTP(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var id int64
		fmt.Sscanf(r.URL.Path, "/%d/info.0.json", &id)
		if id > 3 {
			w.WriteHeader(404)
			return
		}
		fmt.Fprintf(w, `{"id":%d,"title":"Film","year":2020,"director":"Director"}`, id)
	}))
	defer upstream.Close()
	h := New(movies.New(upstream.URL, upstream.Client(), time.Second))
	for _, tt := range []struct {
		url    string
		status int
	}{
		{"/load-movies?from=1&to=3", 200}, {"/load-movies?from=2&to=4", 207}, {"/load-movies?from=4&to=5", 502},
		{"/last-movie", 200}, {"/load-movies", 400}, {"/load-movies?from=a&to=3", 400}, {"/load-movies?from=4&to=1", 400}, {"/load-movies?from=0&to=3", 400}, {"/load-movies?from=1&from=2&to=3", 400},
		{"/swagger/index.html", 200}, {"/swagger/openapi.json", 200}, {"/swagger/assets/swagger-ui-bundle.js", 200}, {"/swagger/assets/swagger-ui.css", 200},
	} {
		t.Run(tt.url, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", tt.url, nil))
			if w.Code != tt.status {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
			if strings.HasPrefix(tt.url, "/load-movies") && tt.status == 200 {
				var m []movies.Movie
				if json.Unmarshal(w.Body.Bytes(), &m) != nil || len(m) != 3 {
					t.Fatal(w.Body)
				}
			}
			if tt.status == 207 {
				var b movies.Batch
				json.Unmarshal(w.Body.Bytes(), &b)
				if len(b.Movies) != 2 || len(b.Errors) != 1 || b.Errors[0].ID != 4 {
					t.Fatal(w.Body)
				}
			}
			if tt.url == "/last-movie" {
				var m movies.Movie
				json.Unmarshal(w.Body.Bytes(), &m)
				if m.ID != 3 {
					t.Fatal(w.Body)
				}
			}
			if tt.url == "/swagger/openapi.json" && !json.Valid(w.Body.Bytes()) {
				t.Fatal("invalid spec")
			}
		})
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/last-movie", nil))
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
}
func TestHTTPUpstreamErrors(t *testing.T) {
	for _, tt := range []struct{ up, want int }{{404, 404}, {429, 503}, {500, 502}} {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tt.up) }))
		h := New(movies.New(upstream.URL, upstream.Client(), time.Second))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/last-movie", nil))
		upstream.Close()
		if w.Code != tt.want {
			t.Fatalf("upstream %d: %d", tt.up, w.Code)
		}
	}
}
