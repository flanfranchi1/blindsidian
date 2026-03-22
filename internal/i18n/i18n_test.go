package i18n

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// newRequest builds a GET request for the given rawURL.
func newRequest(t *testing.T, rawURL string) *http.Request {
	t.Helper()
	return httptest.NewRequest(http.MethodGet, rawURL, nil)
}

// makeBundle creates a temp locale directory with the given files and calls Load.
func makeBundle(t *testing.T, files map[string]string) *Bundle {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write locale file %s: %v", name, err)
		}
	}
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return b
}

// minimalLocaleFiles returns the three JSON files required by Load.
func minimalLocaleFiles() map[string]string {
	return map[string]string{
		"en.json":    `{"greeting": "Hello"}`,
		"es.json":    `{"greeting": "Hola"}`,
		"pt-br.json": `{"greeting": "Olá"}`,
	}
}

// ── Bundle.Load ───────────────────────────────

// TestLoadEmbedded is the primary regression test for the Docker deployment bug
// (locales/pt-br.json: no such file or directory).
// It verifies that LoadEmbedded — which reads from the go:embed FS baked into
// the binary — succeeds without any files on disk and produces translations for
// every supported locale.
func TestLoadEmbedded_AllLocalesPresent(t *testing.T) {
	b, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v; locale files may be missing from internal/i18n/locales/", err)
	}

	// Every BCP 47 tag that the supported map declares must be loaded.
	wantTags := []string{"en", "es", "pt-BR"}
	for _, tag := range wantTags {
		msgs := b.Translations(tag)
		if len(msgs) == 0 {
			t.Errorf("Translations(%q) returned empty map; locale file may be empty or missing", tag)
		}
	}
}

func TestLoadEmbedded_NoDiskDependency(t *testing.T) {
	// Calling LoadEmbedded twice (with no disk state change between calls) must
	// always succeed — this is the core guarantee the embed provides.
	for i := range 3 {
		if _, err := LoadEmbedded(); err != nil {
			t.Fatalf("call %d: LoadEmbedded() error = %v", i+1, err)
		}
	}
}

func TestLoad_OK(t *testing.T) {
	b := makeBundle(t, minimalLocaleFiles())
	if len(b.translations) == 0 {
		t.Error("translations map is empty after Load")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "en.json"), []byte(`{"k":"v"}`), 0o644)
	// es.json and pt-br.json intentionally omitted
	if _, err := Load(dir); err == nil {
		t.Error("expected error for missing locale file, got nil")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	files := minimalLocaleFiles()
	files["en.json"] = `not json`
	dir := t.TempDir()
	for name, content := range files {
		os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	}
	if _, err := Load(dir); err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// ── Bundle.Translations ───────────────────────

func TestTranslations_KnownLocale(t *testing.T) {
	b := makeBundle(t, minimalLocaleFiles())
	cases := []struct{ locale, want string }{
		{"en", "Hello"},
		{"es", "Hola"},
		{"pt-BR", "Olá"},
	}
	for _, tc := range cases {
		t.Run(tc.locale, func(t *testing.T) {
			msgs := b.Translations(tc.locale)
			if got := msgs["greeting"]; got != tc.want {
				t.Errorf("Translations(%q)[greeting] = %q, want %q", tc.locale, got, tc.want)
			}
		})
	}
}

func TestTranslations_UnknownLocaleFallsBackToEnglish(t *testing.T) {
	b := makeBundle(t, minimalLocaleFiles())
	msgs := b.Translations("fr") // French not supported
	if got := msgs["greeting"]; got != "Hello" {
		t.Errorf("fallback: got %q, want %q", got, "Hello")
	}
}

// ── Context helpers ───────────────────────────

func TestWithLocale_AndLocaleFromContext(t *testing.T) {
	ctx := WithLocale(context.Background(), "es")
	if got := LocaleFromContext(ctx); got != "es" {
		t.Errorf("LocaleFromContext = %q, want %q", got, "es")
	}
}

func TestLocaleFromContext_EmptyReturnsDefault(t *testing.T) {
	got := LocaleFromContext(context.Background())
	want := supported[DefaultLocale].Tag
	if got != want {
		t.Errorf("got %q, want default %q", got, want)
	}
}

// ── Detect — priority order ───────────────────

func TestDetect_URLParamWins(t *testing.T) {
	r := newRequest(t, "/?lang=es")
	r.AddCookie(&http.Cookie{Name: LangCookieName, Value: "en"})
	r.Header.Set("Accept-Language", "pt-BR")

	tag, cookieValue := Detect(r)

	if tag != "es" {
		t.Errorf("tag = %q, want %q", tag, "es")
	}
	if cookieValue == "" {
		t.Error("cookieValue should be non-empty when language comes from URL param")
	}
}

func TestDetect_CookieBeatsAcceptLanguage(t *testing.T) {
	r := newRequest(t, "/")
	r.AddCookie(&http.Cookie{Name: LangCookieName, Value: "es"})
	r.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")

	tag, cookieValue := Detect(r)

	if tag != "es" {
		t.Errorf("tag = %q, want %q", tag, "es")
	}
	if cookieValue != "" {
		t.Error("cookieValue should be empty when language comes from cookie")
	}
}

func TestDetect_AcceptLanguageUsedWhenNoCookie(t *testing.T) {
	r := newRequest(t, "/")
	r.Header.Set("Accept-Language", "pt-BR,en;q=0.8")

	tag, cookieValue := Detect(r)

	if tag != "pt-BR" {
		t.Errorf("tag = %q, want %q", tag, "pt-BR")
	}
	if cookieValue != "" {
		t.Error("cookieValue should be empty when language comes from Accept-Language")
	}
}

func TestDetect_DefaultWhenNoSignal(t *testing.T) {
	r := newRequest(t, "/")
	tag, cookieValue := Detect(r)

	if tag != supported[DefaultLocale].Tag {
		t.Errorf("tag = %q, want default %q", tag, supported[DefaultLocale].Tag)
	}
	if cookieValue != "" {
		t.Error("cookieValue should be empty for default fallback")
	}
}

func TestDetect_URLParamSetsCookieValue(t *testing.T) {
	cases := []struct {
		param      string
		wantTag    string
		wantCookie string
	}{
		{"en", "en", "en"},
		{"es", "es", "es"},
		{"pt", "pt-BR", "pt"},
		{"pt-br", "pt-BR", "pt-br"},
	}
	for _, tc := range cases {
		t.Run(tc.param, func(t *testing.T) {
			r := newRequest(t, "/?lang="+tc.param)
			tag, cookieValue := Detect(r)
			if tag != tc.wantTag {
				t.Errorf("tag = %q, want %q", tag, tc.wantTag)
			}
			if cookieValue != tc.wantCookie {
				t.Errorf("cookieValue = %q, want %q", cookieValue, tc.wantCookie)
			}
		})
	}
}

func TestDetect_UnknownLangParamFallsThroughToCookie(t *testing.T) {
	r := newRequest(t, "/?lang=zz") // zz is not supported
	r.AddCookie(&http.Cookie{Name: LangCookieName, Value: "es"})

	tag, _ := Detect(r)
	if tag != "es" {
		t.Errorf("tag = %q, want %q (cookie fallback)", tag, "es")
	}
}

func TestDetect_URLParam_CaseInsensitive(t *testing.T) {
	r := newRequest(t, "/?lang=ES")
	tag, cookieValue := Detect(r)
	if tag != "es" {
		t.Errorf("tag = %q, want %q", tag, "es")
	}
	if cookieValue == "" {
		t.Error("cookieValue should be set")
	}
}

// ── parseAcceptLanguage ───────────────────────

func TestParseAcceptLanguage(t *testing.T) {
	cases := []struct {
		header  string
		wantTag string
	}{
		{"es", "es"},
		{"pt-BR", "pt-BR"},
		{"pt-BR,pt;q=0.9,en;q=0.8", "pt-BR"},
		{"fr,en;q=0.9", "en"}, // fr unsupported; falls through to en
		{"zz", ""},            // fully unsupported
		{"", ""},              // empty header
		{" es ; q=0.9", "es"}, // whitespace around token
	}
	for _, tc := range cases {
		t.Run(tc.header, func(t *testing.T) {
			got := parseAcceptLanguage(tc.header)
			if got != tc.wantTag {
				t.Errorf("parseAcceptLanguage(%q) = %q, want %q", tc.header, got, tc.wantTag)
			}
		})
	}
}
