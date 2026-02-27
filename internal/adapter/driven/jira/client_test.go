package jira

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test fixture constants shared across client tests.
const (
	testEmail             = "user@example.com"
	testIssueKey          = "PROJ-123"
	testBadToken          = "bad-token"
	testHeaderContentType = "Content-Type"
)

// minimalIssueResponse returns a valid Jira v3 issue JSON with ADF description.
func minimalIssueResponse() string {
	return `{
		"key": "PROJ-123",
		"fields": {
			"summary": "Fix login bug",
			"description": {
				"version": 1,
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [
							{"type": "text", "text": "The login page crashes on submit."}
						]
					}
				]
			},
			"status": {"name": "In Progress"},
			"priority": {"name": "High"},
			"assignee": {"displayName": "Alice Smith"},
			"comment": {
				"comments": [
					{
						"author": {"displayName": "Bob Jones"},
						"body": {
							"version": 1,
							"type": "doc",
							"content": [
								{
									"type": "paragraph",
									"content": [
										{"type": "text", "text": "I can reproduce this."}
									]
								}
							]
						},
						"created": "2026-01-15T10:30:00.000+00:00"
					}
				]
			}
		}
	}`
}

func TestGetIssue_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/api/3/issue/PROJ-123", r.URL.Path)
		assert.Contains(t, r.URL.RawQuery, "fields=summary,description,status,priority,assignee,comment")
		assert.Contains(t, r.Header.Get("Authorization"), "Basic ")
		assert.Equal(t, contentTypeJSON, r.Header.Get("Accept"))

		w.Header().Set(testHeaderContentType, contentTypeJSON)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(minimalIssueResponse()))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL, testEmail, "api-token")
	issue, err := client.GetIssue(context.Background(), testIssueKey)

	require.NoError(t, err)
	assert.Equal(t, testIssueKey, issue.Key)
	assert.Equal(t, "Fix login bug", issue.Summary)
	assert.Equal(t, "The login page crashes on submit.", issue.Description)
	assert.Equal(t, "In Progress", issue.Status)
	assert.Equal(t, "High", issue.Priority)
	assert.Equal(t, "Alice Smith", issue.Assignee)
	require.Len(t, issue.Comments, 1)
	assert.Equal(t, "Bob Jones", issue.Comments[0].Author)
	assert.Equal(t, "I can reproduce this.", issue.Comments[0].Body)
	assert.False(t, issue.Comments[0].CreatedAt.IsZero())
}

func TestGetIssue_NilAssignee(t *testing.T) {
	response := `{
		"key": "PROJ-456",
		"fields": {
			"summary": "Unassigned task",
			"description": null,
			"status": {"name": "To Do"},
			"priority": {"name": "Low"},
			"assignee": null,
			"comment": {"comments": []}
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(testHeaderContentType, contentTypeJSON)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL, testEmail, "token")
	issue, err := client.GetIssue(context.Background(), "PROJ-456")

	require.NoError(t, err)
	assert.Equal(t, "", issue.Assignee)
	assert.Equal(t, "", issue.Description)
	assert.Empty(t, issue.Comments)
}

func TestGetIssue_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL, testEmail, testBadToken)
	_, err := client.GetIssue(context.Background(), testIssueKey)

	require.Error(t, err)
	assert.True(t, errors.Is(err, driven.ErrJiraUnauthorized))
}

func TestGetIssue_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL, testEmail, "token")
	_, err := client.GetIssue(context.Background(), "NOPE-999")

	require.Error(t, err)
	assert.True(t, errors.Is(err, driven.ErrJiraNotFound))
}

func TestGetIssue_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL, testEmail, "token")
	_, err := client.GetIssue(context.Background(), testIssueKey)

	require.Error(t, err)
	assert.True(t, errors.Is(err, driven.ErrJiraUnavailable))
}

func TestAddComment_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/api/3/issue/PROJ-123/comment", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, contentTypeJSON, r.Header.Get(testHeaderContentType))

		// Verify the body is valid ADF.
		var req commentRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "doc", req.Body.Type)
		assert.Equal(t, 1, req.Body.Version)
		require.Len(t, req.Body.Content, 1)
		assert.Equal(t, "paragraph", req.Body.Content[0].Type)

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL, testEmail, "token")
	err := client.AddComment(context.Background(), testIssueKey, "This is a test comment")

	require.NoError(t, err)
}

func TestAddComment_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL, testEmail, testBadToken)
	err := client.AddComment(context.Background(), testIssueKey, "comment")

	require.Error(t, err)
	assert.True(t, errors.Is(err, driven.ErrJiraUnauthorized))
}

func TestAddComment_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL, testEmail, "token")
	err := client.AddComment(context.Background(), "NOPE-999", "comment")

	require.Error(t, err)
	assert.True(t, errors.Is(err, driven.ErrJiraNotFound))
}

func TestAddComment_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL, testEmail, "token")
	err := client.AddComment(context.Background(), testIssueKey, "comment")

	require.Error(t, err)
	assert.True(t, errors.Is(err, driven.ErrJiraUnavailable))
}

func TestPing_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/api/3/myself", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.Header.Get("Authorization"), "Basic ")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"accountId": "123"}`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL, testEmail, "token")
	err := client.Ping(context.Background())

	require.NoError(t, err)
}

func TestPing_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL, testEmail, testBadToken)
	err := client.Ping(context.Background())

	require.Error(t, err)
	assert.True(t, errors.Is(err, driven.ErrJiraUnauthorized))
}

func TestPing_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL, testEmail, "token")
	err := client.Ping(context.Background())

	require.Error(t, err)
	assert.True(t, errors.Is(err, driven.ErrJiraUnavailable))
}

func TestExtractADFText_NestedBulletList(t *testing.T) {
	adf := `{
		"version": 1,
		"type": "doc",
		"content": [
			{
				"type": "paragraph",
				"content": [
					{"type": "text", "text": "Requirements:"}
				]
			},
			{
				"type": "bulletList",
				"content": [
					{
						"type": "listItem",
						"content": [
							{
								"type": "paragraph",
								"content": [
									{"type": "text", "text": "First item"}
								]
							}
						]
					},
					{
						"type": "listItem",
						"content": [
							{
								"type": "paragraph",
								"content": [
									{"type": "text", "text": "Second item"}
								]
							}
						]
					}
				]
			}
		]
	}`

	text := extractADFDocText(json.RawMessage(adf))
	assert.Contains(t, text, "Requirements:")
	assert.Contains(t, text, "First item")
	assert.Contains(t, text, "Second item")
}

func TestExtractADFText_HardBreak(t *testing.T) {
	adf := `{
		"version": 1,
		"type": "doc",
		"content": [
			{
				"type": "paragraph",
				"content": [
					{"type": "text", "text": "Line one"},
					{"type": "hardBreak"},
					{"type": "text", "text": "Line two"}
				]
			}
		]
	}`

	text := extractADFDocText(json.RawMessage(adf))
	assert.Equal(t, "Line one\nLine two", text)
}

func TestExtractADFText_CodeBlock(t *testing.T) {
	adf := `{
		"version": 1,
		"type": "doc",
		"content": [
			{
				"type": "codeBlock",
				"content": [
					{"type": "text", "text": "func main() {}"}
				]
			}
		]
	}`

	text := extractADFDocText(json.RawMessage(adf))
	assert.Equal(t, "func main() {}", text)
}

func TestExtractADFText_EmptyAndNull(t *testing.T) {
	assert.Equal(t, "", extractADFDocText(nil))
	assert.Equal(t, "", extractADFDocText(json.RawMessage(`null`)))
	assert.Equal(t, "", extractADFDocText(json.RawMessage(``)))
}

func TestPlainTextToADF(t *testing.T) {
	doc := plainTextToADF("Hello world")

	assert.Equal(t, 1, doc.Version)
	assert.Equal(t, "doc", doc.Type)
	require.Len(t, doc.Content, 1)
	assert.Equal(t, "paragraph", doc.Content[0].Type)
	require.Len(t, doc.Content[0].Content, 1)

	var textNode adfNode
	err := json.Unmarshal(doc.Content[0].Content[0], &textNode)
	require.NoError(t, err)
	assert.Equal(t, "text", textNode.Type)
	assert.Equal(t, "Hello world", textNode.Text)
}

func TestBasicAuthHeader(t *testing.T) {
	header := basicAuthHeader(testEmail, "my-token")
	assert.Equal(t, "Basic dXNlckBleGFtcGxlLmNvbTpteS10b2tlbg==", header)
}
