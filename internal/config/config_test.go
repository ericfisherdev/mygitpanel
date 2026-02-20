package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allConfigKeys lists every MYGITPANEL_ env var that Load() reads.
var allConfigKeys = []string{
	"MYGITPANEL_GITHUB_TOKEN",
	"MYGITPANEL_GITHUB_USERNAME",
	"MYGITPANEL_GITHUB_TEAMS",
	"MYGITPANEL_POLL_INTERVAL",
	"MYGITPANEL_LISTEN_ADDR",
	"MYGITPANEL_DB_PATH",
	"MYGITPANEL_SECRET_KEY",
}

// isolateConfigEnv saves and unsets all MYGITPANEL_ env vars so tests don't
// inherit values from the host environment (e.g. a running dev server).
// t.Cleanup restores original values after the test.
func isolateConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range allConfigKeys {
		if orig, ok := os.LookupEnv(key); ok {
			t.Cleanup(func() { os.Setenv(key, orig) })
		} else {
			t.Cleanup(func() { os.Unsetenv(key) })
		}
		os.Unsetenv(key)
	}
}

func TestLoad_Success(t *testing.T) {
	isolateConfigEnv(t)
	t.Setenv("MYGITPANEL_GITHUB_TOKEN", "ghp_test123")
	t.Setenv("MYGITPANEL_GITHUB_USERNAME", "testuser")
	t.Setenv("MYGITPANEL_POLL_INTERVAL", "10m")
	t.Setenv("MYGITPANEL_LISTEN_ADDR", "0.0.0.0:9090")
	t.Setenv("MYGITPANEL_DB_PATH", "/tmp/test.db")

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, "ghp_test123", cfg.GitHubToken)
	assert.Equal(t, "testuser", cfg.GitHubUsername)
	assert.Equal(t, 10*time.Minute, cfg.PollInterval)
	assert.Equal(t, "0.0.0.0:9090", cfg.ListenAddr)
	assert.Equal(t, "/tmp/test.db", cfg.DBPath)
}

func TestLoad_Defaults(t *testing.T) {
	isolateConfigEnv(t)
	t.Setenv("MYGITPANEL_GITHUB_TOKEN", "ghp_test123")
	t.Setenv("MYGITPANEL_GITHUB_USERNAME", "testuser")

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, cfg.PollInterval)
	assert.Equal(t, "127.0.0.1:8080", cfg.ListenAddr)
	assert.Equal(t, "mygitpanel.db", cfg.DBPath)
}

// TestLoad_MissingToken verifies that a missing GITHUB_TOKEN does not cause
// an error — it only logs a warning and sets an empty token.
func TestLoad_MissingToken(t *testing.T) {
	isolateConfigEnv(t)
	t.Setenv("MYGITPANEL_GITHUB_USERNAME", "testuser")

	cfg, err := Load()

	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "", cfg.GitHubToken)
}

func TestLoad_MissingUsername(t *testing.T) {
	isolateConfigEnv(t)
	t.Setenv("MYGITPANEL_GITHUB_TOKEN", "ghp_test123")

	cfg, err := Load()

	assert.Nil(t, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MYGITPANEL_GITHUB_USERNAME")
}

// TestLoad_EmptyToken verifies that an empty GITHUB_TOKEN does not cause
// an error — it only logs a warning.
func TestLoad_EmptyToken(t *testing.T) {
	isolateConfigEnv(t)
	t.Setenv("MYGITPANEL_GITHUB_TOKEN", "")
	t.Setenv("MYGITPANEL_GITHUB_USERNAME", "testuser")

	cfg, err := Load()

	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "", cfg.GitHubToken)
}

func TestLoad_InvalidPollInterval(t *testing.T) {
	isolateConfigEnv(t)
	t.Setenv("MYGITPANEL_GITHUB_TOKEN", "ghp_test123")
	t.Setenv("MYGITPANEL_GITHUB_USERNAME", "testuser")
	t.Setenv("MYGITPANEL_POLL_INTERVAL", "not-a-duration")

	cfg, err := Load()

	assert.Nil(t, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MYGITPANEL_POLL_INTERVAL")
}

func TestLoad_GitHubTeams(t *testing.T) {
	isolateConfigEnv(t)
	t.Setenv("MYGITPANEL_GITHUB_TOKEN", "ghp_test123")
	t.Setenv("MYGITPANEL_GITHUB_USERNAME", "testuser")
	t.Setenv("MYGITPANEL_GITHUB_TEAMS", "team-a, team-b")

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, []string{"team-a", "team-b"}, cfg.GitHubTeams)
}

func TestLoad_GitHubTeams_Empty(t *testing.T) {
	isolateConfigEnv(t)
	t.Setenv("MYGITPANEL_GITHUB_TOKEN", "ghp_test123")
	t.Setenv("MYGITPANEL_GITHUB_USERNAME", "testuser")

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, []string{}, cfg.GitHubTeams)
}

func TestLoad_SecretKey_Absent(t *testing.T) {
	isolateConfigEnv(t)
	t.Setenv("MYGITPANEL_GITHUB_USERNAME", "testuser")

	cfg, err := Load()

	require.NoError(t, err)
	assert.Nil(t, cfg.SecretKey)
}

func TestLoad_SecretKey_Valid(t *testing.T) {
	isolateConfigEnv(t)
	t.Setenv("MYGITPANEL_GITHUB_USERNAME", "testuser")
	// 64 hex chars = 32 bytes
	t.Setenv("MYGITPANEL_SECRET_KEY", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")

	cfg, err := Load()

	require.NoError(t, err)
	assert.Len(t, cfg.SecretKey, 32)
}

func TestLoad_SecretKey_TooShort(t *testing.T) {
	isolateConfigEnv(t)
	t.Setenv("MYGITPANEL_GITHUB_USERNAME", "testuser")
	t.Setenv("MYGITPANEL_SECRET_KEY", "deadbeef")

	cfg, err := Load()

	assert.Nil(t, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MYGITPANEL_SECRET_KEY")
}

func TestLoad_SecretKey_NotHex(t *testing.T) {
	isolateConfigEnv(t)
	t.Setenv("MYGITPANEL_GITHUB_USERNAME", "testuser")
	// 64 chars but not valid hex
	t.Setenv("MYGITPANEL_SECRET_KEY", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz")

	cfg, err := Load()

	assert.Nil(t, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MYGITPANEL_SECRET_KEY")
}
