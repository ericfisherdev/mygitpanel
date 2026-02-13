package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	GitHubToken    string
	GitHubUsername string
	GitHubTeams    []string
	PollInterval   time.Duration
	ListenAddr     string
	DBPath         string
}

// Load reads configuration from environment variables and returns a validated Config.
// Required variables: MYGITPANEL_GITHUB_TOKEN, MYGITPANEL_GITHUB_USERNAME.
// Optional variables with defaults: MYGITPANEL_POLL_INTERVAL (5m),
// MYGITPANEL_LISTEN_ADDR (127.0.0.1:8080), MYGITPANEL_DB_PATH (mygitpanel.db).
func Load() (*Config, error) {
	token, ok := os.LookupEnv("MYGITPANEL_GITHUB_TOKEN")
	if !ok || token == "" {
		return nil, fmt.Errorf("MYGITPANEL_GITHUB_TOKEN is required but not set")
	}

	username, ok := os.LookupEnv("MYGITPANEL_GITHUB_USERNAME")
	if !ok || username == "" {
		return nil, fmt.Errorf("MYGITPANEL_GITHUB_USERNAME is required but not set")
	}

	pollInterval := 5 * time.Minute
	if v, ok := os.LookupEnv("MYGITPANEL_POLL_INTERVAL"); ok {
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("MYGITPANEL_POLL_INTERVAL has invalid duration %q: %w", v, err)
		}
		pollInterval = parsed
	}

	listenAddr := "127.0.0.1:8080"
	if v, ok := os.LookupEnv("MYGITPANEL_LISTEN_ADDR"); ok {
		listenAddr = v
	}

	dbPath := "mygitpanel.db"
	if v, ok := os.LookupEnv("MYGITPANEL_DB_PATH"); ok {
		dbPath = v
	}

	var githubTeams []string
	if v, ok := os.LookupEnv("MYGITPANEL_GITHUB_TEAMS"); ok && v != "" {
		for _, slug := range strings.Split(v, ",") {
			slug = strings.TrimSpace(slug)
			if slug != "" {
				githubTeams = append(githubTeams, slug)
			}
		}
	}
	if githubTeams == nil {
		githubTeams = []string{}
	}

	return &Config{
		GitHubToken:    token,
		GitHubUsername: username,
		GitHubTeams:    githubTeams,
		PollInterval:   pollInterval,
		ListenAddr:     listenAddr,
		DBPath:         dbPath,
	}, nil
}
