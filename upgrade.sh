#!/bin/bash
# upgrade.sh - Build and deploy a new version of auto-avahi
# Use this when the service is already installed and configured.
set -e

BINARY_NAME="auto-avahi"
INSTALL_DIR="/usr/local/bin"

echo "Building $BINARY_NAME..."
go build -o "$BINARY_NAME" .
echo "Built: ./$BINARY_NAME ($(du -h "$BINARY_NAME" | cut -f1))"

echo "Stopping service..."
sudo systemctl stop "$BINARY_NAME"

echo "Installing to $INSTALL_DIR/$BINARY_NAME..."
sudo cp "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"

echo "Starting service..."
sudo systemctl start "$BINARY_NAME"

echo "Verifying..."
sleep 3
systemctl status "$BINARY_NAME" --no-pager
