# Geliyo mobil uygulaması (Flutter)

Reklamsız, izleyicisiz toplu taşıma uygulaması. Android, iOS ve web.

## Özellikler

- Konumdan otomatik şehir seçimi; yalnızca bulunduğun şehrin verisi iner (desteklenmeyen şehirde hiçbir şey indirilmez)
- Harita (MapLibre + OpenFreeMap, API anahtarı ve izleme yok)
- Yakındaki duraklar: **çevrimdışı**, cihazdaki şehir paketinden hesaplanır
- Durak ekranı: canlı varışlar (İzmir), planlı seferler (metro, tramvay, Konya, Gaziantep), 15 sn'de bir otomatik yenileme, uygulama arka plandayken durur
- Hat ekranı: güzergâh, canlı araçlar, durak sırası, sefer saatleri (sıradaki vurgulu), duyurular
- Anında arama: durak adı, durak numarası veya hat (Türkçe karakter duyarsız, çevrimdışı)
- Favoriler (yalnızca cihazda), duyurular, şehir seçimi, açık/koyu tema
- Veri kaynakları ve lisans ekranı
- Rota planlama (sunucuda MOTIS kuruluysa görünür)
- Ana haritada yakındaki canlı otobüsler
- Sorun bildirme (kalabalık, gelmedi, yanlış durak...)
- Yer ve adres arama (OSM), dokununca konumdan rota
- Hat haritasında başlangıç oku, varış bayrağı ve yön okları
- İlk açılışta izin ekranı; tüm paylaşımlar varsayılan kapalı (Ayarlar → Gizlilik ve veri)

## Çalıştırma

```bash
flutter pub get
flutter run --dart-define=HIBRO_API=http://SUNUCU:8080
```

Android emülatöründe bilgisayardaki sunucu için varsayılan `http://10.0.2.2:8080` kullanılır.

## Yayın derlemesi

```bash
flutter build apk --release --split-per-abi --dart-define=HIBRO_API=https://api.geliyo.site
flutter build appbundle --release --dart-define=HIBRO_API=https://api.geliyo.site   # Play Store
```

Play Store için imzalama anahtarı oluşturulmalı (`android/key.properties`, repoya eklenmez).

## Yasaklar

Reklam SDK'sı, Firebase Analytics/Crashlytics ve 3. taraf izleyiciler bu projeye eklenmez.
