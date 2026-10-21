package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFlags(t *testing.T) {
	for _, args := range [][]string{nil, {"--from=1"}, {"--to=2"}, {"--from=3", "--to=2"}, {"--from=1", "--to=2", "--workers=0"}, {"--from=1", "--to=2", "--timeout=0s"}, {"--from=x", "--to=2"}, {"--from=1", "--to=2", "--timeout=bad"}, {"--from=1", "--to=2", "extra"}, {"--unknown"}} {
		if code := run(context.Background(), args, io.Discard, io.Discard); code == 0 {
			t.Fatalf("accepted %v", args)
		}
	}
	c, err := parseArgs([]string{"--from=0", "--to=2"}, io.Discard)
	if err != nil || c.from != 0 || c.to != 2 || c.workers != 10 || c.timeout != 5*time.Second {
		t.Fatalf("defaults: %+v, %v", c, err)
	}
	c, err = parseArgs([]string{"--from=1", "--to=2", "--workers=3", "--timeout=25ms"}, io.Discard)
	if err != nil || c.workers != 3 || c.timeout != 25*time.Millisecond {
		t.Fatalf("overrides: %+v, %v", c, err)
	}
}

func TestDownloadContinuesAfterErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var id int
		fmt.Sscanf(r.URL.Path, "/%d/info.0.json", &id)
		switch id {
		case 2:
			w.WriteHeader(404)
		case 3:
			w.WriteHeader(429)
		case 4:
			w.WriteHeader(500)
		case 5:
			fmt.Fprint(w, "{broken")
		case 6:
			<-r.Context().Done()
		case 7:
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				conn.Close()
			}
		default:
			fmt.Fprintf(w, `{"id":%d,"title":"Film","year":2020,"director":"Director"}`, id)
		}
	}))
	defer server.Close()
	var out, errs bytes.Buffer
	err := download(context.Background(), &http.Client{Timeout: 100 * time.Millisecond}, server.URL, config{from: 1, to: 8, workers: 3}, &out, &errs)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), " — ") != 6 || !strings.Contains(out.String(), "1 — Film") || !strings.Contains(out.String(), "8 — Film") {
		t.Fatal(out.String())
	}
	for id := 2; id <= 7; id++ {
		if !strings.Contains(errs.String(), fmt.Sprintf("%d: ", id)) {
			t.Fatalf("missing error %d: %s", id, errs.String())
		}
	}
}

func TestWorkerLimit(t *testing.T) {
	var active, peak atomic.Int32
	started := make(chan struct{}, 10)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		started <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		var id int
		fmt.Sscanf(r.URL.Path, "/%d/info.0.json", &id)
		fmt.Fprintf(w, `{"id":%d,"title":"Film","year":2020,"director":"Director"}`, id)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- download(ctx, server.Client(), server.URL, config{from: 1, to: 6, workers: 3}, io.Discard, io.Discard)
	}()
	for i := 0; i < 3; i++ {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("workers did not run concurrently")
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if peak.Load() != 3 {
		t.Fatalf("peak=%d", peak.Load())
	}
}

func TestCancellation(t *testing.T) {
	started := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { started <- struct{}{}; <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- download(ctx, server.Client(), server.URL, config{from: 1, to: 100, workers: 2}, io.Discard, io.Discard)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request not started")
	}
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation hung")
	}
}
