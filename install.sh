#!/bin/bash
# install.sh - Install auto-avahi as a systemd service
set -e

BINARY_NAME="auto-avahi"
INSTALL_DIR="/usr/local/bin"
SERVICE_FILE="auto-avahi.service"
SERVICE_DIR="/etc/systemd/system"

# Check we're root
if [ "$EUID" -ne 0 ]; then
  echo "Please run as root: sudo ./install.sh"
  exit 1
fi

# Check binary exists
if [ ! -f "$BINARY_NAME" ]; then
  echo "Binary '$BINARY_NAME' not found. Build it first:"
  echo "  go build -o $BINARY_NAME"
  exit 1
fi

echo "=== auto-avahi installer ==="
echo

# Choose mode
echo "auto-avahi can run with or without mDNS publishing."
echo "  - mDNS on:  publishes hostnames via avahi-publish for LAN discovery (.local)"
echo "  - mDNS off: only generates TLS certs and Traefik configs (use when DNS is"
echo "              handled elsewhere, e.g. split-horizon, public DNS, /etc/hosts)"
echo
read -rp "Enable mDNS publishing? [Y/n] " MDNS_CHOICE
if [[ "$MDNS_CHOICE" =~ ^[Nn] ]]; then
  MDNS_ENABLE="false"
else
  MDNS_ENABLE="true"
fi

# Always-required dependencies
REQUIRED_CMDS=(mkcert docker)
# avahi-publish only required when MDNS_ENABLE=true
if [ "$MDNS_ENABLE" = "true" ]; then
  REQUIRED_CMDS+=(avahi-publish)
fi

for cmd in "${REQUIRED_CMDS[@]}"; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "ERROR: '$cmd' not found. Please install it first."
    case "$cmd" in
      mkcert) echo "  sudo apt install mkcert  # or see https://github.com/FiloSottile/mkcert" ;;
      avahi-publish) echo "  sudo apt install avahi-utils" ;;
      docker) echo "  See https://docs.docker.com/engine/install/" ;;
    esac
    exit 1
  fi
done

# Hostname suffix(es)
echo
read -rp "Hostname suffix(es), comma-separated (HOSTNAME_SUFFIXES) [local]: " HOSTNAME_SUFFIXES
HOSTNAME_SUFFIXES="${HOSTNAME_SUFFIXES:-local}"

# Server LAN IP (only when mDNS on)
HOST_IP=""
if [ "$MDNS_ENABLE" = "true" ]; then
  read -rp "Server LAN IP address (AVAHI_HOST_IP): " HOST_IP
  if [ -z "$HOST_IP" ]; then
    echo "ERROR: IP address is required when mDNS is enabled."
    exit 1
  fi
fi

read -rp "Host directory for TLS certificates (CERT_DIR) [/srv/auto-avahi/certs]: " CERT_DIR
CERT_DIR="${CERT_DIR:-/srv/auto-avahi/certs}"

read -rp "Certificate path inside Traefik container (CERT_DIR_CONTAINER) [/certs]: " CERT_DIR_CONTAINER
CERT_DIR_CONTAINER="${CERT_DIR_CONTAINER:-/certs}"

read -rp "Host directory for Traefik dynamic configs (TRAEFIK_CONFIG_DIR) [/srv/auto-avahi/dynamic]: " TRAEFIK_DIR
TRAEFIK_DIR="${TRAEFIK_DIR:-/srv/auto-avahi/dynamic}"

echo
echo "mkcert CA root directory (CAROOT)."
echo "  Set this if the service runs as root but your browser trusts a"
echo "  different user's mkcert CA. Find it by running as that user:"
echo "    mkcert -CAROOT"
echo "  Leave blank to use the default CA for root."
read -rp "mkcert CA root (CAROOT) []: " CAROOT
CAROOT="${CAROOT:-}"

echo
echo "Configuration:"
echo "  MDNS_ENABLE=$MDNS_ENABLE"
echo "  HOSTNAME_SUFFIXES=$HOSTNAME_SUFFIXES"
echo "  AVAHI_HOST_IP=$HOST_IP"
echo "  CERT_DIR=$CERT_DIR"
echo "  CERT_DIR_CONTAINER=$CERT_DIR_CONTAINER"
echo "  TRAEFIK_CONFIG_DIR=$TRAEFIK_DIR"
if [ -n "$CAROOT" ]; then
  echo "  CAROOT=$CAROOT"
else
  echo "  CAROOT=(not set, using default)"
fi
echo

read -rp "Proceed with installation? [Y/n] " CONFIRM
if [[ "$CONFIRM" =~ ^[Nn] ]]; then
  echo "Aborted."
  exit 0
fi

# Install binary
echo "Installing binary to $INSTALL_DIR/$BINARY_NAME..."
cp "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
chmod 755 "$INSTALL_DIR/$BINARY_NAME"

# Create directories
echo "Creating directories..."
mkdir -p "$CERT_DIR" "$TRAEFIK_DIR"

# Install service file with configured values.
# All Environment= lines in the template are replaced.
echo "Installing systemd service..."
SED_ARGS=(
  -e "s|^Environment=\"AVAHI_HOST_IP=.*|Environment=\"AVAHI_HOST_IP=$HOST_IP\"|"
  -e "s|^Environment=\"HOSTNAME_SUFFIXES=.*|Environment=\"HOSTNAME_SUFFIXES=$HOSTNAME_SUFFIXES\"|"
  -e "s|^Environment=\"MDNS_ENABLE=.*|Environment=\"MDNS_ENABLE=$MDNS_ENABLE\"|"
  -e "s|^Environment=\"CERT_DIR=.*|Environment=\"CERT_DIR=$CERT_DIR\"|"
  -e "s|^Environment=\"CERT_DIR_CONTAINER=.*|Environment=\"CERT_DIR_CONTAINER=$CERT_DIR_CONTAINER\"|"
  -e "s|^Environment=\"TRAEFIK_CONFIG_DIR=.*|Environment=\"TRAEFIK_CONFIG_DIR=$TRAEFIK_DIR\"|"
)
if [ -n "$CAROOT" ]; then
  SED_ARGS+=(-e "s|^#Environment=\"CAROOT=.*|Environment=\"CAROOT=$CAROOT\"|")
fi
sed "${SED_ARGS[@]}" "$SERVICE_FILE" > "$SERVICE_DIR/$SERVICE_FILE"

# Reload and enable
echo "Enabling service..."
systemctl daemon-reload
systemctl enable "$BINARY_NAME"

echo
echo "=== Installation complete ==="
echo
echo "Start the service:"
echo "  sudo systemctl start auto-avahi"
echo
echo "View logs:"
echo "  journalctl -u auto-avahi -f"
echo
echo "To reconfigure, edit the service:"
echo "  sudo systemctl edit auto-avahi"
echo "  sudo systemctl restart auto-avahi"
