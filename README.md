<div align="center">

# 🎮 discord-bypass

**Türkiye için Optimize Edilmiş, Sıfır Gecikmeli (0 ms Ping) Linux Discord & Roblox Erişim Aracı**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![Platform: Linux](https://img.shields.io/badge/Platform-Linux-FCC624?style=for-the-badge&logo=linux&logoColor=black)](https://kernel.org/)
[![Desteklenenler](https://img.shields.io/badge/Destek-Discord%20%2B%20Roblox-5865F2?style=for-the-badge)](#-genel-bakış)
[![Ping Artışı](https://img.shields.io/badge/Ping%20Art%C4%B1%C5%9F%C4%B1-0%20ms-brightgreen?style=for-the-badge)](#-neden-vpn-değil-vpn-vs-discord-bypass)
[![Gizlilik](https://img.shields.io/badge/Gizlilik-%25100%20Yerel-blue?style=for-the-badge)](#-güvenlik-ve-sıfır-patlama-yarıçapı-zero-blast-radius)
[![TR ISS Uyumluluğu](https://img.shields.io/badge/T%C3%BCrkiye%20ISS-Do%C4%9Fruland%C4%B1-success?style=for-the-badge)](#-türkiye-iss-uyumluluk-tablosu)
[![License: MIT](https://img.shields.io/badge/Lisans-MIT-yellow.svg?style=for-the-badge)](LICENSE)

<br/>

🇹🇷 **Türkçe** | [🇬🇧 English Documentation](README.en.md)

</div>

---

## 📌 Genel Bakış

**discord-bypass**, Türkiye'deki internet servis sağlayıcılarının (Turkcell Superonline, Türk Telekom, TurkNet, Vodafone, Kablonet) **Discord** ve **Roblox** üzerindeki **DNS zehirlemesi** ve **SNI/DPI (Derin Paket İnceleme)** engellemelerini aşmak için tasarlanmış, Linux çekirdeğiyle tam uyumlu açık kaynaklı bir ağ servisidir.

### Desteklenen Servisler:
- 🎧 **Discord:** Masaüstü İstemcisi (.deb, Flatpak, Snap), Web İstemcisi, Gateway WebSocket, CDN Medyası ve **WebRTC Ses Kanalları** (0 ms ek ping).
- 🧱 **Roblox:** Web Sitesi (`roblox.com`), API'ler, Varlık/Oyun İndirme Sunucuları (`setup.rbxcdn.com`), ve Linux çalıştırıcıları (**Sober**, **Vinegar**, **Wine/Proton**).

Tüm internetinizi yavaşlatan veya oyun içi pinginizi fırlatan hantal VPN'lerin aksine, **discord-bypass** yalnızca Discord ve Roblox trafiğini yerel olarak ayrıştırır ve optimize eder; diğer tüm internet trafiğiniz (oyunlar, tarayıcı, bankacılık) doğrudan ve kendi hızınızda akmaya devam eder.

---

## ⚡ Neden VPN Değil? (VPN vs discord-bypass)

| Karşılaştırma Kriteri | Geleneksel VPN | `discord-bypass` |
| :--- | :---: | :---: |
| **Oyun İçi Ping Artışı** | ❌ **+40 - 150 ms** (Ciddi gecikme) | 🟢 **0 ms** (Oyun trafiği doğrudan gider) |
| **Discord Ses & Roblox Hızı** | ⚠️ VPN sunucusunun yoğunluğuna bağlı | 🟢 **İnternetinizin %100 tam bant genişliği** |
| **Sistem Kaynak Tüketimi** | ⚠️ Yüksek CPU / Sürekli tünel şifrelemesi | 🟢 **< 15 MB RAM** (Hafif Go Daemon) |
| **Gizlilik & Veri Güvenliği** | ❌ Tüm veriniz üçüncü taraf sunucudan geçer | 🟢 **%100 Yerel** (Harici sunucu/tünel yoktur) |
| **Bankacılık & Yerel Siteler** | ❌ Güvenlik uyarısı veya erişim engeli | 🟢 **Sorunsuz** (Orijinal IP'niz korunur) |
| **Masaüstü Kullanımı** | ⚠️ Her açılışta elle bağlanma gerektirir | 🟢 **Masaüstü Kontrol Paneli &amp; systemd** |

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

---

### Alternatif Kurulum Yöntemleri

<details>
<summary><b>📦 Debian / Ubuntu / Mint / Zorin için Hazır .deb Paketi</b></summary>

[Releases](https://github.com/latryee/byedpilinux/releases) sayfasından en son `.deb` paketini indirin veya komut satırından kurun:

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

## 💻 Masaüstü Kontrol Paneli (GUI)

Terminal kullanmak istemeyen kullanıcılar ve oyuncular için **Discord temalı, karanlık modda çalışan modern bir Grafiksel Kontrol Paneli** (`discord-bypass-gui`) sunulmaktadır:

- **Masaüstünden veya Menüden Tek Tık:** Kurulum yapıldığında masaüstünüze ve uygulama menünüze (**Uygulamalar -> İnternet -> Discord & Roblox Bypass**) kısayol eklenir. Çift tıkladığınızda doğrudan kontrol merkezi açılır.
- **Canlı Görsel Durum:**
  - 🟢 **Bağlantı & Strateji:** Aktif ve doğrulanmış çalışma stratejisini anlık gösterir.
  - 🎙️ **Discord Ses (WebRTC):** Ses sunucularının erişilebilirliğini ve 0 ms gecikmeli durumu raporlar.
  - 🧱 **Roblox Erişimi:** Web ve oyun sunucularının erişim durumunu canlı gösterir.
  - 🛡️ **Güvenlik Duvarı:** İşlenen paket sayısını listeler.
- **Grafiksel Hızlı İşlemler:**
  - `[ ⚡ Ağımı Otomatik Ayarla (Tune) ]`: Terminal açmadan, grafiksel şifre kutusuyla (`pkexec`) tek tıkla ağınız için en uygun stratejiyi bulur ve uygular.
  - `[ 🔄 Test Et ]`: Canlı Discord ve Roblox gecikmesini test eder.
  - `[ 🛠️ Ağ Teşhisi Yap ]`: 12 adımlı derin ISS teşhis raporunu pencere içinde listeler.
  - `[ 🛑 Servisi Başlat / Durdur ]`: Servis durumunu tek tıkla kontrol etmenizi sağlar.

---

## 🛠️ Temel Komutlar (Terminal Sevenler İçin)

```bash
# Mevcut servis ve bağlantı durumunu görüntüleme (Discord + Roblox)
discord-bypass status

# Bulunduğunuz ağ için çalışan stratejiyi otomatik belirleme ve uygulama
sudo discord-bypass tune

# Grafik kontrol panelini başlatma
discord-bypass-gui

# Mevcut aktif stratejiyi ve doğrulama ayrıntılarını görüntüleme
discord-bypass strategy current

# Kullanılabilir tüm strateji adaylarını listeleme
discord-bypass strategy list

# Stratejiyi manuel olarak değiştirme (otomatik bağlantı doğrulamalı)
sudo discord-bypass strategy set strategy_c

# Masaüstü bildirim durumunu tetikleme
discord-bypass notify-status

# 12 adımlı derin ağ ve ISS teşhisini çalıştırma
discord-bypass diagnose

# Ağ arayüzü izleyicisini başlatma (Wi-Fi/Hotspot geçişlerinde otomatik uyum)
discord-bypass watch

# Servis loglarını canlı takip etme
discord-bypass logs
```

---

## ⚠️ Önemli: Superonline "Güvenli İnternet" Sorunu

Turkcell Superonline fiber abonelerinde, hesapta **"Güvenli İnternet" (Aile veya Çocuk Profili)** aktifse, DPI paketleri incelenmeden doğrudan santral (BRAS) seviyesinde bağlantı sıfırlanır (`TCP RST`).

Eğer `discord-bypass tune` çalıştırdığınızda tüm stratejiler başarısız oluyorsa:
1. **Turkcell Superonline Hesabınıza** (web veya mobil uygulama) giriş yapın.
2. **Güvenli İnternet** ayarlarına gidin.
3. Profilinizi **"Standart Profil"** (Korumasız / Standart İnternet) olarak değiştirin.
4. Modeminizi yeniden başlatın ve ardından tekrar çalıştırın:
   ```bash
   sudo discord-bypass tune
   ```

---

## 🔒 Güvenlik ve Sıfır Patlama Yarıçapı (Zero Blast Radius)

- **İzole Güvenlik Duvarı:** `discord-bypass`, modern Linux `nftables` üzerinde kendine ait izole bir tablo oluşturur (`table inet discord_bypass`). Mevcut güvenlik duvarı kurallarınızı (UFW, Docker, iptables) asla silmez veya bozmaz.
- **Yalnızca Hedef Servisler:** Yalnızca Discord ve Roblox'a ait onaylı alan adları ve IP blokları (`@discord_v4` ve `@discord_v6`) hedeflenir. Bankacılık, oyun, video akış ve genel web trafiğiniz bu tablodan tamamen muaftır.
- **Sıfır Telemetri:** Hiçbir veri, IP adresi, donanım kimliği veya sistem bilgisi toplanmaz ya da harici sunuculara iletilmez. Tüm mantık yerel makinenizde çalışır.
- **Acil Durum Koruması:** Herhangi bir anda tüm kuralları temizlemek ve trafiği anında varsayılana döndürmek için:
  ```bash
  sudo discord-bypass emergency-disable
  ```

---

## 🤝 Katkıda Bulunma

Topluluk geri bildirimleri projenin kalbidir! Farklı bir ISS veya şehirde test sonuçlarınızı paylaşmak için [GitHub Issues](https://github.com/latryee/byedpilinux/issues) üzerinden bir ISS Uyumluluk Raporu oluşturabilirsiniz.

Ayrıntılar için [CONTRIBUTING.md](CONTRIBUTING.md) ve [SECURITY.md](SECURITY.md) belgelerini inceleyebilirsiniz.

---

## 📄 Lisans

Bu proje [MIT Lisansı](LICENSE) altında açık kaynak olarak sunulmaktadır.
DPI desenkronizasyon çekirdeği [bol-van/zapret](https://github.com/bol-van/zapret) projesinden derlenmiştir.
