# Veri Kaynakları Envanteri: Türkiye Toplu Taşıma

Son kontrol: **26 Kasım 2026**. "Test edildi" işaretli kaynaklar o tarihte indirilip içeriği incelendi.

Durum işaretleri:
- ✅ Çalışıyor, güncel
- ⚠️ Var ama sorunlu (eski, erişim kısıtlı, format hatası)
- ❌ Yok veya ulaşılamadı
- 🌍 Yalnızca Türkiye IP'sinden erişilebiliyor olabilir (yurt dışından zaman aşımı)

---

## 1. Özet tablo

| Şehir | Statik GTFS | Canlı araç konumu | Durağa yaklaşan araç | Not |
|---|---|---|---|---|
| **İzmir** | ✅ Metro, Tramvay · 🌍 ESHOT otobüs · ⚠️ İzdeniz | ✅ API, anahtarsız | ✅ API | En iyi başlangıç şehri |
| **Konya** | ✅ 179 hat, 4.494 durak, ücret + shapes | ❌ | ⚠️ 2022 CSV | GTFS çok kaliteli |
| **Gaziantep** | ✅ 167 hat, 3.244 durak, 5 işletmeci, shapes | ❌ | ❌ | GTFS kaliteli |
| **İstanbul** | 🌍 İBB portalında "public-transport-gtfs-data" | 🌍 İETT SOAP servisleri | 🌍 İETT SOAP | En büyük pazar, sunucu TR'de olmalı |
| **Kocaeli** | 🌍 Güncel GTFS (`veri.kocaeli.bel.tr`, CC BY, 2025'ten beri aktif) · ✅ 2018 arşivi yedek olarak çalışıyor | ❌ resmi yok (KentKart özel) | ❌ | Uygulamada çalışıyor; VPS'ten güncel veriye geçecek |
| **Ankara** | ❌ Resmi GTFS yok | ❌ resmi açık API yok | ❌ | OSM'de ~2.700 durak; EGO verisi Ekim 2024'ten beri yayınlanmıyor |
| **Antalya, Sakarya, Kayseri vb. (KentKart şehirleri)** | ⚠️ Topluluk üretimi (kk-gtfs) | ⚠️ KentKart uç noktaları (resmi değil) | ⚠️ | Lisans/izin netleştirilmeli |
| **Bursa** | ❌ bulunamadı | ❌ | ❌ | Açık veri portalı var, ulaşım verisi doğrulanmadı |
| **TCDD (şehirlerarası, Marmaray, Başkentray, İZBAN)** | ⚠️ İZBAN Mobility DB'de (Ocak 2025'te bitiyor) · topluluk TCDD GTFS üreticisi | ❌ | ❌ | |
| **Flixbus Türkiye** | ✅ Mobility DB (Haz-Ara 2026) | ❌ | ❌ | Şehirlerarası, sonraki faz |

---

## 1.1 Sistematik tarama (26.11.2026)

Kaynak: MobilityData kataloğu (`share.mobilitydata.org/catalogs-csv`, 3.687 feed, **25 Türkiye kaydı**) + MobilityData'nın sakladığı aylık kopyalar. Her dosya Hibro'nun GTFS yükleyicisiyle açılıp ölçüldü (`go run ./server/cmd/gtfsinfo`).

| Şehir | Kaynak | Resmi mi? | Durak | Hat | Tarife | Hibro'da |
|---|---|---|---|---|---|---|
| İzmir (otobüs) | ESHOT API + ESHOT GTFS (MobilityData kopyası) | ✅ | 10.483 | 394 | GTFS 2024 (sadece durak sırası için) | ✅ canlı + durak sırası |
| İzmir (İZBAN) | izban.com.tr GTFS | ✅ | 41 | 8 | 2025'te bitmiş | ✅ "eski veri" uyarısıyla |
| İstanbul (İETT) | `data.ibb.gov.tr/dataset/iett-gtfs-verisi` (mdb-3128) | ✅ | ? | ? | ? | 🌍 VPS'ten denenecek |
| Kocaeli | Topluluk (KentKart) Kasım 2026 | ❌ | 8.672 | 415 | Kas-Ara 2026 | ✅ (belediye verisi VPS'ten inerse o kullanılır) |
| Antalya | Topluluk (KentKart) | ❌ | 4.617 | 167 (tramvay dahil) | Kas-Ara 2026 | ✅ `HIBRO_COMMUNITY=on` |
| Adana | Topluluk (KentKart) | ❌ | 3.337 | 132 | Kas-Ara 2026 | ✅ |
| Muğla | Topluluk (KentKart) | ❌ | 3.440 | 321 | Kas-Ara 2026 | ✅ |
| Düzce | Topluluk (KentKart) | ❌ | 1.237 | 50 | Kas-Ara 2026 | ✅ |
| Ordu | Topluluk (KentKart) | ❌ | 910 | 45 | Kas-Ara 2026 | ✅ |
| Mardin | Topluluk (KentKart) | ❌ | 765 | 87 | Kas-Ara 2026 | ✅ |
| Erzurum | Topluluk (KentKart) | ❌ | 747 | 58 | Kas-Ara 2026 | ✅ |
| Burdur | Topluluk (KentKart) | ❌ | 346 | 18 | Kas-Ara 2026 | ✅ |
| Çanakkale | Topluluk (KentKart) | ❌ | 324 | 11 | Kas-Ara 2026 | ✅ |
| Osmaniye | Topluluk (KentKart) | ❌ | 221 | 15 | Kas-Ara 2026 | ✅ |
| Safranbolu | Topluluk (KentKart) | ❌ | 167 | 11 | Kas-Ara 2026 | ✅ |
| Kırklareli | Topluluk (KentKart) | ❌ | 127 | 5 | Kas-Ara 2026 | ✅ |
| Kastamonu | Topluluk (KentKart) | ❌ | 91 | 23 | Kas-Ara 2026 | ✅ |
| Gaziantep | Belediye GTFS (resmi) + topluluk | ✅ | 3.230 | 167 | — | ✅ resmi olan |
| Konya | Belediye GTFS | ✅ | 4.256 | 169 | 2030'a kadar | ✅ |
| Isparta | Bink Teknoloji | ✅ | 61 | 1 | — | ❌ tek hat, yetersiz |
| Şehirlerarası | Flixbus Türkiye (331 hat), Türkiye demiryolları (Ray Atlası, 49 hat) | ✅ / ❌ | | | güncel | ⏳ rota planlayıcıya eklenecek |

**Topluluk verisi hakkında:** `kk-gtfs` projesi KentKart'ın herkese açık web uç noktalarından GTFS üretiyor, MobilityData her ay kopyalıyor. Resmi değildir. Uygulamada "topluluk verisi" olarak etiketlenir ve `HIBRO_COMMUNITY=on` ile açılır. **Herkese açık yayından önce** `kk-gtfs` veri lisansı ve KentKart/belediye izni kontrol edilmeli. En temiz çözüm: KentKart ile resmi veri anlaşması (27 bölge).

**Kocaeli 2018 → 2026 değişimi** (`gtfsdiff`): duraklar %83 aynı yerde, %13 yer değiştirmiş, 2.910 yeni durak; hat numaralarının %64'ü korunmuş, 122 hat kaldırılmış, 194 hat eklenmiş; korunan hatların yalnızca %30'unun güzergâhı aynı. Sonuç: 8 yıllık veri hat ve saat için kullanılmaz; topluluk verisi çok daha doğru.

## 2. Şehir detayları

### 2.1 İzmir (öncelik 1)

Portal: <https://acikveri.bizizmir.com> (CKAN, `api/3/action/...` çalışıyor)
Lisans: İzmir Büyükşehir Belediyesi Açık Veri Lisansı

**Statik GTFS** (`toplu-ulasim-gtfs-verileri`, portalda son güncelleme 2026-04-03):

| Feed | URL | Test sonucu |
|---|---|---|
| ESHOT otobüs | `https://www.eshot.gov.tr/gtfs/bus-eshot-gtfs.zip` | 🌍 Yurt dışından zaman aşımı. TR IP'den denenmeli. Mobility DB: 394 hat. |
| İzmir Metro | `https://www.izmirmetro.com.tr/gtfs/rail-metro-gtfs.zip` | ✅ 2 hat, 24 durak, 844 sefer. Geçerlilik 2026-07-09 → 2027-07-09. `shapes.txt` yok. |
| Tramvay | `https://www.tramizmir.com/gtfs/rail-tramizmir-gtfs.zip` | ✅ 5 hat, 53 durak, 2.021 sefer. Geçerlilik 2026-07-16 → 2027-07-16. `shapes.txt` yok. |
| İzdeniz vapur | `https://www.izdeniz.com.tr/gtfs/ship-izdeniz-gtfs.zip` | ⚠️ 912 byte döndü, geçerli ZIP değil. |

**Canlı / dinamik API'ler** (anahtar gerekmiyor):

| Amaç | Uç nokta | Test |
|---|---|---|
| Hattaki otobüslerin anlık konumu | `https://openapi.izmir.bel.tr/api/iztek/hatotobuskonumlari/{hatNo}` | ✅ Çalışıyor |
| Hattın durağa yaklaşan otobüsleri | `https://openapi.izmir.bel.tr/api/iztek/hattinyaklasanotobusleri/{hatNo}/{durakId}` | Dokümante |
| Durağa yaklaşan otobüsler | `https://acikveriwapi.izmir.bel.tr/AcikVeriEshot/Araclar/DuragaYaklasanOtobusler?durakKodu=` | ✅ Yanıt veriyor |
| Noktaya yakın duraklar | `https://openapi.izmir.bel.tr/api/ibb/cbs/noktayayakinduraklar?x=&y=&inCoordSys=&outCoordSys=` | Parametre formatı netleştirilmeli |
| Hat listesi / Baykuş (gece) hatları | `.../AcikVeriEshot/Hatlar/HatlariGetir`, `.../BaykusHatlariniGetir` | Dokümante |
| Duraklar | `.../AcikVeriEshot/Duraklar/DuraklariGetir` | Dokümante |
| Hareket saatleri | `.../AcikVeriEshot/Seferler/HareketSaatleriniGetir` | Dokümante |
| Hat güzergâhları (CSV) | `https://openfiles.izmir.bel.tr/211488/docs/eshot-otobus-hat-guzergahlari.csv` | Dokümante |
| Hat duyuruları (CSV) | `https://openfiles.izmir.bel.tr/211488/docs/eshot-otobus-hat-duyurulari.csv` | Günlük güncelleniyor |
| Metro/Tramvay durak arası mesafeler | portal: `izmir-metro-tramvay-durak-mesafeleri` | |
| Taksi durakları | `https://openapi.izmir.bel.tr/api/ibb/cbs/taksiduraklari` | |

> ✅ **Kod ile doğrulandı (26.11.2026):** Hat listesi (448 hat), durak listesi (12.012 durak), hareket saatleri (63.512 sefer), güzergâh CSV'si (572 bin nokta), duyurular ve canlı API'lerin hepsi Hibro sunucusunda çalışıyor. ESHOT için GTFS'e gerek kalmadan otobüs verisi tam. Eksik olan tek şey hat-durak sırası.
>
> ⚠️ **Hayalet araçlar:** `DuragaYaklasanOtobusler` yanıtında GPS zamanı saatler önce olan araçlar da var. `gpsTime` alanı 5 dakikadan eskiyse kayıt atılmalı.
>
> ⚠️ **Boş yanıt:** Canlı API ara sıra boş gövde döndürüyor. Yeniden deneme gerekli.

> ⚠️ **Dikkat:** `hatotobuskonumlari` yanıtında alan adları ters: `KoorX` = **enlem** (38.x), `KoorY` = **boylam** (27.x). Ondalık ayırıcı **virgül** (`"38,404575"`). Adaptörde normalize edilmeli.

Resmi doküman: <https://openfiles.izmir.bel.tr/471231/docs/iztek-WebServisKullanimDokumani_1.0.pdf>

### 2.2 Konya

Portal: <https://acikveri.konya.bel.tr> (CKAN) · Lisans: **CC BY 4.0**

- GTFS (`toplu-tasima-gtfs-verileri`, güncelleme 2026-11-18):
  - `gtfs_11_2026.zip`: <https://acikveri.konya.bel.tr/dataset/c2e034e6-e015-49c6-8eec-6e2f7de9c105/resource/c1d5a1a4-87c4-4a1d-9f4e-e1807a5b0d6e/download/gtfs_11_2026.zip>
  - ✅ Test: 179 hat, 4.494 durak, 13.165 sefer. `shapes.txt`, `fare_attributes.txt` ve `fare_rules.txt` var. Takvim 2021-10-01 → 2030-12-31.
  - ⚠️ URL her yıl değişiyor (`gtfs_11_YYYY.zip`). Adaptör `package_show` ile en yeni kaynağı bulmalı.
- Diğer: otobüs tarifeleri (XLSX, 2025-11), akıllı durak ekranları, saatlik ElKart biniş sayıları (kalabalık tahmini için kullanılabilir), durak varış süreleri (2022, eski).

### 2.3 Gaziantep

Portal: <https://acikveri.gaziantep.bel.tr> (CKAN) · Lisans: Gaziantep Açık Veri Lisansı

- GTFS: <https://acikveri.gaziantep.bel.tr/dataset/12d26c98-edc5-4be3-8988-c753764b84f4/resource/b3254aa0-359e-4b7c-87a3-d5d56ad27353/download/gtfs.zip>
  - ✅ Test: 167 hat, 3.244 durak, 33.105 sefer, 5 işletmeci (Gaziulaş, Nizip ÖHO, Sarı ÖHO...). `shapes.txt` ve `translations.txt` var.
  - ⚠️ Takvim 2017-05-10 → 2100-05-10 (sonsuz takvim). Saatlerin güncelliği sahada kontrol edilmeli.

### 2.4 İstanbul (öncelik 2: en büyük pazar)

Portal: <https://data.ibb.gov.tr> · Lisans: İBB Açık Veri Lisansı (CC BY 4.0 eşdeğeri)

- 🌍 Portal ve `api.ibb.gov.tr` **yurt dışı IP'lerden zaman aşımına uğradı.** Veri toplayan sunucu Türkiye'de barınmalı veya TR çıkışlı bir proxy kullanılmalı.
- **GTFS:** `public-transport-gtfs-data` veri seti var. İçeriği (İETT, Metro, Metrobüs, Şehir Hatları kapsamı) TR IP'den doğrulanmalı.
- **İETT SOAP servisleri** (doküman v1.4, veri seti `iett-ibb-web-servisi`):
  - `https://api.ibb.gov.tr/iett/UlasimAnaVeri/HatDurakGuzergah.asmx?wsdl`: hat, durak, güzergâh
  - `.../PlanlananSeferSaati.asmx`: planlı sefer saatleri
  - Duyuru ve filo durumu (anlık araç konumu) servisleri
  - Doküman: <https://data.ibb.gov.tr/dataset/367a12a3-958c-4ecf-b7e8-41b5bf3b5580/resource/5c3e950f-61e5-4713-bd6d-46d1494c745e/download/en_iett-web-service-usage-document-v.1.4.pdf>
- Metro İstanbul için resmi açık canlı API bulunamadı. Statik veri GTFS'ten gelir.
- Topluluk projeleri (referans):
  - `hakanatak/dataibbgovtr` (★41): İETT verisini GeoJSON API olarak sunuyor
  - `onur2677/iett-api`, `rootlus/iett-api`: SOAP'ı JSON'a çeviren Node.js katmanları
  - `pcislocked/iett-middle`: Birden fazla kaynağı cache'leyip birleştiren backend
  - `Rednexie/istnext` (★36): İstanbul ulaşım uygulaması
  - `luna-cant-code/istanbul-gtfs`: İstanbul GTFS derlemesi

### 2.5 Kocaeli (26.11.2026'da araştırıldı ve eklendi)

| Kaynak | Durum |
|---|---|
| **Güncel GTFS:** <https://veri.kocaeli.bel.tr/dataset/toplu-ulasim-verisi> (Mobility DB `mdb-2710`, CC BY, Temmuz 2025'ten beri "aktif") | 🌍 Yalnızca Türkiye IP'sinden erişilebiliyor. GTFS tek zip değil, ayrı `.txt` dosyaları olarak yayınlanıyor; sunucu bunları birleştiriyor. |
| **2018 arşivi:** Mobility DB `mdb-1122` (`files.mobilitydatabase.org/mdb-1122/...zip`) | ✅ İndirildi ve test edildi: 343 hat (5 tramvay, 4 vapur), 6.874 durak, 63.055 sefer, güzergâh çizgileri. Takvim 31.12.2018'de bitiyor. 2024 kopyası 2018 dosyasıyla **birebir aynı**. |
| **KentKart** (`service.kentkart.com`, bölge `004`) | ⚠️ Canlı araç dahil her şey var ama özel şirket API'si; istek "bilgilerinizi eksiksiz giriniz" ile reddediliyor. **İzin olmadan kullanılmayacak.** |
| **Moovit** | ❌ Kocaeli'yi gösteriyor ama verisini kopyalamak kullanım koşullarına ve telif hakkına aykırı. Moovit verisi büyük ihtimalle belediyenin aynı GTFS'inden geliyor. |

**Sunucu davranışı:** Önce güncel veri denenir. Olmazsa 2018 arşivi "eski veri" olarak yüklenir: tarih aralıkları yok sayılır, sadece gün tipine göre (hafta içi/cumartesi/pazar) sefer gösterilir ve uygulamada uyarı çıkar.

**Ne kadar değişmiş?** VPS'te güncel veri inince ölçülecek:

```bash
go run ./server/cmd/gtfsdiff kocaeli-2018.zip kocaeli-guncel.zip
```

Referans: Konya'da **1 yılda** (2025 → 2026) durakların %98'i aynı yerde kalmış; hatların %97'si korunmuş, korunan hatların %83'ünün güzergâhı aynı, %14'ü kısmen, %3'ü büyük ölçüde değişmiş; 16 yeni hat eklenmiş, 4 hat kaldırılmış. Kocaeli için 8 yıllık farkın bundan çok daha büyük olması beklenir: durakların çoğu yerinde olsa da sefer saatleri ve hat güzergâhlarının önemli kısmı değişmiş olabilir.

### 2.6 Ankara

- ❌ EGO'nun resmi GTFS'i yok. "Şeffaf Ankara" verisi günlük hat bazlı toplamlardan ibaret, konum bileşeni yok. Otobüs verisi 20 Ekim 2024'ten sonra yayınlanmamış (arXiv 2602.15470).
- OpenStreetMap'te ~2.700 Ankara durağı haritalanmış. Hat ilişkileri (route relation) eksik.
- **Strateji:** OSM + EGO'nun kamuya açık hat/sefer sayfalarından **kendi GTFS'imizi üretmek** (Faz 3). Ayrıca ABB ile resmi veri paylaşımı görüşmesi yapılmalı.

### 2.7 KentKart kullanan şehirler

KentKart'ın şehir listesinde **27 bölge** var (26.11.2026): Adana, Kocaeli, Sivas, Çanakkale, Muğla, Yozgat, Edirne, Burdur, Alanya, Ereğli, Niğde, Lüleburgaz, Kastamonu, Antalya, Kırklareli, Gaziantep, Alaplı, Altınova, Ordu, Mardin, Osmaniye, Düzce, Safranbolu, Erzurum, Tokat, Akçakoca, Çivril. Bu şehirlerin hat, durak, sefer saati ve **canlı araç** verisi KentKart'ta. KentKart ile resmi bir veri paylaşım anlaşması, Hibro'ya tek seferde 27 şehir kazandırabilir.

- `Egezenn/kk-gtfs`: KentKart kullanan belediyelerin hatlarını GTFS'e çeviren topluluk aracı ve görüntüleyici.
- ⚠️ KentKart uç noktaları **resmi açık veri değil.** Ticari üründe kullanmadan önce belediye veya KentKart'tan yazılı izin alınmalı. Aksi halde bu kaynak yalnızca prototip için kullanılır.
- `TonyFrank707/sakarya-transit-accessibility-pilot-test`: Sakarya için OSM'den GTFS üretme pilotu.

### 2.8 Çok şehirli / ulusal kaynaklar

| Kaynak | Ne işe yarar |
|---|---|
| [Mobility Database](https://mobilitydatabase.org): ESHOT `mdb-1823`, İzmir Metro/Tramvay `mdb-1824/1828/1829`, İZBAN, Flixbus `mdb-2922` | Feed keşfi, geçmiş sürümler |
| [Transitland](https://www.transit.land): `f-eshot~tr`, `f-izdeniz~ferry` | ESHOT'un 11 arşiv sürümü var. **ESHOT'a yurt dışından erişim için yedek kaynak.** |
| [Transitous](https://transitous.org) / MOTIS | Açık kaynak rota motoru. Türkiye feed'lerini oraya katkı olarak da gönderebiliriz. |
| `uzaylikafasi/turkiye-ulasim-api` | 28 ilin hat/durak verisini tek biçimde toplama girişimi (topluluk derlemesi, resmi değil) |
| `yasinkaraaslan/tcdd-gtfs` | TCDD GTFS üreticisi |
| `BasicOS2025/transit-tr` | Hafif, düşük kaynaklı Türkçe toplu taşıma uygulaması (rakip/referans) |
| `3rt4nm4n/turkish-apis` | Türk API'leri listesi (Bursa, Kayseri açık veri portalları dahil) |
| ULAŞAV (Ulaştırma ve Altyapı Bakanlığı) | Ulusal ulaşım veri platformu, 165 kurum (2024). Erişim koşulları araştırılmalı. |
| OpenStreetMap (Geofabrik `turkey-latest.osm.pbf`) | Yürüme ağı, durak konumları, harita karoları |

---

## 3. Bilinen sorunlar ve çözümler

| Sorun | Çözüm |
|---|---|
| İBB ve ESHOT yurt dışı IP'leri engelliyor | Veri toplayıcı (ingest) Türkiye'deki bir VPS'te çalışır. Transitland arşivi yedek olur. |
| Feed URL'leri yıllık değişiyor (Konya) | CKAN `package_show` ile dinamik keşif |
| Sonsuz/yanlış takvim (Gaziantep, Konya) | Doğrulayıcı uyarı verir. Kullanıcıya "planlı saat, canlı değil" etiketi gösterilir. |
| `shapes.txt` eksik (İzmir raylı) | OSM ray geometrisinden üretilir (pfaedle aracı) |
| Koordinat alanları ters, ondalık virgül (İzmir API) | Şehir adaptöründe normalize edilir, birim testle sabitlenir |
| Canlı veri GTFS-RT formatında değil | Kendi "RT köprümüz" belediye JSON/SOAP verisini GTFS-Realtime'a çevirir |
| Ankara'da veri yok | OSM + kamuya açık sayfalardan GTFS üretimi, belediyeyle resmi görüşme |

## 4. Lisans uyumu kontrol listesi

- [ ] Her feed için lisans metni `data/feeds.yaml` içinde
- [ ] Uygulamada "Veri kaynakları" ekranı (atıf zorunluluğu, CC BY 4.0)
- [ ] KentKart ve resmi olmayan kaynaklar için yazılı izin, ya da üretimde kapalı tutulması
- [ ] İBB, İzmir ve Gaziantep lisanslarının ticari kullanım (destekçi üyeliği) maddesi hukukçuya kontrol ettirilmeli
