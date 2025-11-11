#!/bin/bash

# akeyshually Install Script
# Usage: ./install-local.sh

set -e

pushd "$(dirname "$0")/.." > /dev/null

echo "Building akeyshually..."
go build -o akeyshually

if [ $? -eq 0 ]; then
    echo "Build successful!"

    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"

    echo "Installing to $INSTALL_DIR/..."
    cp akeyshually "$INSTALL_DIR/"    
else
    echo "Build failed!"
    exit 1
fi
