package main

import (
	"context"
	"crypto/tls"
	"embed"
	"errors"
	"flag"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

//go:embed web
var web embed.FS

const hashWorkers = 4 // concurrent argon2 checks; each uses 64 MiB

type server struct {
	name       string
	trustProxy bool
	store      Store
	db         *liveDB
	sessions   *sessions
	limiter    *limiter
	hashSlots  chan struct{}
	dummyHash  string // checked for unknown logins so they take as long as known ones
	pages      *template.Template
	css        []byte
}

func newServer(store Store, name string, trustProxy bool) *server {
	css, err := web.ReadFile("web/style.css")
	if err != nil {
		panic(err)
	}
	funcs := template.FuncMap{"size": humanSize}
	return &server{
		name:       name,
		trustProxy: trustProxy,
		store:      store,
		db:         &liveDB{store: store},
		sessions:   newSessions(),
		limiter:    newLimiter(),
		hashSlots:  make(chan struct{}, hashWorkers),
		dummyHash:  hashPassword(newPassword()),
		pages:      template.Must(template.New("").Funcs(funcs).ParseFS(web, "web/*.html")),
		css:        css,
	}
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.catalog)
	mux.HandleFunc("GET /login", s.loginPage)
	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("POST /logout", s.logout)
	mux.HandleFunc("GET /dl/{id}", s.download)
	mux.HandleFunc("GET /style.css", s.style)
	mux.HandleFunc("GET /robots.txt", robots)
	// CrossOriginProtection rejects POSTs sent from other sites (CSRF).
	return s.securityHeaders(http.NewCrossOriginProtection().Handler(mux))
}

func serve(store Store, args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8080", "listen address")
	name := fs.String("name", "SCRIPTS", "site name")
	trustProxy := fs.Bool("trust-proxy", false, "trust X-Forwarded-For/-Proto from a reverse proxy on this host")
	cert := fs.String("cert", "", "TLS certificate file")
	key := fs.String("key", "", "TLS key file")
	fs.Parse(args)
	if (*cert == "") != (*key == "") {
		fatal("-cert and -key go together")
	}
	if fi, err := os.Stat(store.dir); err == nil && fi.Mode().Perm()&0o077 != 0 {
		log.Printf("warning: %s is readable by other users; chmod 700 it", store.dir)
	}

	s := newServer(store, *name, *trustProxy)
	srv := &http.Server{
		Addr:              *addr,
		Handler:           s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      5 * time.Minute, // room for slow downloads
		IdleTimeout:       time.Minute,
		MaxHeaderBytes:    8 << 10,
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS12},
	}

	go func() {
		for range time.Tick(5 * time.Minute) {
			s.sessions.sweep()
			s.limiter.sweep()
		}
	}()
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	log.Printf("listening on %s, data in %s", *addr, store.dir)
	var err error
	if *cert != "" {
		err = srv.ListenAndServeTLS(*cert, *key)
	} else {
		err = srv.ListenAndServe()
	}
	if !errors.Is(err, http.ErrServerClosed) {
		fatal("%v", err)
	}
}

func (s *server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("X-Robots-Tag", "noindex, nofollow")
		h.Set("Cache-Control", "no-store")
		if r.TLS != nil || (s.fromProxy(r) && r.Header.Get("X-Forwarded-Proto") == "https") {
			h.Set("Strict-Transport-Security", "max-age=63072000")
		}
		next.ServeHTTP(w, r)
	})
}

// fromProxy reports whether proxy headers on r can be trusted: only with
// -trust-proxy, and only when the request comes from this host.
func (s *server) fromProxy(r *http.Request) bool {
	if !s.trustProxy {
		return false
	}
	ap, err := netip.ParseAddrPort(r.RemoteAddr)
	return err == nil && ap.Addr().IsLoopback()
}

// clientIP returns the throttling key for the client behind r.
func (s *server) clientIP(r *http.Request) string {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if s.fromProxy(r) {
		// The proxy appends the address it saw, so the last entry is the
		// one it vouches for; anything before it is client-supplied.
		xff := strings.Join(r.Header.Values("X-Forwarded-For"), ",")
		last := strings.TrimSpace(xff[strings.LastIndexByte(xff, ',')+1:])
		if a, err := netip.ParseAddr(last); err == nil {
			ip = a.String()
		}
	}
	return clientKey(ip)
}
