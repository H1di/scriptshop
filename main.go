// scriptshop is a private script catalog. Users log in with credentials
// issued by the owner and download the scripts they were granted.
//
// One binary does both jobs:
//
//	scriptshop serve   runs the website (read-only access to the data)
//	scriptshop user …  admin commands that edit the data directory
//
// Files:
//
//	main.go       command dispatch
//	store.go      data model and the data directory
//	server.go     HTTP server setup, routes, security headers
//	handlers.go   HTTP handlers
//	password.go   argon2id hashing
//	session.go    in-memory sessions
//	limiter.go    login throttling
//	admin.go      admin commands
//	web/          HTML templates and CSS, embedded into the binary
package main

import (
	"fmt"
	"os"
)

const usage = `usage:
  scriptshop serve  [-addr host:port] [-name NAME] [-trust-proxy] [-cert FILE -key FILE]

  scriptshop user   add|passwd|disable|enable|del <login>
  scriptshop user   list

  scriptshop script add  [-name NAME] [-version V] [-price P] [-desc TEXT] <file>
  scriptshop script edit <id> [-name NAME] [-version V] [-price P] [-desc TEXT] [-file FILE]
  scriptshop script del  <id>
  scriptshop script list

  scriptshop grant  <login> <id>...|all
  scriptshop revoke <login> <id>...|all

data directory: $SCRIPTSHOP_DATA (default ./data)
`

func main() {
	if len(os.Args) < 2 {
		usageExit()
	}
	if os.Geteuid() == 0 {
		fatal("refusing to run as root; use: sudo -u scriptshop scriptshop ...")
	}
	st := Store{dir: os.Getenv("SCRIPTSHOP_DATA")}
	if st.dir == "" {
		st.dir = "data"
	}

	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "serve":
		serve(st, args)
	case "user":
		cmdUser(st, args)
	case "script":
		cmdScript(st, args)
	case "grant":
		cmdGrant(st, args, true)
	case "revoke":
		cmdGrant(st, args, false)
	default:
		usageExit()
	}
}

func usageExit() {
	fmt.Fprint(os.Stderr, usage)
	os.Exit(2)
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "scriptshop: "+format+"\n", a...)
	os.Exit(1)
}
