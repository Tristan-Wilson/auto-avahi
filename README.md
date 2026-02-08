# auto-avahi

Automatically publishes Docker containers via mDNS (Avahi) and manages TLS certificates for Traefik.

When a Docker container starts with Traefik routing labels, auto-avahi will:
1. Extract the hostname from `Host()` rules
2. Generate a trusted TLS certificate via mkcert
3. Write a Traefik dynamic config file for the certificate
4. Publish the hostname via mDNS so LAN devices can resolve it

When the container stops, the mDNS record is removed.

## Features

- **Docker Event Monitoring**: Watches for container start/stop events in real-time
- **Traefik Label Parsing**: Extracts hostnames from `traefik.http.routers.*.rule` labels
- **Hostname Validation**: Publishes 2-level and 3-level `.local` hostnames
  - Accepts: `whoami.local`, `api.local`, `app.myserver.local`
  - Rejects: `example.com` (non-.local), `a.b.c.local` (4+ levels)
- **Certificate Generation**: Automatically generates mkcert certificates if they don't exist
- **Certificate Validation**: Verifies certs contain valid PEM data before creating Traefik configs
- **Traefik Config**: Generates Traefik YAML configuration files with proper container paths
- **mDNS Publishing**: Publishes hostnames via `avahi-publish -a -R` to avoid reverse PTR conflicts

## Requirements

- Go 1.21+ (for building)
- Docker (with user in `docker` group for non-root access)
- `mkcert` installed and initialized (`mkcert -install`)
- `avahi-daemon` running
- `avahi-publish` available (from `avahi-utils` package)
- Traefik with file provider and Docker provider enabled

## Quick Start

```bash
# 1. Install dependencies (Debian/Ubuntu)
sudo apt install avahi-utils mkcert

# 2. Initialize the mkcert CA
mkcert -install

# 3. Build
go build -o auto-avahi

# 4. Install
sudo ./install.sh

# 5. Edit the service configuration
sudo systemctl edit auto-avahi
# Set AVAHI_HOST_IP, CERT_DIR, and TRAEFIK_CONFIG_DIR for your environment

# 6. Start
sudo systemctl start auto-avahi
```

## Configuration

Set via environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `AVAHI_HOST_IP` | **Yes** | — | Your server's LAN IP address (e.g., `192.168.1.100`) |
| `CERT_DIR` | No | `/certs` | Host directory where mkcert certificates are written |
| `CERT_DIR_CONTAINER` | No | `/certs` | Path where certs are mounted **inside** the Traefik container |
| `TRAEFIK_CONFIG_DIR` | No | `/traefik/dynamic` | Host directory for Traefik dynamic config YAML files |

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
- Copy the binary to `/usr/local/bin/`
- Install the systemd service file
- Prompt you for configuration values (`AVAHI_HOST_IP`, `CERT_DIR`, `TRAEFIK_CONFIG_DIR`)
- Enable and start the service

### Manual installation

1. Build the binary:
   ```bash
   go build -o auto-avahi
   sudo cp auto-avahi /usr/local/bin/
   ```

2. Copy and edit the service file:
   ```bash
   sudo cp auto-avahi.service /etc/systemd/system/
   ```

   Edit `/etc/systemd/system/auto-avahi.service` and set the `Environment` lines:
   ```ini
   Environment="AVAHI_HOST_IP=192.168.1.100"
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

### Running directly

```bash
export AVAHI_HOST_IP=192.168.1.100
export CERT_DIR=/srv/traefik/certs
export CERT_DIR_CONTAINER=/certs
export TRAEFIK_CONFIG_DIR=/srv/traefik/dynamic

./auto-avahi
```

### Example: Adding a service

Start any Docker container with Traefik Host labels:

```bash
docker run -d \
  --name whoami \
  --label "traefik.enable=true" \
  --label "traefik.http.routers.whoami.rule=Host(\`whoami.local\`)" \
  --label "traefik.http.routers.whoami.entrypoints=websecure" \
  --label "traefik.http.routers.whoami.tls=true" \
  traefik/whoami
```

auto-avahi will automatically:
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

> **Note:** For browsers to trust the certificates without warnings, install the mkcert CA on each client device. Copy `$(mkcert -CAROOT)/rootCA.pem` and add it to the client's trust store.

## Troubleshooting

### Traefik shows "Unable to parse certificate" errors

The paths in the YAML config don't match where Traefik sees the certs:
1. Verify `CERT_DIR_CONTAINER` matches the mount point in your Traefik container
2. Run `docker inspect traefik` to check volume mounts
3. The generated YAML files should use **container** paths, not host paths

### Hostname not resolving via mDNS

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

### Multi-level `.local` hostnames not resolving

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

### mDNS Publishing

Each hostname gets its own `avahi-publish -a -R <hostname> <ip>` child process. The `-R` flag publishes only the forward A record without claiming the reverse PTR mapping, avoiding conflicts with the host's own mDNS name.

### Startup Sync

On startup, auto-avahi scans all currently running containers for Traefik labels and processes them. This ensures hostnames are published even if auto-avahi starts after the containers.

### Lifecycle

- **Container start** → extract hostname → generate cert → write Traefik config → publish mDNS
- **Container stop** → kill the `avahi-publish` process for that hostname
- **auto-avahi shutdown** → kill all `avahi-publish` child processes

## License

MIT
