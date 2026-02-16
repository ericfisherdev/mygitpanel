package application

import (
	"sync"

	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// GitHubClientProvider enables runtime hot-swap of the GitHub client.
// It holds a mutex-protected reference to the current driven.GitHubClient,
// allowing credential updates to take effect without restarting the application.
type GitHubClientProvider struct {
	mu     sync.RWMutex
	client driven.GitHubClient
}

// NewGitHubClientProvider creates a new provider with the given initial client.
// client may be nil if no credentials are available at startup.
func NewGitHubClientProvider(client driven.GitHubClient) *GitHubClientProvider {
	return &GitHubClientProvider{
		client: client,
	}
}

// Get returns the current GitHub client. Callers should check for nil
// if the provider was created without initial credentials.
func (p *GitHubClientProvider) Get() driven.GitHubClient {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.client
}

// Replace swaps the current client with a new one. This is used when
// credentials are updated via the GUI. The next caller of Get() will
// receive the new client.
func (p *GitHubClientProvider) Replace(client driven.GitHubClient) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.client = client
}

// HasClient returns true if a non-nil client is currently held.
func (p *GitHubClientProvider) HasClient() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.client != nil
}
