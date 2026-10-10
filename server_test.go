package main

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// site is a running test instance with one user, "ivan", who owns script 1
// but not script 2.
type site struct {
	t        *testing.T
	store    Store
	url      string
	client   *http.Client
	password string
}

func newSite(t *testing.T) *site {
	t.Helper()
	st := Store{dir: t.TempDir()}
	password := newPassword()
	for _, name := range []string{"owned.sh", "other.sh"} {
		src := filepath.Join(t.TempDir(), name)
		os.WriteFile(src, []byte("echo "+name+"\n"), 0o600)
		blob, err := st.Import(src)
		if err != nil {
			t.Fatal(err)
		}
		st.Update(func(db *DB) error {
			db.NextID++
			db.Scripts = append(db.Scripts, &Script{ID: db.NextID, Name: name, Price: "$10", Blob: blob})
			return nil
		})
	}
	st.Update(func(db *DB) error {
		db.Users["ivan"] = &User{Hash: hashPassword(password), Gen: newGen(), Grants: []int{1}}
		return nil
	})

	ts := httptest.NewTLSServer(newServer(st, "TEST", false).routes())
	t.Cleanup(ts.Close)
	s := &site{t: t, store: st, url: ts.URL, password: password}
	s.client = s.newClient(ts)
	return s
}

func (s *site) newClient(ts *httptest.Server) *http.Client {
	c := ts.Client()
	c.Jar, _ = cookiejar.New(nil)
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return c
}

func (s *site) do(req *http.Request) (*http.Response, string) {
	s.t.Helper()
	resp, err := s.client.Do(req)
	if err != nil {
		s.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

func (s *site) get(path string) (*http.Response, string) {
	req, _ := http.NewRequest("GET", s.url+path, nil)
	return s.do(req)
}

func (s *site) post(path string, form url.Values) (*http.Response, string) {
	req, _ := http.NewRequest("POST", s.url+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return s.do(req)
}

func (s *site) login(password string) *http.Response {
	resp, _ := s.post("/login", url.Values{"login": {"ivan"}, "password": {password}})
	return resp
}

func wantStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("%s %s: status %d, want %d", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, want)
	}
}

func TestLoginRequired(t *testing.T) {
	s := newSite(t)
	for _, path := range []string{"/", "/dl/1"} {
		resp, _ := s.get(path)
		wantStatus(t, resp, http.StatusSeeOther)
		if loc := resp.Header.Get("Location"); loc != "/login" {
			t.Errorf("%s redirects to %q, want /login", path, loc)
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	s := newSite(t)
	resp, _ := s.get("/login")
	for _, h := range []string{"Content-Security-Policy", "X-Frame-Options", "Strict-Transport-Security", "Referrer-Policy"} {
		if resp.Header.Get(h) == "" {
			t.Errorf("missing %s", h)
		}
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q", cc)
	}
}

func TestLogin(t *testing.T) {
	s := newSite(t)

	resp := s.login("wrong")
	wantStatus(t, resp, http.StatusUnauthorized)

	resp = s.login(s.password)
	wantStatus(t, resp, http.StatusSeeOther)
	cookie := resp.Header.Get("Set-Cookie")
	for _, attr := range []string{"__Host-sid=", "HttpOnly", "Secure", "SameSite=Strict"} {
		if !strings.Contains(cookie, attr) {
			t.Errorf("cookie %q lacks %s", cookie, attr)
		}
	}

	resp, body := s.get("/")
	wantStatus(t, resp, http.StatusOK)
	if !strings.Contains(body, `href="/dl/1"`) || strings.Contains(body, `href="/dl/2"`) {
		t.Errorf("catalog should link only the owned script:\n%s", body)
	}
}

func TestDownload(t *testing.T) {
	s := newSite(t)
	s.login(s.password)

	resp, body := s.get("/dl/1")
	wantStatus(t, resp, http.StatusOK)
	if body != "echo owned.sh\n" {
		t.Errorf("body = %q", body)
	}
	if cd := resp.Header.Get("Content-Disposition"); cd != "attachment; filename=owned.sh" {
		t.Errorf("Content-Disposition = %q", cd)
	}

	for _, path := range []string{"/dl/2", "/dl/99", "/dl/x", "/dl/..%2fdb.json"} {
		resp, _ := s.get(path)
		wantStatus(t, resp, http.StatusNotFound)
	}
}

func TestCrossOriginPostRejected(t *testing.T) {
	s := newSite(t)
	req, _ := http.NewRequest("POST", s.url+"/login", strings.NewReader("login=ivan&password="+s.password))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	resp, _ := s.do(req)
	wantStatus(t, resp, http.StatusForbidden)
}

func TestLogoutEndsSession(t *testing.T) {
	s := newSite(t)
	s.login(s.password)
	u, _ := url.Parse(s.url)
	stolen := s.client.Jar.Cookies(u)

	s.post("/logout", nil)

	// replay the old cookie
	jar, _ := cookiejar.New(nil)
	jar.SetCookies(u, stolen)
	s.client.Jar = jar
	resp, _ := s.get("/")
	wantStatus(t, resp, http.StatusSeeOther)
}

func TestDisableEndsSession(t *testing.T) {
	s := newSite(t)
	s.login(s.password)
	s.store.Update(func(db *DB) error {
		db.Users["ivan"].Disabled = true
		return nil
	})
	time.Sleep(1100 * time.Millisecond) // liveDB re-reads at most once per second

	resp, _ := s.get("/")
	wantStatus(t, resp, http.StatusSeeOther)
}

func TestThrottling(t *testing.T) {
	s := newSite(t)
	for range clientMaxFails {
		wantStatus(t, s.login("wrong"), http.StatusUnauthorized)
	}
	// blocked now, even with the right password
	wantStatus(t, s.login(s.password), http.StatusTooManyRequests)
}

func TestPassword(t *testing.T) {
	h := hashPassword("secret")
	if !checkPassword(h, "secret") {
		t.Error("right password rejected")
	}
	if checkPassword(h, "Secret") {
		t.Error("wrong password accepted")
	}
	for _, bad := range []string{"", "secret", "$argon2id$v=19$m=0,t=0,p=0$$"} {
		if checkPassword(bad, "secret") {
			t.Errorf("malformed hash %q accepted", bad)
		}
	}
}

func TestClientKey(t *testing.T) {
	if clientKey("2001:db8::1") != clientKey("2001:db8::ffff") {
		t.Error("addresses in one IPv6 /64 should share a key")
	}
	if clientKey("::ffff:10.0.0.1") != "10.0.0.1" {
		t.Error("IPv4-mapped address should be unmapped")
	}
}
