#!/bin/bash
# build.sh - Build auto-avahi binary
set -e

BINARY_NAME="auto-avahi"

echo "Building $BINARY_NAME..."
go build -o "$BINARY_NAME"

echo "Built: ./$BINARY_NAME ($(du -h "$BINARY_NAME" | cut -f1))"
