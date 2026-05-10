# auto-avahi

Automate TLS certificates for Docker containers behind Traefik, with optional mDNS publishing.

When a Docker container starts with Traefik routing labels, auto-avahi will:
1. Extract the hostname from `Host()` rules
2. Validate it against a configured list of allowed suffixes
3. Generate a trusted TLS certificate via mkcert
4. Write a Traefik dynamic config file pointing at the certificate
5. *(Optional, when `MDNS_ENABLE=true`)* Publish the hostname via `avahi-publish` so LAN devices can resolve it

When the container stops (and mDNS is enabled), the mDNS record is removed.

## Two Configuration Profiles

auto-avahi supports two common deployments:

### Profile A — mDNS publishing (default)

For homelabs that use `.local` mDNS for service discovery. avahi-publish advertises each container's hostname to the LAN.

```ini
HOSTNAME_SUFFIXES=local
MDNS_ENABLE=true
AVAHI_HOST_IP=192.168.1.100
```

Hostnames must be 1 or 2 subdomains under `.local` (e.g. `whoami.local`, `app.myserver.local`). This depth limit is intentional — mDNS resolution of deeper names is fragile across client OSes.

### Profile B — DNS handled elsewhere

For setups where DNS for your domain is handled by another resolver (split-horizon DNS, public DNS, `/etc/hosts`, etc.). auto-avahi only manages the TLS cert + Traefik config; no mDNS publishing happens, no `avahi-daemon` is required.

```ini
HOSTNAME_SUFFIXES=example.com
MDNS_ENABLE=false
# AVAHI_HOST_IP not needed
```

In this profile any depth of subdomain is allowed (e.g. `auth.example.com`, `app.team.example.com`).

You can also list multiple suffixes (`HOSTNAME_SUFFIXES=local,example.com`) — auto-avahi will accept hostnames matching any of them.

## Features

- **Docker event monitoring** — watches for container start/stop in real-time
- **Traefik label parsing** — extracts hostnames from `traefik.http.routers.*.rule` labels
- **Configurable hostname suffixes** — `HOSTNAME_SUFFIXES` controls which Host() values are processed
- **Certificate generation** — mkcert certificates created on demand if missing
- **Certificate validation** — verifies certs contain valid PEM data before writing Traefik configs
- **Traefik dynamic config** — generates per-hostname YAML using the container-side cert path
- **Optional mDNS publishing** — `avahi-publish -a -R` when `MDNS_ENABLE=true`, with health-checked recovery from `avahi-daemon` restarts

## Requirements

Always required:
- Go 1.21+ (for building)
- Docker (with the user in the `docker` group for non-root socket access)
- `mkcert` installed and initialized (`mkcert -install`)
- Traefik with the file provider and Docker provider enabled

Required only when `MDNS_ENABLE=true`:
- `avahi-daemon` running
- `avahi-publish` available (from the `avahi-utils` package)

## Quick Start

```bash
# 1. Install dependencies (Debian/Ubuntu)
sudo apt install mkcert
# Plus avahi-utils if you want mDNS publishing:
sudo apt install avahi-utils

# 2. Initialize the mkcert CA
mkcert -install

# 3. Build
go build -o auto-avahi

# 4. Install (will prompt for the configuration profile)
sudo ./install.sh

# 5. Start
sudo systemctl start auto-avahi

# 6. Tail the logs
journalctl -u auto-avahi -f
```

The install script asks whether you want mDNS publishing and which hostname suffix(es) to accept, then writes the appropriate values into the systemd unit.

## Configuration

Set via environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `HOSTNAME_SUFFIXES` | No | `local` | Comma-separated list of allowed hostname suffixes (no leading dot). Hostnames must end with `.<suffix>` to be processed. |
| `MDNS_ENABLE` | No | `true` | Set to `false` to skip mDNS publishing entirely. When false, `AVAHI_HOST_IP` is not required and no `avahi-publish` processes are spawned. |
| `AVAHI_HOST_IP` | When `MDNS_ENABLE=true` | — | Server LAN IP advertised in mDNS records (e.g. `192.168.1.100`). |
| `CERT_DIR` | No | `/certs` | Host directory where mkcert certificates are written. |
| `CERT_DIR_CONTAINER` | No | `/certs` | Path where certs are mounted **inside** the Traefik container. |
| `TRAEFIK_CONFIG_DIR` | No | `/traefik/dynamic` | Host directory for Traefik dynamic config YAML files. |
| `CAROOT` | No | (mkcert default) | Override the mkcert CA root directory (e.g. when the service runs as root but you want a non-root user's CA). |

### Validation depth and `MDNS_ENABLE`

The hostname depth limit (1 or 2 subdomains before the suffix) is enforced **only when `MDNS_ENABLE=true`**, because mDNS resolution of deeper names is unreliable across client OSes. With mDNS disabled, any depth under a configured suffix is accepted — DNS handles whatever shape you give it.

### Path Mapping

auto-avahi writes certs to the **host** filesystem (`CERT_DIR`) but generates Traefik configs referencing the **container** path (`CERT_DIR_CONTAINER`):

```
Host filesystem                    Traefik container
─────────────────                  ─────────────────
$CERT_DIR/whoami.local.pem    →    $CERT_DIR_CONTAINER/whoami.local.pem
(where mkcert writes)              (what Traefik reads)
```

These must match your Traefik volume mount. For example, if your Traefik compose has:
```yaml
volumes:
  - /srv/traefik/certs:/certs:ro
```
Then set `CERT_DIR=/srv/traefik/certs` and `CERT_DIR_CONTAINER=/certs`.

## Installation

### Using the install script

```bash
go build -o auto-avahi
sudo ./install.sh
```

The install script will:
- Ask whether to enable mDNS publishing
- Ask which hostname suffix(es) to accept (default `local`)
- Verify the relevant dependencies are present (avahi-publish only when mDNS is enabled)
- Prompt for the rest of the configuration
- Copy the binary to `/usr/local/bin/`
- Install the systemd service file with your values baked in
- Enable the service (you start it with `systemctl start auto-avahi`)

### Manual installation

1. Build and install the binary:
   ```bash
   go build -o auto-avahi
   sudo cp auto-avahi /usr/local/bin/
   ```

2. Copy and edit the service file:
   ```bash
   sudo cp auto-avahi.service /etc/systemd/system/
   sudo systemctl edit --full auto-avahi   # or edit the file directly
   ```

   Set the `Environment` lines for your profile. Profile A example:
   ```ini
   Environment="MDNS_ENABLE=true"
   Environment="HOSTNAME_SUFFIXES=local"
   Environment="AVAHI_HOST_IP=192.168.1.100"
   Environment="CERT_DIR=/srv/traefik/certs"
   Environment="CERT_DIR_CONTAINER=/certs"
   Environment="TRAEFIK_CONFIG_DIR=/srv/traefik/dynamic"
   ```

   Profile B example:
   ```ini
   Environment="MDNS_ENABLE=false"
   Environment="HOSTNAME_SUFFIXES=example.com"
   Environment="AVAHI_HOST_IP="
   Environment="CERT_DIR=/srv/traefik/certs"
   Environment="CERT_DIR_CONTAINER=/certs"
   Environment="TRAEFIK_CONFIG_DIR=/srv/traefik/dynamic"
   ```

3. Enable and start:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now auto-avahi
   ```

4. View logs:
   ```bash
   journalctl -u auto-avahi -f
   ```

## Traefik Setup

auto-avahi requires Traefik with:
- **File provider** watching a dynamic config directory
- **Docker provider** for label-based service discovery
- A volume mount for TLS certificates

Example Traefik docker-compose:
```yaml
services:
  traefik:
    image: traefik:v3
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - /srv/traefik/certs:/certs:ro           # Certificate files
      - /srv/traefik/dynamic:/dynamic:ro       # Dynamic config files
    command:
      - "--providers.file.directory=/dynamic"
      - "--providers.file.watch=true"          # Pick up new certs automatically
      - "--providers.docker=true"
      - "--providers.docker.exposedbydefault=false"
      - "--entrypoints.websecure.address=:443"
      - "--entrypoints.websecure.http.tls=true"
    ports:
      - "443:443"
```

## Usage

### Profile A example: mDNS-published `.local` service

```bash
docker run -d \
  --name whoami \
  --label "traefik.enable=true" \
  --label "traefik.http.routers.whoami.rule=Host(\`whoami.local\`)" \
  --label "traefik.http.routers.whoami.entrypoints=websecure" \
  --label "traefik.http.routers.whoami.tls=true" \
  traefik/whoami
```

auto-avahi will:
1. Detect the container via Docker events
2. Extract `whoami.local` from the Traefik label
3. Generate `$CERT_DIR/whoami.local.pem` and `$CERT_DIR/whoami.local-key.pem` via mkcert
4. Create `$TRAEFIK_CONFIG_DIR/whoami.local.yml`:
   ```yaml
   tls:
     certificates:
       - certFile: /certs/whoami.local.pem
         keyFile: /certs/whoami.local-key.pem
   ```
5. Publish `whoami.local` via mDNS

Access from any LAN device: `https://whoami.local`

### Profile B example: DNS-resolved `.example.com` service

Same Traefik labels, just with the configured suffix:

```bash
docker run -d \
  --name auth \
  --label "traefik.enable=true" \
  --label "traefik.http.routers.auth.rule=Host(\`auth.example.com\`)" \
  --label "traefik.http.routers.auth.entrypoints=websecure" \
  --label "traefik.http.routers.auth.tls=true" \
  myorg/auth
```

auto-avahi runs through steps 1–4 above (cert + Traefik config) and skips step 5 (mDNS). Resolution of `auth.example.com` is your DNS resolver's job.

> **Note:** For browsers to trust the certificates without warnings, install the mkcert CA on each client device. Copy `$(mkcert -CAROOT)/rootCA.pem` and add it to the client's trust store.

## Troubleshooting

### Traefik shows "Unable to parse certificate" errors

The paths in the YAML config don't match where Traefik sees the certs:
1. Verify `CERT_DIR_CONTAINER` matches the mount point in your Traefik container
2. Run `docker inspect traefik` to check volume mounts
3. The generated YAML files should use **container** paths, not host paths

### Hostname not matching

Look for the warning line in the journal:
```
Container <id>: WARNING: hostname "<host>" does not end with any configured suffix (...)
```

Check that the hostname's suffix is listed in `HOSTNAME_SUFFIXES`. Suffixes are matched literally — `example.com` will not match `notexample.com` (the suffix must be preceded by a dot).

### Hostname not resolving via mDNS *(Profile A only)*

1. Check if avahi-publish is running:
   ```bash
   ps aux | grep avahi-publish
   ```
2. Test resolution:
   ```bash
   avahi-resolve -n whoami.local
   ```
3. Verify avahi-daemon is active:
   ```bash
   systemctl status avahi-daemon
   ```

### Multi-level `.local` hostnames not resolving *(Profile A only)*

Both 2-level (`myserver.local`) and 3-level (`app.myserver.local`) hostnames are supported. For 3-level names to resolve via mDNS, clients need `libnss-mdns` configured to allow multi-label `.local` names. Create `/etc/mdns.allow` on each client with:
```
.local
.local.
```
- Valid: `myserver.local`, `app.myserver.local`
- Invalid: `a.b.c.local` (4+ levels will log a warning)

### Permission denied on Docker socket

Add your user to the docker group:
```bash
sudo usermod -aG docker $USER
newgrp docker
```

## How It Works

### Lifecycle

- **Container start** → extract hostname → validate suffix → generate cert → write Traefik config → *(if mDNS enabled)* publish mDNS
- **Container stop** → *(if mDNS enabled)* kill the `avahi-publish` process for that hostname
- **auto-avahi shutdown** → *(if mDNS enabled)* kill all `avahi-publish` child processes

### Startup Sync

On startup, auto-avahi scans all currently running containers for Traefik labels and processes them. This ensures hostnames are configured even if auto-avahi starts after the containers.

### mDNS Publishing *(Profile A only)*

Each hostname gets its own `avahi-publish -a -R <hostname> <ip>` child process. The `-R` flag publishes only the forward A record without claiming the reverse PTR mapping, avoiding conflicts with the host's own mDNS name.

A health-check loop runs every 30 seconds and refreshes all registrations if `avahi-resolve` can't see them — this recovers automatically from `avahi-daemon` restarts that would otherwise leave stale entries.

## License

MIT
