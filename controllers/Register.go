package controllers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/astaxie/beego/logs"
	"landlord/common"
)

func Register(w http.ResponseWriter, r *http.Request) {
	username, password, ok := readCredentials(w, r, false)
	if !ok {
		return
	}
	if len([]byte(password)) > 72 {
		writeJSON(w, http.StatusBadRequest, authError{Error: "password_too_long"})
		return
	}

	var existingID int
	err := common.GameConfInfo.Db.QueryRow("SELECT id FROM account WHERE username=?", username).Scan(&existingID)
	if err == nil {
		writeJSON(w, http.StatusConflict, authError{Error: "username_taken"})
		return
	}
	if err != sql.ErrNoRows {
		logs.Error("register account lookup failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, authError{Error: "internal_error"})
		return
	}

	hashed, err := hashPassword(password)
	if err != nil {
		logs.Error("hash registration password failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, authError{Error: "internal_error"})
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	result, err := common.GameConfInfo.Db.Exec(
		"INSERT INTO account(email, username, password, coin, created_date, updated_date) VALUES(?, ?, ?, ?, ?, ?)",
		username, username, hashed, common.StartCoin, now, now,
	)
	if err != nil {
		logs.Error("insert new account failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, authError{Error: "internal_error"})
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		logs.Error("get new account id failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, authError{Error: "internal_error"})
		return
	}

	token, err := common.CreateSession(int(id), username)
	if err != nil {
		logs.Error("create registration session failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, authError{Error: "internal_error"})
		return
	}
	setSessionCookie(w, r, token)
	writeJSON(w, http.StatusCreated, authResult{UID: int(id), Username: username})
}
