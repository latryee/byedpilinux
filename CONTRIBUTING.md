# Contributing to discord-bypass

Thank you for your interest in contributing to `discord-bypass`! This project is maintained to provide a reliable, safe, and transparent Linux networking tool for users affected by DPI and DNS filtering.

---

## 🇹🇷 How to Submit ISP Compatibility Reports

If you are using an ISP in Türkiye (e.g., Türk Telekom, Turkcell Superonline, TurkNet, Vodafone, Kablonet, NetSpeed, Millenicom), you can help improve the auto-tuner candidate matrix:

1. Run the auto-tuner:
   ```bash
   sudo discord-bypass tune
   ```
2. Run the diagnostic suite:
   ```bash
   discord-bypass diagnose
   ```
3. Open a GitHub Issue using our **ISP Compatibility Report** template, including:
   - ISP Name & Autonomous System Number (ASN)
   - City / Region
   - Which candidate strategy succeeded (or failed)
   - Diagnostic output snippet (ensure no personal sensitive data is included)

---

## 💻 Code Contributions

### Development Environment Prerequisites
- Go 1.22+
- `nftables` or `iptables`
- `libnetfilter-queue-dev`
- `make`, `gcc`

### Building & Testing Locally
```bash
# Run unit and integration tests
make test

# Run Go static analysis
go vet ./...

# Build local binaries
make build

# Build Debian .deb package
make deb
```

### Pull Request Guidelines
1. **Preserve Verified Configurations:** Never break or alter existing verified working paths (such as `fakedsplit midsld ttl=6` on Turkcell Superonline) without reproducible evidence and community verification.
2. **Never Add Telemetry:** The project strictly avoids remote telemetry, phone-home features, and tracking identifiers.
3. **FHS Compliance:** Package files must strictly follow the Filesystem Hierarchy Standard (`/usr/bin`, `/lib/systemd/system`, `/etc/discord-bypass`).
4. **All Tests Must Pass:** Ensure `go test -v ./tests/...` and `go vet ./...` pass with zero errors before opening a pull request.
5. **No Blind Shell Invocations:** Subprocesses must be executed via `exec.Command` with argument slices, never raw `sh -c` strings.

---

## Community & Questions
Feel free to open an issue or start a discussion on GitHub for questions, suggestions, or edge-case network setups.
