# Hibro

**Reklamsız, hızlı, açık veriyle çalışan toplu taşıma uygulaması.** Moovit, Ekomobil ve benzeri uygulamalara alternatif.

> Durum: **Planlama aşaması** (Kasım 2026). Henüz kod yok, bu repo yol haritasını ve veri envanterini tutuyor.

## Neden Hibro?

| Sorun (Moovit vb.) | Hibro'nun cevabı |
|---|---|
| Tam ekran reklamlar, banner'lar, "Premium'a geç" baskısı | **Hiç reklam yok. Hiçbir zaman.** Reklam SDK'sı koda girmeyecek. |
| Ağır uygulama (100+ MB), yavaş açılış | Hedef: **< 15 MB APK**, **< 1 sn soğuk açılış** |
| Konum ve kullanım verisinin reklam ağlarıyla paylaşılması | İzleyici (tracker) yok. Analitik yalnızca opt-in ve kendi sunucumuzda. |
| İnternet yoksa çalışmaz | **Çevrimdışı mod:** hat, durak ve sefer saatleri cihazda |
| Belediye verisinden gecikmeli veya hatalı bilgi | Doğrudan belediye GTFS ve canlı API'lerinden veri, şeffaf kaynak gösterimi |

## Dokümanlar

- [`docs/ROADMAP.md`](docs/ROADMAP.md): Fazlara ayrılmış yol haritası, hedef metrikler
- [`docs/VERI_KAYNAKLARI.md`](docs/VERI_KAYNAKLARI.md): Türkiye'deki tüm açık GTFS ve API kaynakları (test edilmiş)
- [`docs/MIMARI.md`](docs/MIMARI.md): Teknik mimari ve teknoloji seçimleri
- [`data/feeds.yaml`](data/feeds.yaml): Veri hattının okuyacağı makine-okunur feed listesi

## İlkeler

1. **Reklamsız.** Gelir modeli bağış, isteğe bağlı destekçi üyeliği ve belediye/B2B lisansıdır. Kullanıcıya reklam gösterilmez.
2. **Hız bir özelliktir.** Her ekranın performans bütçesi var (bkz. ROADMAP, Metrikler).
3. **Gizlilik varsayılan.** Konum cihazdan çıkmadan rota hesaplanabilmeli (çevrimdışı mod).
4. **Açık veri, açık kaynak dostu.** Verinin lisansına uyulur, kaynak her zaman gösterilir.
