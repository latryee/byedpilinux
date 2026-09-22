#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="$ROOT_DIR/packaging/build"
DEB_NAME="discord-bypass_1.0.0_amd64.deb"

echo "[INFO] Preparing package directory structure..."
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR/DEBIAN"
chmod 0755 "$BUILD_DIR"
mkdir -p "$BUILD_DIR/usr/local/bin"
mkdir -p "$BUILD_DIR/etc/discord-bypass"
mkdir -p "$BUILD_DIR/etc/systemd/system"

# Copy DEBIAN metadata
cp "$ROOT_DIR/packaging/debian/control" "$BUILD_DIR/DEBIAN/"
cp "$ROOT_DIR/packaging/debian/postinst" "$BUILD_DIR/DEBIAN/"
cp "$ROOT_DIR/packaging/debian/prerm" "$BUILD_DIR/DEBIAN/"
chmod 0755 "$BUILD_DIR/DEBIAN"
chmod 0755 "$BUILD_DIR/DEBIAN/postinst" "$BUILD_DIR/DEBIAN/prerm"

# Copy binaries
cp "$ROOT_DIR/bin/discord-bypass" "$BUILD_DIR/usr/local/bin/"
cp "$ROOT_DIR/bin/discord-bypass-nfqws" "$BUILD_DIR/usr/local/bin/"
chmod 0755 "$BUILD_DIR/usr/local/bin/discord-bypass" "$BUILD_DIR/usr/local/bin/discord-bypass-nfqws"

# Copy configuration
cp "$ROOT_DIR/config/config.toml" "$BUILD_DIR/etc/discord-bypass/"
cp "$ROOT_DIR/config/domains.txt" "$BUILD_DIR/etc/discord-bypass/"
cp "$ROOT_DIR/config/strategies.toml" "$BUILD_DIR/etc/discord-bypass/"
chmod 0644 "$BUILD_DIR/etc/discord-bypass/"*

# Copy systemd unit
cp "$ROOT_DIR/systemd/discord-bypass.service" "$BUILD_DIR/etc/systemd/system/"
chmod 0644 "$BUILD_DIR/etc/systemd/system/discord-bypass.service"

echo "[INFO] Running dpkg-deb to build $DEB_NAME..."
dpkg-deb --build "$BUILD_DIR" "$ROOT_DIR/$DEB_NAME"

echo "[OK] Debian package successfully created: $ROOT_DIR/$DEB_NAME"
dpkg-deb --info "$ROOT_DIR/$DEB_NAME"
