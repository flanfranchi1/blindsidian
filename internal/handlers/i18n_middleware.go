package handlers

import (
	"net/http"
	"time"

	"github.com/flanfranchi1/notty/internal/i18n"
)

const langCookieMaxAge = 365 * 24 * time.Hour // ~1 year

// I18nMiddleware detects the user's preferred language using the priority order:
//
//  1. ?lang= URL query parameter  (also writes/refreshes the cookie)
//  2. notty_lang persistent cookie
//  3. Accept-Language request header
//  4. Default: "en"
//
// The resolved BCP 47 locale tag (e.g. "pt-BR") is injected into the request
// context so that handlers and RenderTemplate can access it without threading
// an extra parameter through every call site.
func I18nMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locale, cookieValue := i18n.Detect(r)

		// If the language was set via the URL param, persist it in a long-lived
		// cookie so subsequent requests don't require the query string.
		if cookieValue != "" {
			http.SetCookie(w, &http.Cookie{
				Name:     i18n.LangCookieName,
				Value:    cookieValue,
				MaxAge:   int(langCookieMaxAge.Seconds()),
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
		}

		next.ServeHTTP(w, r.WithContext(i18n.WithLocale(r.Context(), locale)))
	})
}
