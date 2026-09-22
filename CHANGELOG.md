# Changelog

All notable changes to the `discord-bypass` project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [1.0.0] - 2026-09-23 - Production Release

### Added
- **Automatic Strategy Tuning (`discord-bypass tune`)**:
  - Automatically identifies whether direct Discord connection is working or filtered.
  - Sweeps prioritized candidate evasion strategies with bounded per-candidate timeouts.
  - Validates candidates against live Discord Gateway (HTTP 101/200) and REST API (HTTP 200/401).
  - Automatically persists the first verified working candidate to `/etc/discord-bypass/config.toml`.
  - Features transactional automatic rollback to previous configuration if candidates fail or tuning is aborted.
- **Native Strategy Management Commands**:
  - `discord-bypass strategy list`: Formatted table showing all 9 registered strategies and candidates.
  - `discord-bypass strategy current`: Displays active strategy parameters, verification status, method, and timestamp.
  - `sudo discord-bypass strategy set <id>`: Safely switches strategies with connectivity verification and rollback.
- **Privacy-Safe Local Network Profile**:
  - Saves locally to `/etc/discord-bypass/network-profile.json` (`0644`).
  - Tracks verification status without telemetry, public IPs, MAC addresses, or tracking identifiers.
- **Centralized Verification Engine (`src/pkg/strategy/verifier.go`)**:
  - Unified 4-stage connectivity probe (DNS, TCP 443, TLS 1.3 ClientHello, Discord Layer-7 Gateway & REST).
- **Desktop Environment Integration**:
  - Standard XDG Desktop Entry (`/usr/share/applications/discord-bypass.desktop`).
  - Scalable vector application icon (`/usr/share/icons/hicolor/scalable/apps/discord-bypass.svg`).
  - Unprivileged desktop notification command (`discord-bypass notify-status`) via `notify-send`.
- **Packaging Polish & FHS Compliance**:
  - Debian package (`.deb`) installs binaries to `/usr/bin/` and unit to `/lib/systemd/system/`.
  - Added `packaging/debian/postrm` supporting complete configuration cleanup on `apt purge`.
  - Archive ownership set to `root:root` via `--root-owner-group`.
- **Documentation**:
  - Native Turkish guide (`README.tr.md`) covering Turkish ISP setups, "Güvenli İnternet" profile handling, and commands.
  - `SECURITY.md`, `CONTRIBUTING.md`, and structured GitHub issue templates.

### Changed
- Upstream packet desynchronization engine upgraded to vendored `bol-van/zapret` `nfqws` v72.13.
- Primary verified default on Turkcell Superonline fiber set to `fakedsplit midsld ttl=6`.
- Diagnostic voice probe updated to `latency.discord.media:443`.
- Unprivileged CLI diagnostics now query `/run/discord-bypass/status.json` for live telemetry without requiring `sudo`.

---

## [0.9.0] - 2026-09-22 - Prototype Baseline
- Initial Go supervisor and netfilter queue implementation.
- Basic DoH updater with `/etc/hosts` sync.
- Dedicated `nftables` isolation with loop prevention mark `0x40000000`.
- 12-point network diagnostic prototype.
