# Karar 001: Mobil çatı olarak Flutter

- **Tarih:** 26 Kasım 2026
- **Durum:** Kabul edildi

## Bağlam

Android ve iOS için tek bir uygulama gerekiyor. Hedefler: soğuk açılış < 1 sn, akıcı harita, çevrimdışı mod. Ekip küçük.

İki aday değerlendirildi: **Flutter** ve **Kotlin Multiplatform + Compose Multiplatform (KMP)**.

## Karşılaştırma

| Kriter | Flutter | KMP + Compose MP |
|---|---|---|
| Tek kod tabanı (UI dahil) | ✅ Olgun | ✅ iOS desteği daha yeni |
| Harita (MapLibre) | ✅ `maplibre_gl`, yaygın kullanılıyor | ⚠️ `maplibre-compose`, daha genç |
| Çevrimdışı SQLite | ✅ `drift` / `sqflite` | ✅ SQLDelight |
| Ana ekran widget'ı, Wear OS | ⚠️ Native kod gerekir (`home_widget`) | ✅ Android'de doğal |
| APK boyutu (ABI başına) | ~7-10 MB taban (sorun değil) | ~4-6 MB taban |
| Geliştirme hızı, hot reload | ✅ Çok hızlı | Orta |
| Öğrenme kaynağı (Türkçe dahil) | ✅ Çok fazla | Az |
| Kullanıcının hissedeceği performans | Arayüz 60/120 fps, Impeller ile takılma yok | Native, çok az daha iyi |

## Karar

**Flutter.** Bu projede asıl yük haritada ve veride. Flutter'ın MapLibre desteği daha olgun, geliştirme hızı daha yüksek. Moovit'i yavaşlatan şey çatısı değil: reklam ve izleme SDK'ları, açılışta yapılan ağ çağrıları ve ağır başlangıç akışı. Geliyo bunları zaten tasarımdan çıkarıyor.

KMP'nin tek net avantajı daha küçük APK ve doğal Android widget'ı. Boyut bu projede öncelik değil. Widget ve Wear OS için gereken küçük native parçalar Flutter'da platform kanalıyla yazılır.

## Performans kuralları (Flutter'a özel)

- `flutter build apk --split-per-abi --obfuscate --split-debug-info` ile yayın. Play Store'a AAB yüklenir.
- Açılışta yalnızca yerel SQLite okunur. Ağ, ilk kare çizildikten sonra başlar.
- Firebase Analytics, Crashlytics ve reklam paketleri **yasak.** Çökme raporu için self-hosted GlitchTip (Sentry uyumlu), opt-in.
- Boyut için katı sınır yok. Öncelik hız ve akıcılık. Boyut yine de her sürümde raporlanır ki sessizce şişmesin.
- Durum yönetimi: Flutter'ın kendi `ChangeNotifier` + `InheritedNotifier` yapısı (ek kütüphane yok). Çevrimdışı paket: JSON dosyası. Harita: maplibre_gl + OpenFreeMap. HTTP: http paketi.

## Ne zaman yeniden değerlendirilir?

Orta segment Android cihazda soğuk açılış 1 sn hedefini veya harita 60 fps hedefini Faz 1 sonunda tutturamazsak, Android uygulaması native Compose'a taşınır.
