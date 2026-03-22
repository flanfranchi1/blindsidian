package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flanfranchi1/notty/internal/i18n"
)

// echoLocaleHandler is a trivial next-handler that writes the locale from context
// into the response body so tests can assert on it.
var echoLocaleHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(i18n.LocaleFromContext(r.Context())))
})

// cookieValue returns the Set-Cookie value for the given cookie name from the
// recorder, or "" if no such cookie was set.
func cookieValue(t *testing.T, w *httptest.ResponseRecorder, name string) string {
	t.Helper()
	resp := w.Result()
	defer resp.Body.Close()
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

func TestI18nMiddleware_DefaultLocale(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	I18nMiddleware(echoLocaleHandler).ServeHTTP(w, r)

	if got, want := w.Body.String(), "en"; got != want {
		t.Errorf("locale = %q, want %q", got, want)
	}
	if cv := cookieValue(t, w, i18n.LangCookieName); cv != "" {
		t.Errorf("no cookie should be set for default locale, got %q", cv)
	}
}

func TestI18nMiddleware_URLParam_SetsLocaleAndCookie(t *testing.T) {
	cases := []struct {
		param      string
		wantLocale string
		wantCookie string
	}{
		{"en", "en", "en"},
		{"es", "es", "es"},
		{"pt", "pt-BR", "pt"},
		{"pt-br", "pt-BR", "pt-br"},
	}
	for _, tc := range cases {
		t.Run(tc.param, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/?lang="+tc.param, nil)
			w := httptest.NewRecorder()

			I18nMiddleware(echoLocaleHandler).ServeHTTP(w, r)

			if got := w.Body.String(); got != tc.wantLocale {
				t.Errorf("locale = %q, want %q", got, tc.wantLocale)
			}
			if got := cookieValue(t, w, i18n.LangCookieName); got != tc.wantCookie {
				t.Errorf("cookie = %q, want %q", got, tc.wantCookie)
			}
		})
	}
}

func TestI18nMiddleware_URLParam_BeatsCookie(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?lang=es", nil)
	r.AddCookie(&http.Cookie{Name: i18n.LangCookieName, Value: "en"})
	w := httptest.NewRecorder()

	I18nMiddleware(echoLocaleHandler).ServeHTTP(w, r)

	if got := w.Body.String(); got != "es" {
		t.Errorf("locale = %q, want %q", got, "es")
	}
}

func TestI18nMiddleware_Cookie_NoCookieWrittenBack(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: i18n.LangCookieName, Value: "es"})
	w := httptest.NewRecorder()

	I18nMiddleware(echoLocaleHandler).ServeHTTP(w, r)

	if got := w.Body.String(); got != "es" {
		t.Errorf("locale = %q, want %q", got, "es")
	}
	// Cookie came from the browser — middleware should NOT write it back.
	if cv := cookieValue(t, w, i18n.LangCookieName); cv != "" {
		t.Errorf("middleware must not re-set cookie when locale comes from existing cookie, got %q", cv)
	}
}

func TestI18nMiddleware_AcceptLanguage(t *testing.T) {
	cases := []struct {
		header     string
		wantLocale string
	}{
		{"pt-BR,pt;q=0.9,en;q=0.8", "pt-BR"},
		{"es,en;q=0.9", "es"},
		{"en", "en"},
		{"fr,de;q=0.9", "en"}, // unsupported → default
	}
	for _, tc := range cases {
		t.Run(tc.header, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set("Accept-Language", tc.header)
			w := httptest.NewRecorder()

			I18nMiddleware(echoLocaleHandler).ServeHTTP(w, r)

			if got := w.Body.String(); got != tc.wantLocale {
				t.Errorf("Accept-Language %q → locale %q, want %q", tc.header, got, tc.wantLocale)
			}
			// No cookie should ever be set when the source is Accept-Language.
			if cv := cookieValue(t, w, i18n.LangCookieName); cv != "" {
				t.Errorf("must not write cookie for Accept-Language source, got %q", cv)
			}
		})
	}
}

func TestI18nMiddleware_UnknownLangParam_FallsThrough(t *testing.T) {
	// ?lang=zz is not a supported locale; the middleware should ignore it.
	r := httptest.NewRequest(http.MethodGet, "/?lang=zz", nil)
	r.AddCookie(&http.Cookie{Name: i18n.LangCookieName, Value: "es"})
	w := httptest.NewRecorder()

	I18nMiddleware(echoLocaleHandler).ServeHTTP(w, r)

	if got := w.Body.String(); got != "es" {
		t.Errorf("locale = %q, want %q (cookie fallback)", got, "es")
	}
}

func TestI18nMiddleware_CookieMaxAge(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?lang=pt", nil)
	w := httptest.NewRecorder()

	I18nMiddleware(echoLocaleHandler).ServeHTTP(w, r)

	resp := w.Result()
	defer resp.Body.Close()
	for _, c := range resp.Cookies() {
		if c.Name == i18n.LangCookieName {
			if c.MaxAge <= 0 {
				t.Errorf("expected positive MaxAge, got %d", c.MaxAge)
			}
			// Should be approximately 1 year (~31 536 000 s). Allow a generous window.
			const oneYear = 365 * 24 * 60 * 60
			if c.MaxAge < oneYear-60 || c.MaxAge > oneYear+60 {
				t.Errorf("MaxAge = %d, expected ~%d (1 year)", c.MaxAge, oneYear)
			}
			return
		}
	}
	t.Error("notty_lang cookie not found in response")
}
