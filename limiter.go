package main

import (
	"net/netip"
	"sync"
	"time"
)

// Login throttling.
//
// Per client (an IPv4 address or an IPv6 /64): from the 5th failure on, the
// client is blocked for 1, 2, 4 ... 64 minutes.
// Per login: from the 10th failure from anywhere, blocked for 15 minutes.
// Counters reset after an hour without failures.
const (
	clientMaxFails = 5
	loginMaxFails  = 10
	loginBlock     = 15 * time.Minute
	failMemory     = time.Hour
)

type counter struct {
	fails int
	last  time.Time
	until time.Time
}

type limiter struct {
	mu     sync.Mutex
	client map[string]*counter
	login  map[string]*counter
}

func newLimiter() *limiter {
	return &limiter{client: map[string]*counter{}, login: map[string]*counter{}}
}

// clientKey groups IPv6 addresses by /64, since one host usually owns a whole /64.
func clientKey(ip string) string {
	a, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	a = a.Unmap()
	if a.Is6() {
		p, _ := a.Prefix(64)
		return p.String()
	}
	return a.String()
}

// login may be "" when it is not a known login; only known logins are
// counted, so random input cannot grow the map.
func (l *limiter) blocked(client, login string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if c := l.client[client]; c != nil && now.Before(c.until) {
		return true
	}
	if c := l.login[login]; c != nil && now.Before(c.until) {
		return true
	}
	return false
}

func (l *limiter) fail(client, login string) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if c := count(l.client, client, now); c.fails >= clientMaxFails {
		// doubles from 1 minute, capped at 2^6 = 64 minutes
		c.until = now.Add(time.Minute << min(c.fails-clientMaxFails, 6))
	}
	if login != "" {
		if c := count(l.login, login, now); c.fails >= loginMaxFails {
			c.until = now.Add(loginBlock)
		}
	}
}

func (l *limiter) success(client, login string) {
	l.mu.Lock()
	delete(l.client, client)
	delete(l.login, login)
	l.mu.Unlock()
}

func (l *limiter) sweep() {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, m := range []map[string]*counter{l.client, l.login} {
		for k, c := range m {
			if now.Sub(c.last) > failMemory && now.After(c.until) {
				delete(m, k)
			}
		}
	}
}

func count(m map[string]*counter, key string, now time.Time) *counter {
	c := m[key]
	if c == nil {
		c = &counter{}
		m[key] = c
	}
	if now.Sub(c.last) > failMemory {
		c.fails = 0
	}
	c.fails++
	c.last = now
	return c
}
