#!/usr/bin/env bash
# Pull the latest code and restart Geliyo with zero manual steps.
set -euo pipefail
cd "$(dirname "$0")/.."
git pull --ff-only
cd deploy
docker compose up -d --build
docker image prune -f >/dev/null
echo "Güncellendi. Loglar: docker compose logs -f hibro"
