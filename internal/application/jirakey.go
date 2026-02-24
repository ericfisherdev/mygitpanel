package application

import "regexp"

// jiraKeyPattern matches Jira issue keys in the standard format (e.g., "PROJ-123").
// Requires project key of at least two uppercase letters followed by a hyphen and digits.
var jiraKeyPattern = regexp.MustCompile(`[A-Z]{2,}-\d+`)

// ExtractJiraKey returns the first Jira issue key found by scanning branch name first,
// then PR title. Returns empty string if no key is detected.
// Branch takes priority: if both branch and title contain keys, the branch key is returned.
func ExtractJiraKey(branch, title string) string {
	if m := jiraKeyPattern.FindString(branch); m != "" {
		return m
	}
	return jiraKeyPattern.FindString(title)
}
