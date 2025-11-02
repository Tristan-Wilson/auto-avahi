#!/bin/bash
# Test setup script for auto-avahi

set -e

echo "=== Auto-Avahi Test Setup ==="
echo

# Create directories
echo "Creating test directories..."
mkdir -p test-certs test-traefik

# Set environment variables
export AVAHI_HOST_IP="${AVAHI_HOST_IP:?AVAHI_HOST_IP must be set to your server's LAN IP}"
export CERT_DIR="$(pwd)/test-certs"
export TRAEFIK_CONFIG_DIR="$(pwd)/test-traefik"

echo "Configuration:"
echo "  AVAHI_HOST_IP: $AVAHI_HOST_IP"
echo "  CERT_DIR: $CERT_DIR"
echo "  TRAEFIK_CONFIG_DIR: $TRAEFIK_CONFIG_DIR"
echo

# Check prerequisites
echo "Checking prerequisites..."
command -v mkcert >/dev/null 2>&1 || { echo "ERROR: mkcert not found. Please install it first."; exit 1; }
command -v avahi-publish >/dev/null 2>&1 || { echo "ERROR: avahi-publish not found. Please install avahi-utils."; exit 1; }
command -v docker >/dev/null 2>&1 || { echo "ERROR: docker not found. Please install Docker."; exit 1; }

echo "All prerequisites found!"
echo

echo "=== To run auto-avahi: ==="
echo "sudo AVAHI_HOST_IP=$AVAHI_HOST_IP CERT_DIR=$CERT_DIR TRAEFIK_CONFIG_DIR=$TRAEFIK_CONFIG_DIR ./auto-avahi"
echo
echo "=== To test with a container: ==="
echo "docker run -d --name test-whoami --label traefik.http.routers.whoami.rule='Host(\`whoami.local\`)' traefik/whoami"
echo
echo "=== Expected behavior: ==="
echo "1. auto-avahi detects the container"
echo "2. Generates certificate: $CERT_DIR/whoami.local.pem"
echo "3. Creates Traefik config: $TRAEFIK_CONFIG_DIR/whoami.local.yml"
echo "4. Publishes whoami.local via mDNS"
echo
echo "=== Test resolution: ==="
echo "avahi-resolve -n whoami.local"
echo "ping whoami.local"
