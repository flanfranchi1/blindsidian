package markup

import (
	"reflect"
	"sort"
	"testing"
)

func TestParseTags(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "no tags",
			content:  "This is a note without tags.",
			expected: []string{},
		},
		{
			name:     "single tag",
			content:  "This is a #note with one tag.",
			expected: []string{"note"},
		},
		{
			name:     "multiple tags",
			content:  "This is a #note with #multiple #tags.",
			expected: []string{"multiple", "note", "tags"},
		},
		{
			name:     "duplicate tags",
			content:  "This is a #note with #note again.",
			expected: []string{"note"},
		},
		{
			name:     "case insensitive",
			content:  "This is a #Note with #NOTE.",
			expected: []string{"note"},
		},
		{
			name:     "ignore non-tag hashes",
			content:  "This is #not a tag, but #tag is.",
			expected: []string{"not", "tag"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseTags(tt.content)
			sort.Strings(result)
			sort.Strings(tt.expected)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ParseTags() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseWikiLinks(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "no links",
			content:  "This is a note without links.",
			expected: []string{},
		},
		{
			name:     "single link",
			content:  "This links to [[Another Note]].",
			expected: []string{"Another Note"},
		},
		{
			name:     "multiple links",
			content:  "Links to [[Note1]] and [[Note2]].",
			expected: []string{"Note1", "Note2"},
		},
		{
			name:     "empty link",
			content:  "This is [[ ]] empty.",
			expected: []string{},
		},
		{
			name:     "link with spaces",
			content:  "This is [[A Note With Spaces]].",
			expected: []string{"A Note With Spaces"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseWikiLinks(tt.content)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ParseWikiLinks() = %v, want %v", result, tt.expected)
			}
		})
	}
}
