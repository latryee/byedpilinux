# discord-bypass (Linux)

> **Sürüm 1.0.0 (Kararlı)**: Upstream `bol-van/zapret` `nfqws` v72.13 motoru, Go denetleyicisi ve izole `nftables` güvenlik duvarı mimarisiyle güçlendirilmiştir.
> Turkcell Superonline (AS34984), Türk Telekom ve diğer Türk ISS hatlarında Discord Masaüstü, Web, Ağ Geçidi (Gateway WebSocket), CDN ve Ses (Voice) erişimi için doğrulanmıştır. **Sistem genelindeki DNS ayarlarınıza veya diğer internet trafiğinize kesinlikle müdahale etmez.**

Linux sistemlerde ISS düzeyindeki DNS zehirlemesini ve TLS SNI tabanlı Derin Paket İncelemesini (DPI) güvenli, sistem dostu ve kalıcı bir şekilde aşarak **Discord** erişimini eksiksiz geri getiren açık kaynaklı ağ aracı.

---

## 🚀 Öne Çıkan Özellikler

- **Linux'a Özel Yerel Çözüm:** Windows'taki *GoodbyeDPI* ve *Zapret* mantığını modern Linux `systemd` ve `nftables` standartlarına uygun bir servis olarak sunar.
- **Sıfır Yan Etki (Zero Blast Radius):** İnternet trafiğinizin tamamını tünellemez veya genel DNS ayarlarınızı bozmaz. Sadece Discord alan adlarını güvenli DoH (DNS over HTTPS) ile çözer ve yalnızca Discord IP'lerini (`@discord_v4` / `@discord_v6`) izole bir kuralda işler. Bankacılık, oyun ve genel internet trafiğinize dokunulmaz.
- **Otomatik Strateji Bulucu (`tune`):** ISS'nizin (Superonline, Türk Telekom, TurkNet, Vodafone vb.) uyguladığı DPI türünü analiz eder, aday stratejileri gerçek Discord sunucularında test eder ve çalışan stratejiyi otomatik olarak kaydedip devreye alır.
- **Güvenli Geri Alma (Automatic Rollback):** Bir strateji test edilirken başarısız olursa veya bağlantı koparsa, sistem otomatik olarak çalışan eski ayarlara geri döner; ağınızı asla bozuk veya belirsiz bir durumda bırakmaz.
- **Gelişmiş 12 Noktalı Ağ Teşhisi (`diagnose`):** DNS zehirlemesi, TCP el sıkışması, TLS SNI engeli, Gateway WebSocket, REST API, CDN ve Ses portlarını tek tek test ederek ISS'nize özel analiz sunar.
- **Masaüstü Entegrasyonu:** Uygulama menüsünde `Discord Bypass` başlatıcısı, modern SVG simgesi ve `notify-status` ile masaüstü bildirim desteği.
- **Tek Tuşla Acil Kurtarma (`emergency-disable`):** Tek bir komutla tüm güvenlik duvarı kurallarını ve süreçleri anında temizler, sistemi fabrika ayarlarına döndürür.

---

## 📦 Kurulum

### Yöntem 1: Debian/Ubuntu `.deb` Paketi (Ubuntu, Debian, Zorin OS, Linux Mint, Pop!_OS)

```bash
# Paketi oluşturun ve kurun:
make deb
sudo dpkg -i discord-bypass_1.0.0_amd64.deb
```

### Yöntem 2: Tek Komutla Otomatik Kurulum (Tüm Dağıtımlar)

```bash
git clone https://github.com/latryee/byedpilinux.git
cd byedpilinux
sudo ./install.sh
```

---

## ⚡ Hızlı Kullanım Rehberi

### 1. Durum Kontrolü
```bash
# Servis, güvenlik duvarı ve paket istatistiklerini görüntüleyin (şifresiz çalışır)
discord-bypass status
```

### 2. Ağınız İçin Çalışan Stratejiyi Otomatik Bulma
```bash
# Hattınız için uygun stratejiyi otomatik belirler, test eder ve kaydeder
sudo discord-bypass tune
```

### 3. Strateji Yönetimi
```bash
# Mevcut tüm DPI aşma stratejilerini ve adayları listeleyin
discord-bypass strategy list

# Aktif olan stratejiyi ve doğrulama durumunu inceleyin
discord-bypass strategy current

# Belirli bir stratejiye güvenli geçiş yapın (çalışmazsa otomatik geri alır)
sudo discord-bypass strategy set strategy_c
```

### 4. Ağ Teşhisi ve Test
```bash
# 12 noktalı derin ağ ve DPI analizini çalıştırın
discord-bypass diagnose

# Masaüstü bildirim baloncuğu gönderin
discord-bypass notify-status
```

### 5. Servis Yönetimi
```bash
# Servisi geçici olarak durdurun
sudo discord-bypass stop

# Servisi yeniden başlatın
sudo discord-bypass start

# Servis günlüklerini (logs) görüntüleyin
discord-bypass logs

# Acil durumlarda tüm kuralları temizleyin ve durdurun
sudo discord-bypass emergency-disable
```

---

## 🇹🇷 Türkiye ISS Özel Durumları ve İpuçları

### 1. Turkcell Superonline (Fiber / VDSL)
- **Engelleme Türü:** 
  1. DNS Zehirlemesi (`195.175.254.2` BTK uyarı sayfasına yönlendirme).
  2. Huawei DPI cihazları üzerinden TLS ClientHello paketine anında TCP RST (Bağlantı Sıfırlama) enjeksiyonu.
- **Doğrulanmış Strateji:** `strategy_c` (`fakedsplit midsld ttl=6`).
- **Önemli Hatırlatma (Güvenli İnternet):** Hattınızda Superonline **"Güvenli İnternet" (Aile/Çocuk Profili)** açıksa, paketler DPI cihazına bile ulaşmadan santral düzeyinde engellenir. Superonline Online İşlemler veya mobil uygulamasından profili **"Standart Profil" (Güvenli İnternet Kapalı)** olarak ayarlamanız gerekir.

### 2. Türk Telekom / TTNet
- **Engelleme Türü:** Sandvine/Procera DPI ve şifresiz UDP 53 DNS korsanlığı.
- **Önerilen:** `sudo discord-bypass tune` komutunu çalıştırın. Genellikle `strategy_c` veya `strategy_split2` ile DoH (`mode = "doh"`) sorunsuz bağlantı sağlar.

### 3. TurkNet, Vodafone ve Kablonet
- `sudo discord-bypass tune` çalıştırarak hattınızdaki router hop sayısına en uygun TTL ve bölme stratejisini (TTL 4, 5 veya 6) 5 saniye içinde otomatik olarak seçebilirsiniz.

---

## 🛡️ Güvenlik ve Gizlilik İlkesi

- **Telemetri Yoktur:** Uygulama hiçbir uzak sunucuya telemetri, IP adresi, MAC adresi veya kullanım verisi göndermez.
- **Ağ Profili Yereldir:** `/etc/discord-bypass/network-profile.json` dosyası sadece bilgisayarınızda yerel olarak tutulur.
- **En Az Yetki İlkesi:** Arka plan servisi tüm root yetkileri yerine yalnızca gerekli olan ağ yetkileriyle (`CAP_NET_ADMIN`, `CAP_NET_RAW`) sınırlandırılmıştır.

---

## 🗑️ Kaldırma (Uninstall)

```bash
# Yapılandırmayı koruyarak kaldırma:
sudo ./uninstall.sh

# Tüm dosyaları ve yapılandırmayı tamamen silerek kaldırma:
sudo ./uninstall.sh --purge

# Veya Debian/Ubuntu paket yöneticisi ile:
sudo apt purge discord-bypass
```
