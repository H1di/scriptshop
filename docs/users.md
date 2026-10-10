# Users

You create every user yourself, on the server, with one command. The website has no sign-up form and no admin page, so nobody gets in without a login from you.

All commands use the [`shop` shortcut](install.md#step-8-set-up-the-shop-shortcut).

## Create a user

**1. Pick a login.** 2 to 32 characters: lowercase letters, digits and `. _ -`, starting with a letter or digit. Examples: `ivan`, `client-042`.

**2. Create it:**

```sh
shop user add ivan
```

```
login:    ivan
password: EXAMPLEPASSWORDXXXXXXXXXXX
```

**3. Copy the password now.** It is shown only this once. The server keeps only a scrambled form (a hash) and cannot show it again. If it is lost, [issue a new one](#manage-existing-users).

**4. Give access** to the scripts the client paid for ([next section](#give-access-to-scripts)). Without it they can log in and see prices, but download nothing.

**5. Hand the login and password to the client** in person (see [rules](#rules-for-handing-out-logins)).

You cannot choose a password yourself: every password is random, 26 characters long. This keeps all accounts equally hard to guess.

## Give access to scripts

Find the script IDs with `shop script list`, then:

```sh
shop grant ivan 1 3       # scripts 001 and 003
shop grant ivan all       # every script in the catalog right now
shop revoke ivan 3        # take script 003 away
```

Each command prints what the user owns afterwards, e.g. `ivan: 001 003`. The site picks up the change within a second; the user just reloads the page.

`grant ivan all` covers only scripts that exist today. A script added later must be granted separately.

## Manage existing users

Every change takes effect within a second, even for a user who is logged in at that moment.

| Task | Command | What happens |
|------|---------|--------------|
| See all users | `shop user list` | Login, status (active / disabled), creation date, owned script IDs |
| Issue a new password | `shop user passwd ivan` | New random password printed once; the old one stops working and the user is logged out |
| Block access | `shop user disable ivan` | User is logged out and cannot log in; grants are kept |
| Restore access | `shop user enable ivan` | User can log in again with the same password and grants |
| Remove for good | `shop user del ivan` | User and grants are deleted; the login can be created again later |

Prefer `disable` over `del` when a client may come back: it keeps their purchases on record.

## How long a client stays logged in

- 30 minutes without activity, or 12 hours after login at most.
- Until the browser is closed.
- Until the server restarts, the password changes, or the user is disabled.

After that, they log in again. Nothing is lost.

## Rules for handing out logins

The login and password are the only thing between a stranger and your scripts, so treat them like keys.

- One login per client. Never share one login between people: you could not cut off one of them alone.
- Hand credentials over in person or through a channel only the client can read. Do not post them in group chats or plain email.
- Do not keep passwords in notes or files. If a client loses theirs, issue a new one with `shop user passwd`.
- Disable a client the day they stop paying or you suspect the login leaked. If it leaked, run `passwd` as well.
- Review `shop user list` from time to time and disable logins nobody uses.

Problems logging in: see [Troubleshooting](troubleshooting.md#logging-in).
