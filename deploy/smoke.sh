#!/usr/bin/env bash
# Quick check of a running Hibro server: ./deploy/smoke.sh https://api.ornek.com
set -uo pipefail
B="${1:?kullanım: $0 https://api.alanadin.com}"
pass=0; fail=0
check() { # name url expected-substring
  local out; out="$(curl -s -m 30 "$2")"
  if printf '%s' "$out" | grep -q "$3"; then echo "✔ $1"; pass=$((pass+1)); else echo "✘ $1  ($2)"; fail=$((fail+1)); fi
}
check "sağlık"                "$B/healthz"                                   '"ok":true'
check "şehir listesi"         "$B/v1/cities"                                 '"izmir"'
check "İzmir yakın duraklar"  "$B/v1/cities/izmir/stops/nearby?lat=38.4189&lon=27.1287" '"id"'
check "İzmir canlı/tahmini"   "$B/v1/cities/izmir/stops/b:10102/arrivals"    '"live":true'
check "İzmir rota"            "$B/v1/cities/izmir/plan?from=38.4600,27.2110&to=38.3955,27.0700" '"itineraries":\['
check "Konya hat"             "$B/v1/cities/konya/lines"                     '"short"'
check "Kocaeli"               "$B/v1/cities/kocaeli"                         '"stopCount"'
check "İstanbul (TR IP ile)"  "$B/v1/cities/istanbul"                        '"stopCount"'
check "gizlilik sayfası"      "$B/gizlilik"                                  'Gizlilik'
echo; echo "$pass başarılı, $fail başarısız"
curl -s -m 30 "$B/v1/cities" | python3 -c 'import sys,json
for c in json.load(sys.stdin): print(f"  {c[\"name\"]:<12} {c[\"stopCount\"]:>6} durak {c[\"lineCount\"]:>4} hat  canlı={c[\"features\"][\"liveArrivals\"]} rota={c[\"features\"][\"planner\"]}")' 2>/dev/null
[ "$fail" -eq 0 ]
