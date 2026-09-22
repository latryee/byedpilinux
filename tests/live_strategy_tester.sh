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
    echo "Candidates:"
    echo "  1  : fake,multisplit (pos=2, ttl=4) [First required test]"
    echo "  2  : fake,multisplit (pos=2, ttl=3)"
    echo "  3  : fake,multisplit (pos=2, ttl=5)"
    echo "  4  : fake,multisplit (pos=2, badsum)"
    echo "  5  : fake,multisplit (pos=2, badseq)"
    echo "  6  : multisplit (midsld)"
    echo "  7  : multisplit (sniext+2, midsld)"
    echo "  8  : multidisorder (pos=2)"
    echo "  9  : multidisorder (midsld)"
    echo "  10 : fake,multidisorder (pos=2, ttl=4)"
    echo "  11 : fake,multidisorder (midsld, ttl=4)"
    echo "  12 : fake,multidisorder (midsld, badsum)"
    echo "  13 : fake (badsum)"
    echo "  14 : fake (ttl=4)"
    echo "  15 : ipfrag2"
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
        systemctl restart discord-bypass || true
        echo -e "${GREEN}Configuration cleanly restored.${NC}"
    fi
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

    # Restart service cleanly
    systemctl restart discord-bypass
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

    # 3. Test TLS connection to discord.com
    echo "  Testing TLS handshake: curl -v --connect-timeout 5 --max-time 15 https://discord.com ..."
    local tls_out tls_code tls_ok="NO"
    set +e
    tls_out=$(curl -s -I -v --connect-timeout 5 --max-time 15 https://discord.com 2>&1)
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
        err_summary=$(echo "$tls_out" | grep -E "(Recv failure|Connection reset|timed out|SSL_connect)" | head -n1 || echo "curl exit code $tls_code")
        echo -e "  ${RED}[FAIL] TLS Handshake blocked:${NC} $err_summary"
    fi

    # 4. Test REST API if TLS passed
    local rest_ok="NO"
    if [ "$tls_ok" = "YES" ]; then
        echo "  Testing REST API: curl -v --connect-timeout 5 --max-time 15 https://discord.com/api/v10/gateway ..."
        local rest_out rest_code
        set +e
        rest_out=$(curl -s -I -v --connect-timeout 5 --max-time 15 https://discord.com/api/v10/gateway 2>&1)
        rest_code=$?
        set -e
        if echo "$rest_out" | grep -qE "HTTP/[12](\.[0-9])? (200|401|429)"; then
            rest_ok="YES"
            local rest_status
            rest_status=$(echo "$rest_out" | grep -E "HTTP/[12]" | head -n1 | tr -d '\r\n')
            echo -e "  ${GREEN}[PASS] REST API reachable!${NC} ($rest_status)"
        else
            local rest_err
            rest_err=$(echo "$rest_out" | grep -E "(Recv failure|Connection reset|timed out)" | head -n1 || echo "curl code $rest_code")
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
            test_candidate "fake,multisplit (pos=2, ttl=4)" \
                "--dpi-desync=fake,multisplit" "--dpi-desync-split-pos=2" "--dpi-desync-ttl=4" || true
            ;;
        2)
            test_candidate "fake,multisplit (pos=2, ttl=3)" \
                "--dpi-desync=fake,multisplit" "--dpi-desync-split-pos=2" "--dpi-desync-ttl=3" || true
            ;;
        3)
            test_candidate "fake,multisplit (pos=2, ttl=5)" \
                "--dpi-desync=fake,multisplit" "--dpi-desync-split-pos=2" "--dpi-desync-ttl=5" || true
            ;;
        4)
            test_candidate "fake,multisplit (pos=2, badsum)" \
                "--dpi-desync=fake,multisplit" "--dpi-desync-split-pos=2" "--dpi-desync-fooling=badsum" || true
            ;;
        5)
            test_candidate "fake,multisplit (pos=2, badseq)" \
                "--dpi-desync=fake,multisplit" "--dpi-desync-split-pos=2" "--dpi-desync-fooling=badseq" || true
            ;;
        6)
            test_candidate "multisplit (midsld)" \
                "--dpi-desync=multisplit" "--dpi-desync-split-pos=midsld" || true
            ;;
        7)
            test_candidate "multisplit (sniext+2, midsld)" \
                "--dpi-desync=multisplit" "--dpi-desync-split-pos=sniext+2,midsld" || true
            ;;
        8)
            test_candidate "multidisorder (pos=2)" \
                "--dpi-desync=multidisorder" "--dpi-desync-split-pos=2" || true
            ;;
        9)
            test_candidate "multidisorder (midsld)" \
                "--dpi-desync=multidisorder" "--dpi-desync-split-pos=midsld" || true
            ;;
        10)
            test_candidate "fake,multidisorder (pos=2, ttl=4)" \
                "--dpi-desync=fake,multidisorder" "--dpi-desync-split-pos=2" "--dpi-desync-ttl=4" || true
            ;;
        11)
            test_candidate "fake,multidisorder (midsld, ttl=4)" \
                "--dpi-desync=fake,multidisorder" "--dpi-desync-split-pos=midsld" "--dpi-desync-ttl=4" || true
            ;;
        12)
            test_candidate "fake,multidisorder (midsld, badsum)" \
                "--dpi-desync=fake,multidisorder" "--dpi-desync-split-pos=midsld" "--dpi-desync-fooling=badsum" || true
            ;;
        13)
            test_candidate "fake (badsum)" \
                "--dpi-desync=fake" "--dpi-desync-fooling=badsum" || true
            ;;
        14)
            test_candidate "fake (ttl=4)" \
                "--dpi-desync=fake" "--dpi-desync-ttl=4" || true
            ;;
        15)
            test_candidate "ipfrag2" \
                "--dpi-desync=ipfrag2" || true
            ;;
        *)
            echo -e "${RED}Unknown candidate number: $num${NC}"
            echo "Available numbers: 1 to 15, or --all"
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
    echo "Running complete systematic strategy sweep (15 candidates)..."
    for c in {1..15}; do
        run_candidate_by_num "$c" || true
    done
    print_summary_table
    exit 0
fi

# Run specific candidate number (default is 1)
echo "Running test for candidate #$MODE..."
run_candidate_by_num "$MODE"
print_summary_table
