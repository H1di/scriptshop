package main

import (
	"bytes"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"strconv"
)

const (
	cookieName   = "__Host-sid" // __Host-: Secure, no Domain, Path=/
	maxFormBytes = 4 << 10
	maxPassword  = 256
)

// account is the logged-in user of a request.
type account struct {
	login string
	user  *User
	db    *DB
}

// account returns the logged-in user, or nil. A session ends as soon as its
// user is deleted, disabled or gets a new password (User.Gen changes).
func (s *server) account(r *http.Request) *account {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil
	}
	sess, ok := s.sessions.get(c.Value)
	if !ok {
		return nil
	}
	db := s.db.get()
	u := db.Users[sess.login]
	if u == nil || u.Disabled || u.Gen != sess.gen {
		s.sessions.delete(c.Value)
		return nil
	}
	return &account{login: sess.login, user: u, db: db}
}

func (s *server) loginPage(w http.ResponseWriter, r *http.Request) {
	if s.account(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, http.StatusOK, "login.html", map[string]any{"Name": s.name})
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	login, pw := r.PostForm.Get("login"), r.PostForm.Get("password")
	client := s.clientIP(r)
	refuse := func(status int, msg string) {
		s.render(w, status, "login.html", map[string]any{"Name": s.name, "Error": msg})
	}

	var u *User
	if validLogin(login) && len(pw) <= maxPassword {
		u = s.db.get().Users[login]
	}
	known := "" // the login, if it exists; the limiter only tracks those
	if u != nil {
		known = login
	}

	if s.limiter.blocked(client, known) {
		log.Printf("login throttled client=%s login=%.40q", client, login)
		refuse(http.StatusTooManyRequests, "Too many attempts. Try again later.")
		return
	}
	if !s.verify(u, pw) {
		s.limiter.fail(client, known)
		log.Printf("login failed client=%s login=%.40q", client, login)
		refuse(http.StatusUnauthorized, "Login incorrect")
		return
	}
	s.limiter.success(client, known)

	if old, err := r.Cookie(cookieName); err == nil {
		s.sessions.delete(old.Value)
	}
	setSessionCookie(w, s.sessions.create(login, u.Gen))
	log.Printf("login ok client=%s login=%q", client, login)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// verify checks pw for an active user. For a nil user it still spends the
// time of a real check, so response times do not reveal which logins exist.
func (s *server) verify(u *User, pw string) bool {
	hash := s.dummyHash
	if u != nil {
		hash = u.Hash
	}
	s.hashSlots <- struct{}{}
	ok := checkPassword(hash, pw)
	<-s.hashSlots
	return ok && u != nil && !u.Disabled
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		s.sessions.delete(c.Value)
	}
	setSessionCookie(w, "")
	w.Header().Set("Clear-Site-Data", `"cookies", "storage"`)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *server) catalog(w http.ResponseWriter, r *http.Request) {
	acc := s.account(r)
	if acc == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	s.render(w, http.StatusOK, "catalog.html", map[string]any{
		"Name":    s.name,
		"Login":   acc.login,
		"User":    acc.user,
		"Scripts": acc.db.Scripts,
	})
}

// download serves a script the user has been granted. Anything else is a 404,
// so users cannot probe which scripts exist.
func (s *server) download(w http.ResponseWriter, r *http.Request) {
	acc := s.account(r)
	if acc == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	id, _ := strconv.Atoi(r.PathValue("id"))
	sc := acc.db.script(id)
	if sc == nil || !acc.user.Owns(id) || !validBlob(sc.File) {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(s.store.blobPath(sc.File))
	if err != nil {
		log.Printf("download %03d: %v", id, err)
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": safeFilename(sc.Name)}))
	log.Printf("download login=%q script=%03d", acc.login, id)
	http.ServeContent(w, r, "", fi.ModTime(), f)
}

func (s *server) style(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(s.css)
}

func robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, "User-agent: *\nDisallow: /\n")
}

// render executes the template fully before writing, so a template error
// never sends half a page.
func (s *server) render(w http.ResponseWriter, status int, page string, data any) {
	var buf bytes.Buffer
	if err := s.pages.ExecuteTemplate(&buf, page, data); err != nil {
		log.Printf("render %s: %v", page, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	w.Write(buf.Bytes())
}

// setSessionCookie sets the session cookie, or deletes it when token is "".
// It has no expiry, so the browser drops it when closed.
func setSessionCookie(w http.ResponseWriter, token string) {
	c := &http.Cookie{
		Name: cookieName, Value: token, Path: "/",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
	}
	if token == "" {
		c.MaxAge = -1
	}
	http.SetCookie(w, c)
}

func humanSize(n int64) string {
	switch {
	case n < 1<<10:
		return fmt.Sprintf("%dB", n)
	case n < 1<<20:
		return fmt.Sprintf("%.1fK", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%.1fM", float64(n)/(1<<20))
	}
}

// safeFilename keeps [A-Za-z0-9._-] and replaces everything else with '_'.
func safeFilename(name string) string {
	b := []byte(name)
	for i, c := range b {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '-' || c == '_') {
			b[i] = '_'
		}
	}
	if len(b) == 0 || b[0] == '.' {
		b = append([]byte("script"), b...)
	}
	return string(b)
}
