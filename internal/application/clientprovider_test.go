package application_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ericfisherdev/mygitpanel/internal/application"
)

func TestGitHubClientProvider_GetReturnsInitialClient(t *testing.T) {
	client := &mockGitHubClient{}
	provider := application.NewGitHubClientProvider(client, "testuser")

	got := provider.Get()
	assert.Same(t, client, got)
	assert.Equal(t, "testuser", provider.Username())
}

func TestGitHubClientProvider_ReplaceSwapsClient(t *testing.T) {
	original := &mockGitHubClient{}
	replacement := &mockGitHubClient{}

	provider := application.NewGitHubClientProvider(original, "user1")
	assert.Same(t, original, provider.Get())

	provider.Replace(replacement, "user2")
	assert.Same(t, replacement, provider.Get())
	assert.Equal(t, "user2", provider.Username())
}

func TestGitHubClientProvider_HasClientReturnsFalseForNil(t *testing.T) {
	provider := application.NewGitHubClientProvider(nil, "")

	require.False(t, provider.HasClient())

	client := &mockGitHubClient{}
	provider.Replace(client, "testuser")

	require.True(t, provider.HasClient())
}

func TestGitHubClientProvider_ConcurrentGetReplaceSafety(t *testing.T) {
	client1 := &mockGitHubClient{}
	client2 := &mockGitHubClient{}
	provider := application.NewGitHubClientProvider(client1, "user1")

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Half the goroutines read, half write.
	for range goroutines {
		go func() {
			defer wg.Done()
			got := provider.Get()
			// Should be either client1 or client2, never nil.
			assert.NotNil(t, got)
		}()
		go func() {
			defer wg.Done()
			provider.Replace(client2, "user2")
		}()
	}

	wg.Wait()

	// After all goroutines finish, client should be client2.
	assert.Same(t, client2, provider.Get())
	assert.Equal(t, "user2", provider.Username())
}
