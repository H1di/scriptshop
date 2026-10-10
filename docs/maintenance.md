# Maintenance

Keeping the site healthy, backed up and up to date. All commands run on the server.

## Routine

| How often | Task | Section |
|-----------|------|---------|
| Weekly | Check the site runs and skim the logs | [Check the site](#check-the-site), [Read the logs](#read-the-logs) |
| Weekly, and before any change | Back up | [Back up](#back-up) |
| Monthly | Install system updates | [Update the server](#update-the-server) |
| Monthly | Review users, disable unused logins | [Users](users.md#manage-existing-users) |
| When a new version exists | Update the program | [Update the program](#update-the-program) |

## Check the site

```sh
systemctl status scriptshop      # "active (running)"
systemctl status caddy           # "active (running)"
```

Then open `https://shop.example.com` and log in with your own login.

Both services start by themselves after a reboot, and `scriptshop` restarts by itself if it crashes.

## Read the logs

```sh
journalctl -u scriptshop --since today        # today's events
journalctl -u scriptshop -f                   # live; Ctrl+C to stop
journalctl -u scriptshop | grep 'login failed' | tail -20
```

What the lines mean:

| Line | Meaning |
|------|---------|
| `login ok client=1.2.3.4 login="ivan"` | Successful login |
| `login failed client=1.2.3.4 login="ivan"` | Wrong login or password |
| `login throttled client=1.2.3.4 ...` | Too many failures; that address is blocked for a while |
| `download login="ivan" script=001` | A file was downloaded |
| `warning: /var/lib/scriptshop is readable by other users` | Run `sudo chmod 700 /var/lib/scriptshop` |

Passwords are never written to the logs.

Many `login failed` lines for one login from many addresses mean someone is guessing. The site already blocks them; if it goes on, issue that user a new password (`shop user passwd <login>`).

## Back up

Everything lives in one folder, `/var/lib/scriptshop`: the user list, the catalog, and the script files. Back it up and you can rebuild the site anywhere.

Do not run `shop` commands while the backup runs.

**1. Pack the folder:**

```sh
sudo tar -C /var/lib -czf ~/scriptshop-$(date +%F).tar.gz scriptshop
sudo chown $USER ~/scriptshop-*.tar.gz
chmod 600 ~/scriptshop-*.tar.gz
```

**2. Copy it off the server.** From your own computer:

```sh
scp you@shop.example.com:'scriptshop-*.tar.gz' .
```

**3. Delete it from the server** once copied:

```sh
rm ~/scriptshop-*.tar.gz
```

Keep backups as private as the server itself: they contain the scripts and the password hashes.

## Restore from a backup

**1. Copy the backup to the server:**

```sh
scp scriptshop-2026-10-10.tar.gz you@shop.example.com:/tmp/
```

**2. On the server, replace the data folder:**

```sh
sudo systemctl stop scriptshop
sudo rm -rf /var/lib/scriptshop
sudo tar -C /var/lib -xzf /tmp/scriptshop-2026-10-10.tar.gz
sudo chown -R scriptshop:scriptshop /var/lib/scriptshop
sudo chmod 700 /var/lib/scriptshop
sudo systemctl start scriptshop
rm /tmp/scriptshop-2026-10-10.tar.gz
```

**3. Check:** `shop user list` and `shop script list` show the restored data.

## Update the program

**1. Build the new version** on the computer with Go (as in [Install, step 2](install.md#step-2-build-the-program)):

```sh
cd scriptshop
git pull
go test ./...
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o scriptshop .
scp scriptshop you@shop.example.com:/tmp/
```

**2. On the server, [back up](#back-up) first.**

**3. Keep the current version, then install the new one:**

```sh
sudo cp /usr/local/bin/scriptshop /usr/local/bin/scriptshop.prev
sudo install -m 755 /tmp/scriptshop /usr/local/bin/scriptshop
sudo systemctl restart scriptshop
```

Restarting logs everyone out; they simply log in again.

**4. Check** the site as in [Check the site](#check-the-site).

**If something is wrong, go back:**

```sh
sudo install -m 755 /usr/local/bin/scriptshop.prev /usr/local/bin/scriptshop
sudo systemctl restart scriptshop
```

If the update changed `deploy/scriptshop.service`, also copy it to `/etc/systemd/system/` and run `sudo systemctl daemon-reload` before restarting.

## Update the server

```sh
sudo apt update
sudo apt upgrade
```

If it says a reboot is needed: `sudo reboot`. The site comes back by itself.

To install security updates automatically:

```sh
sudo apt install unattended-upgrades
sudo dpkg-reconfigure -plow unattended-upgrades     # answer "Yes"
```

HTTPS certificates are renewed by Caddy automatically. Ports 80 and 443 must stay open for that.

## Security routine

- **Log in to the server with an SSH key, not a password.** Once key login works, set `PasswordAuthentication no` in `/etc/ssh/sshd_config` and run `sudo systemctl restart ssh`. Test from a second terminal before closing the first.
- **Only you use the server account.** Anyone with `sudo` can read all scripts and change all users.
- **Keep `/var/lib/scriptshop` at mode 700.** The site warns in the log at start if it is not.
- **Never open port 8080** in the firewall. The program must only be reached through Caddy.

## If the server may be compromised

1. Take the site offline: `sudo systemctl stop caddy`.
2. Copy `/var/lib/scriptshop` off the server for investigation.
3. Set up a clean server ([Install](install.md)) and [restore](#restore-from-a-backup) a backup from before the incident.
4. Issue every user a new password: `shop user passwd <login>` for each login in `shop user list`, and hand them out again.
5. Treat the scripts themselves as leaked.

## Move to a new server

1. [Install](install.md) on the new server (steps 2 to 8).
2. [Back up](#back-up) the old server and [restore](#restore-from-a-backup) on the new one.
3. Point the domain's A record at the new server's IP.
4. Once the new site works, stop the old one: `sudo systemctl disable --now scriptshop caddy`.
