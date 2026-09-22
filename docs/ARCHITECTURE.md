# discord-bypass: Technical Architecture

## 1. High-Level System Architecture

`discord-bypass` is built as a modular, two-tier Linux networking application:
1. **Control Plane & Daemon (Go 1.22+):**
   - Provides user-facing CLI operations (`install`, `start`, `stop`, `status`, `test`, `diagnose`, `emergency-disable`, `uninstall`).
   - Runs as a systemd service (`discord-bypass daemon`).
   - Manages the firewall drivers (`nftables` and `iptables`).
   - Runs the asynchronous DNS over HTTPS (DoH) resolver and dynamic IP set updater.
   - Supervises and health-checks the DPI packet engine backend.
2. **Data Plane / Packet Engine:**
   - **NFQUEUE Backend (`nfqws`):** High-performance C packet manipulation engine utilizing `libnetfilter_queue`.
   - **Native TCP Splitter Backend (`native`):** Pure-Go userspace transparent proxy providing zero-dependency TCP segmentation.

```
+---------------------------------------------------------------------------------+
|                                 USER SPACE                                      |
|                                                                                 |
|  +---------------------------------------------------------------------------+  |
|  |                 discord-bypass CLI & Daemon (Go)                          |  |
|  |  * CLI Subcommand Router (start, stop, status, test, diagnose, recovery)  |  |
|  |  * DoH Client & DNS Poisoning Detector (RFC 8484)                         |  |
|  |  * Dynamic IP Set Updater (Periodically queries DoH -> updates nftables)  |  |
|  |  * Firewall Manager (Isolated table inet discord_bypass)                  |  |
|  |  * 12-Point Comprehensive Diagnostic Engine                               |  |
|  +---------------------------------------------------------------------------+  |
|           |                                                      ^              |
|           | Supervises & passes args                             | Reads stats  |
|           v                                                      |              |
|  +-----------------------------------+        +------------------------------+  |
|  | C NFQUEUE Backend (nfqws)         |        | Native Go TCP Splitter Proxy |  |
|  | * Netfilter Queue handler         |        | * Transparent TCP listener   |  |
|  | * TLS ClientHello SNI extractor   |   OR   | * getsockopt SO_ORIGINAL_DST |  |
|  | * split2 TCP segmenter            |        | * split2 TLS record splitter |  |
|  | * Raw socket packet injection     |        | * TCP_NODELAY stream copy    |  |
|  +-----------------------------------+        +------------------------------+  |
|           ^                                                      ^              |
|           | Netlink NFQUEUE socket                               | REDIRECT     |
+-----------|------------------------------------------------------|--------------+
|           |                                                      |              |
|           v                                                      v              |
|  +---------------------------------------------------------------------------+  |
|  |                             LINUX KERNEL                                  |  |
|  |                                                                           |  |
|  |   [Outbound TCP 443 Packet from Discord Desktop / Web Browser]            |  |
|  |                                |                                          |  |
|  |                                v                                          |  |
|  |   nftables: table inet discord_bypass                                     |  |
|  |   - Match: ip daddr @discord_v4 && tcp dport 443                          |  |
|  |   - Drop:  ip daddr @discord_v4 && udp dport 443 (Forces TCP fallback)    |  |
|  |   - Action: queue num 200 bypass  OR  redirect to :10443                  |  |
|  |                                                                           |  |
|  |   (Unrelated traffic to banking, streaming, other sites is ACCEPTED)      |  |
+---------------------------------------------------------------------------------+
```

---

## 2. Traffic Scoping & Blast Radius Minimization

A fundamental design requirement is avoiding system-wide network interference. Many naive DPI scripts route all port 443 traffic through userspace or modify global MTU settings, causing slowdowns or breakage for unrelated websites.

`discord-bypass` solves this through a three-layer filter:
1. **Dynamic IP Sets in Kernel:**
   - The Go daemon resolves Discord domains via DoH (Cloudflare 1.1.1.1, Google 8.8.8.8, Quad9 9.9.9.9).
   - The resulting IP addresses are inserted into an interval set in nftables: `@discord_v4` and `@discord_v6`.
   - Only packets destined for these specific IP addresses ever match the rule.
2. **Layer 4 Port Scoping:**
   - Only TCP port 443 (HTTPS/WSS) is redirected or sent to the queue.
   - UDP port 443 (QUIC) is dropped specifically for Discord IPs to prevent browsers from using QUIC (which is not segmentable by standard TCP middlebox evasion).
3. **Application Layer (SNI) Validation:**
   - In `nfqws`, `--hostlist=/etc/discord-bypass/domains.txt` validates the TLS ClientHello Server Name Indication. If the domain is not in the allowlist, the packet is instantly issued `NF_ACCEPT` without modification.

---

## 3. Firewall Ruleset Structure

### Modern nftables (`table inet discord_bypass`)
```nft
table inet discord_bypass {
    set discord_v4 {
        type ipv4_addr
        flags interval
        elements = { 162.159.135.232, 162.159.136.232, 162.159.137.232, 162.159.138.232 }
    }

    set discord_v6 {
        type ipv6_addr
        flags interval
    }

    chain output {
        type filter hook output priority 0; policy accept;
        
        # Block QUIC for Discord to force standard TLS TCP
        ip daddr @discord_v4 udp dport 443 counter drop
        ip6 daddr @discord_v6 udp dport 443 counter drop

        # Send Discord TLS traffic to NFQUEUE with bypass flag
        ip daddr @discord_v4 tcp dport 443 counter queue num 200 bypass
        ip6 daddr @discord_v6 tcp dport 443 counter queue num 200 bypass
    }
}
```

#### Why `bypass` flag in `queue num 200 bypass`?
In Linux Netfilter, if an NFQUEUE rule does not have the `bypass` flag and userspace is temporarily unresponsive or terminated, the kernel will fail-closed and DROP packets, severing the user's internet. With `bypass`, if the userspace daemon is not running, the kernel **fails-open** and accepts the packets normally!

#### Atomic Cleanup
Tearing down the firewall requires only a single command:
```bash
nft delete table inet discord_bypass
```
Because all sets, chains, and rules exist entirely inside `table inet discord_bypass`, deleting this table leaves the rest of the user's firewall completely untouched.

---

## 4. DNS Architecture (Two-Stage DNS + DPI Mitigation)

### The Real-World Turkey ISP Problem (Turkcell Superonline Case Study)
In Türkiye, ISPs enforce a coordinated two-stage block:
1. **Stage 1 (DNS Poisoning / Sinkholing):** ISP resolvers return the official Türk Telekom / BTK court-order warning portal (`195.175.254.2` and `2a01:358:4014:a00::3`), serving an invalid certificate (`uyari.btk.gov.tr`). Because system applications query `127.0.0.53` (systemd-resolved), Discord desktop and browsers connect directly to the government warning server rather than Discord's infrastructure. Consequently, firewall rules targeting Discord's legitimate IPs never matched (resulting in zero packet counters).
2. **Stage 2 (SNI DPI Filtering):** If the client manages to reach Discord's genuine IP addresses, ISP middleboxes (e.g. Huawei DPI on Superonline, Sandvine on Türk Telekom) inspect the TLS ClientHello and inject TCP RST packets upon observing `discord.com`.

### The Safe DNS Resolution Hierarchy
Overwriting `/etc/resolv.conf` is dangerous and repeatedly overwritten by NetworkManager, DHCP, and VPNs. Instead, `discord-bypass` implements a multi-tier resolution strategy:
1. **Atomic `/etc/hosts` Tagged Block Synchronization:**
   - According to POSIX and Linux `/etc/nsswitch.conf`, `files` precedes `dns` for host resolution (`hosts: files mdns4_minimal [NOTFOUND=return] dns`).
   - The daemon resolves Discord endpoints via encrypted DoH (Cloudflare 1.1.1.1, Google 8.8.8.8, Quad9 9.9.9.9), verifies the returned IPs are not known sinkholes, and atomically writes an tagged block (`# --- BEGIN DISCORD-BYPASS ---`) into `/etc/hosts`.
   - On shutdown, `discord-bypass` cleanly strips this block, restoring the file to its original state.
2. **Loopback DNS Proxy & `systemd-resolved` Split DNS:**
   - A lightweight loopback DNS proxy runs on `127.0.0.1:5354`, forwarding incoming DNS queries directly over DoH.
   - If `resolvectl` (systemd-resolved) is present, the daemon registers split-DNS routing domains (`~discord.com`, `~discordapp.com`, `~discord.gg`, `~discordapp.net`, `~discord.media`) pointing only Discord traffic to `127.0.0.1:5354`. All unrelated queries continue using the standard ISP/local resolver.
3. **Pre-Seeded Cloudflare/Discord CIDRs:**
   - The nftables set `@discord_v4` and `@discord_v6` are pre-populated with Discord/Cloudflare netblocks (`162.159.128.0/20`, `162.159.135.0/24`, `104.16.0.0/13`) at initialization time, guaranteeing packets match even before the first dynamic DNS cycle completes.

---

## 5. Security & Privilege Model

- **Minimal Privileges in systemd:**
  The systemd unit does not run with unrestricted root. It utilizes Linux capabilities:
  - `AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW`
  - `CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW`
  - `ProtectSystem=full`
  - `ProtectHome=read-only`
  - `PrivateTmp=true`
- **Zero Sensitive Data Logging:**
  The logging subsystem actively filters and redacts tokens, session secrets, passwords, and message payloads. Only connection state and packet telemetry are logged.
- **Fail-Safe Emergency Restore:**
  Running `sudo discord-bypass emergency-disable` or stopping the service immediately removes all nftables tables, reverts `/etc/hosts`, reverts `systemd-resolved`, and terminates all packet processing processes.
