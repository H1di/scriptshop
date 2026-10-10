package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sync"
	"syscall"
	"time"
)

const maxScriptSize = 100 << 20

var (
	validLogin = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,31}$`).MatchString
	validBlob  = regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString
)

type DB struct {
	NextID  int              `json:"next_id"`
	Users   map[string]*User `json:"users"`
	Scripts []*Script        `json:"scripts"`
}

type User struct {
	Hash     string    `json:"hash"`
	Gen      uint64    `json:"gen"` // replaced on passwd/disable, which ends existing sessions
	Disabled bool      `json:"disabled,omitempty"`
	Grants   []int     `json:"grants,omitempty"`
	Created  time.Time `json:"created"`
}

type Script struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Price   string `json:"price,omitempty"`
	Desc    string `json:"desc,omitempty"`
	Blob
	Added time.Time `json:"added"`
}

// Blob is a stored script file. File is a random name inside files/.
type Blob struct {
	File   string `json:"file"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

func emptyDB() *DB { return &DB{Users: map[string]*User{}} }

func (u *User) Owns(id int) bool { return slices.Contains(u.Grants, id) }

func (db *DB) script(id int) *Script {
	for _, s := range db.Scripts {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// Store is the data directory:
//
//	db.json   users, scripts, grants
//	files/    script files under random names
//	.lock     serializes admin writes
//
// Only the admin CLI writes. The server only reads.
type Store struct{ dir string }

func (st Store) dbFile() string              { return filepath.Join(st.dir, "db.json") }
func (st Store) filesDir() string            { return filepath.Join(st.dir, "files") }
func (st Store) blobPath(name string) string { return filepath.Join(st.filesDir(), name) }

func (st Store) Load() (*DB, error) {
	b, err := os.ReadFile(st.dbFile())
	if errors.Is(err, fs.ErrNotExist) {
		return emptyDB(), nil
	}
	if err != nil {
		return nil, err
	}
	db := emptyDB()
	if err := json.Unmarshal(b, db); err != nil {
		return nil, err
	}
	if db.Users == nil {
		db.Users = map[string]*User{}
	}
	return db, nil
}

// Update loads the database, applies fn and saves the result, holding an
// exclusive lock so concurrent admin commands cannot lose each other's changes.
func (st Store) Update(fn func(*DB) error) error {
	if err := os.MkdirAll(st.dir, 0o700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(st.dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	db, err := st.Load()
	if err != nil {
		return err
	}
	if err := fn(db); err != nil {
		return err
	}
	b, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(st.dbFile(), b)
}

// Import copies src into files/ under a random name.
func (st Store) Import(src string) (Blob, error) {
	in, err := os.Open(src)
	if err != nil {
		return Blob{}, err
	}
	defer in.Close()
	if fi, err := in.Stat(); err != nil {
		return Blob{}, err
	} else if !fi.Mode().IsRegular() {
		return Blob{}, errors.New("not a regular file: " + src)
	}
	if err := os.MkdirAll(st.filesDir(), 0o700); err != nil {
		return Blob{}, err
	}

	name := make([]byte, 16)
	rand.Read(name)
	b := Blob{File: hex.EncodeToString(name)}
	out, err := os.OpenFile(st.blobPath(b.File), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return Blob{}, err
	}
	h := sha256.New()
	b.Size, err = io.Copy(io.MultiWriter(out, h), io.LimitReader(in, maxScriptSize+1))
	if err == nil && b.Size > maxScriptSize {
		err = errors.New("file larger than 100 MB")
	}
	if err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		st.Remove(b.File)
		return Blob{}, err
	}
	b.SHA256 = hex.EncodeToString(h.Sum(nil))
	return b, nil
}

func (st Store) Remove(blob string) {
	if validBlob(blob) {
		os.Remove(st.blobPath(blob))
	}
}

// writeAtomic replaces path with data so that readers see either the old or
// the new file, never a partial one, even after a power loss.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// liveDB gives the server the current database, re-reading db.json at most
// once per second when it has changed. A snapshot is never modified after
// loading, so handlers may use it without locks.
type liveDB struct {
	store   Store
	mu      sync.Mutex
	db      *DB
	stat    os.FileInfo
	checked time.Time
}

func (l *liveDB) get() *DB {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.db != nil && time.Since(l.checked) < time.Second {
		return l.db
	}
	l.checked = time.Now()

	fi, err := os.Stat(l.store.dbFile())
	if err != nil {
		// fail closed: without a database nobody can log in
		if !errors.Is(err, fs.ErrNotExist) {
			log.Printf("db: %v", err)
		}
		l.db, l.stat = emptyDB(), nil
		return l.db
	}
	if l.db != nil && l.stat != nil && os.SameFile(fi, l.stat) &&
		fi.ModTime().Equal(l.stat.ModTime()) && fi.Size() == l.stat.Size() {
		return l.db
	}
	db, err := l.store.Load()
	if err != nil {
		// keep serving the last good snapshot
		log.Printf("db: %v", err)
		if l.db == nil {
			l.db = emptyDB()
		}
		return l.db
	}
	l.db, l.stat = db, fi
	return l.db
}
