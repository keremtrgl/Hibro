# Google Play ve App Store'a Yayın Rehberi

## 0. Yayından önce şart olanlar

- [ ] VPS çalışıyor, `https://api.geliyo.site/healthz` yanıt veriyor
- [ ] Uygulama sunucu adresiyle derlendi: `--dart-define=HIBRO_API=https://api.geliyo.site`
- [x] **Uygulama adı ve kimliği:** Geliyo, `com.geliyo.app` (Play'de yayınlandıktan sonra kimlik **değiştirilemez**).
- [ ] Gizlilik metni avukata onaylatıldı, veri sorumlusu bilgileri dolduruldu (`server/internal/api/static/gizlilik.html`, `docs/legal/`)
- [ ] Topluluk verisi (KentKart kaynaklı 13 şehir) için lisans/izin netleşti; olmazsa sunucuda `HIBRO_COMMUNITY=off`

## 1. İmzalama anahtarı (bir kez)

```bash
keytool -genkey -v -keystore ~/geliyo-upload.jks -keyalg RSA -keysize 2048 -validity 10000 -alias upload
```

`app/android/key.properties` (repoya **eklenmez**, `.gitignore`'da):

```
storeFile=/home/SEN/geliyo-upload.jks
storePassword=...
keyAlias=upload
keyPassword=...
```

Anahtarı ve şifreyi **yedekle**. Kaybedersen güncelleme yükleyemezsin (Play App Signing kullanıldığı için Google'dan sıfırlama istenebilir ama zahmetli).

## 2. Derleme

```bash
cd app
flutter build appbundle --release --dart-define=HIBRO_API=https://api.geliyo.site
# çıktı: build/app/outputs/bundle/release/app-release.aab
```

Her yeni sürümde `pubspec.yaml` içindeki `version: 0.4.0+4` değerinin `+` sonrasını artır.

## 3. Play Console

1. <https://play.google.com/console> → geliştirici hesabı (tek seferlik 25 $).
2. **Uygulama oluştur:** Ad "Geliyo", Türkçe, Uygulama, Ücretsiz. (Alternatif mağaza adı: "Geliyo: Otobüs ve Metro Saatleri", 30 karaktere kadar.)
3. **Mağaza girişi:**

**Kısa açıklama (80 karakter):**
> Reklamsız toplu taşıma: canlı otobüs, yakındaki duraklar, rota. Hesap yok.

**Tam açıklama:**
> Geliyo, Türkiye'nin reklamsız toplu taşıma uygulaması.
>
> • Hiç reklam yok. Hiçbir zaman.
> • Canlı otobüs takibi (İzmir): durağa kaç dakikada geleceğini, kaç durak kaldığını, plakayı ve engelli erişimini gör.
> • Yakındaki duraklar ve otobüsler, internet olmadan bile arama.
> • A'dan B'ye rota: otobüs, metro, tramvay ve banliyö aktarmalı.
> • Yer ve adres arama: "Kocaeli Üniversitesi" yaz, konumundan nasıl gidileceğini gör.
> • Hat yönleri net: haritada başlangıç oku, varış bayrağı ve yön okları.
> • Hayalet otobüs filtresi: belediye sisteminde saatlerdir sinyal vermeyen araçları göstermez.
> • Gizlilik önce: hesap yok, izleyici yok, favorilerin sadece telefonunda.
> • Az veri kullanır: şehir verisi bir kez iner, sonra sadece değişenler.
>
> Desteklenen şehirler: İzmir, İstanbul, Konya, Gaziantep, Kocaeli, Antalya, Adana, Muğla, Erzurum, Ordu, Düzce, Mardin, Çanakkale, Burdur, Osmaniye, Safranbolu, Kırklareli, Kastamonu.
>
> Veriler belediyelerin açık verisinden gelir. Bir hata görürsen uygulamadan bildir, birlikte düzeltelim.

4. **Grafikler:** ikon 512×512 (`docs/img/ikon.png` büyük sürümü), öne çıkan görsel 1024×500, en az 2 telefon ekran görüntüsü (`docs/img/` içindekiler örnek).
5. **Gizlilik politikası URL'si:** `https://api.geliyo.site/gizlilik`
6. **Uygulama içeriği:**
   - Reklam içeriyor mu: **Hayır**
   - Hedef kitle: 13+ (genel)
   - İçerik derecelendirmesi anketi: şiddet/kumar vb. hepsi "hayır"

## 4. Veri güvenliği (Data safety) formu

| Soru | Yanıt |
|---|---|
| Veri toplanıyor mu? | Evet |
| Veri üçüncü taraflarla paylaşılıyor mu? | Hayır |
| Aktarımda şifreleniyor mu? | Evet (HTTPS) |
| Kullanıcı silme isteyebilir mi? | Veriler kimliğe bağlı değil; paylaşımlar uygulamadan kapatılabilir |

Toplanan veri türleri:

| Tür | Toplanıyor | Zorunlu mu | Amaç |
|---|---|---|---|
| Konum → Kesin/yaklaşık konum | İşleniyor ama **kaydedilmiyor** (yakındaki otobüs/rota isteği). Play'de "geçici işleme" (ephemeral) olarak işaretlenir | İsteğe bağlı | Uygulama işlevi |
| Uygulama etkinliği → Uygulama içi arama geçmişi | Yer araması metni OSM Nominatim'e iletilir, kimliksiz, 7 gün önbellek | Zorunlu değil | Uygulama işlevi |
| Uygulama etkinliği → Uygulama etkileşimleri | Evet (anonim sayaçlar, izinle) | İsteğe bağlı | Analiz |
| Uygulama etkinliği → Kullanıcı tarafından oluşturulan içerik | Evet (sorun bildirimi notu) | İsteğe bağlı | Uygulama işlevi |
| Kişisel bilgiler, finans, kişiler, fotoğraf, mesaj, cihaz kimliği | Hayır | | |

## 5. Test kanalları

1. **Dahili test** (100 kişiye kadar, inceleme hızlı): ilk APK/AAB buraya.
2. **Kapalı test**: Google yeni kişisel hesaplarda üretime çıkmadan önce **en az 12 test kullanıcısıyla 14 gün** kapalı test istiyor. Arkadaş/aile grubuyla başla.
3. Üretim.

## 6. Arka plan konum beyanı

Gerekmiyor. "Bu araca bindim" modu 0.5.0'da kaldırıldı; uygulama yalnızca açıkken konum kullanır. Ön plan servisi, bildirim ve arka plan konum izni yok.

## 7. App Store (iOS)

- Apple Developer hesabı (yıllık 99 $), Mac ve Xcode gerekir.
- `ios/Runner/Info.plist` konum açıklaması hazır (arka plan konum modu yok).
- App Privacy formu: yukarıdaki tabloyla aynı.
