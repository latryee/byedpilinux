#!/usr/bin/env bash
# discord-bypass installer
# Linux DPI circumvention tool optimized for Discord in Türkiye

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}============================================================${NC}"
echo -e "${BLUE}      discord-bypass - Linux DPI Circumvention Installer     ${NC}"
echo -e "${BLUE}============================================================${NC}"

# Check root
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}[ERROR] Please run the installer as root: sudo ./install.sh${NC}"
    exit 1
fi

# Detect remote execution (curl | bash)
SCRIPT_DIR="$(pwd)"
if [ ! -f "$SCRIPT_DIR/src/cmd/discord-bypass/main.go" ] && [ ! -f "$SCRIPT_DIR/bin/discord-bypass" ]; then
    echo -e "[INFO] Remote execution detected. Fetching latest repository files..."
    TMP_DIR=$(mktemp -d /tmp/discord-bypass-install.XXXXXX)
    trap 'rm -rf "$TMP_DIR"' EXIT
    if command -v git &>/dev/null; then
        git clone --depth=1 https://github.com/latryee/byedpilinux.git "$TMP_DIR"
    else
        curl -fsSL https://github.com/latryee/byedpilinux/archive/refs/heads/main.tar.gz | tar -xz -C "$TMP_DIR" --strip-components=1
    fi
    cd "$TMP_DIR"
fi

# Detect Distro
if [ -f /etc/os-release ]; then
    . /etc/os-release
    DISTRO="${ID:-linux}"
    DISTRO_NAME="${PRETTY_NAME:-$ID}"
else
    DISTRO="unknown"
    DISTRO_NAME="Unknown Linux"
fi

echo -e "[INFO] Detected Operating System: ${GREEN}${DISTRO_NAME}${NC} (Kernel $(uname -r))"

# Dependency Check
MISSING_DEPS=()
for cmd in make gcc go nft; do
    if ! command -v "$cmd" &>/dev/null; then
        MISSING_DEPS+=("$cmd")
    fi
done

if ! pkg-config --exists libnetfilter_queue 2>/dev/null; then
    MISSING_DEPS+=("libnetfilter-queue-dev")
fi

if [ ${#MISSING_DEPS[@]} -gt 0 ]; then
    echo -e "${YELLOW}[WARN] Missing required dependencies:${NC} ${MISSING_DEPS[*]}"
    echo "Installing missing dependencies..."
    case "$DISTRO" in
        ubuntu|debian|zorin|linuxmint|pop)
            apt-get update -y
            apt-get install -y make gcc golang-go nftables libnetfilter-queue-dev libnetfilter-queue1 libmnl-dev
            ;;
        fedora)
            dnf install -y make gcc golang nftables libnetfilter_queue-devel libmnl-devel
            ;;
        arch|manjaro)
            pacman -Sy --noconfirm make gcc go nftables libnetfilter_queue libmnl
            ;;
        *)
            echo -e "${RED}[ERROR] Please install: ${MISSING_DEPS[*]}${NC}"
            exit 1
            ;;
    esac
fi

# Build binaries
echo -e "[INFO] Building binaries..."
make -C "$(dirname "$0")" build

# Install binaries
echo -e "[INFO] Installing binaries to /usr/bin/ and /usr/local/bin/..."
install -m 0755 bin/discord-bypass /usr/bin/discord-bypass
install -m 0755 bin/discord-bypass-nfqws /usr/bin/discord-bypass-nfqws
ln -sf /usr/bin/discord-bypass /usr/local/bin/discord-bypass 2>/dev/null || true
ln -sf /usr/bin/discord-bypass-nfqws /usr/local/bin/discord-bypass-nfqws 2>/dev/null || true

# Install configurations
echo -e "[INFO] Configuring /etc/discord-bypass/..."
mkdir -p /etc/discord-bypass
if [ ! -f /etc/discord-bypass/config.toml ]; then
    install -m 0644 config/config.toml /etc/discord-bypass/config.toml
    echo "  [+] Installed default /etc/discord-bypass/config.toml"
else
    echo "  [*] Preserved existing /etc/discord-bypass/config.toml"
fi

if [ ! -f /etc/discord-bypass/domains.txt ]; then
    install -m 0644 config/domains.txt /etc/discord-bypass/domains.txt
    echo "  [+] Installed default /etc/discord-bypass/domains.txt"
else
    echo "  [*] Preserved existing /etc/discord-bypass/domains.txt"
fi

if [ ! -f /etc/discord-bypass/strategies.toml ]; then
    install -m 0644 config/strategies.toml /etc/discord-bypass/strategies.toml
fi

# Install systemd service
if [ -d /run/systemd/system ]; then
    echo -e "[INFO] Installing systemd service..."
    install -m 0644 systemd/discord-bypass.service /etc/systemd/system/discord-bypass.service
    systemctl daemon-reload
    systemctl enable discord-bypass.service
    echo -e "${GREEN}[OK] systemd service enabled.${NC}"

    echo -e "[INFO] Starting discord-bypass service..."
    systemctl restart discord-bypass.service
    sleep 1
    systemctl is-active --quiet discord-bypass && echo -e "${GREEN}[OK] discord-bypass service is now running!${NC}" || echo -e "${YELLOW}[WARN] Service started with warnings. Check: discord-bypass logs${NC}"
else
    echo -e "${YELLOW}[WARN] systemd not active (container or chroot). You can run 'sudo discord-bypass daemon' manually.${NC}"
fi

# Install desktop integration
echo -e "[INFO] Installing desktop entry and application icon..."
mkdir -p /usr/share/applications
mkdir -p /usr/share/icons/hicolor/scalable/apps
install -m 0644 desktop/discord-bypass.desktop /usr/share/applications/discord-bypass.desktop
install -m 0644 desktop/discord-bypass.svg /usr/share/icons/hicolor/scalable/apps/discord-bypass.svg
if command -v update-desktop-database &>/dev/null; then
    update-desktop-database -q /usr/share/applications || true
fi
if command -v gtk-update-icon-cache &>/dev/null; then
    gtk-update-icon-cache -q /usr/share/icons/hicolor || true
fi

# Run auto-tuning for current network
if [ -d /run/systemd/system ] && systemctl is-active --quiet discord-bypass; then
    echo -e "\n${BLUE}[INFO] Running automatic strategy tuning for current network...${NC}"
    /usr/bin/discord-bypass tune || true
fi

echo -e "\n${GREEN}============================================================${NC}"
echo -e "${GREEN}             Installation Successfully Completed!           ${NC}"
echo -e "${GREEN}============================================================${NC}"
echo "Quick Commands:"
echo "  discord-bypass status          - View service & connectivity status"
echo "  sudo discord-bypass tune       - Automatically find & verify working strategy for your network"
echo "  discord-bypass strategy list   - List all available strategies & candidates"
echo "  discord-bypass strategy current- View active strategy & verification status"
echo "  sudo discord-bypass strategy set <id> - Safely switch strategy with auto-rollback"
echo "  discord-bypass diagnose        - Run full 12-point diagnostic test"
echo "  discord-bypass notify-status   - Dispatch desktop status notification"
echo "  sudo discord-bypass stop       - Temporarily stop bypass"
echo "  sudo discord-bypass start      - Start bypass"
echo "  sudo discord-bypass uninstall  - Clean uninstall"
