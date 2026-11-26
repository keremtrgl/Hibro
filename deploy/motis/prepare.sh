#!/usr/bin/env bash
# Prepares MOTIS data (OSM street network + GTFS feeds) for the planner.
# Heavy step: run it on your own computer if the VPS is small, then copy
# deploy/data/motis to the VPS. Needs: curl, osmium-tool, docker.
set -euo pipefail
cd "$(dirname "$0")/.."
OUT=data/motis
mkdir -p "$OUT" && cd "$OUT"

echo "1/4 OSM Türkiye indiriliyor (~600 MB)"
curl -fL -o turkey.osm.pbf https://download.geofabrik.de/europe/turkey-latest.osm.pbf

echo "2/4 Sadece desteklenen şehirler kesiliyor (RAM ihtiyacını düşürür)"
osmium extract -b 26.6,38.0,27.6,38.9 turkey.osm.pbf -o izmir.osm.pbf --overwrite
osmium extract -b 32.2,37.7,32.8,38.1 turkey.osm.pbf -o konya.osm.pbf --overwrite
osmium extract -b 37.1,36.9,37.9,37.2 turkey.osm.pbf -o gaziantep.osm.pbf --overwrite
osmium merge izmir.osm.pbf konya.osm.pbf gaziantep.osm.pbf -o cities.osm.pbf --overwrite

echo "3/4 GTFS dosyaları indiriliyor"
curl -fL -o izmir-metro.zip https://www.izmirmetro.com.tr/gtfs/rail-metro-gtfs.zip
curl -fL -o izmir-tram.zip https://www.tramizmir.com/gtfs/rail-tramizmir-gtfs.zip
# ESHOT yalnızca Türkiye IP'sinden iniyor:
curl -fL -o izmir-eshot.zip https://www.eshot.gov.tr/gtfs/bus-eshot-gtfs.zip || echo "UYARI: ESHOT GTFS inmedi (TR IP gerekli)"
curl -fL -o konya.zip "https://acikveri.konya.bel.tr/dataset/c2e034e6-e015-49c6-8eec-6e2f7de9c105/resource/c1d5a1a4-87c4-4a1d-9f4e-e1807a5b0d6e/download/gtfs_11_2026.zip"
curl -fL -o gaziantep.zip https://acikveri.gaziantep.bel.tr/dataset/12d26c98-edc5-4be3-8988-c753764b84f4/resource/b3254aa0-359e-4b7c-87a3-d5d56ad27353/download/gtfs.zip

echo "4/4 MOTIS yapılandırma ve içe aktarma"
FEEDS=$(ls *.zip)
docker run --rm -v "$PWD":/data -w /data ghcr.io/motis-project/motis:${MOTIS_VERSION:-latest} motis config cities.osm.pbf $FEEDS
docker run --rm -v "$PWD":/data -w /data ghcr.io/motis-project/motis:${MOTIS_VERSION:-latest} motis import
echo "Tamam. Şimdi: docker compose --profile planner up -d"
