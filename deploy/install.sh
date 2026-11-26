#!/usr/bin/env bash
# Geliyo: one-command VPS setup (Ubuntu 22.04/24.04, run as root).
#
#   git clone <repo> /opt/hibro && cd /opt/hibro
#   sudo ./deploy/install.sh api.geliyo.site
#
# Safe to run again: existing settings are kept.
set -euo pipefail

DOMAIN="${1:-}"
HERE="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="$HERE/.env"

say()  { printf '\n\033[1;32m==> %s\033[0m\n' "$*"; }
warn() { printf '\033[1;33m!!  %s\033[0m\n' "$*"; }
die()  { printf '\033[1;31mXX  %s\033[0m\n' "$*"; exit 1; }

[ "$(id -u)" -eq 0 ] || die "root olarak çalıştır: sudo $0 $*"
command -v apt-get >/dev/null || die "Bu betik Ubuntu/Debian içindir."

if [ -z "$DOMAIN" ] && [ -f "$ENV_FILE" ]; then
  DOMAIN="$(grep -E '^HIBRO_DOMAIN=' "$ENV_FILE" | cut -d= -f2- || true)"
fi
[ -n "$DOMAIN" ] && [ "$DOMAIN" != "api.ornek.com" ] || die "Alan adı gerekli: sudo $0 api.senin-alanadin.com"

say "Paketler"
apt-get update -qq
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq curl git ufw openssl ca-certificates >/dev/null

if ! command -v docker >/dev/null; then
  say "Docker kuruluyor"
  curl -fsSL https://get.docker.com | sh >/dev/null
fi
docker compose version >/dev/null 2>&1 || die "docker compose eklentisi bulunamadı"

say "Güvenlik duvarı (SSH, 80, 443)"
ufw allow OpenSSH >/dev/null
ufw allow 80/tcp >/dev/null
ufw allow 443/tcp >/dev/null
ufw allow 443/udp >/dev/null
ufw --force enable >/dev/null

# Small VPS: add swap so the first data load cannot run out of memory.
MEM_MB=$(awk '/MemTotal/ {print int($2/1024)}' /proc/meminfo)
if [ "$MEM_MB" -lt 3500 ] && ! swapon --show | grep -q .; then
  say "RAM ${MEM_MB} MB: 2 GB swap ekleniyor"
  fallocate -l 2G /swapfile && chmod 600 /swapfile && mkswap /swapfile >/dev/null && swapon /swapfile
  grep -q '^/swapfile' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi

say "Ayarlar ($ENV_FILE)"
if [ ! -f "$ENV_FILE" ]; then
  cp "$HERE/.env.example" "$ENV_FILE"
  chmod 600 "$ENV_FILE"
fi
set_env() { # key value
  if grep -qE "^$1=" "$ENV_FILE"; then
    sed -i "s|^$1=.*|$1=$2|" "$ENV_FILE"
  else
    echo "$1=$2" >> "$ENV_FILE"
  fi
}
set_env HIBRO_DOMAIN "$DOMAIN"
if ! grep -qE '^HIBRO_ADMIN_TOKEN=.+' "$ENV_FILE"; then
  set_env HIBRO_ADMIN_TOKEN "$(openssl rand -hex 32)"
fi
grep -qE '^HIBRO_CITIES=' "$ENV_FILE" || set_env HIBRO_CITIES all

# DNS sanity check (warn only: the certificate needs it).
DNS_IP="$(getent ahostsv4 "$DOMAIN" | awk 'NR==1{print $1}' || true)"
if [ -z "$DNS_IP" ]; then
  warn "$DOMAIN için DNS kaydı bulunamadı. Alan adı panelinde A kaydını bu sunucunun IP'sine yönlendir."
elif ! hostname -I | tr ' ' '\n' | grep -qx "$DNS_IP"; then
  warn "$DOMAIN -> $DNS_IP, ama bu sunucunun IP'leri: $(hostname -I). (NAT'lı VPS'lerde normal olabilir.)"
fi

# The server runs as uid 10001 inside the container; the data folders are
# bind mounts, so they must be writable by that user (cache + reports).
mkdir -p "$HERE/data/hibro" "$HERE/data/collect"
chown -R 10001:10001 "$HERE/data/hibro" "$HERE/data/collect"

say "Geliyo başlatılıyor (ilk derleme birkaç dakika sürebilir)"
cd "$HERE"
docker compose up -d --build

say "Şehir verileri yükleniyor"
ok=""
for i in $(seq 1 60); do
  if docker compose exec -T hibro wget -qO- http://127.0.0.1:8080/healthz 2>/dev/null | grep -q '"ok":true'; then
    ok=1; break
  fi
  sleep 5
done
[ -n "$ok" ] || { docker compose logs --tail 40 hibro; die "Sunucu 5 dakikada hazır olmadı (loglar yukarıda)."; }
docker compose exec -T hibro wget -qO- http://127.0.0.1:8080/healthz; echo

TOKEN="$(grep -E '^HIBRO_ADMIN_TOKEN=' "$ENV_FILE" | cut -d= -f2-)"
say "Hazır!"
cat <<MSG

  API adresi      : https://$DOMAIN
  Sağlık kontrolü : curl https://$DOMAIN/healthz
  Şehirler        : curl https://$DOMAIN/v1/cities
  Toplanan veri   : curl -H "Authorization: Bearer $TOKEN" https://$DOMAIN/admin/stats
  Loglar          : cd $HERE && docker compose logs -f hibro

  Uygulamada: Ayarlar -> Geliyo sunucu adresi -> https://$DOMAIN
  (HTTPS sertifikası ilk istekte otomatik alınır; DNS doğru değilse alınamaz.)

  Yönetici token'ı $ENV_FILE içinde saklı; kimseyle paylaşma.
MSG
