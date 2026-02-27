// Package jira implements the JiraClient port using net/http and Jira Cloud REST API v3.
package jira

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// Compile-time interface satisfaction check.
var _ driven.JiraClient = (*JiraHTTPClient)(nil)

// JiraHTTPClient implements the driven.JiraClient port using Jira Cloud REST API v3.
type JiraHTTPClient struct {
	baseURL    string
	email      string
	token      string
	httpClient *http.Client
}

// NewJiraClient creates a JiraHTTPClient for the given Jira instance.
// baseURL must be the Atlassian Cloud URL (e.g., "https://mycompany.atlassian.net").
// email and token are the Atlassian account credentials for Basic auth.
func NewJiraClient(baseURL, email, token string) *JiraHTTPClient {
	return &JiraHTTPClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		email:      email,
		token:      token,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// basicAuthHeader returns the Basic auth header value for the given credentials.
func basicAuthHeader(email, token string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(email+":"+token))
}

// --- ADF types (Atlassian Document Format) ---

// adfNode represents a node in the ADF content tree.
// Content is json.RawMessage to support recursive unmarshalling without
// requiring a fixed-depth struct hierarchy.
type adfNode struct {
	Type    string            `json:"type"`
	Text    string            `json:"text,omitempty"`
	Content []json.RawMessage `json:"content,omitempty"`
}

// adfDoc is the top-level ADF document envelope.
type adfDoc struct {
	Version int       `json:"version"`
	Type    string    `json:"type"`
	Content []adfNode `json:"content"`
}

// extractADFText recursively extracts plain text from an ADF JSON blob.
// It handles paragraph, text, hardBreak, bulletList, orderedList, listItem,
// and codeBlock node types. Unknown node types are recursed into if they
// have Content children.
func extractADFText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var node adfNode
	if err := json.Unmarshal(raw, &node); err != nil {
		return ""
	}

	return extractNodeText(&node)
}

// extractNodeText extracts text from a parsed ADF node.
func extractNodeText(node *adfNode) string {
	switch node.Type {
	case "text":
		return node.Text

	case "hardBreak":
		return "\n"

	case "paragraph", "codeBlock":
		parts := collectChildText(node.Content)
		return strings.Join(parts, "")

	case "bulletList", "orderedList":
		var items []string
		for _, child := range node.Content {
			text := extractADFText(child)
			if text != "" {
				items = append(items, text)
			}
		}
		return strings.Join(items, "\n")

	case "listItem":
		var parts []string
		for _, child := range node.Content {
			text := extractADFText(child)
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")

	default:
		// Unknown node type — recurse into children if present.
		if len(node.Content) > 0 {
			parts := collectChildText(node.Content)
			return strings.Join(parts, "")
		}
		return ""
	}
}

// collectChildText extracts text from each child raw message.
func collectChildText(children []json.RawMessage) []string {
	var parts []string
	for _, child := range children {
		text := extractADFText(child)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return parts
}

// extractADFDocText extracts plain text from a top-level ADF document.
// Paragraphs are separated by double newlines.
func extractADFDocText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var doc adfDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}

	var paragraphs []string
	for _, block := range doc.Content {
		data, err := json.Marshal(block)
		if err != nil {
			continue
		}
		text := extractADFText(data)
		if text != "" {
			paragraphs = append(paragraphs, text)
		}
	}

	return strings.Join(paragraphs, "\n\n")
}

// plainTextToADF wraps plain text in a minimal ADF document envelope.
func plainTextToADF(text string) adfDoc {
	return adfDoc{
		Version: 1,
		Type:    "doc",
		Content: []adfNode{{
			Type: "paragraph",
			Content: []json.RawMessage{
				mustMarshal(adfNode{Type: "text", Text: text}),
			},
		}},
	}
}

// mustMarshal marshals v to JSON or panics. Used only for constructing
// known-good ADF structures.
func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("jira: failed to marshal ADF node: %v", err))
	}
	return data
}

// commentRequest is the JSON body for POST /rest/api/3/issue/{key}/comment.
type commentRequest struct {
	Body adfDoc `json:"body"`
}

// --- HTTP methods ---

// GetIssue retrieves a Jira issue by key (e.g. "PROJ-123").
// Returns ErrJiraNotFound if the issue does not exist,
// ErrJiraUnauthorized if credentials are invalid,
// ErrJiraUnavailable if the Jira instance is unreachable or returns an unexpected status.
func (c *JiraHTTPClient) GetIssue(ctx context.Context, key string) (model.JiraIssue, error) {
	endpoint := c.baseURL + "/rest/api/3/issue/" + url.PathEscape(key) + "?fields=summary,description,status,priority,assignee,comment"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return model.JiraIssue{}, fmt.Errorf("jira: building request: %w", err)
	}
	req.Header.Set("Authorization", basicAuthHeader(c.email, c.token))
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return model.JiraIssue{}, fmt.Errorf("jira: request failed: %w", driven.ErrJiraUnavailable)
	}
	defer resp.Body.Close()

	if err := mapStatusCode(resp.StatusCode); err != nil {
		return model.JiraIssue{}, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.JiraIssue{}, fmt.Errorf("jira: reading response body: %w", driven.ErrJiraUnavailable)
	}

	var issue struct {
		Key    string `json:"key"`
		Fields struct {
			Summary     string          `json:"summary"`
			Description json.RawMessage `json:"description"`
			Status      struct {
				Name string `json:"name"`
			} `json:"status"`
			Priority struct {
				Name string `json:"name"`
			} `json:"priority"`
			Assignee *struct {
				DisplayName string `json:"displayName"`
			} `json:"assignee"`
			Comment struct {
				Comments []struct {
					Author struct {
						DisplayName string `json:"displayName"`
					} `json:"author"`
					Body    json.RawMessage `json:"body"`
					Created string          `json:"created"`
				} `json:"comments"`
			} `json:"comment"`
		} `json:"fields"`
	}

	if err := json.Unmarshal(body, &issue); err != nil {
		return model.JiraIssue{}, fmt.Errorf("jira: parsing response: %w", err)
	}

	// Extract plain text from ADF description.
	description := extractADFDocText(issue.Fields.Description)

	// Extract assignee display name (may be nil).
	var assignee string
	if issue.Fields.Assignee != nil {
		assignee = issue.Fields.Assignee.DisplayName
	}

	// Map comments.
	comments := make([]model.JiraComment, 0, len(issue.Fields.Comment.Comments))
	for _, c := range issue.Fields.Comment.Comments {
		createdAt := parseJiraTime(c.Created)
		comments = append(comments, model.JiraComment{
			Author:    c.Author.DisplayName,
			Body:      extractADFDocText(c.Body),
			CreatedAt: createdAt,
		})
	}

	return model.JiraIssue{
		Key:         issue.Key,
		Summary:     issue.Fields.Summary,
		Description: description,
		Status:      issue.Fields.Status.Name,
		Priority:    issue.Fields.Priority.Name,
		Assignee:    assignee,
		Comments:    comments,
	}, nil
}

// AddComment posts a plain-text comment on the specified Jira issue.
// The body is wrapped in ADF format before sending.
// Returns ErrJiraUnauthorized if credentials are invalid,
// ErrJiraNotFound if the issue does not exist,
// ErrJiraUnavailable on invalid body or other errors.
func (c *JiraHTTPClient) AddComment(ctx context.Context, key, body string) error {
	endpoint := c.baseURL + "/rest/api/3/issue/" + url.PathEscape(key) + "/comment"

	payload, err := json.Marshal(commentRequest{Body: plainTextToADF(body)})
	if err != nil {
		return fmt.Errorf("jira: marshaling comment: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("jira: building request: %w", err)
	}
	req.Header.Set("Authorization", basicAuthHeader(c.email, c.token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("jira: request failed: %w", driven.ErrJiraUnavailable)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusCreated:
		return nil
	case http.StatusBadRequest:
		return fmt.Errorf("jira: invalid comment body: %w", driven.ErrJiraUnavailable)
	case http.StatusUnauthorized:
		return driven.ErrJiraUnauthorized
	case http.StatusNotFound:
		return driven.ErrJiraNotFound
	case http.StatusTooManyRequests:
		return fmt.Errorf("jira: rate limited: %w", driven.ErrJiraUnavailable)
	default:
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		return fmt.Errorf("jira: unexpected status %d: %w", resp.StatusCode, driven.ErrJiraUnavailable)
	}
}

// Ping validates connectivity and credentials via GET /rest/api/3/myself.
// Returns nil on success, ErrJiraUnauthorized on 401,
// ErrJiraUnavailable on any other error.
func (c *JiraHTTPClient) Ping(ctx context.Context) error {
	endpoint := c.baseURL + "/rest/api/3/myself"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("jira: building request: %w", err)
	}
	req.Header.Set("Authorization", basicAuthHeader(c.email, c.token))
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("jira: request failed: %w", driven.ErrJiraUnavailable)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return driven.ErrJiraUnauthorized
	default:
		return fmt.Errorf("jira: unexpected status %d: %w", resp.StatusCode, driven.ErrJiraUnavailable)
	}
}

// parseJiraTime parses a Jira timestamp string. Jira Cloud uses RFC3339 but
// sometimes omits the colon in the timezone offset (e.g., "+0000" instead of "+00:00").
// Returns zero time if parsing fails.
func parseJiraTime(s string) time.Time {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	// Fallback: Jira's non-standard format with milliseconds and no colon in tz offset.
	if t, err := time.Parse("2006-01-02T15:04:05.000-0700", s); err == nil {
		return t
	}
	return time.Time{}
}

// mapStatusCode converts common Jira HTTP status codes to domain sentinel errors.
// Returns nil for 2xx responses.
func mapStatusCode(code int) error {
	switch code {
	case http.StatusUnauthorized:
		return driven.ErrJiraUnauthorized
	case http.StatusNotFound:
		return driven.ErrJiraNotFound
	case http.StatusTooManyRequests:
		return fmt.Errorf("jira: rate limited: %w", driven.ErrJiraUnavailable)
	default:
		if code >= 200 && code < 300 {
			return nil
		}
		return fmt.Errorf("jira: unexpected status %d: %w", code, driven.ErrJiraUnavailable)
	}
}
