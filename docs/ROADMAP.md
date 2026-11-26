# Geliyo Yol Haritası

> **Vizyon:** Türkiye'nin reklamsız, en hızlı ve en güvenilir toplu taşıma uygulaması.
> **Hedef:** Moovit'ten daha hızlı, daha hafif, daha doğru. Sıfır reklam, sıfır izleyici.

Son güncelleme: 26 Kasım 2026 · Uygulama sürümü **0.5.0** · Sunucu: `https://api.geliyo.site` (Rabisu VPS, Bursa)

Bu doküman üç bölümden oluşur: **1. Nasıl çalışıyor**, **2. Neler yapıldı**, **3. Neler yapılmalı**.

---

## 1. Nasıl çalışıyor?

### 1.1 Genel resim

```
Belediye açık verisi                   Geliyo sunucusu (Türkiye VPS)                 Telefon (Flutter)
────────────────────                   ─────────────────────────────                  ────────────────
GTFS zip (Konya, Gaziantep,  ──gece──►  Go sunucusu (tek ikili, Docker)  ──HTTPS──►  Şehir paketi (bir kez iner,
 Kocaeli, İstanbul İBB,                  • GTFS ayrıştırma, hat birleştirme            sonra yalnızca değişince)
 İzmir metro/tramvay/İZBAN,              • Durak/hat/sefer saatleri                    • Yakındaki duraklar, arama:
 ESHOT GTFS, topluluk şehirleri)         • RAPTOR rota planlayıcı                        internetsiz, telefonda
                                         • Canlı veri önbelleği                       • Durak ekranı: varışlar
ESHOT canlı API'leri (İzmir)  ──anlık──► • Hayalet araç filtresi (GPS > 5 dk)        • Hat ekranı: güzergâh, yön,
 araç konumu, durağa yaklaşan            • Yer arama (OSM Nominatim aracılığı)          canlı otobüsler
                                         • Sorun bildirimleri, anonim sayaçlar        • Rota: konumdan yere/durağa
OpenStreetMap Nominatim       ◄─arama──  Caddy (otomatik HTTPS)                      • Harita: OpenFreeMap karoları
```

### 1.2 Sunucu (`server/`)

| Parça | Ne yapar | Kod |
|---|---|---|
| Şehir kayıt defteri | Şehirleri ilk istekte yükler (aynı anda en fazla 3), kullanılmayan bellek işletim sistemine iade edilir | `internal/city` |
| GTFS okuyucu | Zip'i akışla okur, yönleri birleştirir (ör. İzmir M1), takvimi bitmiş veriyi "eski veri" diye işaretler | `internal/gtfs` |
| İzmir adaptörü | ESHOT canlı varış + araç konumu, 5 dk'dan eski GPS'i atar (hayalet araç), ESHOT boşsa tarifeden **tahmini** saat | `internal/izmir` |
| Rota planlayıcı | Dahili RAPTOR: 3 aktarma, 4 saatlik ufuk, yürüme bağlantıları, baskın olmayan seçenekler. Sorgu ~25-110 ms | `internal/raptor` |
| Yer arama | "Kocaeli Üniversitesi", "Kefken" gibi aramaları şehrin sınırı içinde OSM Nominatim'e sorar. Saniyede en fazla 1 istek, 7 gün önbellek, kullanıcının IP'si dışarı gitmez | `internal/geocode` |
| Önbellek | Süreli önbellek + "eskiyi göster, arkada yenile" (ESHOT yavaşken bile hızlı yanıt), aynı anahtara tek istek | `internal/cache` |
| Veri toplama | Sorun bildirimleri (1 yıl), anonim sayaçlar (beyaz liste), hız sınırı (IP diske yazılmaz) | `internal/collect` |
| API | `/v1/cities`, `/pack` (ETag + gzip), `/stops/{id}/arrivals`, `/lines/{id}`, `/vehicles`, `/vehicles/nearby`, `/plan`, `/places`, `/reports`, `/telemetry`, `/gizlilik`, `/admin/stats` | `internal/api` |

**Kurulum:** `deploy/install.sh` (tek komut: Docker, Caddy, HTTPS, güvenlik duvarı). **Güncelleme:** `cd /opt/hibro && ./deploy/update.sh`. **Kontrol:** `./deploy/smoke.sh`.

### 1.3 Uygulama (`app/`)

- **Şehir:** Konumdan otomatik bulunur; yalnızca o şehrin paketi iner (İzmir ~400 KB gzip). Paket değişmediyse sunucu 304 döner, veri inmez.
- **Yakındaki duraklar:** Konum biliniyorsa **her zaman konumuna göre** hesaplanır; haritayı kaydırmak/uzaklaştırmak listeyi değiştirmez. Konum yoksa harita merkezine göre.
- **Yakındaki otobüsler:** Canlı veri olan şehirde (şu an İzmir) konumunun çevresindeki araçlar 15 sn'de bir; uygulama arka plandayken veya harita sekmesi kapalıyken istek atılmaz.
- **Arama:** Duraklar ve hatlar telefonda anında; 3+ harften sonra **yerler/adresler** sunucudan. Bir yere dokununca konumundan oraya rota açılır.
- **Hat ekranı:** Haritada güzergâh, **başlangıçta yeşil ok**, **varışta damalı bayrak**, hat boyunca yön okları; üstte "kalkış → varış" satırı.
- **Rota:** Konumdan seçilen yere/durağa; seçenekler haritada (araç hat renginde, yürüyüş kesikli).
- **Sorun bildir:** Durak ve hat ekranlarında. Tür + not; konum ve kimlik gönderilmez.
- **Tema:** Açık/koyu/sistem; geçiş anında (animasyonsuz), harita yeniden yüklenmez, yalnızca stili değişir.
- **Sunucu adresi** derlemede sabittir (`--dart-define=HIBRO_API=...`), kullanıcı görmez ve değiştiremez.

### 1.4 Canlı otobüs neden her şehirde yok?

Belediyelerin çoğu **araç konumunu açık veri olarak yayınlamıyor**. Şu an anahtarsız canlı araç verisi veren tek kaynak **İzmir ESHOT**. Diğer şehirlerde süreler **sefer tarifesinden** gelir ve uygulama bunu açıkça söyler ("… belediyesi anlık araç konumu paylaşmıyor"). İstanbul İETT canlı verisi SOAP servisinde var (adaptör sırada). KentKart kullanan 27 şehirde veri KentKart'ta; resmi izin gerekir.

ESHOT'un canlı servisi zaman zaman boş döner; bu durumda durak ekranında "tahmini" saatler gösterilir, haritada otobüs görünmez.

---

## 2. Neler yapıldı? ✅

### Veri ve şehirler
- [x] Türkiye açık veri envanteri (`docs/VERI_KAYNAKLARI.md`, `data/feeds.yaml`)
- [x] **İzmir:** ESHOT canlı varış + araç konumu, ESHOT GTFS (durak sırası), Metro, Tramvay, İZBAN
- [x] **İstanbul:** İBB GTFS (TR IP'den, VPS'te çalışıyor: 7.008 durak, 498 hat)
- [x] **Konya, Gaziantep** (belediye CKAN GTFS), **Kocaeli** (belediye → topluluk 2026 → 2018 arşivi yedek zinciri)
- [x] **13 topluluk şehri** (Antalya, Adana, Muğla, Erzurum, Ordu, Düzce, Mardin, Çanakkale, Burdur, Osmaniye, Safranbolu, Kırklareli, Kastamonu), "topluluk verisi" etiketiyle
- [x] Eski veri uyarısı (kaynak bazında: yalnızca eski olan kaynağın durakları)

### Sunucu
- [x] Go sunucu, Docker + Caddy, tek komut kurulum, otomatik HTTPS (`api.geliyo.site`)
- [x] Dahili RAPTOR rota planlayıcı (MOTIS isteğe bağlı)
- [x] Hayalet araç filtresi, ESHOT boşken tahmini saat
- [x] 100 eş zamanlı kullanıcı yük testi (2 çekirdek): kendi uçlarımız ms seviyesinde; ESHOT darboğazı "eskiyi göster, arkada yenile" ile çözüldü
- [x] Bellek optimizasyonu (tembel şehir yükleme, eş zamanlılık sınırı), sabit paket ETag'i (her gün yeniden indirme yok)
- [x] **Yer/adres arama** (`/places`, OSM Nominatim aracılığıyla) — 0.5.0
- [x] **Sorun bildir düzeltmesi:** veri klasörü izinleri kurulum/güncelleme betiğinde düzeltiliyor (önceden "kaydedilemedi" hatası) — 0.5.0

### Uygulama
- [x] Harita (MapLibre + OpenFreeMap), yakındaki duraklar ve otobüsler, durak/hat ekranları
- [x] Çevrimdışı şehir paketi, internetsiz arama
- [x] Rota planlama, haritada çizim; haritada basılı tut → "Buraya nasıl giderim?"
- [x] Favoriler (telefonda), karanlık mod, veri tasarrufu modu
- [x] İzin ekranı (anonim istatistik, varsayılan kapalı), Gizlilik ve veri ekranı
- [x] Sorun bildirme (kalabalık, gelmedi, hayalet araç, geç/erken, yanlış durak/saat, arıza); "kalabalık bildirildi" etiketi
- [x] Uygulama adı **Geliyo**, paket `com.geliyo.app`, imzalı release APK
- [x] **0.5.0 geri bildirim düzeltmeleri:**
  - [x] Tema geçişi takılmadan (harita yeniden oluşturulmuyor, tema animasyonu kapalı)
  - [x] Sunucu adresi ayarlardan kaldırıldı (görünmez, değiştirilemez)
  - [x] Aramada yerler/adresler → dokununca konumdan rota
  - [x] Yakındaki duraklar konuma göre sabit (harita zoom'u etkilemiyor); izin önceden verildiyse konum açılışta otomatik
  - [x] Hat yönleri: başlangıç oku, varış bayrağı, yön okları, "kalkış → varış" satırı
  - [x] Canlı verisi olmayan şehirde açıklama notu
  - [x] "Bu araca bindim" modu, bildirim paketi, ön plan servisi ve arka plan konum izinleri **kaldırıldı** (uygulama küçüldü, Play beyanı gerekmiyor)
  - [x] Gizlilik metinleri güncellendi (yolculuk verisi yok, yer araması eklendi)

---

## 3. Neler yapılmalı? 🔜

### Hemen (yayın öncesi, 1-2 hafta)
- [ ] VPS'te `./deploy/update.sh` çalıştır (yer arama + sorun bildir düzeltmesi canlıya çıkar)
- [ ] Güvenlik: root şifresini değiştir, SSH anahtarına geç, `HIBRO_ADMIN_TOKEN`'ı yenile
- [ ] Sunucu yedeği (`deploy/data/collect` şifreli, haftalık)
- [ ] 10-20 kişilik kapalı test (Play Console dahili test); en az 12 test kullanıcısı × 14 gün kuralı
- [ ] Veri sorumlusunu belirle; aydınlatma ve açık rıza metinlerini avukata onaylat
- [ ] Topluluk şehirleri (`kk-gtfs`) lisans/izin kontrolü; izin yoksa `HIBRO_COMMUNITY=off`
- [ ] Play Store "Data safety" formu (`docs/MAGAZA.md` §4)

### Kısa vade (1-2 ay)
- [ ] **İstanbul canlı otobüs:** İETT SOAP filo konumu adaptörü
- [ ] Rotada ara durak seçme ("nereden" de seçilebilsin, şu an hep konumdan)
- [ ] Aramada son aranan yerler (telefonda)
- [ ] Bildirim düzeltme katmanı (`overrides`): `wrong_stop`/`wrong_time` bildirimleri haftalık işlenir
- [ ] Hayalet araç dedektörü: aynı araç için tekrarlı "gelmedi" bildirimi → gizle
- [ ] Bildirim kötüye kullanımına karşı güven puanı
- [ ] Gözlemlenebilirlik: kaynak tazeliği, API gecikmesi, hata oranı panosu
- [ ] Ana ekran widget'ı: favori durağın sonraki 3 aracı
- [ ] İngilizce dil desteği

### Orta vade (3-6 ay)
- [ ] **Öğrenen varış tahmini:** sunucu ESHOT araç konumlarını zaman serisi olarak kaydeder → durak arası gerçek süreler → belediye ETA'sından %30 daha az hata hedefi (kullanıcıdan GPS izi gerekmez)
- [ ] Cihazda rota (tam çevrimdışı RAPTOR)
- [ ] MOTIS ile gerçek sokak ağında yürüme süresi
- [ ] Ücret hesaplama (Konya `fare_rules` hazır), aktarma indirimi
- [ ] Erişilebilirlik: TalkBack/VoiceOver, engelli erişimli araç filtresi
- [ ] iOS sürümü (Apple Developer hesabı + Mac gerekir)

### Uzun vade
- [ ] **KentKart ile resmi görüşme:** 27 şehirde canlı araç verisi tek anlaşmayla
- [ ] Belediye görüşmeleri: "GTFS yayınlarsanız reklamsız uygulamada ücretsiz gösteririz"
- [ ] **Ankara:** OSM durakları + EGO'nun açık sayfalarından GTFS üretimi
- [ ] Şehirlerarası: Flixbus Türkiye, TCDD
- [ ] Bisiklet/scooter (Bisim, İsbike GBFS)
- [ ] Topluluk GTFS düzenleyici (moderasyonlu)

---

## Sürdürülebilirlik (reklamsız gelir)

Reklam **asla** yok. Gelir: bağış/destekçi rozeti (ücretli duvar yok), belediyelere GTFS üretim/doğrulama hizmeti, geliştiricilere birleşik Türkiye GTFS API'si, hibe ve fonlar (NLnet, TÜBİTAK, AB). Şu anki maliyet: VPS ~3 $/ay + alan adı.

## Metrikler

| Metrik | Hedef |
|---|---|
| Soğuk açılış (orta segment Android) | < 1 sn |
| Yakındaki duraklar (telefonda) | < 100 ms |
| Rota planlama p95 (sunucu) | < 800 ms |
| Canlı veri tazeliği | ≤ 15 sn |
| Çökme oranı | < %0.5 |
| Reklam / 3. taraf izleyici | **0 / 0** |

## Riskler

| Risk | Önlem |
|---|---|
| Belediye API'leri değişir/kapanır | Adaptör katmanı, tarifeye düşme, eski veri uyarısı |
| Geo-blok (İBB, ESHOT, Kocaeli) | Türkiye VPS (çözüldü) |
| Lisans (topluluk verisi) | Yayından önce izin; yoksa şehri kapat |
| Nominatim kullanım kuralları (1 istek/sn) | Sunucuda hız sınırı + 7 gün önbellek; büyürsek kendi Nominatim/Photon kurulumu |
| Kullanıcı kazanımı | "Reklamsız" mesajı, hız, üniversite toplulukları |
| KVKK | Varsayılan kapalı, kimliksiz, konum kaydı yok, avukat onayı |
