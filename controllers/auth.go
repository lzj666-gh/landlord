package controllers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/astaxie/beego/logs"
	"landlord/common"
)

const maxAuthRequestSize = 16 << 10

type authError struct {
	Error string `json:"error"`
}

type authResult struct {
	UID      int    `json:"uid"`
	Username string `json:"username"`
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		logs.Error("write authentication response failed: %v", err)
	}
}

func readCredentials(w http.ResponseWriter, r *http.Request, acceptEmail bool) (string, string, bool) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, authError{Error: "method_not_allowed"})
		return "", "", false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAuthRequestSize)
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, authError{Error: "invalid_form"})
		return "", "", false
	}

	username := strings.TrimSpace(r.Form.Get("username"))
	if username == "" && acceptEmail {
		username = strings.TrimSpace(r.Form.Get("email"))
	}
	password := r.Form.Get("password")
	if username == "" || password == "" {
		writeJSON(w, http.StatusBadRequest, authError{Error: "missing_credentials"})
		return "", "", false
	}
	return username, password, true
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		MaxAge:   int(common.SessionLifetime.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearAuthCookies(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session"); err == nil {
		common.RevokeSession(cookie.Value)
	}
	for _, name := range []string{"session", "userid", "username", "user"} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: name == "session",
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteLaxMode,
		})
	}
}
