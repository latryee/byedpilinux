<div align="center">

# 🚀 ByeDPI Linux (Discord & Roblox Bypass)

**Türkiye için Optimize Edilmiş, Sıfır Gecikmeli (0 ms Ping) Linux Discord & Roblox Erişim Aracı**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![Platform: Linux](https://img.shields.io/badge/Platform-Linux-FCC624?style=for-the-badge&logo=linux&logoColor=black)](https://kernel.org/)
[![Desteklenenler](https://img.shields.io/badge/Destek-Discord%20%2B%20Roblox-5865F2?style=for-the-badge)](#-genel-bakış)
[![Ping Artışı](https://img.shields.io/badge/Ping%20Art%C4%B1%C5%9F%C4%B1-0%20ms-brightgreen?style=for-the-badge)](#-neden-vpn-değil-vpn-vs-byedpi-linux)
[![Gizlilik](https://img.shields.io/badge/Gizlilik-%25100%20Yerel-blue?style=for-the-badge)](#-güvenlik-ve-sıfır-patlama-yarıçapı-zero-blast-radius)
[![TR ISS Uyumluluğu](https://img.shields.io/badge/T%C3%BCrkiye%20ISS-Do%C4%9Fruland%C4%B1-success?style=for-the-badge)](#-türkiye-iss-uyumluluk-tablosu)
[![License: MIT](https://img.shields.io/badge/Lisans-MIT-yellow.svg?style=for-the-badge)](LICENSE)

<br/>

🇹🇷 **Türkçe** | [🇬🇧 English Documentation](README.en.md)

</div>

---

## 📌 Genel Bakış

**ByeDPI Linux**, Türkiye'deki internet servis sağlayıcılarının (Turkcell Superonline, Türk Telekom, TurkNet, Vodafone, Kablonet) **Discord** ve **Roblox** üzerindeki **DNS zehirlemesi** ve **SNI/DPI (Derin Paket İnceleme)** engellemelerini aşmak için tasarlanmış, Linux çekirdeğiyle (`nftables` / `NFQUEUE`) tam uyumlu açık kaynaklı bir DPI desenkronizasyon servisidir.

### Desteklenen Servisler:
- 🎧 **Discord:** Masaüstü İstemcisi (.deb, Flatpak, Snap), Web İstemcisi, Gateway WebSocket, CDN Medyası ve **WebRTC Ses Kanalları** (0 ms ek ping).
- 🧱 **Roblox:** Web Sitesi (`roblox.com`), API'ler, Varlık/Oyun İndirme Sunucuları (`setup.rbxcdn.com`, `rbxcdn.com`), ve Linux çalıştırıcıları (**Sober**, **Vinegar**, **Wine/Proton**).

Tüm internetinizi yavaşlatan veya oyun içi pinginizi fırlatan hantal VPN'lerin aksine, **ByeDPI Linux** yalnızca Discord ve Roblox trafiğini yerel olarak ayrıştırır ve optimize eder; diğer tüm internet trafiğiniz (oyunlar, tarayıcı, bankacılık) doğrudan ve kendi hızınızda akmaya devam eder.

---

## ⚡ Neden VPN Değil? (VPN vs ByeDPI Linux)

| Karşılaştırma Kriteri | Geleneksel VPN | `ByeDPI Linux` |
| :--- | :---: | :---: |
| **Oyun İçi Ping Artışı** | ❌ **+40 - 150 ms** (Ciddi gecikme) | 🟢 **0 ms** (Oyun trafiği doğrudan gider) |
| **Discord Ses & Roblox Hızı** | ⚠️ VPN sunucusunun yoğunluğuna bağlı | 🟢 **İnternetinizin %100 tam bant genişliği** |
| **Sistem Kaynak Tüketimi** | ⚠️ Yüksek CPU / Sürekli tünel şifrelemesi | 🟢 **< 15 MB RAM** (Hafif Go Daemon) |
| **Gizlilik & Veri Güvenliği** | ❌ Tüm veriniz üçüncü taraf sunucudan geçer | 🟢 **%100 Yerel** (Harici sunucu/tünel yoktur) |
| **Bankacılık & Yerel Siteler** | ❌ Güvenlik uyarısı veya erişim engeli | 🟢 **Sorunsuz** (Orijinal IP'niz korunur) |
| **Masaüstü Kullanımı** | ⚠️ Her açılışta elle bağlanma gerektirir | 🟢 **Masaüstü Kontrol Paneli &amp; Sistem Tepsisi (Tray)** |

---

## 🚀 Tek Komutla Hızlı Kurulum

Terminalinizi açın ve aşağıdaki komutu yapıştırın:

```bash
curl -fsSL https://raw.githubusercontent.com/latryee/byedpilinux/main/install.sh | sudo bash
```

Bu tek komut:
1. Dağıtımınızı (`Ubuntu`, `Debian`, `Zorin`, `Linux Mint`, `Fedora`, `Arch`) otomatik tanır.
2. Gerekli hafif bağımlılıkları (`nftables`, `libnetfilter-queue1`) yapılandırır.
3. Servisi kurar, aktif eder ve **Masaüstünüze tıklanabilir kısayolu** yerleştirir.
4. Ağınızı otomatik test ederek (`tune`) en uygun çalışma stratejisini belirler.
5. Hem `byedpi` hem `discord-bypass` komutlarını terminalinize tanımlar.

---

### Alternatif Kurulum Yöntemleri

<details>
<summary><b>📦 Debian / Ubuntu / Mint / Zorin için Hazır .deb Paketi</b></summary>

[Releases](https://github.com/latryee/byedpilinux/releases) sayfasından en son `.deb` paketini indirin veya komut satırından kurun:

```bash
sudo apt install ./discord-bypass_1.1.0_amd64.deb
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

## 📶 Türkiye ISS Uyumluluk Tablosu

Aşağıdaki stratejiler Türkiye'deki gerçek hatlar üzerinde canlı olarak test edilmiş ve doğrulanmıştır:

| İnternet Servis Sağlayıcı (ISS) | Durum | Doğrulanmış Strateji | Açıklama |
| :--- | :---: | :--- | :--- |
| **Turkcell Superonline** | 🟢 Doğrulandı | `strategy_c` (`fakedsplit midsld ttl=6`) | Huawei DPI middlebox'larını başarıyla atlatır. |
| **Türk Telekom (TTNet)** | 🟢 Doğrulandı | `strategy_c_ttl5` / `strategy_b` | Port 53 DNS korsanlığını ve SNI blokajını aşar. |
| **TurkNet** | 🟢 Doğrulandı | `strategy_b` (DoH) / `strategy_c` | DoH ile temiz IP çözümlemesi sağlar. |
| **Vodafone Net** | 🟢 Doğrulandı | `strategy_c` / `strategy_d` | Giden SNI paketlerini parçalayarak DPI'ı atlatır. |
| **Türksat Kablonet** | 🟢 Doğrulandı | `strategy_c` (`split2`) | Port 443 blokajını doğrudan çözer. |
| **Millenicom / NetSpeed** | 🟢 Doğrulandı | `strategy_c` | Altyapıya göre TT veya Superonline profilini uygular. |

---

## 💻 Masaüstü Kontrol Paneli & Sistem Tepsisi (Tray)

Terminal kullanmak istemeyen kullanıcılar ve oyuncular için **modern, karanlık modda çalışan tam özellikli bir Masaüstü Kontrol Paneli** (`discord-bypass-gui`) sunulmaktadır:

- **Masaüstünden Tek Tık:** Masaüstünüzdeki **"Discord & Roblox Bypass"** kısayoluna çift tıklayarak paneli başlatabilirsiniz.
- **Sistem Tepsisi Entegrasyonu (System Tray):**
  - Uygulama masaüstü görev çubuğunuza (Tray) yerleşir.
  - Pencereyi kapattığınızda (`X`) uygulama kapanmaz, **arka planda sistem tepsisine küçülür**.
  - Tepsi simgesine sağ tıklayarak:
    - 🟢 Anlık bağlantı durumunu görme
    - 🪟 Pencereyi Göster / Gizle
    - ⚡ Tek tıkla ağ ayarlama (`tune`)
    - 🛑 Servisi Başlat / Durdur
    - ❌ Tamamen Çıkış
- **Canlı Ağ & Gecikme Kartları:**
  - 🌐 **ISS ve Operatör:** Bulunduğunuz sağlayıcı ve aktif arayüz.
  - 🎧 **Discord HTTPS & WebRTC Ses:** 0 ms ek ping ile canlı ses hazır durumu.
  - 🧱 **Roblox Durumu:** Web ve oyun istemcisi erişilebilirlik rozeti.
  - 🛡️ **Güvenlik Duvarı Sayaçları:** İşlenen paket ve korunan alan adları sayısı.
- **Strateji Seçici (Dropdown):** İstediğiniz bypass tekniğini arayüzden seçip tek tıkla uygulayabilirsiniz.

---

## 🛠️ Temel Komutlar (Terminal Sevenler İçin)

Hem `byedpi` hem de `discord-bypass` komutları kullanılabilir:

```bash
# Mevcut servis ve bağlantı durumunu görüntüleme (Discord + Roblox)
byedpi status
# veya: discord-bypass status

# Bulunduğunuz ağ için çalışan stratejiyi otomatik belirleme ve uygulama
sudo byedpi tune

# Grafik kontrol panelini başlatma
discord-bypass-gui

# Mevcut aktif stratejiyi ve doğrulama ayrıntılarını görüntüleme
byedpi strategy current

# Kullanılabilir tüm strateji adaylarını listeleme
byedpi strategy list

# Stratejiyi manuel olarak değiştirme (otomatik bağlantı doğrulamalı)
sudo byedpi strategy set strategy_c

# 12 adımlı derin ağ ve ISS teşhisini çalıştırma
byedpi diagnose

# Ağ arayüzü izleyicisini başlatma (Wi-Fi/Hotspot geçişlerinde otomatik uyum)
byedpi watch

# Servis loglarını canlı takip etme
byedpi logs
```

---

## ⚠️ Önemli: Superonline "Güvenli İnternet" Sorunu

Turkcell Superonline fiber abonelerinde, hesapta **"Güvenli İnternet" (Aile veya Çocuk Profili)** aktifse, DPI paketleri incelenmeden doğrudan santral (BRAS) seviyesinde bağlantı sıfırlanır (`TCP RST`).

Eğer `byedpi tune` çalıştırdığınızda tüm stratejiler başarısız oluyorsa:
1. **Turkcell Superonline Hesabınıza** (web veya mobil uygulama) giriş yapın.
2. **Güvenli İnternet** ayarlarına gidin.
3. Profilinizi **"Standart Profil"** (Korumasız / Standart İnternet) olarak değiştirin.
4. Modeminizi yeniden başlatın ve ardından tekrar çalıştırın:
   ```bash
   sudo byedpi tune
   ```

---

## 🔒 Güvenlik ve Sıfır Patlama Yarıçapı (Zero Blast Radius)

- **İzole Güvenlik Duvarı:** `ByeDPI Linux`, modern Linux `nftables` üzerinde kendine ait izole bir tablo oluşturur (`table inet discord_bypass`). Mevcut güvenlik duvarı kurallarınızı (UFW, Docker, iptables) asla silmez veya bozmaz.
- **Yalnızca Hedef Servisler:** Yalnızca Discord ve Roblox'a ait onaylı alan adları ve IP blokları (`@discord_v4` ve `@discord_v6`) hedeflenir. Bankacılık, oyun, video akış ve genel web trafiğiniz bu tablodan tamamen muaftır.
- **Sıfır Telemetri:** Hiçbir veri, IP adresi, donanım kimliği veya sistem bilgisi toplanmaz ya da harici sunuculara iletilmez. Tüm mantık yerel makinenizde çalışır.
- **Acil Durum Koruması:** Herhangi bir anda tüm kuralları temizlemek ve trafiği anında varsayılana döndürmek için:
  ```bash
  sudo byedpi emergency-disable
  ```

---

## 🤝 Katkıda Bulunma

Topluluk geri bildirimleri projenin kalbidir! Farklı bir ISS veya şehirde test sonuçlarınızı paylaşmak için [GitHub Issues](https://github.com/latryee/byedpilinux/issues) üzerinden bir ISS Uyumluluk Raporu oluşturabilirsiniz.

---

## 📄 Lisans

Bu proje [MIT Lisansı](LICENSE) altında açık kaynak olarak sunulmaktadır.
DPI desenkronizasyon çekirdeği [bol-van/zapret](https://github.com/bol-van/zapret) projesinden derlenmiştir.
