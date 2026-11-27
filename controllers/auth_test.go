package controllers

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
	"landlord/common"
)

func setupAuthDatabase(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	previousDB := common.GameConfInfo.Db
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE account (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL,
		username TEXT NOT NULL,
		password TEXT NOT NULL,
		coin INTEGER,
		created_date TEXT,
		updated_date TEXT
	)`)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	common.GameConfInfo.Db = db
	return db, func() {
		common.GameConfInfo.Db = previousDB
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}
}

func postForm(target string, values url.Values) *http.Request {
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func TestRegisterLoginAndLogout(t *testing.T) {
	db, cleanup := setupAuthDatabase(t)
	defer cleanup()

	form := url.Values{"username": {"alice"}, "password": {"correct horse"}}
	registerResponse := httptest.NewRecorder()
	Register(registerResponse, postForm("/reg", form))
	if registerResponse.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d: %s", registerResponse.Code, http.StatusCreated, registerResponse.Body)
	}
	var result authResult
	if err := json.Unmarshal(registerResponse.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.UID == 0 || result.Username != "alice" {
		t.Fatalf("unexpected registration response: %+v", result)
	}
	var storedPassword string
	if err := db.QueryRow("SELECT password FROM account WHERE username=?", "alice").Scan(&storedPassword); err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte("correct horse")); err != nil {
		t.Fatalf("registered password was not bcrypt hashed: %v", err)
	}
	registerCookie := findCookie(registerResponse.Result().Cookies(), "session")
	if registerCookie == nil || !registerCookie.HttpOnly || registerCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("registration session cookie is missing expected security attributes: %#v", registerCookie)
	}

	duplicateResponse := httptest.NewRecorder()
	Register(duplicateResponse, postForm("/reg", form))
	if duplicateResponse.Code != http.StatusConflict {
		t.Fatalf("duplicate registration status = %d, want %d", duplicateResponse.Code, http.StatusConflict)
	}

	invalidResponse := httptest.NewRecorder()
	Login(invalidResponse, postForm("/login", url.Values{"username": {"alice"}, "password": {"wrong"}}))
	if invalidResponse.Code != http.StatusUnauthorized {
		t.Fatalf("invalid login status = %d, want %d", invalidResponse.Code, http.StatusUnauthorized)
	}
	if findCookie(invalidResponse.Result().Cookies(), "session") != nil {
		t.Fatal("invalid login created a session cookie")
	}

	loginResponse := httptest.NewRecorder()
	Login(loginResponse, postForm("/login", form))
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d: %s", loginResponse.Code, http.StatusOK, loginResponse.Body)
	}
	loginCookie := findCookie(loginResponse.Result().Cookies(), "session")
	if loginCookie == nil {
		t.Fatal("successful login did not create a session cookie")
	}
	if session, ok := common.GetSession(loginCookie.Value); !ok || session.UserID != result.UID {
		t.Fatalf("successful login created an invalid session: %+v, %v", session, ok)
	}

	logoutRequest := httptest.NewRequest(http.MethodPost, "/loginOut", nil)
	logoutRequest.AddCookie(loginCookie)
	logoutResponse := httptest.NewRecorder()
	LoginOut(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusOK {
		t.Fatalf("logout status = %d, want %d", logoutResponse.Code, http.StatusOK)
	}
	if _, ok := common.GetSession(loginCookie.Value); ok {
		t.Fatal("logout did not revoke the server-side session")
	}
	expiredCookie := findCookie(logoutResponse.Result().Cookies(), "session")
	if expiredCookie == nil || expiredCookie.MaxAge >= 0 {
		t.Fatalf("logout did not expire session cookie: %#v", expiredCookie)
	}
}

func TestLoginUpgradesLegacyMD5Password(t *testing.T) {
	db, cleanup := setupAuthDatabase(t)
	defer cleanup()

	sum := md5.Sum([]byte("legacy-password"))
	legacyHash := hex.EncodeToString(sum[:])
	if _, err := db.Exec(
		"INSERT INTO account(email, username, password, coin, created_date, updated_date) VALUES(?, ?, ?, ?, ?, ?)",
		"legacy", "legacy", legacyHash, 1000, "now", "now",
	); err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	Login(response, postForm("/login", url.Values{"username": {"legacy"}, "password": {"legacy-password"}}))
	if response.Code != http.StatusOK {
		t.Fatalf("legacy login status = %d, want %d: %s", response.Code, http.StatusOK, response.Body)
	}
	var upgradedHash string
	if err := db.QueryRow("SELECT password FROM account WHERE username=?", "legacy").Scan(&upgradedHash); err != nil {
		t.Fatal(err)
	}
	if upgradedHash == legacyHash || bcrypt.CompareHashAndPassword([]byte(upgradedHash), []byte("legacy-password")) != nil {
		t.Fatal("legacy password hash was not upgraded to bcrypt")
	}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}
