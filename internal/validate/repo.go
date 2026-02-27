// Package validate provides shared input validation helpers.
package validate

import "strings"

// IsValidRepoName validates that name is in owner/repo format where each part
// contains only alphanumeric characters, hyphens, dots, or underscores.
func IsValidRepoName(name string) bool {
	parts := strings.SplitN(name, "/", 3)
	if len(parts) != 2 {
		return false
	}

	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, ch := range part {
			if !IsValidRepoChar(ch) {
				return false
			}
		}
	}

	return true
}

// IsValidRepoChar returns true if the rune is allowed in a repository owner or name.
func IsValidRepoChar(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') ||
		ch == '-' || ch == '.' || ch == '_'
}
