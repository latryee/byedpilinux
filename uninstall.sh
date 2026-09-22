#!/usr/bin/env bash
# discord-bypass uninstaller
# Completely purges discord-bypass binaries, systemd service, and firewall rules

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}============================================================${NC}"
echo -e "${BLUE}      discord-bypass - Clean Uninstallation                ${NC}"
echo -e "${BLUE}============================================================${NC}"

if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}[ERROR] Please run the uninstaller as root: sudo ./uninstall.sh${NC}"
    exit 1
fi

echo -e "[INFO] Deactivating bypass and clearing firewall rules..."
if command -v discord-bypass &>/dev/null; then
    discord-bypass emergency-disable || true
fi

# Stop and disable systemd service
if [ -d /run/systemd/system ]; then
    echo -e "[INFO] Stopping and removing systemd service..."
    systemctl stop discord-bypass.service 2>/dev/null || true
    systemctl disable discord-bypass.service 2>/dev/null || true
    rm -f /etc/systemd/system/discord-bypass.service
    systemctl daemon-reload
    echo "  [+] Removed /etc/systemd/system/discord-bypass.service"
fi

# Kill any remaining background processes
pkill -9 -f "discord-bypass daemon" 2>/dev/null || true
pkill -9 -f "discord-bypass-nfqws" 2>/dev/null || true

# Clean nftables table inet discord_bypass
if command -v nft &>/dev/null; then
    nft delete table inet discord_bypass 2>/dev/null || true
fi

# Clean iptables rules
if command -v iptables &>/dev/null; then
    iptables -t mangle -D OUTPUT -j DISCORD_BYPASS 2>/dev/null || true
    iptables -t mangle -F DISCORD_BYPASS 2>/dev/null || true
    iptables -t mangle -X DISCORD_BYPASS 2>/dev/null || true
    iptables -t nat -D OUTPUT -j DISCORD_BYPASS_NAT 2>/dev/null || true
    iptables -t nat -F DISCORD_BYPASS_NAT 2>/dev/null || true
    iptables -t nat -X DISCORD_BYPASS_NAT 2>/dev/null || true
fi

if command -v ip6tables &>/dev/null; then
    ip6tables -t mangle -D OUTPUT -j DISCORD_BYPASS 2>/dev/null || true
    ip6tables -t mangle -F DISCORD_BYPASS 2>/dev/null || true
    ip6tables -t mangle -X DISCORD_BYPASS 2>/dev/null || true
    ip6tables -t nat -D OUTPUT -j DISCORD_BYPASS_NAT 2>/dev/null || true
    ip6tables -t nat -F DISCORD_BYPASS_NAT 2>/dev/null || true
    ip6tables -t nat -X DISCORD_BYPASS_NAT 2>/dev/null || true
fi

# Remove DNS overrides (/etc/hosts and systemd-resolved)
if [ -f /etc/hosts ] && grep -q "BEGIN DISCORD-BYPASS" /etc/hosts; then
    echo -e "[INFO] Removing discord-bypass entries from /etc/hosts..."
    sed -i '/# --- BEGIN DISCORD-BYPASS/,/# --- END DISCORD-BYPASS/d' /etc/hosts 2>/dev/null || true
fi

if command -v resolvectl &>/dev/null; then
    resolvectl revert discord-dns 2>/dev/null || true
    resolvectl revert "" 2>/dev/null || true
    ip link del dev discord-dns 2>/dev/null || true
    DEF_IFACE=$(ip route show default 2>/dev/null | awk '{print $5}' | head -n1)
    if [ -n "$DEF_IFACE" ]; then
        resolvectl revert "$DEF_IFACE" 2>/dev/null || true
        command -v nmcli &>/dev/null && nmcli dev reapply "$DEF_IFACE" 2>/dev/null || true
    fi
    resolvectl flush-caches 2>/dev/null || true
fi

# Remove binaries
echo -e "[INFO] Removing installed binaries..."
rm -f /usr/bin/discord-bypass
rm -f /usr/bin/discord-bypass-nfqws
rm -f /usr/bin/discord-bypass-gui
rm -f /usr/local/bin/discord-bypass
rm -f /usr/local/bin/discord-bypass-nfqws
rm -f /usr/local/bin/discord-bypass-gui
echo "  [+] Removed installed binaries and symlinks"

# Remove desktop integration
echo -e "[INFO] Removing desktop integration..."
rm -f /usr/share/applications/discord-bypass.desktop
rm -f /usr/share/icons/hicolor/scalable/apps/discord-bypass.svg
if command -v update-desktop-database &>/dev/null; then
    update-desktop-database -q /usr/share/applications || true
fi
if command -v gtk-update-icon-cache &>/dev/null; then
    gtk-update-icon-cache -q /usr/share/icons/hicolor || true
fi
echo "  [+] Removed desktop entry and application icon"

# Configuration directory handling
PURGE="${1:-}"
if [ "$PURGE" = "--purge" ]; then
    echo -e "[INFO] Purging configuration directory /etc/discord-bypass..."
    rm -rf /etc/discord-bypass
    echo "  [+] Removed /etc/discord-bypass"
else
    echo -e "${YELLOW}[NOTE] Configuration kept at /etc/discord-bypass${NC}"
    echo "To remove configuration: sudo rm -rf /etc/discord-bypass (or run: sudo ./uninstall.sh --purge)"
fi

echo -e "\n${GREEN}============================================================${NC}"
echo -e "${GREEN}      discord-bypass has been cleanly and completely removed!${NC}"
echo -e "${GREEN}============================================================${NC}"
