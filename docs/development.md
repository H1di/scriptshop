# Development

How the code is organized and how to change it without weakening it.

## Rules that keep the project simple and safe

Every change should keep these true. If one must go, discuss it first.

1. **The server only reads the data. Only the admin CLI writes it.** No web route ever changes users, scripts or grants.
2. **No JavaScript, no external resources.** The Content-Security-Policy header blocks them; do not loosen it.
3. **One dependency** (`golang.org/x/crypto`, for argon2). Prefer the standard library.
4. **`db.json` stays readable by older and newer versions.** New fields get `omitempty`; never rename or repurpose a field.
5. **Fail closed.** When something is missing or broken (database, file, session), deny access.

## Code map

| File | Contents |
|------|----------|
| `main.go` | Command dispatch and usage text |
| `store.go` | Data model (`DB`, `User`, `Script`, `Blob`), the data folder, atomic writes, live reload for the server |
| `server.go` | Server setup, routes, security headers, client IP |
| `handlers.go` | One function per page: login, logout, catalog, download |
| `password.go` | argon2id hashing and checking |
| `session.go` | In-memory sessions |
| `limiter.go` | Login throttling |
| `admin.go` | `user`, `script`, `grant`, `revoke` commands |
| `server_test.go` | End-to-end tests of the security behavior |
| `web/` | `login.html`, `catalog.html`, `style.css`, embedded into the binary |
| `deploy/` | systemd unit and Caddy config |

## How a request flows

```
browser → Caddy (HTTPS) → securityHeaders → CrossOriginProtection → router → handler
                                                                               │
                                                     s.account(r): cookie → session → user in current DB snapshot
```

The DB snapshot comes from `liveDB`, which re-reads `db.json` at most once per second when it has changed. A snapshot is never modified, so handlers need no locks.

## Run it locally

**1. Build and create test data:**

```sh
go build -o scriptshop .
SCRIPTSHOP_DATA=./data ./scriptshop user add test        # note the password
SCRIPTSHOP_DATA=./data ./scriptshop script add -price '$10' -desc 'Test.' ./main.go
SCRIPTSHOP_DATA=./data ./scriptshop grant test all
```

**2. Start the server:**

```sh
SCRIPTSHOP_DATA=./data ./scriptshop serve
```

**3. Open `http://localhost:8080`** (exactly `localhost`: browsers accept the secure cookie there without HTTPS). Stop with Ctrl+C.

`data/` and the `scriptshop` binary are in `.gitignore`. Never commit them.

## Before every commit

```sh
gofmt -l .          # must print nothing
go vet ./...        # must print nothing
go test ./...       # must print "ok"
```

The tests cover: login required, security headers, wrong and right password, cookie flags, download only of granted scripts, cross-origin POST rejected, logout and disable end sessions, throttling, password hashing.

If you change any security behavior, change or add the test for it in `server_test.go` in the same commit.

## Common changes, step by step

### Change the look

1. Edit `web/style.css` and the templates in `web/`.
2. Keep all styles in `style.css`: the CSP blocks `style="..."` attributes and `<style>` tags.
3. Rebuild, run locally, check the login page, the catalog, and a phone-width window.

### Add a field to scripts (example: `-author`)

1. `store.go`: add `Author string \`json:"author,omitempty"\`` to `Script`.
2. `admin.go`: in `scriptFlags` add `fs.String("author", "", "author")`; in `applyFlags` add `case "author": s.Author = v`.
3. `web/catalog.html`: show `{{.Author}}` where you want it.
4. `main.go`: add `-author` to the usage text. Update [Scripts](scripts.md).
5. Run the checks above.

Existing `db.json` files keep working: scripts without the field show it empty.

### Change limits

| What | Where |
|------|-------|
| Session idle and maximum time, sessions per user | constants in `session.go` |
| Login blocking thresholds and durations | constants in `limiter.go` |
| Password hashing cost | constants in `password.go` (affects new hashes only; old ones keep working) |
| Maximum script size | `maxScriptSize` in `store.go` |

Update the numbers in the docs ([Users](users.md), [Troubleshooting](troubleshooting.md)) when you change them.

## Security notes for reviewers

- Templates use `html/template`, which escapes everything. Never pass user data through `template.HTML`.
- Never log passwords or session tokens.
- `download` checks three things: the script exists, the user owns it, the stored file name is valid. Keep all three.
- Unknown logins are checked against a dummy hash so response time does not reveal which logins exist. Keep that path when touching `login`.

## Release

1. Commit and push to `main`.
2. Deploy as in [Maintenance → Update the program](maintenance.md#update-the-program).
