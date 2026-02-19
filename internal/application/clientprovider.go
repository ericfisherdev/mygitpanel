package application

import (
	"sync"

	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// GitHubClientProvider enables runtime hot-swap of the GitHub client.
// It holds a mutex-protected reference to the current driven.GitHubClient
// and associated username, allowing credential updates to take effect
// without restarting the application.
type GitHubClientProvider struct {
	mu       sync.RWMutex
	client   driven.GitHubClient
	username string
}

// NewGitHubClientProvider creates a new provider with the given initial client
// and username. client may be nil if no credentials are available at startup.
func NewGitHubClientProvider(client driven.GitHubClient, username string) *GitHubClientProvider {
	return &GitHubClientProvider{
		client:   client,
		username: username,
	}
}

// Get returns the current GitHub client. Callers should check for nil
// if the provider was created without initial credentials.
func (p *GitHubClientProvider) Get() driven.GitHubClient {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.client
}

// Username returns the current GitHub username associated with the client.
func (p *GitHubClientProvider) Username() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.username
}

// Replace swaps the current client and username with new ones. This is used
// when credentials are updated via the GUI. The next caller of Get() or
// Username() will receive the new values.
func (p *GitHubClientProvider) Replace(client driven.GitHubClient, username string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.client = client
	p.username = username
}

// HasClient returns true if a non-nil client is currently held.
func (p *GitHubClientProvider) HasClient() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.client != nil
}
