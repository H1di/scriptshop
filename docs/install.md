# Install

From an empty Linux server to a working site at `https://your-domain`. Takes about 30 minutes.

## What you need

- A Linux server with systemd (Debian or Ubuntu are assumed below) and `sudo` access over SSH.
- A domain name, e.g. `shop.example.com`.
- A computer with [Go 1.26 or newer](https://go.dev/dl/) to build the program (Linux or macOS). It can be the server itself.
- Access to the repository: `https://github.com/H1di/scriptshop`.

## Step 1. Point the domain at the server

At your domain registrar, create an **A record** for `shop.example.com` with the server's public IP address.

If the server is at home or in an office behind a router, forward ports **80** and **443** on the router to the server.

Check (from any computer; it may take up to an hour to work):

```sh
ping shop.example.com        # must show the server's IP
```

## Step 2. Build the program

On the computer with Go:

```sh
git clone https://github.com/H1di/scriptshop.git
cd scriptshop
go test ./...                # must print "ok"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o scriptshop .
```

For an ARM server (e.g. Raspberry Pi) use `GOARCH=arm64`. Not sure? Run `uname -m` on the server: `x86_64` → `amd64`, `aarch64` → `arm64`.

## Step 3. Copy the files to the server

```sh
scp scriptshop deploy/scriptshop.service deploy/Caddyfile you@shop.example.com:/tmp/
```

All following steps run **on the server**:

```sh
ssh you@shop.example.com
```

## Step 4. Create the service user and data folder

The site runs as its own user, `scriptshop`, which can read nothing else on the machine.

```sh
sudo useradd --system --home /var/lib/scriptshop --shell /usr/sbin/nologin scriptshop
sudo install -d -m 700 -o scriptshop -g scriptshop /var/lib/scriptshop
```

## Step 5. Install and start the program

```sh
sudo install -m 755 /tmp/scriptshop /usr/local/bin/scriptshop
sudo install -m 644 /tmp/scriptshop.service /etc/systemd/system/scriptshop.service
sudo systemctl daemon-reload
sudo systemctl enable --now scriptshop
```

Check:

```sh
systemctl status scriptshop          # must say "active (running)"
curl -sI http://127.0.0.1:8080/login | head -1     # must say "HTTP/1.1 200 OK"
```

The program listens only on `127.0.0.1`, so it is not reachable from outside yet. That is the next step.

## Step 6. Put Caddy in front (HTTPS)

Caddy receives visitors, gets a free TLS certificate automatically and passes requests to the program.

1. Install Caddy: follow [caddyserver.com/docs/install](https://caddyserver.com/docs/install#debian-ubuntu-raspbian).
2. Set your domain in the config and install it:

```sh
sed -i 's/example.com/shop.example.com/' /tmp/Caddyfile
sudo install -m 644 /tmp/Caddyfile /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

3. Open `https://shop.example.com` in a browser. You should see the login page.

The site works **only over HTTPS**: over plain HTTP the browser refuses the login cookie. This is intentional.

## Step 7. Turn on the firewall

Allow only SSH and web traffic. Allow SSH **first**, or you will lock yourself out.

```sh
sudo apt install ufw
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
sudo ufw status              # must list 22, 80, 443
```

## Step 8. Set up the `shop` shortcut

All admin commands must run as the `scriptshop` user. This shortcut does that for you.

Add this line to `~/.bashrc` on the server:

```sh
alias shop='sudo -u scriptshop SCRIPTSHOP_DATA=/var/lib/scriptshop scriptshop'
```

Then:

```sh
source ~/.bashrc
shop user list               # prints an empty table: LOGIN STATUS CREATED GRANTS
```

## Step 9. Check everything end to end

Create a login for yourself and look at the site as a client would:

```sh
shop user add owner          # copy the password it prints
```

Log in at `https://shop.example.com` with `owner` and that password. You should see `No scripts.`

Done. Next: [put scripts on the site](scripts.md).

## Optional: change the site name

The name at the top of every page is `SCRIPTS`. To change it, edit the service:

```sh
sudo nano /etc/systemd/system/scriptshop.service
```

Add `-name "YOUR NAME"` to the end of the `ExecStart=` line, then:

```sh
sudo systemctl daemon-reload
sudo systemctl restart scriptshop
```

Restarting logs everyone out; they simply log in again.
