# Karar 002: Kullanıcının mobil verisini korumak

- **Tarih:** 26 Kasım 2026
- **Durum:** Uygulandı

## İlke

Kullanıcı bir şeyi **yalnızca bir kez** indirir. Tekrar sorulduğunda sunucu "değişmedi" der (304, 0 bayt gövde). Canlı veri yalnızca ekranda görünürken ve gerektiği sıklıkta istenir.

## Ölçümler (gerçek İzmir verisi, gzip)

| İstek | Boyut | Ne sıklıkla |
|---|---|---|
| Şehir paketi (tüm durak + hatlar) | İzmir ~400 KB, Konya ~100 KB | Kurulumda bir kez. Sonra en fazla 6 saatte bir kontrol; değişmediyse **0 B** (304) |
| Durak varışları | ~0,2-1 KB | Durak ekranı açıkken: araç yakınsa 15 sn, uzaksa 30 sn, sadece planlı saat varsa 2 dk |
| Yakındaki otobüsler | ~0,1-1 KB | Harita sekmesi açık, uygulama önde ve yakınlaştırılmışken 15 sn |
| Hat detayı (güzergâh + saatler) | ~2-30 KB | Hat açılınca; aynı oturumda tekrar açılırsa **0 B** (304) |
| Hat araçları | ~0,2 KB | Hat ekranı açıkken 10 sn |
| Harita karoları | Şehir başına birkaç MB | MapLibre cache'i (150 MB) sayesinde çoğunlukla bir kez |

**Tipik günlük kullanım** (uygulamayı açmak, 5 dakika bir durağı izlemek, bir hat açmak): **< 50 KB** (yeni harita alanı gezilmediyse).

## Uygulanan teknikler

| Teknik | Nerede |
|---|---|
| Çevrimdışı şehir paketi: yakındaki duraklar ve arama internetsiz | `AppState.nearby`, `AppState.search` |
| Sadece bulunulan şehrin verisi; diğer şehirlerin paketleri silinir | Karar: konuma göre şehir |
| Paket ETag'i yalnızca içerikten hesaplanır (yükleme saati hariç) → sunucu her gün yeniden yüklense de telefon tekrar indirmez | `City.PackVersion` |
| Tüm önbelleklenebilir uç noktalarda ETag + 304 | `writeCached` |
| Uygulamada bellek içi ETag önbelleği (300 kayıt) | `HibroApi._etags` |
| Paket kontrolü en fazla 6 saatte bir | `AppState.init` |
| Uyarlanabilir yenileme aralıkları | `StopScreen._nextInterval` |
| Arka planda yenileme yok (uygulama arka plana geçince tüm zamanlayıcılar durur) | `didChangeAppLifecycleState` |
| gzip (sunucu) + zstd/gzip (Caddy) | `api.middleware`, `Caddyfile` |
| Harita karo önbelleği 50 → 150 MB | `main.dart` |
| "Veri tasarrufu" modu: aralıklar ×2, yakındaki otobüsler sadece "Konumum" ile | Ayarlar |
| Reklam SDK'sı, analitik SDK'sı yok → arka planda reklam görseli/izleyici trafiği yok | İlke |

## Bilinçli olarak yapılmayanlar (şimdilik)

| Seçenek | Neden ertelendi |
|---|---|
| WebSocket/SSE ile canlı veri | Yanıtlar zaten 0,2-1 KB; HTTP/2 üzerinde kısa yoklama daha basit, mobil ağ geçişlerinde daha dayanıklı. Kullanıcı sayısı artınca tekrar değerlendirilecek |
| Paket için delta güncelleme | Değişiklik olmadıkça zaten 0 B; değişince 100-400 KB kabul edilebilir |
| İkili format (protobuf) | gzip'li JSON'a göre ~%30-40 kazanç; paket nadiren indiği için karmaşıklığa değmez |
| Wi-Fi'de şehir haritasını çevrimdışı indirme | MapLibre `downloadOfflineRegion` hazır; sıradaki iş (bkz. ROADMAP) |
