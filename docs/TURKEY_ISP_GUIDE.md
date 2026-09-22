# Turkey ISP-Specific Guide & Troubleshooting

In Turkey, access to Discord was blocked on October 9, 2024 by an Ankara court decision enforced by the Information Technologies and Communication Authority (BTK). Different Turkish Internet Service Providers (ISPs) implement this restriction using distinct network hardware and filtering configurations.

This guide provides tested configurations and practical advice for each major Turkish ISP.

---

## 1. Turkcell Superonline (AS34984)

### Infrastructure & Dual-Stage Filtering Behavior
Turkcell Superonline deploys a coordinated two-stage censorship architecture:
1. **Stage 1 - DNS Poisoning / BTK Sinkhole:**
   - Superonline's default DNS servers (e.g. `213.74.x.x` or default gateway) resolve `discord.com` and related domains to BTK's official access block sinkhole: `195.175.254.2` (IPv4) and `2a01:358:4014:a00::3` (IPv6).
   - Port 443 on `195.175.254.2` hosts a government warning web server with an untrusted certificate (`localhost.localdomain` / `uyari.btk.gov.tr`). Any HTTPS request fails immediately with:
     `tls: failed to verify certificate: x509: certificate is not valid for any names, but wanted to match discord.com`
   - Because standard applications query systemd-resolved (`127.0.0.53`), traffic goes directly to BTK's server and **never reaches Discord's IPs**. This was the exact cause of the zero-counter symptom in initial testing.
2. **Stage 2 - Huawei DPI Middlebox SNI RST Injection:**
   - When connecting directly to legitimate Discord IPs (`162.159.128.0/20`), Superonline's Huawei aggregation middleboxes inspect the TLS `ClientHello` packet on TCP 443.
   - If `discord.com` is detected in a standard TLS segment, the middlebox injects a spoofed TCP `RST` to immediately abort the connection.

### The Working Solution
`discord-bypass` neutralizes both stages simultaneously:
- **Overcoming Stage 1 (DNS):**
  Set `sync_hosts = true` in `/etc/discord-bypass/config.toml`. The daemon queries DoH (Cloudflare 1.1.1.1) and atomically updates `/etc/hosts` with legitimate Discord IPs. In Linux libc, `/etc/hosts` is checked **before** DNS (`files` before `dns` in `/etc/nsswitch.conf`), guaranteeing the Discord client connects directly to Discord's genuine servers.
- **Overcoming Stage 2 (DPI):**
  Uses `strategy = "strategy_c"` (`split2` TCP segmentation) with the `nfqws` backend. Splitting the TLS ClientHello at the 2nd byte of the SNI prevents the Huawei DPI engine from reconstructing the forbidden domain name while remaining RFC-compliant for Cloudflare.
- **Voice Connectivity:**
  Ensure `prefer_ipv4 = true` and `block_quic = false`. Superonline often announces IPv6 routes that blackhole UDP traffic, so prioritizing IPv4 ensures snappy WebRTC voice connection.

### Recommended Configuration
```toml
[general]
backend = "nfqws"
strategy = "strategy_c"
prefer_ipv4 = true

[dns]
mode = "doh"
doh_provider = "cloudflare"
update_interval_sec = 60
sync_hosts = true
local_dns_port = 5354

[firewall]
driver = "auto"
block_quic = false
```

### Critical Prerequisite: "Güvenli İnternet"
If your Superonline subscription has the **"Güvenli İnternet" (Family or Child profile)** enabled:
- Superonline routes all blocked traffic directly to an IP blackhole or captive portal at the BRAS level, superseding any DPI bypass tool.
- **Fix:** Log in to the Superonline Online İşlemler portal (or use the Superonline mobile app) and select **"Standart Profil" (Güvenli İnternet Kapalı)**. The change takes effect within 5 minutes.

---

## 2. Türk Telekom / TTNet (AS9121)

### Infrastructure & Behavior
Türk Telekom utilizes Sandvine / Procera DPI solutions and extensive DNS hijacking.
- **Filtering Method:** Both transparent UDP 53 DNS interception and SNI inspection.
- Even if you configure `8.8.8.8` or `1.1.1.1` in your router or NetworkManager, plain UDP port 53 packets are intercepted and forged by TTNet edge routers.

### Recommended Configuration
- **Strategy:** `strategy_c` with `doh` enabled.
- **DNS Mode:** Ensure `mode = "doh"` and `doh_provider = "cloudflare"` or `"google"`.
- `discord-bypass` will query Cloudflare over encrypted HTTPS (port 443), evading TTNet's DNS hijacking completely.

---

## 3. Vodafone Net (AS15897)

### Infrastructure & Behavior
Vodafone applies standard SNI inspection and DNS manipulation.
- **Recommended Configuration:** `strategy_c` with `doh`.
- If connection drops after several minutes, switch to `strategy_d` (fake packet desynchronization with `ttl = 4`).

---

## 4. TurkNet (AS12735)

### Infrastructure & Behavior
TurkNet implements BTK-mandated blocks using standard SNI inspection.
- **Recommended Configuration:** `strategy_c` works out-of-the-box.
- TurkNet generally does not hijack external DoH traffic, making DNS over HTTPS 100% reliable.

---

## 5. Türksat Kablonet (AS47524)

### Infrastructure & Behavior
Kablonet inspects SNI on port 443.
- **Recommended Configuration:** `strategy_c` (split2) works cleanly.

---

## Diagnostic Workflow for Turkish Networks

When diagnosing connectivity on any Turkish ISP:

```bash
# 1. Run the 12-point diagnostic
discord-bypass diagnose
```

Examine the output lines:
1. **If DNS Resolution reports [WARN] or [FAIL]:**
   - Your ISP is poisoning DNS. Ensure `mode = "doh"` in `/etc/discord-bypass/config.toml`.
2. **If TLS SNI Test reports [FAIL] with "DPI Middlebox RST Injection DETECTED!":**
   - Confirms that SNI filtering is active on your line. Ensure the bypass service is running (`sudo discord-bypass start`).
3. **If Voice UDP Probe reports [WARN] or Discord voice status shows "No Route":**
   - Verify `prefer_ipv4 = true` in `/etc/discord-bypass/config.toml`.
   - In Discord Desktop App: Go to **User Settings -> Voice & Video -> Advanced**, and toggle **"Enable Quality of Service High Packet Priority"** off.
