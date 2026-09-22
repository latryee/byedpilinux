# Security Policy

## Security Architecture & Design Principles

`discord-bypass` is built as an open-source, system-level Linux networking utility. Because it interacts with kernel Netfilter chains, system resolvers, and raw sockets, it is architected with strict least-privilege principles and zero-blast-radius controls:

### 1. Strict Traffic Scoping (Zero Blast Radius)
- `discord-bypass` **never** indiscriminately routes or queues all internet traffic.
- Only destinations within Discord's authoritative IP blocks (`@discord_v4` and `@discord_v6`) resolved via secure DNS over HTTPS (DoH) are queued to Netfilter NFQUEUE.
- Normal internet browsing, banking, gaming, and general network traffic completely bypass packet desynchronization and Netfilter queues.

### 2. Linux Capability Bounding
- The systemd unit (`discord-bypass.service`) isolates process capabilities using:
  ```ini
  AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW
  CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW
  ProtectSystem=full
  ProtectHome=read-only
  PrivateTmp=true
  ```
- The daemon does not run with unrestricted root superpowers; it is bounded strictly to raw socket creation and Netfilter queue interactions.

### 3. Loop Prevention
- Packet injection uses Netfilter marks (`0x40000000`).
- The Netfilter chain places `meta mark 0x40000000 counter accept` before any queue rule, mathematically preventing infinite reinjection loops.

### 4. Privacy & Zero Telemetry
- `discord-bypass` sends **zero telemetry, analytics, or tracking data** to any external server or third party.
- The network profile (`/etc/discord-bypass/network-profile.json`) is strictly local. It contains no public IP addresses, no MAC addresses, and no persistent device fingerprints.

### 5. Safe Command Execution
- All external binaries (`nfqws`, `systemctl`, `notify-send`) are invoked using explicit argument slices in Go (`exec.Command(binary, args...)`). Shell string concatenation (`sh -c`) is never used, eliminating shell injection attack surfaces.

### 6. Atomic File Operations
- All configuration changes (`config.toml`) and profile updates (`network-profile.json`) use temporary files with `fsync()` before atomic rename (`os.Rename`), preventing half-written or corrupted state during power loss or system crashes.

---

## Reporting a Vulnerability

If you discover a potential security vulnerability in `discord-bypass`, please do **not** open a public issue.

Instead, report it privately to the maintainers:
- **Email**: `latryee@proton.me` or via GitHub Private Vulnerability Reporting on the repository.
- Please include:
  1. Description of the vulnerability.
  2. Steps or proof-of-concept to reproduce.
  3. Affected distribution, kernel version, and component.

You will receive an acknowledgment within 48 hours and regular updates on the remediation status.
