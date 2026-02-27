package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractJiraKey(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		title  string
		want   string
	}{
		{
			name:   "key in branch",
			branch: "feature/PROJ-123-add-widget",
			title:  "Add widget feature",
			want:   "PROJ-123",
		},
		{
			name:   "key in title only",
			branch: "feature/add-widget",
			title:  "PROJ-456: Add widget feature",
			want:   "PROJ-456",
		},
		{
			name:   "branch takes priority over title",
			branch: "fix/ABC-10-bug",
			title:  "XYZ-99: fix bug",
			want:   "ABC-10",
		},
		{
			name:   "no key anywhere",
			branch: "feature/add-widget",
			title:  "Add widget feature",
			want:   "",
		},
		{
			name:   "single letter project key ignored",
			branch: "A-1",
			title:  "B-2",
			want:   "",
		},
		{
			name:   "two letter project key accepted",
			branch: "AB-1",
			title:  "",
			want:   "AB-1",
		},
		{
			name:   "multiple keys returns first",
			branch: "feature/PROJ-100-and-PROJ-200",
			title:  "",
			want:   "PROJ-100",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractJiraKey(tc.branch, tc.title)
			assert.Equal(t, tc.want, got)
		})
	}
}
