// Package config loads application configuration from environment variables.
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

// HasGitHubCredentials returns true when both GitHubToken and GitHubUsername
// are non-empty. Used by the composition root to decide whether to create a
// real GitHub client at startup or start with a nil client in the provider.
func (c *Config) HasGitHubCredentials() bool {
	return c.GitHubToken != "" && c.GitHubUsername != ""
}

// Load reads configuration from environment variables and returns a validated Config.
// GitHub credentials (MYGITPANEL_GITHUB_TOKEN, MYGITPANEL_GITHUB_USERNAME) are optional;
// if absent, the app starts but polling is inactive until credentials are provided via GUI.
// Optional variables with defaults: MYGITPANEL_POLL_INTERVAL (5m),
// MYGITPANEL_LISTEN_ADDR (127.0.0.1:8080), MYGITPANEL_DB_PATH (mygitpanel.db).
func Load() (*Config, error) {
	token := os.Getenv("MYGITPANEL_GITHUB_TOKEN")
	username := os.Getenv("MYGITPANEL_GITHUB_USERNAME")

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
