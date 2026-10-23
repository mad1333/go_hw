package httpapi

import (
	"context"
	_ "embed"
	"encoding/json"
	swaggerfiles "github.com/swaggo/files/v2"
	"go_hw/internal/movies"
	"net/http"
	"strconv"
	"time"
)

//go:embed openapi.json
var spec []byte

const index = `<!doctype html><html><head><meta charset="utf-8"><title>Movies API</title><link rel="stylesheet" href="/swagger/assets/swagger-ui.css"></head><body><div id="swagger-ui"></div><script src="/swagger/assets/swagger-ui-bundle.js"></script><script>SwaggerUIBundle({url:"/swagger/openapi.json",dom_id:"#swagger-ui"});</script></body></html>`

func New(s *movies.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /load-movies", func(w http.ResponseWriter, r *http.Request) {
		from, e1 := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
		to, e2 := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64)
		if e1 != nil || e2 != nil || len(r.URL.Query()["from"]) != 1 || len(r.URL.Query()["to"]) != 1 {
			write(w, 400, map[string]string{"error": "from and to must each be a single integer"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		b, err := s.Load(ctx, from, to)
		if err != nil {
			writeError(w, err)
			return
		}
		if len(b.Errors) > 0 {
			code := http.StatusMultiStatus
			if len(b.Movies) == 0 {
				code = http.StatusBadGateway
			}
			write(w, code, b)
			return
		}
		write(w, 200, b.Movies)
	})
	mux.HandleFunc("GET /last-movie", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		m, err := s.Last(ctx)
		if err != nil {
			writeError(w, err)
			return
		}
		write(w, 200, m)
	})
	mux.HandleFunc("GET /swagger/index.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(index))
	})
	mux.HandleFunc("GET /swagger/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(spec)
	})
	mux.Handle("GET /swagger/assets/", http.StripPrefix("/swagger/assets/", http.FileServer(http.FS(swaggerfiles.FS))))
	return mux
}
func write(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, err error) {
	code := http.StatusBadGateway
	switch movies.Code(err) {
	case "invalid_argument":
		code = 400
	case "not_found":
		code = 404
	case "rate_limited":
		code = 503
	case "timeout":
		code = 504
	case "canceled":
		code = 408
	}
	write(w, code, map[string]string{"error": err.Error()})
}
