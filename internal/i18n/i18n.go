// Package i18n provides locale detection, translation loading, and
// request-context helpers for the Notty application.
package i18n

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// localeFiles holds the JSON translation files compiled into the binary.
// This ensures the application works correctly even when the binary is deployed
// without the source tree (e.g. inside a minimal Docker image).
//
//go:embed locales/*.json
var localeFiles embed.FS

// contextKey is an unexported type used as a key in request contexts to avoid
// collisions with keys from other packages.
type contextKey struct{}

// localeContextKey is the single instance used to store/retrieve the locale.
var localeContextKey = contextKey{}

// Supported maps a URL/cookie language identifier to its canonical BCP 47 tag
// (used in <html lang="…">) and the JSON file stem.
//
//	key (query param / cookie value) → {bcp47 tag, json file stem}
var supported = map[string]localeEntry{
	"en":    {Tag: "en", FileStem: "en"},
	"es":    {Tag: "es", FileStem: "es"},
	"pt":    {Tag: "pt-BR", FileStem: "pt-br"},
	"pt-br": {Tag: "pt-BR", FileStem: "pt-br"},
}

// DefaultLocale is used when no preference can be determined.
const DefaultLocale = "en"

// LangCookieName is the cookie Notty uses to persist the user's language choice.
const LangCookieName = "notty_lang"

type localeEntry struct {
	Tag      string // BCP 47 tag written to <html lang>
	FileStem string // stem of the JSON file in the locales directory
}

// ─────────────────────────────────────────────
// Bundle
// ─────────────────────────────────────────────

// Bundle holds the pre-loaded translations for every supported language.
// It is created once at startup and shared across requests (read-only).
type Bundle struct {
	// translations maps a BCP 47 tag to its key→value message map.
	translations map[string]map[string]string
}

// Load reads every *.json file in dir, parses it as a flat key→value map,
// and returns a Bundle ready for use.  It expects filenames like en.json,
// es.json, pt-br.json whose stems match the FileStem values in [supported].
func Load(dir string) (*Bundle, error) {
	b := &Bundle{
		translations: make(map[string]map[string]string),
	}

	for _, entry := range supported {
		path := filepath.Join(dir, entry.FileStem+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("i18n: read %s: %w", path, err)
		}
		msgs := make(map[string]string)
		if err := json.Unmarshal(data, &msgs); err != nil {
			return nil, fmt.Errorf("i18n: parse %s: %w", path, err)
		}
		b.translations[entry.Tag] = msgs
	}
	return b, nil
}

// LoadEmbedded loads translations from the locale JSON files that were compiled
// into the binary via go:embed.  This is the preferred loader for production
// because it never fails due to missing files on the deployment host.
func LoadEmbedded() (*Bundle, error) {
	return loadFromFS(localeFiles, "locales")
}

// loadFromFS is the shared implementation used by both Load and LoadEmbedded.
func loadFromFS(fsys fs.FS, dir string) (*Bundle, error) {
	b := &Bundle{
		translations: make(map[string]map[string]string),
	}
	seen := make(map[string]bool)
	for _, entry := range supported {
		if seen[entry.Tag] {
			continue
		}
		seen[entry.Tag] = true
		path := dir + "/" + entry.FileStem + ".json"
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, fmt.Errorf("i18n: read %s: %w", path, err)
		}
		msgs := make(map[string]string)
		if err := json.Unmarshal(data, &msgs); err != nil {
			return nil, fmt.Errorf("i18n: parse %s: %w", path, err)
		}
		b.translations[entry.Tag] = msgs
	}
	return b, nil
}

// Translations returns the message map for the given BCP 47 locale tag.
// Falls back to English if the tag is not found.
func (b *Bundle) Translations(locale string) map[string]string {
	if msgs, ok := b.translations[locale]; ok {
		return msgs
	}
	return b.translations[supported[DefaultLocale].Tag]
}

// ─────────────────────────────────────────────
// Language detection helpers
// ─────────────────────────────────────────────

// Detect resolves the best locale for a request using the priority order:
//  1. ?lang= URL query parameter
//  2. notty_lang cookie
//  3. Accept-Language header
//  4. DefaultLocale ("en")
//
// It returns the canonical BCP 47 tag (e.g. "pt-BR") and, when the language
// was set via the URL parameter, the raw identifier to write back into the
// cookie (e.g. "pt-br").  cookieValue is empty when the source is not the URL.
func Detect(r *http.Request) (tag string, cookieValue string) {
	// Priority 1: URL query param
	if lang := r.URL.Query().Get("lang"); lang != "" {
		if entry, ok := resolve(lang); ok {
			return entry.Tag, strings.ToLower(lang)
		}
	}

	// Priority 2: persistent cookie
	if cookie, err := r.Cookie(LangCookieName); err == nil {
		if entry, ok := resolve(cookie.Value); ok {
			return entry.Tag, ""
		}
	}

	// Priority 3: Accept-Language header
	if tag := parseAcceptLanguage(r.Header.Get("Accept-Language")); tag != "" {
		return tag, ""
	}

	// Priority 4: default
	return supported[DefaultLocale].Tag, ""
}

// resolve maps a raw identifier (case-insensitive) to a localeEntry.
func resolve(raw string) (localeEntry, bool) {
	entry, ok := supported[strings.ToLower(strings.TrimSpace(raw))]
	return entry, ok
}

// parseAcceptLanguage returns the BCP 47 tag for the first language in the
// Accept-Language header that we support, or "" if none match.
//
// It handles the common forms: "pt-BR,pt;q=0.9,en;q=0.8"
func parseAcceptLanguage(header string) string {
	if header == "" {
		return ""
	}
	// Split by comma, try each token in order (quality values are ignored;
	// the browser already orders them descending by preference).
	for _, token := range strings.Split(header, ",") {
		// Drop optional quality value: "pt-BR;q=0.9" → "pt-BR"
		lang := strings.TrimSpace(strings.SplitN(token, ";", 2)[0])
		if entry, ok := resolve(lang); ok {
			return entry.Tag
		}
		// Also try just the primary subtag: "pt-BR" → try "pt"
		if idx := strings.IndexByte(lang, '-'); idx != -1 {
			if entry, ok := resolve(lang[:idx]); ok {
				return entry.Tag
			}
		}
	}
	return ""
}

// ─────────────────────────────────────────────
// Context helpers
// ─────────────────────────────────────────────

// WithLocale returns a copy of ctx with the locale stored inside.
func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeContextKey, locale)
}

// LocaleFromContext retrieves the locale stored by the middleware.
// Returns DefaultLocale if none is present.
func LocaleFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(localeContextKey).(string); ok && v != "" {
		return v
	}
	return supported[DefaultLocale].Tag
}
