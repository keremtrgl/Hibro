# Veri Toplama Politikası ve Teknik Tasarım

Son güncelleme: 26 Kasım 2026

> **Özet:** Geliyo, belediye verisindeki eksik ve hataları kapatmak için kullanıcılardan **izinli ve anonim** veri toplar. Reklam yok, veri satışı yok, üçüncü taraf SDK yok. Her paylaşım **varsayılan olarak kapalıdır** ve kullanıcı istediği an kapatabilir.

## 1. Neden veri topluyoruz?

Belediye verisinin bilinen sorunları:

| Sorun | Örnek | Topluluk verisiyle çözüm |
|---|---|---|
| Hayalet araçlar | ESHOT, GPS'i saatlerdir susan araçları "3 dk" diye gösteriyor | "Araç gelmedi" bildirimleri + yolculuk izleri |
| Yanlış varış süresi | Belediye ETA'sı trafiği hesaba katmıyor | Yolculuk izlerinden gerçek segment süreleri, öğrenen tahmin |
| Eksik durak sırası | ESHOT API'si hat-durak ilişkisi vermiyor | Yolculuk izlerinin durak koordinatlarıyla eşleştirilmesi |
| Yanlış durak konumu/adı | Duraklar haritada yanlış yerde | "Durak yanlış" bildirimleri |
| Kalabalık bilgisi yok | Hiçbir belediye yayınlamıyor | "Kalabalık" bildirimleri (son 20 dk varışlarda gösterilir) |
| Hangi özellik/şehir önemli | Bilinmiyor | Anonim kullanım sayaçları, "sonuçsuz arama" sayısı |

## 2. Ne topluyoruz? (tam liste)

| Veri | Ne zaman | Ne içerir | Ne İÇERMEZ | Saklama |
|---|---|---|---|---|
| **Sorun bildirimi** | Kullanıcı "Sorun bildir"e basınca | Tür, durak ID, hat ID, plaka (varsa), not (≤280 karakter), uygulama sürümü, sunucu saati | Konum, IP, cihaz kimliği | 365 gün |
| **Anonim kullanım sayacı** | Yalnızca izin varsa, 60 sn'de bir toplu | `şehir/olay → sayı` (ör. `izmir/screen_stop: 3`). Olaylar sunucuda beyaz listeyle sınırlı | Kimlik, oturum, konum, zaman damgası | Günlük toplam olarak süresiz |
| **Yolculuk izi** | Yalnızca "Bu araca bindim" modunda ve izin varsa | Hat, yön, GPS noktaları (zaman, enlem, boylam, doğruluk, hız), günlük değişen rastgele oturum kimliği | Yolculuğun ilk/son 300 m'si, IP, cihaz kimliği | Ham veri 30 gün, sonra yalnızca toplu istatistik |

Toplanmayanlar: isim, telefon, e-posta, reklam kimliği, cihaz modeli, kişi listesi, arka planda sürekli konum.

## 3. Koruma önlemleri (kodda uygulanan)

| Önlem | Nerede |
|---|---|
| Varsayılan kapalı, ilk açılışta açık seçim | `app/lib/widgets/consent_sheet.dart` |
| İzin her an geri alınabilir | `app/lib/screens/privacy_screen.dart` |
| Baş/son 300 m kırpma (önce telefonda, sonra sunucuda tekrar) | `app/lib/screens/ride_screen.dart` `trimTrip`, `server/internal/collect/trips.go` `trimEnds` |
| Günlük değişen rastgele oturum kimliği | `AppState.dailySession` |
| Koordinatlar ~1 m'ye yuvarlanır, doğruluğu kötü (>80 m) ve mantıksız hızlı noktalar atılır | `TripUpload.Clean` |
| Şehir sınırı dışındaki noktalar atılır | `TripUpload.Clean` + `City.Bounds` |
| Sayaçlarda beyaz liste: tanımsız olay kaydedilmez | `collect.TelemetryEvents` |
| Bildirimlerde bilinmeyen alanlar reddedilir (ör. e-posta alanı eklenirse 400) | `DisallowUnknownFields` |
| IP adresi diske yazılmaz; hız sınırı yalnızca bellekte, tuzlu özetle | `collect.Limiter` |
| Caddy erişim logları kapalı | `deploy/Caddyfile` |
| Otomatik silme (6 saatte bir) | `cmd/hibro-server/collect.go` |
| Yönetici istatistikleri token korumalı, token yoksa uç nokta kapalı | `GET /admin/stats` |
| Veri Türkiye'deki sunucuda | VPS kararı |

## 4. Veri akışı

```
Telefon                                   Sunucu (Türkiye)                       Kullanım
───────                                   ────────────────                       ────────
Sorun bildir ──────────────► POST /reports ──► collected/reports/GÜN.jsonl ──► veri düzeltme listesi
                                          └──► bellek: son 2 saat ─────────────► "Kalabalık bildirildi"
Sayaçlar (izinle) ─────────► POST /telemetry ─► collected/telemetry/GÜN.json ──► öncelik kararları
Bindim modu (izinle)
  └─ telefonda kırp ───────► POST /trips ─────► collected/trips/GÜN.jsonl ───► (Faz 4) segment süreleri,
                                               (30 gün sonra silinir)            öğrenen varış tahmini
```

## 5. Toplanan veriye bakmak

```bash
# .env içinde HIBRO_ADMIN_TOKEN=uzun-rastgele-bir-değer
curl -H "Authorization: Bearer $HIBRO_ADMIN_TOKEN" https://api.geliyo.site/admin/stats?days=7
```

Dönen özet: bildirim türleri, en çok bildirim alan hat ve duraklar, son notlar, yolculuk sayısı, hat bazında yolculuk, günlük sayaçlar.

Ham dosyalar: VPS'te `deploy/data/collect/` (JSONL, satır başına bir kayıt). Yedeklenirken de **şifreli** saklanmalı.

## 6. Veriyi uygulamayı geliştirmek için nasıl kullanacağız?

1. **Haftalık veri düzeltme turu:** `wrong_stop` / `wrong_time` bildirimleri → belediyeye iletilir, kendi düzeltme katmanımıza (`overrides`) eklenir.
2. **Hayalet araç dedektörü:** Bir araç için "gelmedi" bildirimi + aynı anda o hatta kimsenin binmemesi → aracı gizle, GPS yaş eşiğini hat bazında ayarla.
3. **Öğrenen varış tahmini (Faz 4):** Yolculuk izleri → durak arası gerçek süreler (saat dilimi × gün tipi × hat) → belediye ETA'sını düzelten model. Başarı ölçütü: tahmin hatasının medyanı belediye ETA'sından en az %30 düşük.
4. **ESHOT durak sırası:** İzlerin durak koordinatlarıyla eşleştirilmesiyle hat-durak sırası çıkarılır (GTFS gelene kadar).
5. **Ürün kararları:** "sonuçsuz arama" çok olan şehir/semt → veri önceliği; az kullanılan ekran → sadeleştirme.

## 7. KVKK kontrol listesi (yayından önce)

- [ ] Veri sorumlusu (şirket/şahıs) belirlensin, iletişim adresi uygulamaya ve [aydınlatma metnine](legal/aydinlatma-metni-taslak.md) yazılsın
- [ ] Aydınlatma metni ve [açık rıza metni](legal/acik-riza-metni-taslak.md) **avukata kontrol ettirilsin**
- [ ] Konum verisi için açık rıza akışı uygulamadaki ilk açılış ekranıyla eşleşiyor mu kontrol edilsin
- [ ] VERBİS kayıt yükümlülüğü değerlendirilsin (çalışan sayısı / bilanço eşiği ve faaliyet türüne bağlı)
- [ ] Kişisel veri saklama ve imha politikası yazılsın (bu dokümandaki süreler esas alınabilir)
- [ ] Veri ihlali müdahale planı (72 saat bildirim yükümlülüğü)
- [ ] Play Store "Data safety" ve App Store "Privacy nutrition label" formları bu tabloya göre doldurulsun
- [ ] Yedekler şifreli olsun; VPS erişimi yalnızca SSH anahtarıyla

> Bu doküman hukuki danışmanlık değildir. Teknik tasarımı açıklar; yasal metinler bir avukat tarafından onaylanmalıdır.
