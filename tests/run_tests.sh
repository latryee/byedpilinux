#!/usr/bin/env bash
set -euo pipefail

echo "============================================================"
echo "          discord-bypass Automated Test Suite              "
echo "============================================================"

# 1. Run Go Unit Tests
echo "[1/4] Running Go Unit Tests..."
go test -v ./tests/...

# 2. Check Binary Compilation
echo -e "\n[2/4] Verifying Binary Compilation..."
make build

# 3. Test CLI Help and Status Commands
echo -e "\n[3/4] Verifying CLI Commands (Unprivileged)..."
./bin/discord-bypass help >/dev/null
./bin/discord-bypass version
./bin/discord-bypass-nfqws --help >/dev/null
./bin/discord-bypass status >/dev/null

# 4. Verify Debian Package Build
echo -e "\n[4/4] Verifying Debian Packaging..."
make deb >/dev/null
if [ -f discord-bypass_1.0.0_amd64.deb ]; then
    echo "  [OK] discord-bypass_1.0.0_amd64.deb verified."
fi

echo -e "\n============================================================"
echo "          All automated tests completed successfully!       "
echo "============================================================"
