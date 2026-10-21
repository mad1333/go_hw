package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"
)

type movie struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Year     int    `json:"year"`
	Director string `json:"director"`
}

type config struct {
	from, to, workers int
	timeout           time.Duration
}

func parseArgs(args []string, stderr io.Writer) (config, error) {
	var c config
	fs := flag.NewFlagSet("movies", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.IntVar(&c.from, "from", 0, "first movie ID (required)")
	fs.IntVar(&c.to, "to", 0, "last movie ID (required)")
	fs.IntVar(&c.workers, "workers", 10, "number of workers")
	fs.DurationVar(&c.timeout, "timeout", 5*time.Second, "request timeout, e.g. 5s")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	provided := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { provided[f.Name] = true })
	if !provided["from"] || !provided["to"] {
		return c, errors.New("--from and --to are required")
	}
	if fs.NArg() != 0 {
		return c, errors.New("unexpected positional arguments")
	}
	if c.from > c.to {
		return c, errors.New("--from must not exceed --to")
	}
	if c.workers <= 0 {
		return c, errors.New("--workers must be positive")
	}
	if c.timeout <= 0 {
		return c, errors.New("--timeout must be positive")
	}
	return c, nil
}

func fetchMovie(ctx context.Context, client *http.Client, baseURL string, id int) (movie, error) {
	var m movie
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%d/info.0.json", baseURL, id), nil)
	if err != nil {
		return m, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return m, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return m, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil {
		return m, err
	}
	if len(data) > 1<<20 {
		return m, errors.New("response too large")
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("invalid JSON: %w", err)
	}
	if m.ID != id || m.Title == "" || m.Director == "" || m.Year == 0 {
		return m, errors.New("invalid movie fields")
	}
	return m, nil
}

func download(ctx context.Context, client *http.Client, baseURL string, c config, stdout, stderr io.Writer) error {
	type result struct {
		id    int
		movie movie
		err   error
	}
	jobs := make(chan int)
	results := make(chan result)
	var wg sync.WaitGroup
	for i := 0; i < c.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				if ctx.Err() != nil {
					return
				}
				m, err := fetchMovie(ctx, client, baseURL, id)
				select {
				case results <- result{id, m, err}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for id := c.from; ; id++ {
			select {
			case jobs <- id:
			case <-ctx.Done():
				return
			}
			if id == c.to {
				return
			}
		}
	}()
	go func() { wg.Wait(); close(results) }()
	for r := range results {
		if r.err != nil {
			fmt.Fprintf(stderr, "%d: %v\n", r.id, r.err)
			continue
		}
		fmt.Fprintf(stdout, "%d — %s — %d — %s\n", r.movie.ID, r.movie.Title, r.movie.Year, r.movie.Director)
	}
	return ctx.Err()
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	c, err := parseArgs(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	client := &http.Client{Timeout: c.timeout}
	defer client.CloseIdleConnections()
	if err := download(ctx, client, "https://homeworksite.site", c, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "download cancelled")
		return 130
	}
	return 0
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
