package controllers

import (
	"net/http"
)

func LoginOut(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, authError{Error: "method_not_allowed"})
		return
	}
	clearAuthCookies(w, r)
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}
