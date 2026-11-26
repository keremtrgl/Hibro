# Hibro Yol Haritası

> **Vizyon:** Türkiye'nin reklamsız, en hızlı ve en güvenilir toplu taşıma uygulaması.
> **Hedef:** Moovit'ten daha hızlı, daha hafif, daha doğru. Sıfır reklam.

Son güncelleme: 26 Kasım 2026

---

## 0. Moovit'i nerede geçeceğiz?

| Alan | Moovit (gözlem) | Hibro hedefi | Nasıl |
|---|---|---|---|
| Reklam | Banner, tam ekran, rota arası reklam | **0** | Reklam SDK'sı yok. Gelir: bağış, destekçi üyeliği, B2B. |
| Uygulama boyutu | ~100 MB+ | Makul (katı sınır yok) | İzleme/reklam SDK'sı yok, vektör harita, SVG |
| Soğuk açılış | 2-4 sn | **< 1 sn** (orta seviye Android) | İlk ekran tamamen yerel SQLite'tan |
| "Yakındaki duraklar" | Ağ çağrısı bekler | **< 100 ms, internetsiz** | Çevrimdışı şehir paketi |
| Rota hesaplama | 1-3 sn | **p95 < 800 ms** | MOTIS + cache |
| Canlı araç güncelleme | Değişken | **≤ 15 sn** gecikme | Doğrudan belediye API'si + SSE |
| Çevrimdışı | Sınırlı | **Tam:** durak, saat, planlı rota | Cihazda RAPTOR (Faz 4) |
| Gizlilik | Çok sayıda 3. taraf SDK | **0 izleyici**, opt-in analitik | Self-hosted |
| Veri şeffaflığı | Kaynak belirsiz | Her bilgide "kaynak: ESHOT, 12 sn önce" | RT köprüsü metadata'sı |
| Erişilebilirlik | Orta | TalkBack/VoiceOver tam destek, büyük yazı | Tasarım sisteminde zorunlu |

---

## Faz 0: Temel ve doğrulama (2 hafta)

**Amaç:** Verinin gerçekten kullanılabilir olduğunu kanıtlamak.

- [x] Türkiye açık veri envanteri (`docs/VERI_KAYNAKLARI.md`, `data/feeds.yaml`)
- [ ] Türkiye'de bir VPS kiralamak (İBB ve ESHOT geo-blok yüzünden şart). Karar verildi, gereksinimler [MIMARI §4.3](MIMARI.md)'te.
- [ ] TR IP'den ESHOT GTFS'i ve İBB `public-transport-gtfs-data` veri setini indirip doğrulamak
- [ ] Tüm feed'leri MobilityData `gtfs-validator` ile raporlamak
- [ ] İzmir canlı API'lerini 24 saat boyunca loglamak: güncelleme sıklığı, kesinti oranı, ESHOT GTFS `trip_id` ile eşleşebilirlik
- [ ] Lisans kontrolü: İzmir, Konya (CC BY 4.0), Gaziantep, İBB lisanslarının destekçi üyeliğine izin verip vermediği
- [x] Mobil çatı kararı: **Flutter** ([Karar 001](kararlar/001-mobil-cati.md))

**Çıkış kriteri:** En az 1 şehirde geçerli GTFS ve çalışan canlı veri doğrulanmış olmalı.

---

## Faz 1: MVP, İzmir (6-8 hafta)

> **Durum (26 Kasım 2026):** Faz 1'in büyük kısmı tamamlandı. Aşağıda ✅ olanlar gerçek veriyle test edildi.

**Neden İzmir?** Metro, tramvay ve otobüs GTFS'i var. **Anahtarsız canlı otobüs konumu ve durağa yaklaşan araç API'si** var. Türkiye'deki en iyi açık veri seti.

### Backend
- [x] Ingest servisi: `feeds.yaml` okuyucu, gece çekimi, hash kontrolü, doğrulama
- [x] İzmir adaptörü: koordinat düzeltmesi (`KoorX`↔`KoorY`, virgül→nokta), hat/durak ID eşleme
- [ ] **RT köprüsü:** İzmir JSON → GTFS-Realtime `VehiclePositions` (canlı veri şimdilik kendi API'mizden sunuluyor; GTFS-RT çıktısı MOTIS gecikme entegrasyonu için gerekli)
- [ ] MOTIS kurulumu: ESHOT + Metro + Tramvay + OSM yürüme ağı (`deploy/motis/prepare.sh` hazır, VPS'te test edilecek)
- [x] API: `/nearby`, `/stop/{id}/departures`, `/route/{id}`, `/plan`, `/live/route/{id}` (SSE)
- [ ] Gözlemlenebilirlik: feed tazeliği, API gecikmesi, hata oranı panosu

### Mobil uygulama
- [x] Harita (MapLibre + OSM vektör karo) ve yakındaki duraklar
- [x] Durak ekranı: planlı + canlı varışlar, "canlı" ve "planlı" etiketleri net ayrılmış
- [x] Hat ekranı: güzergâh ve haritada canlı otobüsler
- [x] A→B rota planlama arayüzü ve sunucu istemcisi (MOTIS kurulunca açılır)
- [x] Favoriler (durak, hat, ev/iş) cihazda saklanır, hesap gerekmez
- [ ] Türkçe + İngilizce (şu an yalnızca Türkçe)
- [x] Karanlık mod
- [x] "Veri kaynakları ve lisanslar" ekranı

### Kalite kapısı
- [ ] Soğuk açılış < 1 sn, harita 60 fps (Moto G / Redmi sınıfı cihazda)
- [ ] Rota p95 < 800 ms
- [ ] 20 kişilik kapalı beta (İzmir), sonra Play Store "açık test"

**Çıkış kriteri:** 500 aktif kullanıcı, Play Store puanı ≥ 4.5, çökme oranı < %0.5.

---

## Bulunan veri sorunları ve çözümleri

- **Hayalet araçlar:** ESHOT canlı API'si GPS'i saatlerdir güncellenmeyen araçları da "yaklaşıyor" diye gönderiyor (öğlen bir durakta gece 02:13'ten kalma gece otobüsleri). Hibro GPS'i 5 dakikadan eski kayıtları gizliyor. Bu, Moovit'e göre somut bir doğruluk avantajı.
- **ESHOT durak sırası yok:** API hat-durak ilişkisi vermiyor. ESHOT GTFS'i TR IP'den inince hat ekranındaki "Duraklar" sekmesi dolacak.
- **İzmir Metro M1 iki ayrı hat olarak yayınlanıyor:** Sunucu aynı numaralı yönleri tek hatta birleştiriyor.

## Faz 2: Çok şehir ve İstanbul (8-10 hafta)

- [x] **Konya** ve **Gaziantep** (hazır GTFS, eklemesi en kolay şehirler)
- [ ] **İstanbul:**
  - [ ] İBB GTFS ingest (TR VPS üzerinden)
  - [ ] İETT SOAP adaptörü: hat, durak, planlı sefer, **filo konumu → GTFS-RT**
  - [ ] Metro, Metrobüs, Marmaray, Şehir Hatları, Marmaray-İETT aktarma
  - [ ] Ölçek testi: İstanbul ~ 800+ hat, MOTIS RAM ve yanıt süresi ölçülecek
- [x] Şehir seçimi ve otomatik şehir algılama: konumdan şehir bulunur, desteklenmeyen şehirde hiçbir veri indirilmez, telefonda yalnızca seçili şehrin paketi tutulur, seyahatte "şehri değiştir" önerisi
- [x] **Çevrimdışı şehir paketleri:** JSON + ETag (İzmir 400 KB gzip). Yakındaki duraklar ve arama internetsiz çalışır.
- [x] Servis duyuruları: İzmir (İETT duyuru servisi İstanbul ile gelecek)
- [ ] Bildirim: "Favori hattında aksama var"

**Çıkış kriteri:** 5 şehir, 10.000 aktif kullanıcı.

---

## Faz 3: Verisi olmayan şehirler (sürekli)

Ankara, Bursa, Antalya, Kocaeli, Kayseri ve diğerleri.

- [ ] **Belediye görüşmeleri:** "GTFS yayınlarsanız ücretsiz, reklamsız uygulamanızda gösteririz." Ücretsiz GTFS danışmanlığı teklif edilir.
- [x] **Kocaeli eklendi** (26.11.2026): güncel belediye GTFS'i (VPS'ten) + 2018 arşivi yedek, "eski veri" uyarısıyla
- [ ] VPS'te güncel Kocaeli verisi inince `gtfsdiff` ile 2018'e göre değişimi ölç, sonucu buraya yaz
- [ ] **KentKart ile resmi görüşme:** 27 şehirde (Adana, Antalya, Erzurum, Ordu, Düzce...) hat, durak ve canlı araç verisi onlarda. Anlaşma olursa tek adımda 27 şehir. `kk-gtfs` yaklaşımı **yalnızca izin alındıktan sonra** kullanılır
- [ ] **Ankara:** OSM durakları (~2.700) + EGO'nun kamuya açık hat/sefer sayfalarından GTFS üretimi. Hat ilişkileri OSM'e topluluk katkısıyla eklenir.
- [ ] Topluluk GTFS düzenleyici: gönüllülerin hat/durak düzeltmesi yapabileceği web aracı (moderasyonlu)
- [ ] Üretilen feed'ler Transitous ve Mobility Database'e geri katkı olarak gönderilir

---

## Faz 2.5: Topluluk verisi (izinli, anonim) — BAŞLADI

Ayrıntılı tasarım ve KVKK kontrol listesi: [VERI_TOPLAMA.md](VERI_TOPLAMA.md)

**Tamamlanan (26 Kasım 2026):**
- [x] Ana haritada yakındaki canlı otobüsler (konuma göre, 15 sn'de bir, yalnızca harita açıkken)
- [x] İlk açılışta izin ekranı: "anonim istatistik" ve "yolculuk verisi", ikisi de varsayılan kapalı
- [x] Ayarlar → Gizlilik ve veri ekranı (sade dille açıklama, her an kapatılabilir)
- [x] Sorun bildirme: kalabalık, gelmedi, hayalet araç, geç/erken, yanlış durak, yanlış saat, arıza
- [x] "Kalabalık bildirildi" etiketi (son 20 dk bildirimleri varışlarda)
- [x] "Bu araca bindim" yolculuk modu: inilecek durak uyarısı + izinle yolculuk izi paylaşımı
- [x] Gizlilik korumaları: baş/son 300 m kırpma, günlük rastgele kimlik, IP saklamama, otomatik silme (30 gün iz, 1 yıl bildirim)
- [x] Sunucuda depolama (günlük JSONL), hız sınırı, token korumalı `/admin/stats`
- [x] Aydınlatma metni ve açık rıza metni **taslakları** (`docs/legal/`)

**Sıradaki:**
- [ ] Aydınlatma ve açık rıza metinlerini avukata onaylatmak; veri sorumlusunu belirlemek
- [ ] Haftalık veri düzeltme turu: `wrong_stop`/`wrong_time` bildirimlerini işleyen düzeltme katmanı (`overrides`)
- [ ] Hayalet araç dedektörü: "gelmedi" bildirimleri + yolculuk izleriyle hat bazlı GPS yaş eşiği
- [ ] Yolculuk izlerinden ESHOT hat-durak sırası çıkarımı
- [ ] Segment süreleri → öğrenen varış tahmini (hedef: belediye ETA'sından %30 daha az hata)
- [ ] İnilecek durak için gerçek sistem bildirimi (`flutter_local_notifications`)
- [ ] Bildirim kötüye kullanımına karşı güven puanı (aynı oturumdan tekrarlı bildirimler, çelişen bildirimler)
- [ ] Gizlilik dostu çökme raporları (self-hosted GlitchTip, izinle)
- [ ] Play Store "Data safety" formu

---

## Faz 4: Moovit'i geçen özellikler (Faz 2 ile paralel)

| Özellik | Açıklama | Öncelik |
|---|---|---|
| **İneceğim durak alarmı** | ✅ İlk sürüm: "Bu araca bindim" modunda titreşim + ses (ekran kapalıyken Android ön plan servisi). Sıradaki: sistem bildirimi, pil dostu geofence | Yüksek |
| **Cihazda rota (tam çevrimdışı)** | Şehir paketinde RAPTOR, internet yokken bile A→B | Yüksek |
| **Gecikme tahmini (ML)** | Canlı konum geçmişi + kullanıcı yolculuk izlerinden durağa varış tahmini; belediyenin ETA'sından daha doğru olması hedeflenir. Veri toplama altyapısı hazır (Faz 2.5) | Yüksek |
| **Kalabalık tahmini** | ✅ İlk sürüm: kullanıcı "kalabalık" bildirimleri varışlarda gösteriliyor. Sıradaki: Konya ElKart saatlik biniş verisi, İzmir biniş raporlarıyla tahmin | Orta |
| **Ana ekran widget'ı** | Favori durağın sonraki 3 aracı | Yüksek |
| **Wear OS / Apple Watch** | Sonraki araç, inme alarmı | Orta |
| **Erişilebilirlik modu** | Asansör/rampa bilgisi, sesli yönlendirme | Orta |
| **Ücret hesaplama** | `fare_rules` (Konya'da hazır), aktarma indirimi | Orta |
| **Bisiklet/scooter** | Bisim (İzmir), İsbike (İstanbul) GBFS veya API | Düşük |
| **Kart bakiyesi** | İzmirimKart, İstanbulkart (resmi entegrasyon gerekir) | Düşük, izne bağlı |

---

## Faz 5: Sürdürülebilirlik (reklamsız gelir)

Reklam **asla** yok. Gelir modeli:

1. **Bağış / destekçi üyeliği:** Tüm özellikler herkese ücretsiz kalır. Destekçiler isim rozeti, tema ve erken erişim alır. Ücretli duvar yoktur.
2. **B2B / belediye:** GTFS üretim ve doğrulama hizmeti, yolcu akışı analizi (anonim, toplu veri). Belediyelerin GTFS yayınlamasını da hızlandırır.
3. **Açık API (ücretli kademe):** Geliştiricilere birleşik Türkiye GTFS + GTFS-RT erişimi
4. **Hibe ve fonlar:** NLnet, TÜBİTAK, AB açık veri ve ulaşım fonları

Hedef maliyet (Faz 2 sonu): 2 VPS (TR ingest + API/MOTIS), CDN. Aylık ~ 50-150 $.

---

## Metrikler (her sürümde ölçülür)

| Metrik | Hedef |
|---|---|
| Harita akıcılığı | 60 fps (orta segment) |
| Soğuk açılış (orta segment Android) | < 1.0 sn |
| Yakındaki duraklar (yerel) | < 100 ms |
| Rota planlama p95 (sunucu) | < 800 ms |
| Canlı veri tazeliği | ≤ 15 sn |
| Feed tazeliği (statik) | ≤ 24 saat |
| Çökme oranı | < %0.5 |
| 3. taraf izleyici sayısı | **0** |
| Reklam sayısı | **0** |

---

## Riskler

| Risk | Olasılık | Etki | Önlem |
|---|---|---|---|
| Belediye API'leri değişir veya kapanır | Yüksek | Yüksek | Adaptör katmanı, sağlık kontrolü, planlı saatlere düşme |
| Geo-blok (İBB, ESHOT) | Kesin | Orta | TR VPS, Transitland arşivi yedek |
| Lisans ticari kullanıma izin vermez | Orta | Yüksek | Hukuki kontrol (Faz 0), gerekirse tamamen ücretsiz model |
| Verisi olmayan büyük şehirler (Ankara) | Kesin | Yüksek | Faz 3 stratejisi, belediye görüşmesi |
| Kullanıcı kazanımı (Moovit'in ağ etkisi) | Yüksek | Yüksek | "Reklamsız" mesajı, hız, yerel topluluklar, üniversiteler |
| İstanbul ölçeğinde performans | Orta | Orta | MOTIS ölçek testi, yatay ölçek, cache |
| Konum verisi nedeniyle KVKK riski | Orta | Yüksek | Varsayılan kapalı + açık rıza, kırpma, kimliksizlik, 30 gün silme, avukat onayı |
| Sahte/troll bildirimler | Orta | Orta | Hız sınırı, beyaz liste, güven puanı (sıradaki), tek bildirimle veri değiştirmeme |

---

## Hemen sonraki adımlar

1. Repoyu **gizli** yap (GitHub → Settings → Danger Zone → Change visibility)
2. Türkiye VPS'i kirala ve [deploy/README.md](../deploy/README.md) ile kur (`HIBRO_ADMIN_TOKEN` ayarla)
3. APK'yı 5-10 kişilik kapalı betaya ver, "Bu araca bindim" ile ilk yolculuk verisini topla
4. Aydınlatma/açık rıza metinlerini avukata onaylat
5. İstanbul (İBB GTFS + İETT canlı) ve MOTIS rota planlayıcıyı VPS'te aç
