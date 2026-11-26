# Veri Toplama Politikası ve Teknik Tasarım

Son güncelleme: 26 Kasım 2026

> **Özet:** Geliyo, belediye verisindeki eksik ve hataları kapatmak için kullanıcılardan **izinli ve anonim** veri toplar. Reklam yok, veri satışı yok, üçüncü taraf SDK yok. Her paylaşım **varsayılan olarak kapalıdır** ve kullanıcı istediği an kapatabilir.

## 1. Neden veri topluyoruz?

Belediye verisinin bilinen sorunları:

| Sorun | Örnek | Topluluk verisiyle çözüm |
|---|---|---|
| Hayalet araçlar | ESHOT, GPS'i saatlerdir susan araçları "3 dk" diye gösteriyor | GPS yaşı filtresi (5 dk) + "Araç gelmedi" bildirimleri |
| Yanlış varış süresi | Belediye ETA'sı trafiği hesaba katmıyor | Canlı araç konumlarının sunucuda zaman serisi olarak izlenmesi (kullanıcı verisi gerekmez, Faz 4) |
| Eksik durak sırası | ESHOT API'si hat-durak ilişkisi vermiyor | ESHOT GTFS ile eşleştirme (yapıldı) + "durak yanlış" bildirimleri |
| Yanlış durak konumu/adı | Duraklar haritada yanlış yerde | "Durak yanlış" bildirimleri |
| Kalabalık bilgisi yok | Hiçbir belediye yayınlamıyor | "Kalabalık" bildirimleri (son 20 dk varışlarda gösterilir) |
| Hangi özellik/şehir önemli | Bilinmiyor | Anonim kullanım sayaçları, "sonuçsuz arama" sayısı |

## 2. Ne topluyoruz? (tam liste)

| Veri | Ne zaman | Ne içerir | Ne İÇERMEZ | Saklama |
|---|---|---|---|---|
| **Sorun bildirimi** | Kullanıcı "Sorun bildir"e basınca | Tür, durak ID, hat ID, plaka (varsa), not (≤280 karakter), uygulama sürümü, sunucu saati | Konum, IP, cihaz kimliği | 365 gün |
| **Anonim kullanım sayacı** | Yalnızca izin varsa, 60 sn'de bir toplu | `şehir/olay → sayı` (ör. `izmir/screen_stop: 3`). Olaylar sunucuda beyaz listeyle sınırlı | Kimlik, oturum, konum, zaman damgası | Günlük toplam olarak süresiz |
| **Yer araması** | Kullanıcı arama kutusuna en az 3 harf yazınca | Arama metni + şehir; sunucu bunu OSM Nominatim'e iletir (saniyede en fazla 1 istek) | Konum, IP (Nominatim sunucumuzun IP'sini görür), kimlik | Sonuç önbelleği 7 gün |

> **Kaldırılan:** "Bu araca bindim" yolculuk izi toplama 0.5.0'da kaldırıldı (kullanıcı geri bildirimi: gereksiz). Sunucudaki `/trips` ucu eski sürümler için duruyor; yeni sürüm hiçbir GPS izi göndermez.

Toplanmayanlar: isim, telefon, e-posta, reklam kimliği, cihaz modeli, kişi listesi, arka planda sürekli konum.

## 3. Koruma önlemleri (kodda uygulanan)

| Önlem | Nerede |
|---|---|
| Varsayılan kapalı, ilk açılışta açık seçim | `app/lib/widgets/consent_sheet.dart` |
| İzin her an geri alınabilir | `app/lib/screens/privacy_screen.dart` |
| Yer araması sunucu üzerinden: kullanıcının IP'si üçüncü tarafa gitmez, istekler önbelleklenir | `server/internal/geocode` |
| Sunucu adresi uygulamaya gömülü, kullanıcı görmez/değiştiremez | `AppState` (`HIBRO_API` derleme değişkeni) |
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
Yer araması ───────────────► GET /places ────► önbellek (7 gün) ──► OSM Nominatim (sunucu IP'siyle)
```

## 5. Toplanan veriye bakmak

```bash
# .env içinde HIBRO_ADMIN_TOKEN=uzun-rastgele-bir-değer
curl -H "Authorization: Bearer $HIBRO_ADMIN_TOKEN" https://api.geliyo.site/admin/stats?days=7
```

Dönen özet: bildirim türleri, en çok bildirim alan hat ve duraklar, son notlar, günlük sayaçlar.

Ham dosyalar: VPS'te `deploy/data/collect/` (JSONL, satır başına bir kayıt). Yedeklenirken de **şifreli** saklanmalı.

## 6. Veriyi uygulamayı geliştirmek için nasıl kullanacağız?

1. **Haftalık veri düzeltme turu:** `wrong_stop` / `wrong_time` bildirimleri → belediyeye iletilir, kendi düzeltme katmanımıza (`overrides`) eklenir.
2. **Hayalet araç dedektörü:** Bir araç için birden çok "gelmedi" bildirimi → aracı gizle, GPS yaş eşiğini hat bazında ayarla.
3. **Öğrenen varış tahmini (Faz 4):** Sunucu, belediyenin yayınladığı canlı araç konumlarını kaydeder → durak arası gerçek süreler (saat dilimi × gün tipi × hat) → belediye ETA'sını düzelten model. Kullanıcıdan GPS izi gerekmez. Başarı ölçütü: tahmin hatasının medyanı belediye ETA'sından en az %30 düşük.
4. **ESHOT durak sırası:** ESHOT GTFS'inden alınıyor (yapıldı); bildirimlerle düzeltilir.
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
