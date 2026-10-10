package main

import (
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

// --- user ---

func cmdUser(st Store, args []string) {
	if len(args) == 1 && args[0] == "list" {
		userList(st)
		return
	}
	if len(args) != 2 {
		usageExit()
	}
	cmd, login := args[0], args[1]
	if !validLogin(login) {
		fatal("invalid login: 2-32 characters of a-z 0-9 . _ -, starting with a letter or digit")
	}

	var password string
	err := st.Update(func(db *DB) error {
		u := db.Users[login]
		if cmd == "add" {
			if u != nil {
				return fmt.Errorf("login exists: %s", login)
			}
			password = newPassword()
			db.Users[login] = &User{Hash: hashPassword(password), Gen: newGen(), Created: time.Now().UTC()}
			return nil
		}
		if u == nil {
			return fmt.Errorf("no such login: %s", login)
		}
		switch cmd {
		case "passwd":
			password = newPassword()
			u.Hash, u.Gen = hashPassword(password), newGen()
		case "disable":
			u.Disabled, u.Gen = true, newGen()
		case "enable":
			u.Disabled = false
		case "del":
			delete(db.Users, login)
		default:
			usageExit()
		}
		return nil
	})
	if err != nil {
		fatal("%v", err)
	}

	if password != "" {
		fmt.Printf("login:    %s\npassword: %s\n", login, password)
	} else {
		fmt.Printf("%s: %s\n", login, cmd)
	}
}

func userList(st Store) {
	db := mustLoad(st)
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer tw.Flush()
	fmt.Fprintln(tw, "LOGIN\tSTATUS\tCREATED\tGRANTS")
	for _, login := range slices.Sorted(maps.Keys(db.Users)) {
		u := db.Users[login]
		status := "active"
		if u.Disabled {
			status = "disabled"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", login, status, u.Created.Format(time.DateOnly), formatIDs(u.Grants))
	}
}

// --- script ---

func cmdScript(st Store, args []string) {
	if len(args) == 0 {
		usageExit()
	}
	switch cmd, args := args[0], args[1:]; cmd {
	case "list":
		scriptList(st)
	case "add":
		scriptAdd(st, args)
	case "edit":
		scriptEdit(st, args)
	case "del":
		scriptDel(st, args)
	default:
		usageExit()
	}
}

func scriptList(st Store) {
	db := mustLoad(st)
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer tw.Flush()
	fmt.Fprintln(tw, "ID\tNAME\tVERSION\tPRICE\tSIZE\tSHA256")
	for _, s := range db.Scripts {
		fmt.Fprintf(tw, "%03d\t%s\t%s\t%s\t%s\t%.16s\n", s.ID, s.Name, s.Version, s.Price, humanSize(s.Size), s.SHA256)
	}
}

func scriptAdd(st Store, args []string) {
	fs := scriptFlags("script add")
	fs.Parse(args)
	if fs.NArg() != 1 {
		usageExit()
	}
	src := fs.Arg(0)

	blob, err := st.Import(src)
	if err != nil {
		fatal("%v", err)
	}
	s := &Script{Name: filepath.Base(src), Blob: blob, Added: time.Now().UTC()}
	applyFlags(fs, s)
	err = st.Update(func(db *DB) error {
		db.NextID++
		s.ID = db.NextID
		db.Scripts = append(db.Scripts, s)
		return nil
	})
	if err != nil {
		st.Remove(blob.File)
		fatal("%v", err)
	}
	fmt.Printf("added %03d %s\n", s.ID, s.Name)
}

func scriptEdit(st Store, args []string) {
	if len(args) == 0 {
		usageExit()
	}
	id := parseID(args[0])
	fs := scriptFlags("script edit")
	src := fs.String("file", "", "replace the file")
	fs.Parse(args[1:])

	var blob Blob
	if *src != "" {
		var err error
		if blob, err = st.Import(*src); err != nil {
			fatal("%v", err)
		}
	}
	var replaced string
	err := st.Update(func(db *DB) error {
		s := db.script(id)
		if s == nil {
			return fmt.Errorf("no such script: %03d", id)
		}
		applyFlags(fs, s)
		if blob.File != "" {
			replaced, s.Blob = s.File, blob
		}
		return nil
	})
	if err != nil {
		st.Remove(blob.File)
		fatal("%v", err)
	}
	st.Remove(replaced)
	fmt.Printf("updated %03d\n", id)
}

func scriptDel(st Store, args []string) {
	if len(args) != 1 {
		usageExit()
	}
	id := parseID(args[0])
	var file string
	err := st.Update(func(db *DB) error {
		i := slices.IndexFunc(db.Scripts, func(s *Script) bool { return s.ID == id })
		if i < 0 {
			return fmt.Errorf("no such script: %03d", id)
		}
		file = db.Scripts[i].File
		db.Scripts = slices.Delete(db.Scripts, i, i+1)
		for _, u := range db.Users {
			u.Grants = slices.DeleteFunc(u.Grants, func(g int) bool { return g == id })
		}
		return nil
	})
	if err != nil {
		fatal("%v", err)
	}
	st.Remove(file)
	fmt.Printf("deleted %03d\n", id)
}

// scriptFlags declares the editable text fields of a script.
func scriptFlags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.String("name", "", "display name (default: file name)")
	fs.String("version", "", "version")
	fs.String("price", "", "price, free text")
	fs.String("desc", "", "one-line description")
	return fs
}

// applyFlags copies the fields that were given on the command line into s.
func applyFlags(fs *flag.FlagSet, s *Script) {
	fs.Visit(func(f *flag.Flag) {
		v := f.Value.String()
		switch f.Name {
		case "name":
			s.Name = v
		case "version":
			s.Version = v
		case "price":
			s.Price = v
		case "desc":
			s.Desc = v
		}
	})
}

// --- grant / revoke ---

func cmdGrant(st Store, args []string, grant bool) {
	if len(args) < 2 {
		usageExit()
	}
	login, all := args[0], len(args) == 2 && args[1] == "all"
	var ids []int
	if !all {
		for _, a := range args[1:] {
			ids = append(ids, parseID(a))
		}
	}

	var grants []int
	err := st.Update(func(db *DB) error {
		u := db.Users[login]
		if u == nil {
			return fmt.Errorf("no such login: %s", login)
		}
		if all {
			for _, s := range db.Scripts {
				ids = append(ids, s.ID)
			}
		}
		for _, id := range ids {
			if db.script(id) == nil {
				return fmt.Errorf("no such script: %03d", id)
			}
		}
		if grant {
			u.Grants = slices.Compact(slices.Sorted(slices.Values(append(u.Grants, ids...))))
		} else {
			u.Grants = slices.DeleteFunc(u.Grants, func(g int) bool { return slices.Contains(ids, g) })
		}
		grants = u.Grants
		return nil
	})
	if err != nil {
		fatal("%v", err)
	}
	fmt.Printf("%s: %s\n", login, formatIDs(grants))
}

// --- helpers ---

func mustLoad(st Store) *DB {
	db, err := st.Load()
	if err != nil {
		fatal("%v", err)
	}
	return db
}

func parseID(s string) int {
	id, err := strconv.Atoi(s)
	if err != nil || id <= 0 {
		fatal("invalid script id: %s", s)
	}
	return id
}

func formatIDs(ids []int) string {
	if len(ids) == 0 {
		return "-"
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%03d", id)
	}
	return strings.Join(parts, " ")
}
