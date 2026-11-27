package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServeWsRejectsLegacyIdentityCookies(t *testing.T) {
	request := httptest.NewRequest("GET", "/ws", nil)
	request.AddCookie(&http.Cookie{Name: "userid", Value: "10"})
	request.AddCookie(&http.Cookie{Name: "username", Value: "forged"})
	response := httptest.NewRecorder()

	ServeWs(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestWebSocketOriginMustMatchHost(t *testing.T) {
	request := httptest.NewRequest("GET", "http://example.test/ws", nil)
	request.Host = "example.test"
	request.Header.Set("Origin", "http://example.test")
	if !upGrader.CheckOrigin(request) {
		t.Fatal("same-origin WebSocket request was rejected")
	}

	request.Header.Set("Origin", "http://attacker.test")
	if upGrader.CheckOrigin(request) {
		t.Fatal("cross-origin WebSocket request was accepted")
	}
}
