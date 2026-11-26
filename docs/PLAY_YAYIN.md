# Geliyo → Google Play yayın talimatı (yerel Claude oturumu için)

> **Bu dosya kimin için?** Kullanıcının kendi bilgisayarında çalışan Claude Code oturumu için. Amaç: Geliyo Android uygulamasını Google Play'e profesyonel biçimde, Play politikalarına uygun olarak yüklemek.
> **Dil:** Kullanıcıyla Türkçe konuş. Kısa ve net ol.
> **Tarih:** 26 Kasım 2026 · uygulama 0.5.0 (`version: 0.5.0+5`)

---

## 0. Önce bunları oku ve kurallara uy

### Proje
- Repo: `keremtrgl/hibro` (gizli), çalışma dalı `ccr-fa51e996-vegm1b`. Flutter uygulaması `app/` klasöründe, Go sunucusu `server/` klasöründe.
- Uygulama adı **Geliyo**, paket kimliği **`com.geliyo.app`**. Kimlik Play'de yayınlandıktan sonra **değiştirilemez**, dokunma.
- Sunucu: `https://api.geliyo.site` (Türkiye VPS, Caddy + Docker). Gizlilik politikası: `https://api.geliyo.site/gizlilik`.
- Sunucu adresi derlemede sabittir: `--dart-define=HIBRO_API=https://api.geliyo.site` (varsayılan değer de bu).
- Diğer dokümanlar: `docs/MAGAZA.md` (eski mağaza notları), `docs/VERI_TOPLAMA.md` (ne toplanıyor), `docs/ROADMAP.md`, `docs/legal/` (KVKK taslakları).

### Güvenlik kuralları (kesin)
1. **Şifre, keystore şifresi, Google hesabı bilgisi veya servis hesabı JSON'unu sohbete yazdırma, yazma, loglama, commitleme.** Kullanıcı şifreyi terminalde kendisi girsin (`keytool` sorar).
2. `*.jks`, `key.properties`, `play-service-account.json` **asla** git'e girmez (`.gitignore` zaten `*.jks`, `/app/android/key.properties` içeriyor; servis hesabı dosyasını da ekle).
3. Play Console'da **geri alınamaz** adımlar (üretime yayın, uygulama silme, kimlik seçimi, ödeme) öncesinde kullanıcıya sor.
4. Kullanıcının kişisel bilgilerini (ad, adres, e-posta) mağaza metnine kendiliğinden yazma; ondan iste.

### İnsanın yapması gerekenler (Claude yapamaz)
- Google Play geliştirici hesabı açmak (tek seferlik **25 $**), kimlik doğrulaması (kimlik belgesi, telefon), kişisel/kuruluş hesabı seçimi. Kuruluş hesabı için D-U-N-S numarası gerekir.
- Ödeme, Google hesabıyla giriş, 2 adımlı doğrulama.
- Keystore şifresini belirlemek ve yedeklemek.
- Kapalı test için 12+ gerçek test kullanıcısı bulmak.

---

## 1. Yayından önce kontrol listesi (engelleyiciler)

Her maddeyi kontrol et, eksik olanları kullanıcıya raporla:

- [ ] `curl -s https://api.geliyo.site/healthz` yanıt veriyor. `curl -s https://api.geliyo.site/v1/cities | head -c 300` şehir listesi dönüyor.
- [ ] VPS'te son kod çalışıyor (yer arama ucu): `curl -s "https://api.geliyo.site/v1/cities/izmir/places?q=Ege%20%C3%9Cniversitesi"`. Değeri `places` dizisi değilse kullanıcı VPS'te şunu çalıştırmalı: `cd /opt/hibro && git pull && ./deploy/update.sh`
- [ ] Gizlilik politikasında `[ŞİRKET / KİŞİ ADI]`, `[ADRES]`, `[E-POSTA]` yer tutucuları doldurulmuş (`server/internal/api/static/gizlilik.html`). **Doldurulmadan Play'e gönderme**: Play, iletişim bilgisi olmayan politikayı reddedebilir. Bilgileri kullanıcıdan al, dosyayı güncelle, commitle, VPS'te `update.sh`.
- [ ] Topluluk verisi (KentKart kaynaklı 13 şehir) için izin durumu. İzin yoksa kullanıcıya hatırlat: sunucuda `HIBRO_COMMUNITY=off` (`deploy/.env`) ve `docker compose up -d`. Mağaza açıklamasındaki şehir listesini buna göre kısalt.
- [ ] `targetSdk` Play'in güncel şartını karşılıyor. Şu an Flutter varsayılanı **36**. Play'in o anki şartını <https://developer.android.com/google/play/requirements/target-sdk> adresinden doğrula.
- [ ] Uygulamada reklam SDK'sı, analitik SDK'sı yok (doğru, ekleme).

---

## 2. Upload (yükleme) anahtarı

Bulut oturumunda derlenen APK'lar **debug anahtarıyla** imzalandı. Play için yeni bir upload anahtarı gerekiyor. Sonuç olarak kullanıcının telefonundaki elle kurulmuş APK, Play sürümüyle **imza uyuşmazlığı** verir. Play'den kurmadan önce eski Geliyo'yu kaldırmasını söyle.

```bash
# Kullanıcı şifreyi kendisi girer. Anahtarı repo dışında tut.
keytool -genkeypair -v -keystore ~/keys/geliyo-upload.jks -storetype JKS \
  -keyalg RSA -keysize 2048 -validity 10000 -alias upload
```

`app/android/key.properties` (git'e girmez). Bu dosyayı oluştur, şifre alanlarını kullanıcı doldursun:

```properties
storeFile=/MUTLAK/YOL/keys/geliyo-upload.jks
storePassword=
keyAlias=upload
keyPassword=
```

`app/android/app/build.gradle.kts` bu dosya varsa release imzasını otomatik kullanır, yoksa debug'a düşer. **Kontrol:** derlemeden sonra
`keytool -printcert -jarfile app/build/app/outputs/bundle/release/app-release.aab` çıktısında `CN=Android Debug` **olmamalı**.

Kullanıcıya söyle: `.jks` dosyasını ve şifreleri **iki ayrı yerde yedeklesin** (ör. şifre yöneticisi + harici disk). Play App Signing açık olduğu için kaybolursa Google'dan upload anahtarı sıfırlaması istenebilir, ama bu günler sürer.

---

## 3. Profesyonel derleme hazırlığı

### 3.1 Uyarlanabilir (adaptive) ikon
Şu an `mipmap-*` altında yalnızca düz `ic_launcher.png` var. Android 8+ ve Play için uyarlanabilir ikon ekle:
- `flutter_launcher_icons` paketini **dev_dependencies** olarak ekle. Kaynak: `docs/img/ikon.png` (gerekirse 1024×1024'e büyüt; ön plan için kenarlarda %20 güvenli boşluk bırak).
- `adaptive_icon_background` olarak marka rengi, `adaptive_icon_foreground` olarak şeffaf zeminli logo. `dart run flutter_launcher_icons` çalıştır, sonucu emülatörde kontrol et.

### 3.2 Sürüm numarası
`app/pubspec.yaml` → `version: 0.5.0+5`. Play'e her yüklemede `+` sonrasındaki **versionCode artmalı**. İlk Play sürümü için `1.0.0+6` öner, kullanıcıya sor.

### 3.3 Küçültme ve sembol dosyaları
```bash
cd app
flutter clean && flutter pub get
flutter analyze && flutter test
flutter build appbundle --release \
  --dart-define=HIBRO_API=https://api.geliyo.site \
  --obfuscate --split-debug-info=build/symbols
```
- Çıktı: `build/app/outputs/bundle/release/app-release.aab`
- `build/symbols` klasörünü sürüm numarasıyla sakla (çökme izlerini çözmek için). Git'e girmez.
- Play Console "Native debug symbols yok" uyarısı verirse: `build/app/intermediates/merged_native_libs/release/out/lib` klasörünü zip'leyip App bundle explorer → sürüm → "Hata ayıklama sembollerini yükle" ile yükle (isteğe bağlı ama profesyonel).
- AAB boyutu kontrolü: APK ~86 MB'tı; AAB ile kullanıcıya inen boyut ABI bölünmesi sayesinde ~25-35 MB olmalı. Play Console → App bundle explorer'da doğrula.

### 3.4 Yayın öncesi cihaz testi
- Release AAB'den APK üretip gerçek telefonda dene: `bundletool build-apks --bundle=... --output=geliyo.apks --mode=universal --ks=... --ks-key-alias=upload`, ardından `bundletool install-apks`. Bunun yerine Play "Dahili test" kanalını da kullanabilirsin.
- Kontrol et: ilk açılış izin ekranı; konum izni reddedilince uygulama çökmüyor; şehir otomatik bulunuyor; arama (durak + yer); rota; hat ekranında ok/bayrak; sorun bildir başarılı; açık/koyu tema; uçak modunda yakındaki duraklar.

---

## 4. Play Console adımları

Kullanıcı Play Console'a kendisi girer. Tarayıcı aracın varsa (Claude in Chrome vb.) form doldurmada yardım edebilirsin. Her **"Gönder/Yayınla"** öncesinde onay al.

### 4.1 Uygulama oluştur
- Uygulama adı: **Geliyo** (mağaza başlığı için alternatif: "Geliyo: Otobüs, Metro, Rota", 30 karaktere kadar)
- Varsayılan dil: **Türkçe (tr-TR)** · Tür: **Uygulama** · **Ücretsiz** (ücretsizden ücretliye sonradan geçilemez, bu doğru)
- Beyanlar: geliştirici program politikaları + ABD ihracat yasaları onayı (kullanıcı onaylar)

### 4.2 Play App Signing
Açık kalsın (varsayılan). İlk AAB yüklenince Google uygulama imzalama anahtarını kendisi yönetir; bizim `.jks` yalnızca upload anahtarıdır.

### 4.3 Uygulama içeriği (Policy → App content) — doğru cevaplar

| Bölüm | Cevap |
|---|---|
| Gizlilik politikası | `https://api.geliyo.site/gizlilik` |
| Uygulama erişimi | Tüm işlevler kısıtlama olmadan kullanılabilir (hesap/giriş yok) |
| Reklamlar | **Hayır, reklam içermiyor** |
| İçerik derecelendirmesi (IARC) | Kategori: "Yardımcı Program / Verimlilik / İletişim / Diğer". Şiddet, cinsellik, kumar, uyuşturucu: hepsi **Hayır**. Kullanıcılar birbiriyle etkileşim kuruyor mu: **Hayır** (sorun bildirimleri herkese açık gösterilmez, yalnızca "kalabalık" sayısı görünür). Konum paylaşımı başka kullanıcılarla: **Hayır**. Beklenen sonuç: **3+ / Herkes** |
| Hedef kitle | **13 yaş ve üzeri** (13-15, 16-17, 18+). 13 altını seçme, yoksa Aileler politikası devreye girer. "Çocukların ilgisini çekebilir mi": Hayır |
| Haber uygulaması | Hayır |
| COVID-19 temaslı takip | Hayır |
| Veri güvenliği | Aşağıda §4.4 |
| Devlet uygulaması | **Hayır.** Geliyo bir belediye ya da devlet uygulaması değildir |
| Finansal özellikler | Uygulama finansal özellik sunmuyor |
| Sağlık | Sağlık özelliği yok |
| İzinler / Konum | Yalnızca ön planda konum (`ACCESS_FINE_LOCATION`, `ACCESS_COARSE_LOCATION`). Arka plan konum, ön plan servisi, bildirim izni **yok**, bu yüzden ek beyan gerekmez. Form çıkarsa amaç: "yakındaki durakları ve otobüsleri göstermek, kullanıcının konumundan rota hesaplamak" |

> **Taklit politikası (önemli):** Geliyo belediyelerin açık verisini kullanıyor ama **resmi değil**. Mağaza metninde ve ekran görüntülerinde belediye logosu, "resmi", "ESHOT uygulaması", "İETT" gibi resmiyet çağrıştıran ifadeler **kullanma**. Açıklamanın sonuna şu feragati ekle: *"Geliyo bağımsız bir uygulamadır; herhangi bir belediye veya ulaşım işletmesiyle bağlantılı değildir. Veriler belediyelerin açık veri portallarından alınır."*

### 4.4 Veri güvenliği (Data safety) formu
Kaynak gerçek davranıştır: `docs/VERI_TOPLAMA.md`, `app/lib/api/client.dart`. Kod değişirse formu da güncelle.

- Veri topluyor veya paylaşıyor mu: **Evet** (sunucuya gönderilen veri Play tanımında "toplama"dır)
- Aktarımda şifreleniyor mu: **Evet** (yalnızca HTTPS)
- Kullanıcı veri silme isteyebilir mi: Hesap yok, veri kimliğe bağlı değil. "Kullanıcılar verilerinin silinmesini isteyebilir" seçeneğini işaretleme. Gerekirse gizlilik politikasındaki e-postayı göster.
- Bağımsız güvenlik incelemesi: Hayır

| Veri türü | Toplanıyor | Paylaşılıyor | Geçici işleme | Zorunlu/isteğe bağlı | Amaç |
|---|---|---|---|---|---|
| Konum → Yaklaşık konum | Evet | Hayır | **Evet** (kaydedilmez) | İsteğe bağlı (izin verilmezse harita merkezi kullanılır) | Uygulama işlevselliği |
| Konum → Kesin konum | Evet | Hayır | **Evet** | İsteğe bağlı | Uygulama işlevselliği |
| Uygulama etkinliği → Uygulama içi arama geçmişi | Evet (yer arama metni sunucuya gider) | **Evet**: sunucu sorguyu OpenStreetMap Nominatim'e iletir (kimliksiz) | Hayır (7 gün önbellek) | İsteğe bağlı | Uygulama işlevselliği |
| Uygulama etkinliği → Kullanıcı tarafından oluşturulan diğer içerik | Evet ("Sorun bildir" notu) | Hayır | Hayır | İsteğe bağlı | Uygulama işlevselliği |
| Uygulama etkinliği → Uygulama etkileşimleri | Evet (anonim sayaç, **yalnızca izinle**, varsayılan kapalı) | Hayır | Hayır | İsteğe bağlı | Analiz |
| Kişisel bilgi, finans, sağlık, mesaj, fotoğraf, ses, dosya, takvim, kişiler, cihaz kimliği | Hayır | | | | |

> Nominatim aktarımını "paylaşım" saymama gerekçesi tartışılabilir (hizmet sağlayıcı istisnası). Emin değilsen kullanıcıya sor; daha temkinli olan "paylaşılıyor: Evet" cevabıdır.

### 4.5 Mağaza girişi (Main store listing)

**Kısa açıklama (≤80 karakter):**
> Reklamsız toplu taşıma: yakındaki duraklar, canlı otobüs, rota. Hesap yok.

**Tam açıklama (≤4000 karakter):**
```
Geliyo, Türkiye için reklamsız ve hızlı bir toplu taşıma uygulaması.

• Hiç reklam yok. Hesap yok. İzleyici yok.
• Yakındaki duraklar: konumuna göre, internet olmadan bile.
• Canlı otobüs (İzmir): durağa kaç dakikada geleceğini, plakayı ve engelli erişimini gör.
• Yer ve adres arama: "üniversite", "hastane", mahalle adı yaz, konumundan nasıl gidileceğini gör.
• A'dan B'ye rota: otobüs, metro, tramvay ve banliyö aktarmalı.
• Net hat yönleri: haritada başlangıç oku, varış bayrağı ve yön okları.
• Hayalet otobüs filtresi: sinyali kesilmiş araçları "geliyor" diye göstermez.
• Az veri: şehir verisi bir kez iner, sonra yalnızca değişince güncellenir.
• Açık ve koyu tema, favoriler (yalnızca telefonunda).
• Sorun bildir: yanlış durak, gelmeyen araç, kalabalık; verileri birlikte düzeltelim.

Şehirler: İzmir, İstanbul, Konya, Gaziantep, Kocaeli ve diğerleri. Canlı araç konumu şu an yalnızca İzmir'de (diğer şehirlerde belediyeler bu veriyi yayınlamıyor; süreler sefer tarifesinden gelir).

Gizlilik: Konumun kaydedilmez. Kullanım istatistiği paylaşımı isteğe bağlıdır ve varsayılan olarak kapalıdır.

Geliyo bağımsız bir uygulamadır; herhangi bir belediye veya ulaşım işletmesiyle bağlantılı değildir. Veriler belediyelerin açık veri portallarından alınır.
```
Şehir listesini sunucudaki gerçek duruma göre ayarla (`/v1/cities`). İzin yoksa topluluk şehirlerini yazma.

**Grafikler** (oluştur ve kullanıcıya göster):
| Öğe | Şart |
|---|---|
| Uygulama ikonu | 512×512 PNG, 32-bit, ≤1 MB (`docs/img/ikon.png` kaynağından) |
| Öne çıkan görsel | 1024×500 PNG/JPG, kenarlarda yazı yok, logo + "Reklamsız toplu taşıma" |
| Telefon ekran görüntüleri | En az 2, önerilen 4-8. 9:16, kısa kenar ≥1080 px. Gerçek cihaz/emülatör görüntüsü, cihaz çerçevesi isteğe bağlı |
| 7" / 10" tablet | İsteğe bağlı (atla) |

Ekran görüntüsü önerisi (sırasıyla): 1) ana harita + yakındaki duraklar, 2) durak ekranı canlı varışlar (İzmir), 3) yer araması "Kocaeli Üniversitesi", 4) rota sonucu haritada, 5) hat ekranı ok/bayrak, 6) koyu tema. Emülatörde konum ayarla (`adb emu geo fix <lon> <lat>`), `adb exec-out screencap -p > 1.png`. Üstüne kısa Türkçe başlıklar eklenebilir (ör. "Reklam yok. Hiç."). Durum çubuğunu temizlemek için sistem demo modunu kullan: `adb shell settings put global sysui_demo_allowed 1`.

**Kategori:** Harita ve Navigasyon · **Etiketler:** Toplu taşıma, Navigasyon · **İletişim e-postası:** kullanıcıdan al (zorunlu) · **Web sitesi:** `https://geliyo.site` (varsa)

### 4.6 Ülkeler
Yalnızca **Türkiye** ile başla (veri yalnızca Türkiye için anlamlı).

---

## 5. Test kanalları → üretim

1. **Dahili test:** AAB'yi yükle, sürüm notu yaz, kullanıcının kendi e-postasını test listesine ekle. Play'den kur, §3.4 kontrollerini tekrarla. Pre-launch report (Firebase Test Lab) sonuçlarını incele: çökme, erişilebilirlik uyarısı (dokunma hedefi < 48 dp, kontrast) varsa düzelt.
2. **Kapalı test (zorunlu olabilir):** 13 Kasım 2023'ten sonra açılan **kişisel** hesaplarda üretime erişim için **en az 12 test kullanıcısının en az 14 gün kesintisiz** kapalı testte olması gerekir. Kuralı Play Console'daki "Üretime erişim" panelinden doğrula (değişebilir). Test kullanıcılarını Google Grubu veya e-posta listesiyle ekle, katılım bağlantısını kullanıcıya ver. 14 gün boyunca en az 1-2 güncelleme yayınlamak başvuruyu güçlendirir.
3. **Üretime erişim başvurusu:** Play'in sorduğu test geri bildirimi sorularını kullanıcıyla birlikte doldur (testi nasıl yaptınız, ne düzelttiniz).
4. **Üretim:** Önce **kademeli yayın** (%10 → %50 → %100). Android vitals'ta çökme oranı < %1.09 ve ANR < %0.47 eşiklerinin altında kalmalı.

**Sürüm notu örneği (tr-TR, ≤500 karakter):**
```
İlk sürüm: reklamsız toplu taşıma. Yakındaki duraklar, İzmir'de canlı otobüs, yer/adres arama, aktarmalı rota, hat yön işaretleri, açık/koyu tema.
```

---

## 6. (İsteğe bağlı) Yükleme otomasyonu

Kullanıcı isterse sonraki sürümler için:
- Play Console → Kurulum → API erişimi → Google Cloud servis hesabı → yalnızca bu uygulama için "Sürümleri yönet" izni. JSON anahtarını repo **dışında** tut.
- `fastlane supply` ile meta veri (`fastlane/metadata/android/tr-TR/{title,short_description,full_description}.txt`, `changelogs/<versionCode>.txt`, ekran görüntüleri) ve AAB yükleme: `fastlane supply --aab app-release.aab --track internal --json_key ~/keys/play-sa.json --package_name com.geliyo.app`.
- Üretime otomatik yükleme yapma. Otomasyon yalnızca `internal` kanalına gitsin, terfiyi insan yapsın.

---

## 7. Yayın sonrası

- Play Console → Android vitals ve kullanıcı yorumlarını haftalık kontrol et.
- Gizlilik politikası veya toplanan veri değişirse Data safety formunu **aynı gün** güncelle.
- Sunucu adresi değişirse yeni derleme gerekir (`HIBRO_API` derlemeye gömülü).
- Her sürümde `version` artır, `build/symbols` klasörünü sakla, `git tag v<sürüm>` at ve pushla.

## 8. İşi bitirince kullanıcıya rapor

Kısa bir liste ver: tamamlanan adımlar, kullanıcının yapması gerekenler (ödeme, kimlik doğrulaması, test kullanıcıları, şifre yedeği), açık riskler (gizlilik politikası yer tutucuları, topluluk verisi lisansı, kapalı test süresi) ve bir sonraki somut adım.
