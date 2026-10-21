package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct{ BaseURL, HTTPAddr, GRPCAddr string }

func Load() (Config, error) {
	c := Config{value("MOVIE_BASE_URL", "https://homeworksite.site"), value("HTTP_ADDR", ":8080"), value("GRPC_ADDR", ":9090")}
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" {
		return c, fmt.Errorf("invalid MOVIE_BASE_URL")
	}
	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	if c.HTTPAddr == "" || c.GRPCAddr == "" {
		return c, fmt.Errorf("server addresses must not be empty")
	}
	return c, nil
}
func value(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
