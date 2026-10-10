# scriptshop: Managing Users

You create every user yourself, on the server, with one command: `shop user add <login>`. The website has no sign-up form and no admin page, so nobody can get in without a login from you.

## Before you start

All user commands run on the server, as the `scriptshop` service user. Set this up once.

1. Connect to the server: `ssh you@your-server`
2. Add a shortcut to your shell config (`~/.bashrc`, or `~/.config/fish/config.fish` for fish):

```sh
alias shop='sudo -u scriptshop SCRIPTSHOP_DATA=/var/lib/scriptshop scriptshop'
```

3. Open a new terminal and check it works: `shop user list` prints a table of users, empty at first.

Always use `shop`. Running `scriptshop` directly as root is refused on purpose: files created by root would be unreadable for the website.

## Create a user

A new user takes two commands, add and grant; the password is generated for you and shown only once.

1. Pick a login: 2 to 32 characters, lowercase letters, digits and `. _ -`, starting with a letter or digit. Example: `ivan`, `client-042`.
2. Create the user:

```sh
shop user add ivan
```

The output looks like this:

```
login:    ivan
password: EXAMPLEPASSWORDXXXXXXXXXXX
```

3. Copy the password now. It is stored only as a hash and cannot be shown again; if it is lost, issue a new one (see Manage existing users).
4. Give the user access to the scripts they paid for (next section). Without it they can log in and see prices, but download nothing.
5. Hand the login and password to the client in person (see Rules for handing out logins).

You cannot choose a password yourself: every password is random, 26 characters long. This keeps all accounts equally hard to guess.

## Give access to scripts

Users see the whole catalog with prices, but can download only the scripts you grant. Granted scripts show `[download]` instead of the price.

Find script IDs with `shop script list`, then:

```sh
shop grant ivan 1 3       # scripts 001 and 003
shop grant ivan all       # every script in the catalog right now
shop revoke ivan 3        # take script 003 away
```

Each command prints what the user owns afterwards, e.g. `ivan: 001 003`. Changes reach the website within a second; the user just reloads the page.

`grant ivan all` covers only scripts that exist today. A script added later must be granted separately.

## Manage existing users

Every change takes effect within a second, even for a user who is logged in at that moment.

| Task | Command | What happens |
| --- | --- | --- |
| See all users | `shop user list` | Table of login, status (active / disabled), creation date, owned script IDs |
| Issue a new password | `shop user passwd ivan` | New random password printed once; the old one stops working and the user is logged out |
| Block access | `shop user disable ivan` | User is logged out and cannot log in; grants are kept |
| Restore access | `shop user enable ivan` | User can log in again with the same password and grants |
| Remove for good | `shop user del ivan` | User and grants are deleted; the login can be created again later |

Prefer `disable` over `del` when a client may come back: it keeps their purchases on record.

## Troubleshooting

| Symptom | Cause | Fix |
| --- | --- | --- |
| Client sees `Login incorrect` | Mistyped login or password, or the user is disabled | Check `shop user list`; if the password is lost, run `shop user passwd` |
| Client sees `Too many attempts. Try again later.` | 5 wrong passwords from one address (blocked 1 to 64 min), or 10 on one login (blocked 15 min) | Wait it out. `sudo systemctl restart scriptshop` clears all blocks, but logs every user out |
| Client is logged out unexpectedly | 30 min without activity, 12 h since login, browser closed, password changed, or server restarted | Log in again; this is expected |
| Client sees a price instead of `[download]` | Script not granted | `shop grant <login> <id>` |
| `refusing to run as root` | Command run with plain `sudo` or as root | Use the `shop` shortcut |
| `login exists` | That login is taken | Choose another, or `shop user passwd` for the existing one |
| `invalid login` | Uppercase, spaces or other characters | 2 to 32 of `a-z 0-9 . _ -` |
| `no such login` | Typo | Check `shop user list` |

## Rules for handing out logins

The login and password are the only thing between a stranger and your scripts, so treat them like keys.

- One login per client. Never share one login between people: you could not cut off one of them alone.
- Hand credentials over in person or through a channel only the client can read. Do not post them in group chats or plain email.
- Do not keep passwords in notes or files. If a client loses theirs, issue a new one with `shop user passwd`.
- Disable a client the day they stop paying or you suspect the login leaked. If it leaked, run `passwd` as well.
- Review `shop user list` from time to time and disable logins nobody uses.
