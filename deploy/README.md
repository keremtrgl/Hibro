# VPS Kurulumu (adım adım)

Hedef: Türkiye'deki bir VPS'te Hibro sunucusunu HTTPS ile çalıştırmak. Toplam süre ~20 dakika.

## 0. Gerekenler

| | Öneri |
|---|---|
| VPS | Türkiye lokasyonu, **2 vCPU / 4 GB RAM / 40 GB disk**, Ubuntu 24.04 |
| Alan adı | Örn. `api.hibro.app`. DNS'te **A kaydı → VPS IP adresi** |

Neden Türkiye? İBB ve ESHOT'un bazı servisleri yurt dışı IP'lere yanıt vermiyor.

Sunucu gerçek verilerle test edildi: İzmir + Konya + Gaziantep birlikte **~250 MB RAM** kullanıyor.

## 1. Sunucuya bağlan ve güncelle

```bash
ssh root@VPS_IP
apt update && apt upgrade -y
```

## 2. Güvenlik duvarı

```bash
ufw allow OpenSSH
ufw allow 80,443/tcp
ufw allow 443/udp
ufw enable
```

## 3. Docker kur

```bash
curl -fsSL https://get.docker.com | sh
```

## 4. Kodu indir

Repo gizli olduğu için GitHub'da bir **Deploy key** (salt okunur) ekle:

```bash
ssh-keygen -t ed25519 -f ~/.ssh/hibro -N ""
cat ~/.ssh/hibro.pub   # bunu GitHub → repo → Settings → Deploy keys → Add'e yapıştır
GIT_SSH_COMMAND="ssh -i ~/.ssh/hibro" git clone git@github.com:keremtrgl/Hibro.git /opt/hibro
```

## 5. Ayarla ve başlat

```bash
cd /opt/hibro/deploy
cp .env.example .env
nano .env          # HIBRO_DOMAIN=api.senin-alanadin.com
docker compose up -d --build
```

İlk açılışta şehir verileri indirilir (İzmir ~1 dk). Takip için:

```bash
docker compose logs -f hibro
# "city loaded city=izmir stops=12089 ..." satırlarını görmelisin
```

## 6. Test et

```bash
curl https://api.senin-alanadin.com/healthz
curl https://api.senin-alanadin.com/v1/cities
```

## 7. Uygulamayı sunucuna bağla

- **Derlerken:** `flutter build apk --dart-define=HIBRO_API=https://api.senin-alanadin.com`
- **Ya da** uygulamada: Ayarlar → Hibro sunucu adresi.

## Güncelleme

```bash
cd /opt/hibro && GIT_SSH_COMMAND="ssh -i ~/.ssh/hibro" git pull
cd deploy && docker compose up -d --build
```

## Rota planlayıcı (opsiyonel, "A'dan B'ye nasıl giderim")

MOTIS açık kaynak rota motoru. Ön hazırlık RAM ister; küçük VPS'te bunu kendi bilgisayarında yapıp sonucu kopyalaman önerilir.

```bash
apt install -y osmium-tool
./motis/prepare.sh                     # OSM + GTFS indirir, MOTIS verisini hazırlar
echo "HIBRO_MOTIS_URL=http://motis:8080" >> .env
docker compose --profile planner up -d
```

> ⚠️ MOTIS kurulumu bu ortamda test edilemedi (Docker ve MOTIS indirmesi erişilebilir değildi). İlk kurulumda `docker compose logs motis` çıktısına bakarak doğrula. MOTIS sürümünü sabitlemek için `.env` içine `MOTIS_VERSION=vX.Y.Z` ekle.

## Docker'sız kurulum

`go build -o hibro-server ./server/cmd/hibro-server` ile tek dosya çıkar. `hibro.service` systemd dosyasını kullan; önüne Caddy veya Nginx koy.

## Gizlilik

- Caddy erişim logları kapalı. IP ve konum saklanmaz.
- Sunucu logları yalnızca hata ve şehir yükleme bilgisi içerir.
- Kullanıcı hesabı, çerez veya analitik yok.
