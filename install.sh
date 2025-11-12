#!/usr/bin/env bash

# ===== CONFIGURATION =====
# Required parameters
PROJECT_NAME="akeyshually"
BINARY_NAME="akeyshually"
REPO_USER="DeprecatedLuar"
REPO_NAME="akeyshually"
INSTALL_DIR="$HOME/.local/bin"
BUILD_CMD="go build -ldflags='-s -w' -o akeyshually ./cmd"

MSG_FINAL="Big hug from Luar"
NEXT_STEPS="Errm... Try running: akeyshually --help|Config will be auto-generated in ~/.config/akeyshually/ on first run|Actually... add your user to the input group: sudo usermod -aG input \$USER"
ASCII_ART=''

# ===== END CONFIGURATION =====

set -e

# Command routing
case "${1:-}" in
    local)
        # Local installation - no satellite needed
        echo "Installing locally. Stand by"

        # Find binary in current directory
        if [ ! -f "$BINARY_NAME" ]; then
            echo "Error: $BINARY_NAME not found. Build it first (e.g., go build)"
            exit 1
        fi

        # Stop running instance
        if pgrep -x "$BINARY_NAME" > /dev/null 2>&1; then
            echo "Stopping running instance..."
            pkill -TERM -x "$BINARY_NAME" 2>/dev/null || true
            sleep 1
        fi

        # Install
        mkdir -p "$INSTALL_DIR"
        cp "$BINARY_NAME" "$INSTALL_DIR/"
        chmod +x "$INSTALL_DIR/$BINARY_NAME"

        echo "Installed to $INSTALL_DIR/$BINARY_NAME"
        ;;

    update|version)
        # Pass through to satellite for update/version commands
        curl -sSL https://raw.githubusercontent.com/$REPO_USER/the-satellite/main/satellite.sh | \
            bash -s -- "$1" \
                "$PROJECT_NAME" \
                "$BINARY_NAME" \
                "$REPO_USER" \
                "$REPO_NAME" \
                "$INSTALL_DIR" \
                "$BUILD_CMD" \
                "$ASCII_ART" \
                "$MSG_FINAL" \
                "$NEXT_STEPS"
        ;;

    *)
        # Standard installation via satellite
        curl -sSL https://raw.githubusercontent.com/$REPO_USER/the-satellite/main/satellite.sh | \
            bash -s -- install \
                "$PROJECT_NAME" \
                "$BINARY_NAME" \
                "$REPO_USER" \
                "$REPO_NAME" \
                "$INSTALL_DIR" \
                "$BUILD_CMD" \
                "$ASCII_ART" \
                "$MSG_FINAL" \
                "$NEXT_STEPS"
        ;;
esac

