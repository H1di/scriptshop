# Troubleshooting

Find the symptom, check the cause, apply the fix. Commands run on the server.

## The site does not open

| Symptom | Cause | Fix |
|---------|-------|-----|
| Browser waits, then "can't reach this page" | Domain points elsewhere, firewall or router blocks 443 | `ping shop.example.com` must show the server IP; `sudo ufw status` must list 443; check router port forwarding |
| `502 Bad Gateway` | The program is not running | `systemctl status scriptshop`; see why: `journalctl -u scriptshop -n 50`; start: `sudo systemctl start scriptshop` |
| Certificate warning | Caddy could not get a certificate | Ports 80 and 443 must be open and the domain must point at the server; see `journalctl -u caddy -n 50` |

## Logging in

| Symptom | Cause | Fix |
|---------|-------|-----|
| `Login incorrect` | Mistyped login or password, or the user is disabled | Check `shop user list`; if the password is lost, `shop user passwd <login>` |
| `Too many attempts. Try again later.` | 5 wrong passwords from one address (blocked 1 to 64 min), or 10 on one login (blocked 15 min) | Wait. `sudo systemctl restart scriptshop` clears all blocks but logs everyone out |
| Correct password, but the login page comes back | Site opened over `http://` or by IP address | Open `https://shop.example.com` |
| `403` or "cross-origin request" when logging in | The proxy changes the request's Host | Use `deploy/Caddyfile` unchanged except the domain |
| Logged out unexpectedly | 30 min idle, 12 h since login, browser closed, password changed, server restarted | Log in again; this is expected |

## Admin commands

| Symptom | Cause | Fix |
|---------|-------|-----|
| `shop: command not found` | Shortcut not loaded | `source ~/.bashrc`, or set it up: [Install, step 8](install.md#step-8-set-up-the-shop-shortcut) |
| `refusing to run as root` | Command run with plain `sudo` or as root | Use `shop` |
| Changes do not show on the site | Command ran on a different data folder (`scriptshop` without the shortcut writes to `./data`) | Always use `shop`; then reload the page |
| `permission denied` when adding a script | The `scriptshop` user cannot read the file | Put it in `/tmp` and `chmod 644` it: [Scripts](scripts.md#put-a-new-script-on-the-site) |
| `login exists` | That login is taken | Choose another, or `shop user passwd` for the existing one |
| `invalid login` | Uppercase, spaces or other characters | 2 to 32 of `a-z 0-9 . _ -` |
| `no such login` / `no such script` | Typo | Check `shop user list` / `shop script list` |
| `invalid character ...` or `unexpected end of JSON` | `/var/lib/scriptshop/db.json` is damaged | [Restore a backup](maintenance.md#restore-from-a-backup). The site keeps working on the last good copy until it restarts |

## Downloads

| Symptom | Cause | Fix |
|---------|-------|-----|
| Client sees a price instead of `[download]` | Script not granted | `shop grant <login> <id>` |
| `[download]` gives `404 page not found` | The script's file is missing on the server | Upload it again: `shop script edit <id> -file /tmp/file`, or restore a backup. The log shows `download 001: open ...` |
