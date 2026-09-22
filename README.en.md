# discord-bypass

<div align="center">
[🇹🇷 Türkçe Dokümantasyon](README.md) | 🇬🇧 **English**
</div>

> **Production Release (v1.0.0)**: Powered by vendored upstream `bol-van/zapret` `nfqws` v72.13 with a robust Go supervisor and dedicated `nftables` isolation.
> Live-tested, confirmed, and verified on Turkish ISP networks (including Turkcell Superonline AS34984 and Türk Telekom) with full access to Discord Desktop, Web, Gateway, CDN, and Voice — with **zero impact** on system-wide DNS or unrelated traffic.

A reliable, production-grade, open-source Linux application for circumventing ISP-level DNS poisoning and SNI-based Deep Packet Inspection (DPI) filtering to restore full connectivity to **Discord** (Desktop App, Web Client, Gateway WebSocket, CDN Assets, and WebRTC Voice).

Designed specifically for users in **Türkiye** (Türk Telekom, Turkcell Superonline, Vodafone, TurkNet, Kablonet) and other regions where Discord connections are disrupted by middlebox firewalls.

---

## Highlights & Engineering Features

- **Linux-Native DPI Circumvention:** Provides functionality comparable to Windows *GoodbyeDPI* and *Zapret*, tailored and packaged as a standard Linux service.
- **Strict Traffic Scoping (Zero Blast Radius):** Does **NOT** blindly intercept or alter all internet traffic. Resolves Discord domains via secure DNS over HTTPS (DoH) and dynamically maintains an isolated IP set (`@discord_v4` and `@discord_v6`) in nftables. Your banking, streaming, and general web traffic are completely untouched.
- **Safe Firewall Management:** Creates an isolated, dedicated table (`table inet discord_bypass`) in modern `nftables` (with legacy `iptables` fallback). **Never flushes your existing firewall rules**, never disrupts Docker or UFW, and cleanly tears down rules on stop or uninstall.
- **Built-in DNS over HTTPS (DoH):** Automatically evades UDP port 53 DNS poisoning and transparent DNS hijacking. Detects poisoned addresses (such as `0.0.0.0` or ISP block page redirects). Fully compatible with NetworkManager and systemd-resolved without overwriting `/etc/resolv.conf`.
- **IPv4 Preference Option:** Many Turkish ISPs feature broken or blackholed IPv6 routes for Discord. `prefer_ipv4 = true` avoids IPv6 handshake hangs while keeping local IPv6 intact for other services.
- **Multi-Backend DPI Engine:**
  - **NFQUEUE (`nfqws`) Backend** *(Default / High Performance)*: Transparent packet manipulation using Netfilter queue (`libnetfilter_queue`). Supports `split2` (segmenting ClientHello at byte 2), `fake` packets, and `disorder2`.
  - **Native Transparent TCP Splitter** *(Pure Go Fallback)*: Zero-dependency built-in proxy that accepts redirected TCP connections and segments TLS ClientHello in userspace.
  - **ByeDPI (`ciadpi`) Backend**: Userspace proxy engine support.
- **Comprehensive 12-Point Diagnostics:** `discord-bypass diagnose` tests DNS, IPv4, IPv6, TCP port 443, TLS SNI RST injection, REST API, WebSocket Gateway, CDN, Voice UDP, firewall counters, and identifies your Turkish ISP with customized recommendations.
- **Failsafe Emergency Disable:** `sudo discord-bypass emergency-disable` immediately purges all firewall tables and stops processes, guaranteeing your machine is never left without network connectivity.

---

## Quick Start

### 1. Installation

#### Option A: One-Line Script (Ubuntu, Debian, Zorin, Mint, Fedora, Arch)
```bash
git clone https://github.com/latryee/byedpilinux.git
cd byedpilinux
sudo ./install.sh
```

#### Option B: Debian/Ubuntu `.deb` Package
```bash
make deb
sudo dpkg -i discord-bypass_1.0.0_amd64.deb
```

---

### 2. Basic Usage

```bash
# Check service, firewall, and connection status
discord-bypass status

# Automatically detect, verify, and apply a working strategy for your network
sudo discord-bypass tune

# Inspect available strategies and candidate variants
discord-bypass strategy list

# View currently active strategy, parameters, and verification status
discord-bypass strategy current

# Manually switch to a specific strategy (with verification & automatic rollback)
sudo discord-bypass strategy set strategy_c

# Run non-destructive connectivity verification
discord-bypass test

# Run deep 12-point network diagnostic
discord-bypass diagnose

# Start the bypass service (if not already running)
sudo discord-bypass start

# Stop the bypass service and cleanly deactivate firewall rules
sudo discord-bypass stop

# View recent service logs
discord-bypass logs

# Failsafe emergency recovery (purges all rules instantly)
sudo discord-bypass emergency-disable

# Clean uninstallation
sudo discord-bypass uninstall
```

---

### Strategy Management & Auto-Tuning

| Workflow | Command | Privileges | Behavior |
| :--- | :--- | :--- | :--- |
| **Automatic Tuning** | `sudo discord-bypass tune` | `sudo` | Automatically determines if direct connectivity works; if blocked, tests candidates sequentially, verifies with Layer 7 probes, persists the working strategy, and updates the local profile. |
| **Strategy Listing** | `discord-bypass strategy list` | Unprivileged | Shows all available strategies and candidate variants with `[ACTIVE]` and `[VERIFIED]` flags. |
| **Current Strategy** | `discord-bypass strategy current` | Unprivileged | Shows active strategy, parameters, verification source, method, and timestamp. |
| **Manual Selection** | `sudo discord-bypass strategy set <id>` | `sudo` | Applies strategy, restarts service, validates reachability with Layer 7 probe, and automatically rolls back if the probe fails (use `--force` to bypass check). |


## How It Works (DPI Evasion Mechanism)

When an ISP blocks Discord via SNI filtering:
1. Your Discord client initiates a standard TCP handshake to a Discord IP on port 443 (`SYN` -> `SYN/ACK` -> `ACK`).
2. The client transmits the **TLS ClientHello** packet containing the server name (`discord.com` or `gateway.discord.gg`).
3. An ISP middlebox (e.g. Huawei, Sandvine, Procera) inspects the packet, detects the forbidden SNI, and injects a spoofed **TCP RST (Reset)** packet towards your computer, abruptly terminating the connection.

### The Verified Strategy C (`fakedsplit midsld ttl=6`) Solution:
On Turkish ISPs (notably Turkcell Superonline AS34984), middleboxes utilize aggressive stateful tracking that intercepts pure two-byte splits. To reliably bypass this:
- **Fake Segment (TTL=6):** `discord-bypass-nfqws` sends an initial fake TLS ClientHello segment with an IP TTL of 6. The ISP's edge DPI middlebox (located 3–5 hops away) inspects the fake segment and desynchronizes its tracking state. Because the TTL is restricted to 6, the fake segment expires in the ISP core and **never reaches Cloudflare's Frankfurt edge** (~11 hops away).
- **Split ClientHello (`midsld`):** The real TLS ClientHello is segmented right in the middle of the second-level domain name (`disc` + `ord.com`).
- **Cloudflare Edge Reassembly:** The Cloudflare destination server reassembles the real TCP stream, verifies the TLS 1.3 handshake, and completes encrypted communication without reset.

---

## Documented Strategies

| Strategy | Name | Description | Recommended For |
| :--- | :--- | :--- | :--- |
| **Strategy A** | Direct (Baseline) | Normal connection without bypass. Used to benchmark ISP filtering. | Baseline testing |
| **Strategy B** | Secure DNS Workaround | Routes DNS queries over HTTPS (DoH) without packet manipulation. | ISPs with DNS-only blocking |
| **Strategy C** | Fake Split (`fakedsplit`, TTL=6) | Splits ClientHello at mid-SLD with a 6-hop fake segment. | **Default: Verified for Superonline & Türk Telekom** |
| **Strategy D** | Fake Out-of-Order (`fakeddisorder`, TTL=6) | Out-of-order fake split with TTL=6. | Alternative verified strategy |

To change strategies, edit `/etc/discord-bypass/config.toml`:
```toml
[general]
strategy = "strategy_c" # or "strategy_d"
```
and restart the service:
```bash
sudo discord-bypass restart
```

---

## Diagnostic Output Example

Running `discord-bypass diagnose` outputs:

```text
============================================================
      discord-bypass - Linux DPI Circumvention Tool        
               Optimized for Discord in TR                  
============================================================
Running comprehensive 12-point network diagnostic...

[INFO]  ISP Detection             : Turkcell Superonline (AS34984, TR)
[OK]    DNS Resolution            : Clean (Resolved 4 IPs: [162.159.135.232 162.159.136.232])
[OK]    IPv4 Connectivity         : Outbound IPv4 routing is active
[INFO]  IPv6 Connectivity         : IPv6 is not configured or disabled on local network
[OK]    TCP Handshake (443)       : 3-way handshake established in 24.1ms
[OK]    TLS SNI Test (discord.com): Negotiated TLS 1.3 (TLS_AES_256_GCM_SHA384)
[OK]    Discord REST API          : HTTP 200 OK (28 bytes read)
[OK]    Gateway (wss)             : Gateway endpoint reached (HTTP 400, latency 26.3ms)
[OK]    Discord CDN               : HTTP 200 OK (512 bytes read)
[OK]    Voice UDP Probe           : Voice UDP socket open (voice.discord.media:443)
[OK]    Service Status            : Systemd service 'discord-bypass' is active and running
[OK]    Firewall Rules            : Rules active on nftables (Processed: 142 packets, 62410 bytes)

============================================================
OVERALL VERDICT: Discord Connectivity is FULLY FUNCTIONAL!
RECOMMENDATION : All Discord subsystems (DNS, TLS, REST API, WebSocket Gateway, CDN) are responding properly.
ISP INSIGHT    : Turkcell Superonline utilizes Huawei DPI middleboxes injecting TCP RST upon seeing discord.com SNI. Strategy C (split2) or Strategy D (fake+split2) is highly effective. If voice RTC connects slowly, verify IPv4 preference is enabled.
============================================================
```

---

## Turkish ISP Specific Notes

- **Turkcell Superonline:**
  - Employs Huawei DPI middleboxes with aggressive TCP RST injection upon reading the SNI.
  - **Fix:** Strategy C (`split2`) bypasses this cleanly.
  - *Important:* If your Superonline line has **"Güvenli İnternet" (Family/Child Profile)** activated, this must be turned off in the Superonline Online İşlemler portal, otherwise the ISP blocks traffic by IP address.
- **Türk Telekom (TTNet):**
  - Frequently hijacks standard UDP port 53 DNS queries and applies SNI inspection.
  - **Fix:** Strategy C with DoH enabled resolves both DNS poisoning and SNI inspection.
- **Vodafone Net:**
  - Applies SNI inspection and DNS manipulation.
  - **Fix:** Strategy C works reliably.
- **TurkNet & Kablonet:**
  - Follows BTK court orders via standard SNI inspection.
  - **Fix:** Strategy C works out-of-the-box.

---

## Configuration (`/etc/discord-bypass/config.toml`)

```toml
[general]
log_level = "info"
domains_file = "/etc/discord-bypass/domains.txt"
prefer_ipv4 = true
backend = "nfqws"          # "nfqws", "native", or "byedpi"
strategy = "strategy_c"     # "strategy_a", "strategy_b", "strategy_c", "strategy_d"

[dns]
mode = "doh"               # "doh", "system", "custom"
doh_provider = "cloudflare"# "cloudflare", "google", "quad9"
update_interval_sec = 300  # Refresh dynamic IP set every 300s
sync_hosts = false         # Handled cleanly by systemd-resolved; keep false by default
local_dns_port = 5354      # Optional local loopback DoH DNS proxy

[firewall]
driver = "auto"            # "auto", "nftables", "iptables"
table_name = "discord_bypass"
queue_num = 200
proxy_port = 10443
manage_ipv6 = true
block_quic = false         # False allows WebRTC voice UDP without interference
```

---

## Testing & Verification

Run the automated unit and dry-run test suite:
```bash
make test
```

Run the automated real host validation suite (checks packaging, nfqws, nftables loop prevention, DNS, and crash recovery):
```bash
sudo ./tests/host_validation.sh
```

Run the deterministic live strategy testing suite (systematically benchmarks candidate DPI desynchronization modes against live Discord endpoints):
```bash
# Test candidate 1 (fake,multisplit ttl=4 pos=2):
sudo ./tests/live_strategy_tester.sh 1

# Run complete systematic candidate sweep (15 desync modes):
sudo ./tests/live_strategy_tester.sh --all
```

---

## Uninstallation

To completely remove `discord-bypass` and restore your network to its default state:
```bash
sudo ./uninstall.sh
```
To purge configuration files as well:
```bash
sudo ./uninstall.sh --purge
```

---

## License

Released under the [MIT License](LICENSE).
Inspiration and Netfilter packet manipulation principles adapted from mature open-source implementations including *Zapret* (MIT) and *ByeDPI* (MIT).
