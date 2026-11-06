package config

import "testing"

func TestConfig(t *testing.T) {
	t.Setenv("MOVIE_BASE_URL", "http://localhost:1234/prefix/")
	t.Setenv("HTTP_ADDR", ":1235")
	t.Setenv("GRPC_ADDR", ":1236")
	c, err := Load()
	if err != nil || c.BaseURL != "http://localhost:1234/prefix" || c.HTTPAddr != ":1235" || c.GRPCAddr != ":1236" {
		t.Fatalf("%+v %v", c, err)
	}
	for _, v := range []string{"", ":bad", "file:///tmp/movies", "https://example.org?x=y"} {
		t.Setenv("MOVIE_BASE_URL", v)
		if _, err := Load(); err == nil {
			t.Fatalf("accepted %q", v)
		}
	}
}
