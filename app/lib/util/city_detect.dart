import '../api/models.dart';
import 'geo.dart';

/// Picks the supported city that contains (lat, lon).
///
/// A city matches when the point is inside its stop bounds (plus ~5 km)
/// or, for cities without bounds, within 40 km of its centre. When several
/// match, the closest centre wins. Returns null outside every supported
/// city, e.g. in İzmit today: then nothing is downloaded.
CityInfo? detectCity(List<CityInfo> cities, double lat, double lon) {
  const margin = 0.05; // degrees, about 5 km
  CityInfo? best;
  var bestD = double.infinity;
  for (final c in cities) {
    final b = c.bounds;
    final inside = b != null
        ? lat >= b[0] - margin && lat <= b[2] + margin && lon >= b[1] - margin && lon <= b[3] + margin
        : distanceM(lat, lon, c.lat, c.lon) <= 40000;
    if (!inside) continue;
    final d = distanceM(lat, lon, c.lat, c.lon);
    if (d < bestD) {
      bestD = d;
      best = c;
    }
  }
  return best;
}
