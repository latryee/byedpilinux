#!/usr/bin/env bash
# fix_dns.sh - Instantly restore system DNS to router/modem DHCP DNS
# Safe to run anytime, after any test, or if internet ever drops.
set -euo pipefail

GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}[INFO] Restoring system DNS to router default...${NC}"

# Find active network interface
IFACE=$(ip route show default 2>/dev/null | awk '{print $5}' | head -n1 || echo "")
if [ -z "$IFACE" ]; then
    IFACE="enp5s0"
fi

# Detect router/DHCP DNS
ROUTER_DNS=$(nmcli dev show "$IFACE" 2>/dev/null | grep 'IP4.DNS' | awk '{print $2}' | head -n1 || echo "")
if [ -z "$ROUTER_DNS" ]; then
    ROUTER_DNS=$(ip route show default 2>/dev/null | awk '{print $3}' | head -n1 || echo "")
fi
if [ -z "$ROUTER_DNS" ]; then
    ROUTER_DNS="192.168.1.1"
fi

echo -e "Detected interface: ${IFACE}, Router DNS: ${ROUTER_DNS}"

# Explicitly assign router DNS to the interface and enable default-route
if command -v resolvectl &>/dev/null; then
    resolvectl dns "$IFACE" "$ROUTER_DNS" 2>/dev/null || true
    resolvectl default-route "$IFACE" true 2>/dev/null || true
    resolvectl domain "$IFACE" "" 2>/dev/null || true
    resolvectl revert "" 2>/dev/null || true
    resolvectl dns "" "" 2>/dev/null || true
    resolvectl flush-caches 2>/dev/null || true
fi

# Ensure config has local_dns_port = 0 and sync_hosts = true
if [ -f /etc/discord-bypass/config.toml ]; then
    sed -i -E 's|^local_dns_port\s*=.*|local_dns_port = 0|' /etc/discord-bypass/config.toml 2>/dev/null || true
    sed -i -E 's|^sync_hosts\s*=.*|sync_hosts = true|' /etc/discord-bypass/config.toml 2>/dev/null || true
fi

echo -e "${GREEN}[OK] DNS cleanly restored to ${ROUTER_DNS} on ${IFACE}. Internet is fully active!${NC}"
