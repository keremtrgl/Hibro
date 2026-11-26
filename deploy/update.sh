#!/usr/bin/env bash
# Pull the latest code and restart Geliyo with zero manual steps.
set -euo pipefail
cd "$(dirname "$0")/.."
git pull --ff-only
cd deploy
# Data folders must be writable by the container user (uid 10001).
mkdir -p data/hibro data/collect
chown -R 10001:10001 data/hibro data/collect 2>/dev/null || sudo chown -R 10001:10001 data/hibro data/collect
docker compose up -d --build
docker image prune -f >/dev/null
echo "Güncellendi. Loglar: docker compose logs -f hibro"
