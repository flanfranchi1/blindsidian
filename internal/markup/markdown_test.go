package markup

import (
	"reflect"
	"sort"
	"strings"
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

func TestSlugifyHeading(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Hello World", "hello-world"},
		{"  Trimmed  ", "trimmed"},
		{"Multiple   Spaces", "multiple-spaces"},
		{"Café & résumé", "caf-rsum"},       // non-ASCII stripped
		{"C++ pointers", "c-pointers"},      // punctuation stripped
		{"<code>snippet</code>", "snippet"}, // HTML tags stripped
		{"&amp; ampersand", "ampersand"},    // entities decoded, non-alnum stripped
		{"", ""},
		{"---", ""},
		{"Already-slug", "already-slug"},
		{"Duplicate--hyphens", "duplicate-hyphens"},
	}
	for _, tt := range tests {
		if got := SlugifyHeading(tt.in); got != tt.want {
			t.Errorf("SlugifyHeading(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExtractToCHeadings(t *testing.T) {
	content := `# Title (h1 — not extracted)

## Introduction

Some text.

### Sub-section A

More text.

### Sub-section A

Duplicate heading should have a suffix.

## Conclusion
`
	entries := ExtractToCHeadings(content)
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d: %+v", len(entries), entries)
	}
	want := []ToCEntry{
		{Level: 2, Text: "Introduction", ID: "introduction"},
		{Level: 3, Text: "Sub-section A", ID: "sub-section-a"},
		{Level: 3, Text: "Sub-section A", ID: "sub-section-a-1"},
		{Level: 2, Text: "Conclusion", ID: "conclusion"},
	}
	for i, w := range want {
		if entries[i] != w {
			t.Errorf("entry[%d]: got %+v, want %+v", i, entries[i], w)
		}
	}
}

func TestAddHeadingIDsRoundtrip(t *testing.T) {
	// Verify that IDs injected into rendered HTML match what ExtractToCHeadings
	// produces from the same Markdown source — the critical invariant for the
	// sidebar ToC anchor links to resolve correctly.
	content := "## My Heading\n\n### Sub Heading\n"

	toc := ExtractToCHeadings(content)
	if len(toc) != 2 {
		t.Fatalf("expected 2 ToC entries, got %d", len(toc))
	}

	html, err := RenderMarkdownToHTML(content)
	if err != nil {
		t.Fatalf("RenderMarkdownToHTML: %v", err)
	}

	for _, entry := range toc {
		needle := `id="` + entry.ID + `"`
		if !strings.Contains(html, needle) {
			t.Errorf("rendered HTML missing %s\nHTML: %s", needle, html)
		}
	}
}
