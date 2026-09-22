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
echo -e "[INFO] Installing binaries to /usr/local/bin/..."
install -m 0755 bin/discord-bypass /usr/local/bin/discord-bypass
install -m 0755 bin/discord-bypass-nfqws /usr/local/bin/discord-bypass-nfqws

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

echo -e "\n${GREEN}============================================================${NC}"
echo -e "${GREEN}             Installation Successfully Completed!           ${NC}"
echo -e "${GREEN}============================================================${NC}"
echo "Quick Commands:"
echo "  discord-bypass status          - View service & connectivity status"
echo "  discord-bypass diagnose        - Run full 12-point diagnostic test"
echo "  discord-bypass test            - Test Discord endpoints"
echo "  sudo discord-bypass stop       - Temporarily stop bypass"
echo "  sudo discord-bypass start      - Start bypass"
echo "  sudo discord-bypass uninstall  - Clean uninstall"
