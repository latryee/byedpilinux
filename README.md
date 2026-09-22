<div align="center">

# 🎮 discord-bypass

**Türkiye için Optimize Edilmiş, Sıfır Gecikmeli (0 ms Ping) Linux Discord Erişim Aracı**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![Platform: Linux](https://img.shields.io/badge/Platform-Linux-FCC624?style=for-the-badge&logo=linux&logoColor=black)](https://kernel.org/)
[![Ping Artışı](https://img.shields.io/badge/Ping%20Art%C4%B1%C5%9F%C4%B1-0%20ms-brightgreen?style=for-the-badge)](#-neden-vpn-değil-vpn-vs-discord-bypass)
[![Gizlilik](https://img.shields.io/badge/Gizlilik-%25100%20Yerel-blue?style=for-the-badge)](#-güvenlik-ve-sıfır-patlama-yarıçapı-zero-blast-radius)
[![TR ISS Uyumluluğu](https://img.shields.io/badge/T%C3%BCrkiye%20ISS-Do%C4%9Fruland%C4%B1-success?style=for-the-badge)](#-türkiye-iss-uyumluluk-tablosu)
[![License: MIT](https://img.shields.io/badge/Lisans-MIT-yellow.svg?style=for-the-badge)](LICENSE)

<br/>

🇹🇷 **Türkçe** | [🇬🇧 English Documentation](README.en.md)

</div>

---

## 📌 Genel Bakış

**discord-bypass**, Türkiye'deki internet servis sağlayıcılarının (Turkcell Superonline, Türk Telekom, TurkNet, Vodafone, Kablonet) Discord üzerindeki **DNS zehirlemesi** ve **SNI/DPI (Derin Paket İnceleme)** engellemelerini aşmak için özel olarak tasarlanmış, Linux çekirdeğiyle tam uyumlu, açık kaynaklı bir ağ servisidir.

Tüm internetinizi yavaşlatan veya oyun içi pinginizi fırlatan hantal VPN'lerin aksine, **discord-bypass** yalnızca Discord trafiğini yerel olarak ayrıştırır ve optimize eder; diğer tüm internet trafiğiniz (oyunlar, tarayıcı, bankacılık) doğrudan ve kendi hızınızda akmaya devam eder.

---

## ⚡ Neden VPN Değil? (VPN vs discord-bypass)

| Karşılaştırma Kriteri | Geleneksel VPN | `discord-bypass` |
| :--- | :---: | :---: |
| **Oyun İçi Ping Artışı** | ❌ **+40 - 150 ms** (Ciddi gecikme) | 🟢 **0 ms** (Oyun trafiği doğrudan gider) |
| **Discord Ses & Yayın Kalitesi** | ⚠️ VPN sunucusunun yoğunluğuna bağlı | 🟢 **İnternetinizin %100 tam bant genişliği** |
| **Sistem Kaynak Tüketimi** | ⚠️ Yüksek CPU / Sürekli tünel şifrelemesi | 🟢 **< 15 MB RAM** (Hafif Go Daemon) |
| **Gizlilik & Veri Güvenliği** | ❌ Tüm veriniz üçüncü taraf sunucudan geçer | 🟢 **%100 Yerel** (Harici sunucu/tünel yoktur) |
| **Bankacılık & Yerel Siteler** | ❌ Güvenlik uyarısı veya erişim engeli | 🟢 **Sorunsuz** (Orijinal IP'niz korunur) |
| **Kullanım Kolaylığı** | ⚠️ Her açılışta elle bağlanma gerektirir | 🟢 **systemd** ile arka planda sessizce çalışır |

---

## 🚀 Tek Komutla Hızlı Kurulum

Terminalinizi açın ve aşağıdaki komutu yapıştırın:

```bash
curl -fsSL https://raw.githubusercontent.com/latryee/byedpilinux/main/install.sh | sudo bash
```

Bu tek komut:
1. Dağıtımınızı (`Ubuntu`, `Debian`, `Zorin`, `Linux Mint`, `Fedora`, `Arch`) otomatik tanır.
2. Gerekli hafif bağımlılıkları (`nftables`, `libnetfilter-queue1`) yapılandırır.
3. Servisi kurar ve aktif eder.
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

<details>
<summary><b>🛠️ Kaynak Koddan Manuel Derleme (Tüm Dağıtımlar)</b></summary>

```bash
git clone https://github.com/latryee/byedpilinux.git
cd byedpilinux
make build
sudo ./install.sh
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

## 🛠️ Temel Komutlar ve Kullanım

```bash
# Mevcut servis ve bağlantı durumunu görüntüleme
discord-bypass status

# Bulunduğunuz ağ için çalışan stratejiyi otomatik belirleme ve uygulama
sudo discord-bypass tune

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
4. Modeminizi yeniden başlatın ve ardından terminalde tekrar çalıştırın:
   ```bash
   sudo discord-bypass tune
   ```

---

## 🔒 Güvenlik ve Sıfır Patlama Yarıçapı (Zero Blast Radius)

- **İzole Güvenlik Duvarı:** `discord-bypass`, modern Linux `nftables` üzerinde kendine ait izole bir tablo oluşturur (`table inet discord_bypass`). Mevcut güvenlik duvarı kurallarınızı (UFW, Docker, iptables) asla silmez veya bozmaz.
- **Sadece Discord Trafiği:** Yalnızca Discord'a ait onaylı domain ve IP blokları (`@discord_v4` ve `@discord_v6`) hedeflenir. Bankacılık, oyun, video akış ve genel web trafiğiniz bu tablodan tamamen muaftır.
- **Sıfır Telemetri:** Hiçbir veri, IP adresi, donanım kimliği veya sistem bilgisi toplanmaz ya da harici sunuculara iletilmez. Tüm mantık yerel makinenizde çalışır.
- **Acil Durum Koruması:** Herhangi bir anda tüm kuralları temizlemek ve trafiği anında varsayılana döndürmek için:
  ```bash
  sudo discord-bypass emergency-disable
  ```

---

## 💻 Masaüstü Entegrasyonu

Uygulama, sisteminize kurulduğunda başlatıcı menünüze (**Uygulamalar -> İnternet -> Discord Bypass**) otomatik olarak eklenir. Sağ tık menüsü üzerinden tek tıkla durum kontrolü ve ağ ayarı yapabilirsiniz:

- **Durumu Göster**
- **Ağ Teşhisi Yap (Diagnose)**
- **Otomatik Strateji Ayarla (Tune)**

---

## 🤝 Katkıda Bulunma

Topluluk geri bildirimleri projenin kalbidir! Farklı bir ISS veya şehirde test sonuçlarınızı paylaşmak için [GitHub Issues](https://github.com/latryee/byedpilinux/issues) üzerinden bir ISS Uyumluluk Raporu oluşturabilirsiniz.

Ayrıntılar için [CONTRIBUTING.md](CONTRIBUTING.md) ve [SECURITY.md](SECURITY.md) belgelerini inceleyebilirsiniz.

---

## 📄 Lisans

Bu proje [MIT Lisansı](LICENSE) altında açık kaynak olarak sunulmaktadır.
DPI desenkronizasyon çekirdeği [bol-van/zapret](https://github.com/bol-van/zapret) projesinden derlenmiştir.
