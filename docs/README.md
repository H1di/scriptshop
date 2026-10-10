# scriptshop documentation

Read these in order the first time. Afterwards, jump to the page you need.

| # | Page | For | What it covers |
|---|------|-----|----------------|
| 1 | [Install](install.md) | whoever sets up the server | From an empty server to a working site over HTTPS |
| 2 | [Scripts](scripts.md) | the shop owner | Put scripts on the site, change prices, replace files, remove scripts |
| 3 | [Users](users.md) | the shop owner | Create logins, give access to scripts, block and remove users |
| 4 | [Maintenance](maintenance.md) | whoever looks after the server | Health checks, logs, backups, updates, security routine |
| 5 | [Troubleshooting](troubleshooting.md) | everyone | Symptom → cause → fix |
| 6 | [Development](development.md) | developers | How the code is organized and how to change it safely |

## The idea in one minute

- The website shows a login page, then a catalog of scripts. Nothing else.
- There is no sign-up and no admin page. Everything is managed with commands on the server.
- The owner creates a login for each client, hands it over in person, and grants the scripts the client paid for.
- The client sees the whole catalog with prices. Granted scripts show `[download]` instead of a price.

## Words used in these pages

| Word | Meaning |
|------|---------|
| server | The physical machine the site runs on |
| `shop` | A shortcut for the admin command, set up in [Install](install.md#step-8-set-up-the-shop-shortcut) |
| login | A user name on the site, e.g. `ivan` |
| script ID | The number of a script in the catalog: `001`, `002` … |
| grant | Permission for one user to download one script |
