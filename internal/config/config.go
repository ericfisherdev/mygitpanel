package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	GitHubToken    string
	GitHubUsername string
	PollInterval   time.Duration
	ListenAddr     string
	DBPath         string
}

// Load reads configuration from environment variables and returns a validated Config.
// Required variables: REVIEWHUB_GITHUB_TOKEN, REVIEWHUB_GITHUB_USERNAME.
// Optional variables with defaults: REVIEWHUB_POLL_INTERVAL (5m),
// REVIEWHUB_LISTEN_ADDR (127.0.0.1:8080), REVIEWHUB_DB_PATH (reviewhub.db).
func Load() (*Config, error) {
	token, ok := os.LookupEnv("REVIEWHUB_GITHUB_TOKEN")
	if !ok || token == "" {
		return nil, fmt.Errorf("REVIEWHUB_GITHUB_TOKEN is required but not set")
	}

	username, ok := os.LookupEnv("REVIEWHUB_GITHUB_USERNAME")
	if !ok || username == "" {
		return nil, fmt.Errorf("REVIEWHUB_GITHUB_USERNAME is required but not set")
	}

	pollInterval := 5 * time.Minute
	if v, ok := os.LookupEnv("REVIEWHUB_POLL_INTERVAL"); ok {
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("REVIEWHUB_POLL_INTERVAL has invalid duration %q: %w", v, err)
		}
		pollInterval = parsed
	}

	listenAddr := "127.0.0.1:8080"
	if v, ok := os.LookupEnv("REVIEWHUB_LISTEN_ADDR"); ok {
		listenAddr = v
	}

	dbPath := "reviewhub.db"
	if v, ok := os.LookupEnv("REVIEWHUB_DB_PATH"); ok {
		dbPath = v
	}

	return &Config{
		GitHubToken:    token,
		GitHubUsername: username,
		PollInterval:   pollInterval,
		ListenAddr:     listenAddr,
		DBPath:         dbPath,
	}, nil
}
