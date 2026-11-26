# hibro-server

Belediyelerin açık verisini (GTFS + canlı API'ler) tek tip, hızlı bir JSON API'ye çevirir.

```bash
go test ./...
go run ./cmd/hibro-server          # :8080, şehirler arka planda yüklenir
```

| Ortam değişkeni | Varsayılan | Açıklama |
|---|---|---|
| `HIBRO_ADDR` | `:8080` | Dinleme adresi |
| `HIBRO_DATA_DIR` | `./data-cache` | İndirilen verinin disk cache'i |
| `HIBRO_CITIES` | `izmir,konya,gaziantep` | Yüklenecek şehirler |
| `HIBRO_MOTIS_URL` | boş | MOTIS adresi; verilirse `/plan` açılır |
| `HIBRO_REFRESH` | `24h` | Statik veri yenileme aralığı |

## Mimari

```
cmd/hibro-server   giriş noktası, şehir tanımları (cities.go)
internal/api       HTTP katmanı: gzip, ETag, CORS, hata yönetimi
internal/city      Source arayüzü, şehir birleştirme, arama, yakındaki duraklar
internal/gtfs      GTFS yükleyici (sıkıştırılmış bellek düzeni) + Source
internal/izmir     ESHOT adaptörü (hat, durak, sefer, güzergâh, duyuru, canlı)
internal/cache     TTL cache + istek birleştirme
internal/fetch     disk cache'li indirici, CKAN keşfi
internal/plan      MOTIS istemcisi
```

Yeni şehir eklemek: GTFS'i varsa `cmd/hibro-server/cities.go` içine tek satır (`ckanCity(...)`). Canlı API'si varsa `internal/<şehir>` altında bir `city.Source` yazılır.

## API

Tüm ID'ler kaynak önekli: `b:12345` (ESHOT durağı), `m:1` (İzmir Metro), `k:1130a` (Konya)...

| Uç nokta | Açıklama | Cache |
|---|---|---|
| `GET /healthz` | Sağlık | – |
| `GET /v1/cities` | Şehirler, özellikler, veri kaynakları | 5 dk |
| `GET /v1/cities/{city}/pack` | Çevrimdışı paket: tüm durak + hatlar (ETag, 304) | 1 sa |
| `GET /v1/cities/{city}/stops/nearby?lat=&lon=&radius=&limit=` | Yakındaki duraklar | 30 sn |
| `GET /v1/cities/{city}/stops/{id}` | Durak | 1 sa |
| `GET /v1/cities/{city}/stops/{id}/arrivals` | Varışlar (canlı veya planlı). `live`, `degraded` alanları | 10 sn |
| `GET /v1/cities/{city}/lines` | Hatlar (raylı önce, numaraya göre sıralı) | 1 sa |
| `GET /v1/cities/{city}/lines/{id}?date=YYYY-MM-DD` | Hat: yönler, güzergâh, duraklar, sefer saatleri, duyurular | 10 dk |
| `GET /v1/cities/{city}/lines/{id}/vehicles` | Canlı araç konumları | 5 sn |
| `GET /v1/cities/{city}/alerts` | Aktif duyurular | 5 dk |
| `GET /v1/cities/{city}/search?q=` | Türkçe karakter duyarsız arama | 1 dk |
| `GET /v1/cities/{city}/plan?from=lat,lon&to=lat,lon` | Rota (MOTIS gerekli, yoksa 501) | – |

## Veri kalitesi kuralları

- **Hayalet araç filtresi:** ESHOT, GPS'i saatlerdir güncellenmeyen araçları da "yaklaşıyor" diye gönderiyor (öğlen 12'de gece otobüsü "3 dk"). GPS zamanı 5 dakikadan eski kayıtlar gösterilmez.
- **Boş yanıt:** ESHOT API'si ara sıra boş gövde döner. 3 kez yeniden denenir; yine olmazsa uygulamaya `degraded: true` gönderilir ("sistem yanıt vermiyor", "araç yok" ile karışmaz).
- **Yön başına ayrı yayınlanan hatlar** (İzmir Metro M1) tek hatta birleştirilir.
- **Koordinat düzeltme:** İzmir API'sinde `KoorX` = enlem, ondalık virgül.
- **Eski veriyle çalışmaya devam:** İndirme başarısız olursa diskteki son sağlam kopya kullanılır.
