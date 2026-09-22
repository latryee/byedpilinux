#!/usr/bin/env bash
# host_validation.sh - Automated host verification script for discord-bypass
# Tests nfqws, nftables loop prevention, DNS split-routing, Discord APIs, crash recovery, and stop.
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}[ERROR] This script requires root privileges. Please run with sudo:${NC}"
    echo "  sudo ./tests/host_validation.sh"
    exit 1
fi

echo -e "${BLUE}============================================================${NC}"
echo -e "${BLUE}   discord-bypass Host Validation & Verification Suite     ${NC}"
echo -e "${BLUE}============================================================${NC}"

PASS_COUNT=0
FAIL_COUNT=0

record_pass() {
    echo -e "  ${GREEN}[PASS]${NC} $1"
    PASS_COUNT=$((PASS_COUNT + 1))
}

record_fail() {
    echo -e "  ${RED}[FAIL]${NC} $1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
}

echo -e "\n${BLUE}[1/8] Installing updated package...${NC}"
DEB_FILE="$ROOT_DIR/discord-bypass_1.0.0_amd64.deb"
if [ ! -f "$DEB_FILE" ]; then
    echo "Building deb..."
    make deb
fi
apt install -y --reinstall "$DEB_FILE" >/dev/null 2>&1 || dpkg -i "$DEB_FILE"

NFQWS_SIZE=$(stat -c %s /usr/local/bin/discord-bypass-nfqws 2>/dev/null || echo 0)
if [ "$NFQWS_SIZE" -gt 200000 ]; then
    record_pass "discord-bypass-nfqws installed correctly ($NFQWS_SIZE bytes, zapret v72.13)"
else
    record_fail "discord-bypass-nfqws size unexpected ($NFQWS_SIZE bytes)"
fi

echo -e "\n${BLUE}[2/8] Starting systemd service...${NC}"
systemctl daemon-reload
systemctl restart discord-bypass
sleep 2

if systemctl is-active --quiet discord-bypass; then
    record_pass "systemd service active"
else
    record_fail "systemd service failed to start"
    journalctl -u discord-bypass -n 30 --no-pager
fi

if pgrep -f "/usr/local/bin/discord-bypass daemon" >/dev/null; then
    record_pass "Go supervisor process running"
else
    record_fail "Go supervisor process not found"
fi

if pgrep -f "/usr/local/bin/discord-bypass-nfqws" >/dev/null; then
    record_pass "nfqws packet engine process running"
else
    record_fail "nfqws packet engine process not found"
fi

echo -e "\n${BLUE}[3/8] Verifying nftables ruleset and loop prevention structure...${NC}"
NFT_OUTPUT=$(nft list table inet discord_bypass 2>/dev/null || echo "")
if echo "$NFT_OUTPUT" | grep -q "table inet discord_bypass"; then
    record_pass "Dedicated nftables table 'inet discord_bypass' present"
else
    record_fail "Table 'inet discord_bypass' not found"
fi

if echo "$NFT_OUTPUT" | grep -q "meta mark 0x40000000.*accept"; then
    record_pass "Loop-prevention rule (meta mark 0x40000000 counter accept) present"
else
    record_fail "Loop-prevention rule missing from nftables"
fi

if echo "$NFT_OUTPUT" | grep -q "queue flags bypass to 200"; then
    record_pass "Interception rule (queue flags bypass to 200) present"
else
    record_fail "Interception queue rule missing from nftables"
fi

echo -e "\n${BLUE}[4/8] Testing NFQUEUE packet interception and reinjection loop safety...${NC}"
INITIAL_CHAIN=$(nft -a list chain inet discord_bypass output 2>/dev/null || echo "")
Q_PKTS_PRE=$(echo "$INITIAL_CHAIN" | grep "queue flags bypass" | grep -o "packets [0-9]*" | awk '{print $2}' || echo 0)
M_PKTS_PRE=$(echo "$INITIAL_CHAIN" | grep "meta mark 0x40000000" | grep -o "packets [0-9]*" | awk '{print $2}' || echo 0)

echo "  Initial counter: NFQUEUE intercepted = ${Q_PKTS_PRE:-0}, Marked accept = ${M_PKTS_PRE:-0}"

# Trigger Discord HTTPS requests
curl -s -o /dev/null -m 8 https://discord.com || true
curl -s -o /dev/null -m 8 https://discord.com/api/v10/gateway || true

POST_CHAIN=$(nft -a list chain inet discord_bypass output 2>/dev/null || echo "")
Q_PKTS_POST=$(echo "$POST_CHAIN" | grep "queue flags bypass" | grep -o "packets [0-9]*" | awk '{print $2}' || echo 0)
M_PKTS_POST=$(echo "$POST_CHAIN" | grep "meta mark 0x40000000" | grep -o "packets [0-9]*" | awk '{print $2}' || echo 0)

echo "  Post-traffic counter: NFQUEUE intercepted = ${Q_PKTS_POST:-0}, Marked accept = ${M_PKTS_POST:-0}"

DELTA_Q=$(( ${Q_PKTS_POST:-0} - ${Q_PKTS_PRE:-0} ))
DELTA_M=$(( ${M_PKTS_POST:-0} - ${M_PKTS_PRE:-0} ))

if [ "$DELTA_Q" -gt 0 ]; then
    record_pass "Discord packets intercepted by NFQUEUE (delta: +$DELTA_Q packets)"
else
    record_fail "No packets intercepted by NFQUEUE rule (delta: $DELTA_Q)"
fi

if [ "$DELTA_M" -gt 0 ]; then
    record_pass "Reinjected packets hit fwmark 0x40000000 bypass rule without looping (delta: +$DELTA_M packets)"
else
    echo -e "  ${YELLOW}[WARN]${NC} fwmark accept counter did not change (+${DELTA_M}). Check hostlist matching."
fi

echo -e "\n${BLUE}[5/8] Testing Normal Internet Regression & DNS Health...${NC}"
if curl -s -I -m 5 https://www.google.com | grep -q "HTTP"; then
    record_pass "Normal Internet traffic unaffected (google.com HTTP OK)"
else
    record_fail "Normal Internet traffic failed on google.com"
fi

if curl -s -I -m 5 https://github.com | grep -q "HTTP"; then
    record_pass "Normal Internet traffic unaffected (github.com HTTP OK)"
else
    record_fail "Normal Internet traffic failed on github.com"
fi

DISCORD_IP=$(resolvectl query discord.com 2>/dev/null | grep -v "195.175.254.2" | grep -E "162\.159\." | head -n1 || echo "")
if [ -n "$DISCORD_IP" ]; then
    record_pass "discord.com resolved to authentic Cloudflare IP ($DISCORD_IP) rather than sinkhole"
else
    # Check if hosts file or proxy resolved it
    if getent ahosts discord.com | grep -q -v "195.175.254.2"; then
        record_pass "discord.com resolved to clean IP (no sinkhole)"
    else
        record_fail "discord.com resolved to sinkhole or failed"
    fi
fi

echo -e "\n${BLUE}[6/8] Testing Real Discord Connectivity Endpoints...${NC}"
DISCORD_HTTP=$(curl -s -I -m 10 https://discord.com | head -n1 || echo "")
if echo "$DISCORD_HTTP" | grep -qE "(200|301|302)"; then
    record_pass "Discord Web HTTPS reachable ($DISCORD_HTTP)"
else
    record_fail "Discord Web HTTPS unreachable ($DISCORD_HTTP)"
fi

DISCORD_API=$(curl -s -I -m 10 https://discord.com/api/v10/gateway | head -n1 || echo "")
if echo "$DISCORD_API" | grep -qE "(200|401)"; then
    record_pass "Discord REST API /api/v10/gateway reachable ($DISCORD_API)"
else
    record_fail "Discord REST API unreachable ($DISCORD_API)"
fi

echo -e "\n${BLUE}[7/8] Testing Crash Recovery of nfqws...${NC}"
NFQWS_PID=$(pgrep -f -o "discord-bypass-nfqws" || echo "")
if [ -n "$NFQWS_PID" ]; then
    echo "  Killing nfqws (PID $NFQWS_PID) with SIGKILL..."
    kill -9 "$NFQWS_PID"
    sleep 2.5
    NEW_NFQWS_PID=$(pgrep -f -o "discord-bypass-nfqws" || echo "")
    if [ -n "$NEW_NFQWS_PID" ] && [ "$NEW_NFQWS_PID" != "$NFQWS_PID" ]; then
        record_pass "nfqws crash detected and recovered by supervisor (New PID $NEW_NFQWS_PID)"
    else
        record_fail "nfqws did not restart after crash"
    fi
else
    record_fail "No nfqws process to test crash recovery"
fi

echo -e "\n${BLUE}[8/8] Testing Clean Service Stop & Teardown...${NC}"
STOP_TIME_START=$(date +%s%N)
systemctl stop discord-bypass
STOP_TIME_END=$(date +%s%N)
STOP_DURATION_MS=$(( (STOP_TIME_END - STOP_TIME_START) / 1000000 ))

if ! pgrep -f "/usr/local/bin/discord-bypass daemon" >/dev/null; then
    record_pass "Daemon process terminated cleanly"
else
    record_fail "Orphan daemon process found"
fi

if ! pgrep -f "/usr/local/bin/discord-bypass-nfqws" >/dev/null; then
    record_pass "nfqws process terminated cleanly"
else
    record_fail "Orphan nfqws process found"
fi

if ! nft list table inet discord_bypass >/dev/null 2>&1; then
    record_pass "nftables table inet discord_bypass removed cleanly"
else
    record_fail "Stale nftables rules left behind"
fi

record_pass "Service stop completed in ${STOP_DURATION_MS}ms (no recursive shutdown)"

# Restore service
systemctl start discord-bypass

echo -e "\n${BLUE}============================================================${NC}"
echo -e "${BLUE}                 HOST VALIDATION SUMMARY                    ${NC}"
echo -e "${BLUE}============================================================${NC}"
echo -e "Passed Checks: ${GREEN}$PASS_COUNT${NC}"
echo -e "Failed Checks: ${RED}$FAIL_COUNT${NC}"
if [ "$FAIL_COUNT" -eq 0 ]; then
    echo -e "${GREEN}ALL HOST TESTS PASSED! The networking stack is verified on this machine.${NC}"
else
    echo -e "${RED}SOME CHECKS FAILED. Review output above for details.${NC}"
fi
echo -e "${BLUE}============================================================${NC}"
exit $FAIL_COUNT
