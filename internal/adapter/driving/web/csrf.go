package web

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const (
	csrfCookieName = "csrf_token"
	csrfFormField  = "csrf_token"
	csrfTokenBytes = 32
)

// csrfToken ensures a CSRF token cookie is set on the response. If the request
// already has a valid CSRF cookie, this is a no-op. Otherwise, a new token is
// generated and set. The token is readable by csrf.js to set X-CSRF-Token on
// HTMX requests.
func csrfToken(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(csrfCookieName); err == nil && cookie.Value != "" {
		return
	}

	token := generateToken()
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false, // readable by csrf.js to set X-CSRF-Token header on HTMX requests
		SameSite: http.SameSiteStrictMode,
		Secure:   false, // set true when served over HTTPS
	})
}

// validateCSRF checks that the CSRF token (from header or form field) matches
// the cookie. Returns true if the tokens match and are non-empty.
func validateCSRF(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}

	// Check header first (HTMX sends it here), then fall back to form field.
	token := r.Header.Get("X-CSRF-Token")
	if token == "" {
		token = r.FormValue(csrfFormField)
	}

	return token != "" && token == cookie.Value
}

func generateToken() string {
	b := make([]byte, csrfTokenBytes)
	if _, err := rand.Read(b); err != nil {
		panic("csrf: failed to generate random token: " + err.Error())
	}
	return hex.EncodeToString(b)
}
