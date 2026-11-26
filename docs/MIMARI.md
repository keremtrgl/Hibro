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
| Mobil | **Kotlin Multiplatform (Compose MP)** veya Flutter | Native performans, küçük APK. Bkz. karar 4.1 | React Native |
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

### 4.1 Mobil çatı: KMP mi Flutter mı?
- **KMP + Compose:** En iyi Android performansı, küçük APK, iOS'ta SwiftUI de kullanılabilir. Ekosistem daha genç.
- **Flutter:** Tek kod tabanı, hızlı geliştirme, MapLibre eklentisi olgun. APK ~8-10 MB daha büyük.
- **Öneri:** Ekip küçükse Flutter ile MVP. Performans bütçesi aşılırsa Android'i native Compose'a taşı.

### 4.2 Rota motorunu sunucuda mı cihazda mı çalıştıralım?
- MVP: Sunucuda (MOTIS).
- Faz 4: Tek şehir için cihazda hafif RAPTOR (çevrimdışı rota). Bir şehrin zaman tablosu birkaç MB tutar, bu yapılabilir.

## 5. Veri hattı adımları (her gece 03:00 TR)

1. `data/feeds.yaml` içindeki her feed'i indir (CKAN'da en yeni kaynağı keşfet)
2. Hash değişmediyse atla
3. MobilityData `gtfs-validator` çalıştır, hata varsa önceki sürümde kal ve alarm üret
4. Normalizasyon: ajans ID önekleri (`izm-eshot:`, `kny:`), Türkçe karakter düzeltme, `shapes` üretimi (pfaedle + OSM)
5. Şehir bazında birleştir, MOTIS'e yükle (sıfır kesinti: blue/green)
6. Çevrimdışı şehir paketlerini (SQLite) üret, CDN'e yükle
