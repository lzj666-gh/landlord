package common

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

const SessionLifetime = 24 * time.Hour

type Session struct {
	UserID   int
	Username string
	expires  time.Time
}

var (
	sessionMu sync.Mutex
	sessions  = make(map[string]Session)
)

func CreateSession(userID int, username string) (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}

	token := base64.RawURLEncoding.EncodeToString(random)
	sessionMu.Lock()
	defer sessionMu.Unlock()

	now := time.Now()
	for existingToken, session := range sessions {
		if !session.expires.After(now) {
			delete(sessions, existingToken)
		}
	}
	sessions[token] = Session{
		UserID:   userID,
		Username: username,
		expires:  now.Add(SessionLifetime),
	}
	return token, nil
}

func GetSession(token string) (Session, bool) {
	sessionMu.Lock()
	defer sessionMu.Unlock()

	session, ok := sessions[token]
	if !ok {
		return Session{}, false
	}
	if !session.expires.After(time.Now()) {
		delete(sessions, token)
		return Session{}, false
	}
	return session, true
}

func RevokeSession(token string) {
	sessionMu.Lock()
	delete(sessions, token)
	sessionMu.Unlock()
}
