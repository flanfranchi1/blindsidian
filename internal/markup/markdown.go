package markup

import (
	"bytes"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	htmlRenderer "github.com/yuin/goldmark/renderer/html"
)

var wikiLinkRegex = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

// ── Table-of-Contents and heading ID support ──────────────────────────────

// ToCEntry represents a single heading extracted from Markdown content.
type ToCEntry struct {
	Level int    // ATX heading level: 2 for ##, 3 for ###
	Text  string // plain-text heading (no Markdown formatting)
	ID    string // URL-safe slug used as the HTML id attribute
}

// Package-level compiled regexes for heading processing.
var (
	// Matches ATX headings ## and ### at the start of a line.
	atxHeadingRe = regexp.MustCompile(`(?m)^(#{2,3})\s+(.+)$`)

	// Matches rendered <h2>–<h6> elements (single-line, no newlines inside).
	// Capture groups: (1) tag name, (2) inner HTML, (3) closing tag name.
	renderedHeadingRe = regexp.MustCompile(`<(h[2-6])>(.*?)</(h[2-6])>`)

	// Used by SlugifyHeading to clean the text.
	slugHTMLTagRe  = regexp.MustCompile(`<[^>]+>`)
	slugSpaceRe    = regexp.MustCompile(`\s+`)
	slugNonAlnumRe = regexp.MustCompile(`[^a-z0-9-]`)
	slugCollapseRe = regexp.MustCompile(`-{2,}`)
)

// SlugifyHeading converts a heading text (which may contain inline HTML) to a
// URL-safe, lowercase, hyphen-separated anchor ID.
//
// The algorithm:
//  1. Strip HTML tags (e.g. <code>, <em>).
//  2. Decode HTML entities (&amp; → &).
//  3. Lowercase and replace whitespace runs with "-".
//  4. Remove any character that is not [a-z0-9-].
//  5. Collapse consecutive hyphens; trim leading/trailing hyphens.
func SlugifyHeading(text string) string {
	text = slugHTMLTagRe.ReplaceAllString(text, "")
	text = html.UnescapeString(text)
	text = strings.ToLower(strings.TrimSpace(text))
	text = slugSpaceRe.ReplaceAllString(text, "-")
	text = slugNonAlnumRe.ReplaceAllString(text, "")
	text = slugCollapseRe.ReplaceAllString(text, "-")
	return strings.Trim(text, "-")
}

// ExtractToCHeadings parses raw Markdown content and returns a slice of
// ToCEntry values for every ## and ### heading, in document order.
// Duplicate heading texts receive a numeric suffix ("-1", "-2", …) so that
// every ID in the resulting slice is unique — matching the behaviour of
// addHeadingIDs which applies the same deduplication to the rendered HTML.
func ExtractToCHeadings(content string) []ToCEntry {
	matches := atxHeadingRe.FindAllStringSubmatch(content, -1)
	entries := make([]ToCEntry, 0, len(matches))
	seen := map[string]int{}
	for _, m := range matches {
		level := len(m[1])
		text := strings.TrimSpace(m[2])
		id := SlugifyHeading(text)
		if id == "" {
			continue
		}
		count := seen[id]
		seen[id]++
		if count > 0 {
			id = fmt.Sprintf("%s-%d", id, count)
		}
		entries = append(entries, ToCEntry{Level: level, Text: text, ID: id})
	}
	return entries
}

// addHeadingIDs post-processes sanitised HTML to inject id attributes on every
// heading element (h2–h6).  The ID is the slug of the heading's inner text,
// with a numeric suffix appended for duplicates (matching ExtractToCHeadings).
//
// This runs after bluemonday sanitisation so the injected id values never pass
// through the sanitiser.  Because SlugifyHeading only produces [a-z0-9-]
// characters, there is no XSS risk.
func addHeadingIDs(sanitizedHTML string) string {
	seen := map[string]int{}
	return renderedHeadingRe.ReplaceAllStringFunc(sanitizedHTML, func(match string) string {
		parts := renderedHeadingRe.FindStringSubmatch(match)
		// Guard: ignore mismatched open/close tags (shouldn't happen from goldmark).
		if len(parts) < 4 || parts[1] != parts[3] {
			return match
		}
		tag, inner := parts[1], parts[2]
		id := SlugifyHeading(inner)
		if id == "" {
			return match
		}
		count := seen[id]
		seen[id]++
		if count > 0 {
			id = fmt.Sprintf("%s-%d", id, count)
		}
		return fmt.Sprintf(`<%s id="%s">%s</%s>`, tag, id, inner, tag)
	})
}

func ParseWikiLinks(content string) []string {
	matches := wikiLinkRegex.FindAllStringSubmatch(content, -1)
	titles := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			title := strings.TrimSpace(match[1])
			if title != "" {
				titles = append(titles, title)
			}
		}
	}
	return titles
}

var (
	tagRegex         = regexp.MustCompile(`#([a-zA-Z][a-zA-Z0-9_]*)`)
	atxHeadingLineRe = regexp.MustCompile(`(?m)^#{1,6}[ \t].*$`)
)

func ParseTags(content string) []string {
	stripped := atxHeadingLineRe.ReplaceAllString(content, "")
	matches := tagRegex.FindAllStringSubmatch(stripped, -1)
	tagSet := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			tag := strings.ToLower(strings.TrimSpace(match[1]))
			if tag != "" {
				tagSet[tag] = true
			}
		}
	}
	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	return tags
}

func RenderMarkdownToHTML(raw string) (string, error) {
	return RenderMarkdownWithWikiLinks(raw, nil)
}

func RenderMarkdownWithWikiLinks(raw string, noteResolver func(title string) (id string, exists bool, err error)) (string, error) {
	processed := wikiLinkRegex.ReplaceAllStringFunc(raw, func(value string) string {
		sub := wikiLinkRegex.FindStringSubmatch(value)
		if len(sub) < 2 {
			return value
		}
		title := strings.TrimSpace(sub[1])
		if title == "" {
			return value
		}
		classes := "wiki-link"
		var href string
		if noteResolver != nil {
			id, exists, err := noteResolver(title)
			if err != nil {
				return value
			}
			if exists {
				href = "/notes/" + id
			} else {
				href = "/notes?create=" + url.QueryEscape(title)
				classes += " is-ghost"
			}
		} else {
			href = "/notes/view?title=" + url.QueryEscape(title)
		}
		return fmt.Sprintf(`<a class="%s" href="%s">%s</a>`, classes, href, html.EscapeString(title))
	})

	var buf bytes.Buffer
	md := goldmark.New(
		goldmark.WithRendererOptions(htmlRenderer.WithUnsafe()),
	)
	if err := md.Convert([]byte(processed), &buf); err != nil {
		return "", err
	}
	sanitizer := bluemonday.UGCPolicy()
	sanitized := sanitizer.SanitizeBytes(buf.Bytes())
	result := strings.TrimSpace(html.UnescapeString(string(sanitized)))
	return addHeadingIDs(result), nil
}
