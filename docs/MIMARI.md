# Teknik Mimari

## 1. Genel görünüm

```
 ┌──────────────── Veri kaynakları ────────────────┐
 │ Belediye GTFS (ZIP) · CKAN portalları · İETT SOAP │
 │ İzmir REST · Transitland arşivi · OpenStreetMap   │
 └───────────────┬──────────────────────────────────┘
                 │  (Türkiye'deki VPS: geo-blok nedeniyle)
        ┌────────▼────────┐
        │  Ingest servisi │  şehir adaptörleri, zamanlanmış çekim
        └────────┬────────┘
     statik      │        canlı (10-30 sn)
   ┌─────────────▼───┐   ┌───────────────────┐
   │ GTFS doğrula +  │   │  RT köprüsü       │  belediye JSON/SOAP → GTFS-Realtime
   │ birleştir +     │   │  (VehiclePosition,│
   │ shapes üret     │   │   TripUpdate)     │
   └───────┬─────────┘   └────────┬──────────┘
           │                      │
     ┌─────▼──────────────────────▼─────┐
     │       MOTIS rota motoru          │  RAPTOR tabanlı, GTFS + GTFS-RT + OSM
     └─────────────┬────────────────────┘
                   │
     ┌─────────────▼────────────┐      ┌──────────────────────┐
     │  API ağ geçidi (Go/Rust) │──────│ Redis cache, Postgres│
     │  REST + WebSocket/SSE    │      │ + PostGIS            │
     └─────────────┬────────────┘      └──────────────────────┘
                   │
     ┌─────────────▼────────────┐
     │ Mobil uygulama           │  çevrimdışı paket (SQLite + vektör karo)
     │ Android & iOS            │
     └──────────────────────────┘
```

## 2. Teknoloji seçimleri

| Katman | Seçim | Neden | Alternatif |
|---|---|---|---|
| Rota motoru | **MOTIS 2** | Açık kaynak, hızlı (RAPTOR), GTFS-RT desteği, OSM ile yürüme/bisiklet, Transitous'ta 60+ ülkede kanıtlanmış | OpenTripPlanner 2 (Java, daha fazla RAM) |
| Ingest / ETL | **Python** (`gtfs-kit`, `partridge`) + `gtfs-validator` (MobilityData) | GTFS ekosistemi Python'da güçlü | Go |
| RT köprüsü & API | **Go** | Düşük gecikme, tek binary, düşük RAM | Rust (axum) |
| Veritabanı | PostgreSQL + PostGIS, Redis | Coğrafi sorgu ("yakındaki duraklar"), canlı veri cache | |
| Harita | **MapLibre** + kendi vektör karolarımız (Protomaps/PMTiles, OSM) | Google Maps ücreti yok, çevrimdışı karo mümkün, izleyici yok | |
| Mobil | **Flutter** (Riverpod, drift, maplibre_gl) | Olgun MapLibre desteği, tek kod tabanı, hızlı geliştirme. Bkz. [Karar 001](kararlar/001-mobil-cati.md) | KMP + Compose |
| Çevrimdışı veri | SQLite (şehir paketi, ~5-20 MB) | Cihazda durak/saat sorgusu, internetsiz rota | |
| Bildirimler | Self-hosted ntfy / UnifiedPush + FCM fallback | Google bağımlılığını azaltır | |
| Analitik | Opt-in, self-hosted (Plausible/Umami tarzı, olay sayımı) | Gizlilik, reklam ağı yok | |
| Barındırma | Türkiye VPS (ingest) + Avrupa/TR sunucu (API) | Geo-blok ve KVKK | |

## 3. Performans tasarımı: Moovit'ten hızlı olmanın yolu

1. **Önce yerel:** Yakındaki duraklar ve planlı saatler cihazdaki SQLite'tan gelir (0 ağ çağrısı, < 50 ms).
2. **Canlı veri delta olarak:** Sadece görünen hatların araç konumu SSE/WebSocket ile akar, tam liste çekilmez.
3. **Rota sonucu cache'i:** Sık sorgulanan A→B çiftleri 60 sn cache'lenir.
4. **Küçük bundle:** Reklam ve izleme SDK'sı yok (Moovit'te 10+ SDK var). Harita karoları vektör, ikonlar SVG.
5. **Soğuk açılışta ağ yok:** İlk ekran (favoriler + yakın duraklar) tamamen yerelden çizilir.
6. **Pil:** Konum yalnızca uygulama ön plandayken veya "inmem gereken durak" alarmı aktifken alınır.

## 4. Açık kararlar

### 4.1 Mobil çatı: ✅ Karar verildi, Flutter
Ayrıntılar ve performans kuralları: [Karar 001](kararlar/001-mobil-cati.md).

### 4.2 Rota motorunu sunucuda mı cihazda mı çalıştıralım?
- MVP: Sunucuda (MOTIS).
- Faz 4: Tek şehir için cihazda hafif RAPTOR (çevrimdışı rota). Bir şehrin zaman tablosu birkaç MB tutar, bu yapılabilir.

## 4.3 Türkiye VPS gereksinimleri

İBB ve ESHOT yurt dışı IP'lerini engellediği için sunucu Türkiye'de olacak.

| Aşama | vCPU | RAM | Disk | Ne çalışır |
|---|---|---|---|---|
| Faz 0-1 (İzmir, beta) | 2 | 4 GB | 40 GB SSD | Hepsi tek makinede, Docker Compose |
| Faz 2 (+ İstanbul, Konya, Gaziantep) | 4 | 8 GB | 80 GB | Kullanıcı ve şehir sayısı artınca yükseltilir |

Ölçüm (26.11.2026): 17 şehir birlikte (İzmir canlı + 16 GTFS şehri) sunucu belleği **~650 MB** (GOGC=50, en fazla 3 şehir aynı anda yüklenir, yükleme sonrası bellek işletim sistemine iade edilir; önceki ayarla ~820 MB). İlk yükleme ~80 sn. 2 vCPU / 4 GB VPS yeterli.

İleride gerekirse: sefer tablolarını desen (pattern) bazında saklamak satır başına ~8 bayt, toplamda ~90 MB daha kazandırır.

Neden bu kadar küçük yeterli:
- OSM verisinin tamamı (Türkiye) değil, yalnızca şehir sınırı kesilir (`osmium extract`). İzmir kesiti ~100 MB, tüm Türkiye ~1 GB+.
- RAM'i asıl tüketen MOTIS ön işlemesidir. Bu işlem kendi bilgisayarında yapılıp hazır veri sunucuya yüklenebilir. Sunucu yalnızca çalıştırır.
- Canlı veri cache'lenir. 100 kişi aynı hattı izlese bile belediyeye tek istek gider.
- Darboğaz görülürse VPS paneldeki tek tıkla büyütülür. Baştan büyük almaya gerek yok.

Seçim kriterleri:
- Türkiye lokasyonu (İstanbul veya Ankara veri merkezi), statik IPv4
- NVMe disk, aylık en az 2 TB trafik
- Snapshot/yedek desteği
- KVKK uyumu: kullanıcı verisi Türkiye'de kalır (zaten minimum veri tutuyoruz)

Kurulum: Ubuntu 24.04 LTS, Docker Compose, Caddy (otomatik HTTPS), sadece 22/80/443 açık, SSH yalnızca anahtarla.

## 5. Veri hattı adımları (her gece 03:00 TR)

1. `data/feeds.yaml` içindeki her feed'i indir (CKAN'da en yeni kaynağı keşfet)
2. Hash değişmediyse atla
3. MobilityData `gtfs-validator` çalıştır, hata varsa önceki sürümde kal ve alarm üret
4. Normalizasyon: ajans ID önekleri (`izm-eshot:`, `kny:`), Türkçe karakter düzeltme, `shapes` üretimi (pfaedle + OSM)
5. Şehir bazında birleştir, MOTIS'e yükle (sıfır kesinti: blue/green)
6. Çevrimdışı şehir paketlerini (SQLite) üret, CDN'e yükle
