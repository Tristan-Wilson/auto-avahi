#!/bin/bash
# uninstall.sh - Uninstall auto-avahi systemd service and binary
set -e

BINARY_NAME="auto-avahi"
INSTALL_DIR="/usr/local/bin"
SERVICE_NAME="auto-avahi"
SERVICE_FILE="$SERVICE_NAME.service"
SERVICE_DIR="/etc/systemd/system"

# Check we're root
if [ "$EUID" -ne 0 ]; then
  echo "Please run as root: sudo ./uninstall.sh"
  exit 1
fi

echo "=== auto-avahi uninstaller ==="
echo

# Read current config from the service file before removing
if [ -f "$SERVICE_DIR/$SERVICE_FILE" ]; then
  CERT_DIR=$(grep -oP '(?<=Environment="CERT_DIR=)[^"]+' "$SERVICE_DIR/$SERVICE_FILE" 2>/dev/null || true)
  TRAEFIK_DIR=$(grep -oP '(?<=Environment="TRAEFIK_CONFIG_DIR=)[^"]+' "$SERVICE_DIR/$SERVICE_FILE" 2>/dev/null || true)

  echo "Current installation:"
  echo "  Binary:       $INSTALL_DIR/$BINARY_NAME"
  echo "  Service:      $SERVICE_DIR/$SERVICE_FILE"
  [ -n "$CERT_DIR" ]    && echo "  Cert dir:     $CERT_DIR"
  [ -n "$TRAEFIK_DIR" ] && echo "  Traefik dir:  $TRAEFIK_DIR"
else
  echo "Service file not found at $SERVICE_DIR/$SERVICE_FILE"
  echo "Checking for binary only..."
fi

echo
read -rp "Proceed with uninstall? [y/N] " CONFIRM
if [[ ! "$CONFIRM" =~ ^[Yy] ]]; then
  echo "Aborted."
  exit 0
fi

# Stop and disable the service
if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
  echo "Stopping $SERVICE_NAME..."
  systemctl stop "$SERVICE_NAME"
fi

if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
  echo "Disabling $SERVICE_NAME..."
  systemctl disable "$SERVICE_NAME"
fi

# Remove service file
if [ -f "$SERVICE_DIR/$SERVICE_FILE" ]; then
  echo "Removing service file..."
  rm "$SERVICE_DIR/$SERVICE_FILE"
  systemctl daemon-reload
fi

# Remove binary
if [ -f "$INSTALL_DIR/$BINARY_NAME" ]; then
  echo "Removing binary..."
  rm "$INSTALL_DIR/$BINARY_NAME"
fi

echo
echo "=== Uninstall complete ==="
echo
echo "The following were NOT removed (contain your data):"
[ -n "$CERT_DIR" ]    && echo "  Certificates:     $CERT_DIR"
[ -n "$TRAEFIK_DIR" ] && echo "  Traefik configs:  $TRAEFIK_DIR"
echo
echo "Remove them manually if no longer needed."
