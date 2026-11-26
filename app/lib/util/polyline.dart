/// Decodes a Google encoded polyline into [lat, lon] pairs.
List<List<double>> decodePolyline(String s, {int precision = 5}) {
  final out = <List<double>>[];
  var index = 0, lat = 0, lon = 0;
  final factor = precision == 6 ? 1e6 : (precision == 7 ? 1e7 : 1e5);
  int next() {
    var shift = 0, result = 0, b = 0;
    do {
      b = s.codeUnitAt(index++) - 63;
      result |= (b & 0x1f) << shift;
      shift += 5;
    } while (b >= 0x20 && index < s.length);
    // Not `~(result >> 1)`: on the web (dart2js) bitwise NOT yields an
    // unsigned 32-bit number, which would turn negative deltas huge.
    return (result & 1) != 0 ? -(result >> 1) - 1 : result >> 1;
  }

  while (index < s.length) {
    lat += next();
    if (index >= s.length) break;
    lon += next();
    out.add([lat / factor, lon / factor]);
  }
  return out;
}
