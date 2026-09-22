#!/usr/bin/env bash
# live_strategy_tester.sh - Systematic live testing of nfqws TLS desync strategies
# Tests candidate strategies on the running systemd/nftables stack without permanently modifying config.
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
CONFIG_FILE="/etc/discord-bypass/config.toml"
BACKUP_FILE="/etc/discord-bypass/config.toml.autobak"

if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
    echo "Usage:"
    echo "  sudo ./tests/live_strategy_tester.sh [candidate_number]"
    echo "  sudo ./tests/live_strategy_tester.sh --all"
    echo "  sudo ./tests/live_strategy_tester.sh --single \"Name\" \"arg1\" \"arg2\" ..."
    echo ""
    echo "Candidates (fakedsplit, fakeddisorder & fake,multisplit for Superonline):"
    echo "  1  : fakedsplit (midsld, ttl=5) [DPI bypassed, fake segment expires before Cloudflare]"
    echo "  2  : fakedsplit (midsld, ttl=4) [Fake segment expires at hop 4]"
    echo "  3  : fakedsplit (midsld, ttl=6) [Fake segment expires at hop 6]"
    echo "  4  : fakedsplit (midsld, ttl=3) [Fake segment expires at hop 3]"
    echo "  5  : fakedsplit (midsld, badsum) [Fake segment dropped by Cloudflare via bad checksum]"
    echo "  6  : fakedsplit (midsld, badseq) [Fake segment dropped by Cloudflare via bad sequence]"
    echo "  7  : fakedsplit (midsld, seqovl=16, ttl=5) [Fake segment + 16-byte sequence overlap]"
    echo "  8  : fakedsplit (midsld, seqovl=16, ttl=4) [Fake segment + 16-byte sequence overlap, TTL=4]"
    echo "  9  : fakeddisorder (midsld, ttl=5) [Out-of-order fake split with TTL=5]"
    echo "  10 : fakeddisorder (midsld, ttl=4) [Out-of-order fake split with TTL=4]"
    echo "  11 : fakeddisorder (midsld, ttl=6) [Out-of-order fake split with TTL=6]"
    echo "  12 : fakeddisorder (midsld, badsum) [Out-of-order fake split with badsum]"
    echo "  13 : fakedsplit (pos=2, ttl=5) [TLS record header fake split with TTL=5]"
    echo "  14 : fakedsplit (pos=2, ttl=4) [TLS record header fake split with TTL=4]"
    echo "  15 : fake,multisplit (midsld, ttl=5) [Full fake ClientHello + multisplit at midsld]"
    echo "  16 : fake,multisplit (midsld, ttl=4) [Full fake ClientHello + multisplit at midsld, TTL=4]"
    echo "  17 : fake,multisplit (pos=2, ttl=5) [Full fake ClientHello + multisplit at pos=2]"
    echo "  18 : fake,multisplit (pos=2, ttl=4) [Full fake ClientHello + multisplit at pos=2, TTL=4]"
    echo "  19 : fakedsplit (sniext+2, ttl=5) [SNI extension header fake split with TTL=5]"
    echo "  20 : fakeddisorder (pos=2, ttl=5) [Out-of-order pos=2 with TTL=5]"
    exit 0
fi

if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}[ERROR] This script requires root privileges. Please run with sudo:${NC}"
    echo "  sudo ./tests/live_strategy_tester.sh [candidate_number|--all]"
    exit 1
fi

if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}[ERROR] Configuration file not found at $CONFIG_FILE${NC}"
    exit 1
fi

echo -e "${BLUE}============================================================${NC}"
echo -e "${BLUE}       discord-bypass Live Strategy Test Suite              ${NC}"
echo -e "${BLUE}============================================================${NC}"

# Backup production configuration
cp "$CONFIG_FILE" "$BACKUP_FILE"
echo -e "Backed up existing configuration to ${CYAN}$BACKUP_FILE${NC}"

cleanup() {
    echo -e "\n${BLUE}Restoring original configuration and restarting service...${NC}"
    if [ -f "$BACKUP_FILE" ]; then
        cp "$BACKUP_FILE" "$CONFIG_FILE"
        rm -f "$BACKUP_FILE"
    fi
    # Always guarantee local_dns_port is 0 and sync_hosts is true in restored config
    sed -i -E 's|^local_dns_port\s*=.*|local_dns_port = 0|' "$CONFIG_FILE" 2>/dev/null || true
    sed -i -E 's|^sync_hosts\s*=.*|sync_hosts = true|' "$CONFIG_FILE" 2>/dev/null || true

    systemctl reset-failed discord-bypass || true
    systemctl restart discord-bypass || true

    # Call fix_dns.sh automatically
    if [ -f "$ROOT_DIR/fix_dns.sh" ]; then
        "$ROOT_DIR/fix_dns.sh" >/dev/null 2>&1 || true
    fi
    echo -e "${GREEN}Configuration cleanly restored. DNS is using native router default.${NC}"
}
trap cleanup EXIT INT TERM

# Arrays to store results for table
STRAT_NAMES=()
NFQWS_STARTS=()
NFQ_WORKS=()
TLS_SUCCEEDS=()
REST_SUCCEEDS=()
FINAL_RESULTS=()

record_result() {
    STRAT_NAMES+=("$1")
    NFQWS_STARTS+=("$2")
    NFQ_WORKS+=("$3")
    TLS_SUCCEEDS+=("$4")
    REST_SUCCEEDS+=("$5")
    FINAL_RESULTS+=("$6")
}

test_candidate() {
    local name="$1"
    shift
    local raw_args=("$@")

    echo -e "\n------------------------------------------------------------"
    echo -e "${CYAN}Testing Strategy:${NC} ${BLUE}$name${NC}"
    echo -e "Arguments: ${raw_args[*]}"

    # Format args as TOML array of quoted strings
    local toml_array="["
    for ((i=0; i<${#raw_args[@]}; i++)); do
        if [ $i -gt 0 ]; then
            toml_array+=", "
        fi
        toml_array+="\"${raw_args[$i]}\""
    done
    toml_array+="]"

    # Write temporary config using sed
    sed -i -E "s|^strategy\s*=.*|strategy = \"strategy_c\"|" "$CONFIG_FILE"
    sed -i -E "s|^strategy_c_args\s*=.*|strategy_c_args = $toml_array|" "$CONFIG_FILE"
    sed -i -E "s|^sync_hosts\s*=.*|sync_hosts = true|" "$CONFIG_FILE"
    sed -i -E "s|^local_dns_port\s*=.*|local_dns_port = 0|" "$CONFIG_FILE"

    # Reset any rate limits and restart service cleanly
    systemctl reset-failed discord-bypass || true
    systemctl restart discord-bypass || true
    sleep 1.8

    # 1. Check if nfqws starts
    local nfqws_pid
    nfqws_pid=$(pgrep -f -o "discord-bypass-nfqws" || echo "")
    if [ -z "$nfqws_pid" ]; then
        echo -e "  ${RED}[FAIL]${NC} nfqws failed to start with these arguments"
        journalctl -u discord-bypass -n 15 --no-pager
        record_result "$name" "NO" "NO" "NO" "NO" "FAILED (nfqws start error)"
        return 1
    fi

    # Verify command line in /proc
    local proc_cmdline
    proc_cmdline=$(tr '\0' ' ' < "/proc/$nfqws_pid/cmdline")
    echo -e "  [OK] nfqws running (PID $nfqws_pid)"
    echo -e "  Cmdline: $proc_cmdline"

    # 2. Check initial NFQUEUE counters
    local init_chain q_pre
    init_chain=$(nft -a list chain inet discord_bypass output 2>/dev/null || echo "")
    q_pre=$(echo "$init_chain" | grep "queue flags bypass" | grep -o "packets [0-9]*" | awk '{print $2}' || echo 0)

    # 3. Test TLS connection to discord.com (pinned to authentic Cloudflare Discord IP)
    echo "  Testing TLS handshake: curl -v --resolve discord.com:443:162.159.137.232 https://discord.com ..."
    local tls_out tls_code tls_ok="NO"
    set +e
    tls_out=$(curl -s -I -v --connect-timeout 5 --max-time 15 --resolve discord.com:443:162.159.137.232 https://discord.com 2>&1)
    tls_code=$?
    set -e

    local post_chain q_post delta_q=0 nfq_ok="NO"
    post_chain=$(nft -a list chain inet discord_bypass output 2>/dev/null || echo "")
    q_post=$(echo "$post_chain" | grep "queue flags bypass" | grep -o "packets [0-9]*" | awk '{print $2}' || echo 0)
    delta_q=$(( ${q_post:-0} - ${q_pre:-0} ))

    if [ "$delta_q" -gt 0 ]; then
        nfq_ok="YES"
        echo -e "  ${GREEN}[OK]${NC} NFQUEUE intercepted packets (delta: +$delta_q)"
    else
        echo -e "  ${YELLOW}[WARN]${NC} No NFQUEUE packet delta ($delta_q)"
    fi

    if echo "$tls_out" | grep -qE "HTTP/[12](\.[0-9])? (200|301|302)"; then
        tls_ok="YES"
        local http_status
        http_status=$(echo "$tls_out" | grep -E "HTTP/[12]" | head -n1 | tr -d '\r\n')
        echo -e "  ${GREEN}[PASS] TLS Handshake succeeded!${NC} ($http_status)"
    else
        local err_summary
        err_summary=$(echo "$tls_out" | grep -E "(Recv failure|Connection reset|timed out|SSL_connect|alert decode|alert handshake|error:)" | head -n1 || echo "")
        if [ -z "$err_summary" ]; then
            err_summary=$(echo "$tls_out" | grep -E "^\* " | tail -n2 | tr '\n' ' ' || echo "curl exit code $tls_code")
        fi
        echo -e "  ${RED}[FAIL] TLS Handshake blocked:${NC} $err_summary"
    fi

    # 4. Test REST API if TLS passed
    local rest_ok="NO"
    if [ "$tls_ok" = "YES" ]; then
        echo "  Testing REST API: curl -v --resolve discord.com:443:162.159.137.232 https://discord.com/api/v10/gateway ..."
        local rest_out rest_code
        set +e
        rest_out=$(curl -s -I -v --connect-timeout 5 --max-time 15 --resolve discord.com:443:162.159.137.232 https://discord.com/api/v10/gateway 2>&1)
        rest_code=$?
        set -e
        if echo "$rest_out" | grep -qE "HTTP/[12](\.[0-9])? (200|401|429)"; then
            rest_ok="YES"
            local rest_status
            rest_status=$(echo "$rest_out" | grep -E "HTTP/[12]" | head -n1 | tr -d '\r\n')
            echo -e "  ${GREEN}[PASS] REST API reachable!${NC} ($rest_status)"
        else
            local rest_err
            rest_err=$(echo "$rest_out" | grep -E "(Recv failure|Connection reset|timed out|SSL_connect|alert|error:)" | head -n1 || echo "")
            if [ -z "$rest_err" ]; then
                rest_err=$(echo "$rest_out" | grep -E "^\* " | tail -n2 | tr '\n' ' ' || echo "curl code $rest_code")
            fi
            echo -e "  ${RED}[FAIL] REST API failed:${NC} $rest_err"
        fi
    fi

    local overall="FAILED"
    if [ "$tls_ok" = "YES" ] && [ "$rest_ok" = "YES" ]; then
        overall="SUCCESS"
        echo -e "  ${GREEN}>>> STRATEGY SUCCESSFUL FOR DISCORD! <<<${NC}"
    elif [ "$tls_ok" = "YES" ]; then
        overall="PARTIAL (TLS OK, REST failed)"
    else
        overall="FAILED (DPI Reset/Drop)"
    fi

    record_result "$name" "YES" "$nfq_ok" "$tls_ok" "$rest_ok" "$overall"

    if [ "$overall" = "SUCCESS" ]; then
        return 0
    else
        return 1
    fi
}

print_summary_table() {
    echo -e "\n${BLUE}========================================================================================================${NC}"
    echo -e "${BLUE}                                      STRATEGY TEST RESULTS TABLE                                       ${NC}"
    echo -e "${BLUE}========================================================================================================${NC}"
    printf "| %-38s | %-12s | %-13s | %-12s | %-13s | %-22s |\n" \
        "Strategy" "nfqws starts" "NFQUEUE works" "TLS succeeds" "REST succeeds" "Result"
    echo "|----------------------------------------|--------------|---------------|--------------|---------------|------------------------|"
    for ((i=0; i<${#STRAT_NAMES[@]}; i++)); do
        printf "| %-38s | %-12s | %-13s | %-12s | %-13s | %-22s |\n" \
            "${STRAT_NAMES[$i]}" "${NFQWS_STARTS[$i]}" "${NFQ_WORKS[$i]}" "${TLS_SUCCEEDS[$i]}" "${REST_SUCCEEDS[$i]}" "${FINAL_RESULTS[$i]}"
    done
    echo -e "${BLUE}========================================================================================================${NC}"
}

run_candidate_by_num() {
    local num="$1"
    case "$num" in
        1)
            test_candidate "fakedsplit (midsld, ttl=5)" \
                "--dpi-desync=fakedsplit" "--dpi-desync-split-pos=midsld" "--dpi-desync-ttl=5" || true
            ;;
        2)
            test_candidate "fakedsplit (midsld, ttl=4)" \
                "--dpi-desync=fakedsplit" "--dpi-desync-split-pos=midsld" "--dpi-desync-ttl=4" || true
            ;;
        3)
            test_candidate "fakedsplit (midsld, ttl=6)" \
                "--dpi-desync=fakedsplit" "--dpi-desync-split-pos=midsld" "--dpi-desync-ttl=6" || true
            ;;
        4)
            test_candidate "fakedsplit (midsld, ttl=3)" \
                "--dpi-desync=fakedsplit" "--dpi-desync-split-pos=midsld" "--dpi-desync-ttl=3" || true
            ;;
        5)
            test_candidate "fakedsplit (midsld, badsum)" \
                "--dpi-desync=fakedsplit" "--dpi-desync-split-pos=midsld" "--dpi-desync-fooling=badsum" || true
            ;;
        6)
            test_candidate "fakedsplit (midsld, badseq)" \
                "--dpi-desync=fakedsplit" "--dpi-desync-split-pos=midsld" "--dpi-desync-fooling=badseq" || true
            ;;
        7)
            test_candidate "fakedsplit (midsld, seqovl=16, ttl=5)" \
                "--dpi-desync=fakedsplit" "--dpi-desync-split-pos=midsld" "--dpi-desync-split-seqovl=16" "--dpi-desync-ttl=5" || true
            ;;
        8)
            test_candidate "fakedsplit (midsld, seqovl=16, ttl=4)" \
                "--dpi-desync=fakedsplit" "--dpi-desync-split-pos=midsld" "--dpi-desync-split-seqovl=16" "--dpi-desync-ttl=4" || true
            ;;
        9)
            test_candidate "fakeddisorder (midsld, ttl=5)" \
                "--dpi-desync=fakeddisorder" "--dpi-desync-split-pos=midsld" "--dpi-desync-ttl=5" || true
            ;;
        10)
            test_candidate "fakeddisorder (midsld, ttl=4)" \
                "--dpi-desync=fakeddisorder" "--dpi-desync-split-pos=midsld" "--dpi-desync-ttl=4" || true
            ;;
        11)
            test_candidate "fakeddisorder (midsld, ttl=6)" \
                "--dpi-desync=fakeddisorder" "--dpi-desync-split-pos=midsld" "--dpi-desync-ttl=6" || true
            ;;
        12)
            test_candidate "fakeddisorder (midsld, badsum)" \
                "--dpi-desync=fakeddisorder" "--dpi-desync-split-pos=midsld" "--dpi-desync-fooling=badsum" || true
            ;;
        13)
            test_candidate "fakedsplit (pos=2, ttl=5)" \
                "--dpi-desync=fakedsplit" "--dpi-desync-split-pos=2" "--dpi-desync-ttl=5" || true
            ;;
        14)
            test_candidate "fakedsplit (pos=2, ttl=4)" \
                "--dpi-desync=fakedsplit" "--dpi-desync-split-pos=2" "--dpi-desync-ttl=4" || true
            ;;
        15)
            test_candidate "fake,multisplit (midsld, ttl=5)" \
                "--dpi-desync=fake,multisplit" "--dpi-desync-split-pos=midsld" "--dpi-desync-ttl=5" || true
            ;;
        16)
            test_candidate "fake,multisplit (midsld, ttl=4)" \
                "--dpi-desync=fake,multisplit" "--dpi-desync-split-pos=midsld" "--dpi-desync-ttl=4" || true
            ;;
        17)
            test_candidate "fake,multisplit (pos=2, ttl=5)" \
                "--dpi-desync=fake,multisplit" "--dpi-desync-split-pos=2" "--dpi-desync-ttl=5" || true
            ;;
        18)
            test_candidate "fake,multisplit (pos=2, ttl=4)" \
                "--dpi-desync=fake,multisplit" "--dpi-desync-split-pos=2" "--dpi-desync-ttl=4" || true
            ;;
        19)
            test_candidate "fakedsplit (sniext+2, ttl=5)" \
                "--dpi-desync=fakedsplit" "--dpi-desync-split-pos=sniext+2" "--dpi-desync-ttl=5" || true
            ;;
        20)
            test_candidate "fakeddisorder (pos=2, ttl=5)" \
                "--dpi-desync=fakeddisorder" "--dpi-desync-split-pos=2" "--dpi-desync-ttl=5" || true
            ;;
        *)
            echo -e "${RED}Unknown candidate number: $num${NC}"
            echo "Available numbers: 1 to 20, or --all"
            return 1
            ;;
    esac
}

# Parse options
MODE="${1:-1}"


if [ "$MODE" = "--single" ]; then
    shift
    CAND_NAME="$1"
    shift
    test_candidate "$CAND_NAME" "$@" || true
    print_summary_table
    exit 0
fi

if [ "$MODE" = "--all" ]; then
    echo "Running complete systematic strategy sweep (20 candidates)..."
    for c in {1..20}; do
        run_candidate_by_num "$c" || true
    done
    print_summary_table
    exit 0
fi

# Run specific candidate number (default is 1)
echo "Running test for candidate #$MODE..."
run_candidate_by_num "$MODE"
print_summary_table
