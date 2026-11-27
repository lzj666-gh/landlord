package controllers

import (
	"crypto/md5"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/astaxie/beego/logs"
	"golang.org/x/crypto/bcrypt"
	"landlord/common"
)

func Login(w http.ResponseWriter, r *http.Request) {
	username, password, ok := readCredentials(w, r, true)
	if !ok {
		return
	}

	var account common.Account
	err := common.GameConfInfo.Db.QueryRow(
		"SELECT id, email, username, password, coin, created_date, updated_date FROM account WHERE username=?",
		username,
	).Scan(&account.Id, &account.Email, &account.Username, &account.Password, &account.Coin, &account.CreatedDate, &account.UpdateDate)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusUnauthorized, authError{Error: "invalid_credentials"})
			return
		}
		logs.Error("login account lookup failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, authError{Error: "internal_error"})
		return
	}

	valid, legacy := checkPassword(account.Password, password)
	if !valid {
		writeJSON(w, http.StatusUnauthorized, authError{Error: "invalid_credentials"})
		return
	}
	if legacy && len([]byte(password)) <= 72 {
		hashed, err := hashPassword(password)
		if err != nil {
			logs.Error("upgrade legacy password hash failed: %v", err)
			writeJSON(w, http.StatusInternalServerError, authError{Error: "internal_error"})
			return
		}
		_, err = common.GameConfInfo.Db.Exec(
			"UPDATE account SET password=?, updated_date=? WHERE id=? AND password=?",
			hashed, time.Now().Format("2006-01-02 15:04:05"), account.Id, account.Password,
		)
		if err != nil {
			logs.Error("upgrade legacy password hash failed: %v", err)
			writeJSON(w, http.StatusInternalServerError, authError{Error: "internal_error"})
			return
		}
	}

	token, err := common.CreateSession(account.Id, account.Username)
	if err != nil {
		logs.Error("create login session failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, authError{Error: "internal_error"})
		return
	}
	setSessionCookie(w, r, token)
	writeJSON(w, http.StatusOK, authResult{UID: account.Id, Username: account.Username})
}

func hashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func checkPassword(storedHash, password string) (valid, legacy bool) {
	if len(storedHash) == 32 {
		decoded, err := hex.DecodeString(storedHash)
		if err == nil && len(decoded) == md5.Size {
			sum := md5.Sum([]byte(password))
			expected := []byte(hex.EncodeToString(sum[:]))
			return subtle.ConstantTimeCompare([]byte(storedHash), expected) == 1, true
		}
	}
	return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) == nil, false
}
