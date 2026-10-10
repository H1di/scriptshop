package main

import (
	"crypto/rand"
	"crypto/sha256"
	"sync"
	"time"
)

// Sessions live only in memory: restarting the server logs everyone out.
const (
	sessionIdle    = 30 * time.Minute
	sessionMax     = 12 * time.Hour
	sessionPerUser = 5
)

type session struct {
	login   string
	gen     uint64 // User.Gen at login time
	created time.Time
	seen    time.Time
}

func (s *session) expired(now time.Time) bool {
	return now.Sub(s.seen) > sessionIdle || now.Sub(s.created) > sessionMax
}

// sessions is keyed by a hash of the token, so the map never holds
// a usable cookie value.
type sessions struct {
	mu sync.Mutex
	m  map[[32]byte]*session
}

func newSessions() *sessions { return &sessions{m: map[[32]byte]*session{}} }

func tokenKey(token string) [32]byte { return sha256.Sum256([]byte(token)) }

// create starts a session and returns its token. The oldest session of the
// same login is dropped once the login has sessionPerUser of them.
func (s *sessions) create(login string, gen uint64) string {
	token := rand.Text()
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var oldest [32]byte
	count := 0
	for k, v := range s.m {
		if v.login == login {
			if count == 0 || v.created.Before(s.m[oldest].created) {
				oldest = k
			}
			count++
		}
	}
	if count >= sessionPerUser {
		delete(s.m, oldest)
	}
	s.m[tokenKey(token)] = &session{login: login, gen: gen, created: now, seen: now}
	return token
}

// get returns the session for token and marks it as used.
func (s *sessions) get(token string) (session, bool) {
	k := tokenKey(token)
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.m[k]
	if v == nil {
		return session{}, false
	}
	if v.expired(now) {
		delete(s.m, k)
		return session{}, false
	}
	v.seen = now
	return *v, true
}

func (s *sessions) delete(token string) {
	s.mu.Lock()
	delete(s.m, tokenKey(token))
	s.mu.Unlock()
}

func (s *sessions) sweep() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.m {
		if v.expired(now) {
			delete(s.m, k)
		}
	}
}
