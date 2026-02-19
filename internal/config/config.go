// Package config loads application configuration from environment variables.
package config

import (
	"encoding/hex"
	"fmt"
	"log/slog"
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
	SecretKey      []byte // 32-byte AES-256 key; nil when MYGITPANEL_SECRET_KEY is not set.
}

// Load reads configuration from environment variables and returns a validated Config.
// Required variables: MYGITPANEL_GITHUB_USERNAME.
// Optional variables: MYGITPANEL_GITHUB_TOKEN (warns when absent; polling disabled until set),
// MYGITPANEL_SECRET_KEY (warns when absent; credential storage disabled).
// Optional variables with defaults: MYGITPANEL_POLL_INTERVAL (5m),
// MYGITPANEL_LISTEN_ADDR (127.0.0.1:8080), MYGITPANEL_DB_PATH (mygitpanel.db).
func Load() (*Config, error) {
	var cfg Config

	// MYGITPANEL_GITHUB_TOKEN is optional — app starts without it but polling is
	// disabled until credentials are configured via the GUI.
	token, tokenSet := os.LookupEnv("MYGITPANEL_GITHUB_TOKEN")
	if !tokenSet || token == "" {
		slog.Warn("MYGITPANEL_GITHUB_TOKEN not set — polling disabled until credentials configured via GUI")
		cfg.GitHubToken = ""
	} else {
		cfg.GitHubToken = token
	}

	username, ok := os.LookupEnv("MYGITPANEL_GITHUB_USERNAME")
	if !ok || username == "" {
		return nil, fmt.Errorf("MYGITPANEL_GITHUB_USERNAME is required but not set")
	}
	cfg.GitHubUsername = username

	// MYGITPANEL_SECRET_KEY is optional — credential storage is disabled when absent.
	if keyHex, ok := os.LookupEnv("MYGITPANEL_SECRET_KEY"); ok && keyHex != "" {
		if len(keyHex) != 64 {
			return nil, fmt.Errorf("MYGITPANEL_SECRET_KEY must be a 64-character hex string (32 bytes)")
		}
		key, err := hex.DecodeString(keyHex)
		if err != nil {
			return nil, fmt.Errorf("MYGITPANEL_SECRET_KEY must be a 64-character hex string (32 bytes)")
		}
		cfg.SecretKey = key
	} else {
		slog.Warn("MYGITPANEL_SECRET_KEY not set — credential storage disabled")
		cfg.SecretKey = nil
	}

	cfg.PollInterval = 5 * time.Minute
	if v, ok := os.LookupEnv("MYGITPANEL_POLL_INTERVAL"); ok {
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("MYGITPANEL_POLL_INTERVAL has invalid duration %q: %w", v, err)
		}
		cfg.PollInterval = parsed
	}

	cfg.ListenAddr = "127.0.0.1:8080"
	if v, ok := os.LookupEnv("MYGITPANEL_LISTEN_ADDR"); ok {
		cfg.ListenAddr = v
	}

	cfg.DBPath = "mygitpanel.db"
	if v, ok := os.LookupEnv("MYGITPANEL_DB_PATH"); ok {
		cfg.DBPath = v
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
	cfg.GitHubTeams = githubTeams

	return &cfg, nil
}
