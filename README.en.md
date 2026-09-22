<div align="center">

# 🎮 discord-bypass (ByeDPI Linux)

**Zero-Latency (0 ms Added Ping) Linux Discord & Roblox Access Utility Optimized for Türkiye**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![Platform: Linux](https://img.shields.io/badge/Platform-Linux-FCC624?style=for-the-badge&logo=linux&logoColor=black)](https://kernel.org/)
[![Supported Services](https://img.shields.io/badge/Supported-Discord%20%2B%20Roblox-5865F2?style=for-the-badge)](#-overview)
[![Ping Impact](https://img.shields.io/badge/Ping%20Impact-0%20ms-brightgreen?style=for-the-badge)](#-why-not-vpn-vpn-vs-discord-bypass)
[![Privacy](https://img.shields.io/badge/Privacy-100%25%20Local-blue?style=for-the-badge)](#-security-and-zero-blast-radius)
[![Turkish ISPs](https://img.shields.io/badge/Turkish%20ISPs-Verified-success?style=for-the-badge)](#-turkish-isp-compatibility-matrix)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=for-the-badge)](LICENSE)

<br/>

[🇹🇷 Türkçe Dokümantasyon](README.md) | 🇬🇧 **English**

</div>

---

## 📌 Overview

**discord-bypass** is an open-source, kernel-integrated network utility built specifically to circumvent ISP-level **DNS poisoning** and **SNI/DPI (Deep Packet Inspection)** filtering on **Discord** and **Roblox** across Turkish internet service providers (Turkcell Superonline, Türk Telekom, TurkNet, Vodafone, Kablonet).

### Supported Platforms & Services:
- 🎧 **Discord:** Desktop Client (.deb, Flatpak, Snap), Web Client, Gateway WebSocket, CDN Media, and **WebRTC Voice Channels** (0 ms added ping).
- 🧱 **Roblox:** Main Website (`roblox.com`), APIs, Asset/Game Download CDNs (`setup.rbxcdn.com`, `rbxcdn.com`), and Linux game runners (**Sober**, **Vinegar**, **Wine/Proton**).

Unlike cumbersome VPNs that slow down your entire connection or spike your gaming ping, **discord-bypass** selectively intercepts and desynchronizes only Discord and Roblox handshakes. All other traffic (games, browser, banking, streaming) routes natively at full ISP speed.

---

## ⚡ Why Not a VPN? (VPN vs discord-bypass)

| Comparison Metric | Traditional VPN | `discord-bypass` |
| :--- | :---: | :---: |
| **In-Game Ping Impact** | ❌ **+40 - 150 ms** (Severe latency) | 🟢 **0 ms** (Game traffic connects directly) |
| **Discord Voice & Roblox Speed**| ⚠️ Bottlenecked by VPN server load | 🟢 **100% Native ISP Bandwidth** |
| **System Resource Usage** | ⚠️ High CPU / Constant encryption | 🟢 **< 15 MB RAM** (Lightweight Go Daemon) |
| **Privacy & Security** | ❌ All traffic routes through 3rd party servers | 🟢 **100% Local** (No external proxies or tunnels) |
| **Local Services & Banking** | ❌ Blocked or triggers fraud alerts | 🟢 **Zero Impact** (Your authentic IP is preserved) |
| **Desktop Usability** | ⚠️ Requires manual connection each session | 🟢 **Desktop GUI & Background systemd Service** |

---

## 🚀 One-Line Quick Install

Open your terminal and paste:

```bash
curl -fsSL https://raw.githubusercontent.com/latryee/byedpilinux/main/install.sh | sudo bash
```

This automated script:
1. Detects your Linux distribution (`Ubuntu`, `Debian`, `Zorin`, `Linux Mint`, `Fedora`, `Arch`).
2. Configures lightweight requirements (`nftables`, `libnetfilter-queue1`).
3. Installs and enables the background service.
4. Places a clickable shortcut directly onto your **Desktop** (`~/Desktop` or `~/Masaüstü`) and applications menu.
5. Runs `tune` to automatically test and select the verified strategy for your current network.

---

### Alternative Installation Methods

<details>
<summary><b>📦 Debian / Ubuntu / Mint / Zorin Pre-built .deb Package</b></summary>

Download the latest `.deb` package from the [Releases](https://github.com/latryee/byedpilinux/releases) page or install via CLI:

```bash
sudo apt install ./discord-bypass_1.0.0_amd64.deb
```
</details>

<details>
<summary><b>🏹 Arch Linux / Manjaro / CachyOS (AUR / PKGBUILD)</b></summary>

```bash
git clone https://github.com/latryee/byedpilinux.git
cd byedpilinux/packaging/arch
makepkg -si
```
</details>

---

## 📶 Turkish ISP Compatibility Matrix

The following strategies have been tested and verified live on actual Turkish ISP connections:

| Internet Service Provider (ISP) | Status | Verified Strategy | Technical Note |
| :--- | :---: | :--- | :--- |
| **Turkcell Superonline** | 🟢 Verified | `strategy_c` (`fakedsplit midsld ttl=6`) | Defeats Huawei DPI middlebox RST injection. |
| **Türk Telekom (TTNet)** | 🟢 Verified | `strategy_c_ttl5` / `strategy_b` | Bypasses Port 53 DNS hijacking and SNI RSTs. |
| **TurkNet** | 🟢 Verified | `strategy_b` (DoH) / `strategy_c` | Resolves unpoisoned IPs via secure DoH engine. |
| **Vodafone Net** | 🟢 Verified | `strategy_c` / `strategy_d` | Desynchronizes outbound SNI segments past DPI. |
| **Türksat Kablonet** | 🟢 Verified | `strategy_c` (`split2`) | Handles Port 443 TCP reset circumvention. |
| **Millenicom / NetSpeed** | 🟢 Verified | `strategy_c` | Inherits underlying TT or Superonline profile. |

---

## 💻 Desktop Graphical Control Panel (GUI)

For users who prefer a modern graphical interface, **discord-bypass-gui** provides a clean, Discord-themed dark mode panel:

- **Launch from Desktop or Menu:** Click the "Discord & Roblox Bypass" icon on your Desktop or open it via **Applications -> Internet -> Discord & Roblox Bypass**.
- **Live Visual Status:**
  - 🟢 **Connection & Strategy:** Real-time indicator showing active and verified strategy.
  - 🎙️ **Discord Voice (WebRTC):** Direct UDP voice connectivity confirmation with 0 ms ping.
  - 🧱 **Roblox Access:** Real-time web and game client reachability status.
  - 🛡️ **Firewall Counters:** Number of intercepted and desynchronized packets.
- **Graphical One-Click Actions:**
  - `[ ⚡ Auto-Tune Network ]`: Benchmarks all 9 bypass candidates live with graphical authentication (`pkexec`) and activates the lowest-latency working strategy.
  - `[ 🔄 Test Connection ]`: Runs a 2-second live Discord and Roblox connectivity test.
  - `[ 🛠️ Diagnose Network ]`: Runs a deep 12-point network and ISP diagnostic directly into the embedded log console.
  - `[ 🛑 Start / Stop Service ]`: Toggle the background systemd service with a single click.

---

## 🛠️ CLI Commands (For Power Users)

```bash
# Display service, firewall, voice, and Roblox connection status
discord-bypass status

# Automatically benchmark and apply the verified strategy for your network
sudo discord-bypass tune

# Launch the desktop graphical control panel
discord-bypass-gui

# Inspect currently active strategy and verification details
discord-bypass strategy current

# List all available strategies and candidates
discord-bypass strategy list

# Manually switch strategy with automatic connectivity rollback
sudo discord-bypass strategy set strategy_c

# Trigger a desktop notification with current connection status
discord-bypass notify-status

# Run deep 12-point network and ISP diagnostic
discord-bypass diagnose

# Start network interface watcher (auto-adapts when roaming Wi-Fi / Hotspot)
discord-bypass watch

# View live systemd service journal logs
discord-bypass logs
```

---

## ⚠️ Important: Superonline "Secure Internet" Note

On Turkcell Superonline fiber connections, if **"Güvenli İnternet" (Family or Child Profile)** is enabled on your ISP account, DPI packets are rejected directly at the BRAS/central exchange level.

If `sudo discord-bypass tune` fails all candidates on Superonline:
1. Log in to your **Turkcell Superonline Online İşlemler** account.
2. Navigate to **Güvenli İnternet** settings.
3. Switch profile to **"Standart Profil"** (Unfiltered / Standard Internet).
4. Reboot your fiber router and run again:
   ```bash
   sudo discord-bypass tune
   ```

---

## 🔒 Security and Zero Blast Radius

- **Dedicated Firewall Isolation:** `discord-bypass` creates an isolated table in modern `nftables` (`table inet discord_bypass`). It never flushes or interferes with your existing firewall (UFW, Docker, iptables).
- **Targeted Services Only:** Only verified Discord and Roblox domains and IP sets (`@discord_v4` and `@discord_v6`) are scoped. Gaming, banking, video streaming, and normal browsing are completely unaffected.
- **Zero Telemetry:** Zero analytics, IP tracking, or external server calls. Everything executes 100% locally on your machine.
- **Failsafe Emergency Disable:** Purge all rules and revert DNS instantaneously with:
  ```bash
  sudo discord-bypass emergency-disable
  ```

---

## 🤝 Contributing

Community feedback is welcomed! If you are testing on different ISPs or regions in Türkiye, submit an ISP Compatibility Report on [GitHub Issues](https://github.com/latryee/byedpilinux/issues).

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
The low-level DPI desynchronization core is powered by [bol-van/zapret](https://github.com/bol-van/zapret).
