#!/usr/bin/env bash
set -euo pipefail

# Munin Screen Agent Installer for Raspberry Pi / Linux
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/naueramant/munin/master/install.sh | bash

REPO="naueramant/munin"
BIN_DIR="/usr/local/bin"
CURRENT_USER="${SUDO_USER:-$USER}"
USER_HOME=$(eval echo "~${CURRENT_USER}")

echo "=== Munin Screen Agent Installer ==="
echo "Installing for user: ${CURRENT_USER} (Home: ${USER_HOME})"

# 1. Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)
        TARGET_ARCH="x86_64"
        ;;
    aarch64|arm64)
        TARGET_ARCH="arm64"
        ;;
    armv7l|armv7)
        TARGET_ARCH="armv7"
        ;;
    armv6l)
        TARGET_ARCH="armv6"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

echo "Detected architecture: ${ARCH} (target: ${TARGET_ARCH})"

# 2. Check and install system prerequisites
echo "Checking and installing required dependencies (Chromium, cec-utils, cron)..."

if command -v apt-get &>/dev/null; then
    echo "Using apt-get package manager..."
    sudo apt-get update -y

    # Core utilities
    sudo apt-get install -y --no-install-recommends \
        curl \
        tar \
        cron \
        cec-utils \
        openssh-client \
        unclutter \
        fonts-liberation \
        fonts-noto-color-emoji || true

    # Chromium browser (package name varies between Debian/Raspberry Pi OS releases)
    if ! command -v chromium-browser &>/dev/null && ! command -v chromium &>/dev/null; then
        echo "Installing Chromium browser..."
        sudo apt-get install -y chromium-browser || sudo apt-get install -y chromium || true
    fi

    # Enable and start cron service
    sudo systemctl enable --now cron 2>/dev/null || true

    # Add user to video, render, and input groups for CEC and GPU acceleration permissions
    echo "Configuring hardware permissions for user ${CURRENT_USER}..."
    sudo usermod -aG video,render,input "${CURRENT_USER}" 2>/dev/null || sudo usermod -aG video "${CURRENT_USER}" 2>/dev/null || true

elif command -v dnf &>/dev/null; then
    echo "Using dnf package manager..."
    sudo dnf install -y curl tar cronie cec-utils chromium openssh-clients || true
    sudo systemctl enable --now cronie 2>/dev/null || true
    sudo usermod -aG video "${CURRENT_USER}" 2>/dev/null || true

elif command -v pacman &>/dev/null; then
    echo "Using pacman package manager..."
    sudo pacman -Sy --noconfirm curl tar cronie cec-utils chromium openssh || true
    sudo systemctl enable --now cronie 2>/dev/null || true
    sudo usermod -aG video "${CURRENT_USER}" 2>/dev/null || true

else
    echo "Warning: Unsupported package manager. Please ensure chromium, cec-utils, and cron are installed manually."
fi

# Verify dependencies
echo "Verifying prerequisites..."
MISSING_DEPS=0

if command -v chromium-browser &>/dev/null; then
    echo "  [✓] Chromium browser found: $(command -v chromium-browser)"
elif command -v chromium &>/dev/null; then
    echo "  [✓] Chromium browser found: $(command -v chromium)"
elif command -v google-chrome &>/dev/null; then
    echo "  [✓] Chrome browser found: $(command -v google-chrome)"
else
    echo "  [✗] Chromium browser is NOT installed!"
    MISSING_DEPS=1
fi

if command -v cec-client &>/dev/null; then
    echo "  [✓] cec-client (cec-utils) found: $(command -v cec-client)"
else
    echo "  [✗] cec-client (cec-utils) is NOT installed! (HDMI CEC power scheduling will not function)"
    MISSING_DEPS=1
fi

if command -v crontab &>/dev/null; then
    echo "  [✓] crontab found: $(command -v crontab)"
else
    echo "  [✗] crontab is NOT installed! (Native cron scheduling will not function)"
    MISSING_DEPS=1
fi

if [ $MISSING_DEPS -ne 0 ]; then
    echo "Warning: Some dependencies could not be automatically installed. You may need to install them manually."
fi

# 3. Download latest release
echo "Fetching latest release information from GitHub..."
RELEASE_JSON=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" || true)

DOWNLOAD_URL=""
if [ -n "$RELEASE_JSON" ]; then
    DOWNLOAD_URL=$(echo "$RELEASE_JSON" | grep "browser_download_url" | grep -i "linux" | grep -i "${TARGET_ARCH}" | cut -d '"' -f 4 | head -n 1)
fi

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

if [ -n "$DOWNLOAD_URL" ]; then
    echo "Downloading release: ${DOWNLOAD_URL}..."
    curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/munin.tar.gz"
    tar -xzf "${TMP_DIR}/munin.tar.gz" -C "$TMP_DIR"
    if [ -f "${TMP_DIR}/munin" ]; then
        sudo install -m 0755 "${TMP_DIR}/munin" "${BIN_DIR}/munin"
    elif [ -f "${TMP_DIR}/mir" ]; then
        sudo install -m 0755 "${TMP_DIR}/mir" "${BIN_DIR}/munin"
    fi
else
    echo "Notice: Prebuilt release asset not found on GitHub."
    if command -v go &>/dev/null; then
        echo "Building from source using local Go compiler..."
        go build -o "${TMP_DIR}/munin" .
        sudo install -m 0755 "${TMP_DIR}/munin" "${BIN_DIR}/munin"
    else
        echo "Error: No release binary found and Go is not installed to compile from source."
        exit 1
    fi
fi

echo "Installed munin binary to ${BIN_DIR}/munin"

# 4. Launch interactive setup wizard
echo ""
echo "Launching Munin Setup Wizard..."
echo ""

if [ -t 0 ]; then
    sudo -u "${CURRENT_USER}" "${BIN_DIR}/munin" init
elif [ -e /dev/tty ]; then
    sudo -u "${CURRENT_USER}" "${BIN_DIR}/munin" init < /dev/tty
else
    sudo -u "${CURRENT_USER}" "${BIN_DIR}/munin" init
fi
