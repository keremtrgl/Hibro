import 'dart:convert';

import 'package:http/http.dart' as http;

import 'models.dart';

class ApiException implements Exception {
  final String message;
  final int? status;
  ApiException(this.message, [this.status]);
  @override
  String toString() => message;
}

/// Thin client for the Geliyo server (hibro-server). No API keys, no tracking headers.
class GeliyoApi {
  GeliyoApi(this.baseUrl, {http.Client? client}) : _http = client ?? http.Client();

  String baseUrl;
  final http.Client _http;

  /// ETag cache: url -> (etag, body). Re-opening a line or the alerts
  /// list sends If-None-Match and gets an empty 304 when nothing changed.
  final Map<String, (String, String)> _etags = {};
  static const _maxEtags = 300;

  /// Bytes received from the API this session (shown in Ayarlar).
  int bytesReceived = 0;

  Uri _u(String path, [Map<String, String>? q]) {
    final base = baseUrl.endsWith('/') ? baseUrl.substring(0, baseUrl.length - 1) : baseUrl;
    return Uri.parse('$base$path').replace(queryParameters: q);
  }

  static String seg(String s) => Uri.encodeComponent(s);

  Future<dynamic> _get(String path, {Map<String, String>? q, Duration timeout = const Duration(seconds: 15)}) async {
    final uri = _u(path, q);
    final key = uri.toString();
    final cached = _etags[key];
    final http.Response r;
    try {
      r = await _http.get(uri, headers: {'If-None-Match': ?cached?.$1}).timeout(timeout);
    } catch (_) {
      throw ApiException('Sunucuya ulaşılamadı. İnternet bağlantını kontrol et.');
    }
    bytesReceived += r.bodyBytes.length + 200; // ~200 B of headers
    if (r.statusCode == 304 && cached != null) {
      return jsonDecode(cached.$2);
    }
    final body = utf8.decode(r.bodyBytes);
    final etag = r.headers['etag'];
    if (r.statusCode == 200 && etag != null) {
      if (_etags.length >= _maxEtags) _etags.remove(_etags.keys.first);
      _etags[key] = (etag, body);
    }
    if (r.statusCode != 200) {
      String msg = 'Sunucu hatası (${r.statusCode})';
      try {
        msg = (jsonDecode(body) as Map<String, dynamic>)['error'] as String? ?? msg;
      } catch (_) {}
      throw ApiException(msg, r.statusCode);
    }
    return jsonDecode(body);
  }

  Future<List<CityInfo>> cities() async {
    final j = await _get('/v1/cities') as List;
    return j.map((e) => CityInfo.fromJson(e as Map<String, dynamic>)).toList();
  }

  /// Downloads the offline pack. Returns null when [etag] is still valid.
  Future<({String body, String? etag})?> pack(String city, {String? etag}) async {
    final http.Response r;
    try {
      r = await _http.get(_u('/v1/cities/${seg(city)}/pack'), headers: {'If-None-Match': ?etag}).timeout(const Duration(seconds: 60));
    } catch (_) {
      throw ApiException('Şehir verisi indirilemedi.');
    }
    bytesReceived += r.bodyBytes.length + 200;
    if (r.statusCode == 304) return null;
    if (r.statusCode != 200) throw ApiException('Şehir verisi indirilemedi (${r.statusCode})', r.statusCode);
    return (body: utf8.decode(r.bodyBytes), etag: r.headers['etag']);
  }

  Future<ArrivalsResult> arrivals(String city, String stopId) async {
    final j = await _get('/v1/cities/${seg(city)}/stops/${seg(stopId)}/arrivals');
    return ArrivalsResult.fromJson(j as Map<String, dynamic>);
  }

  Future<LineDetail> line(String city, String lineId) async {
    final j = await _get('/v1/cities/${seg(city)}/lines/${seg(lineId)}');
    return LineDetail.fromJson(j as Map<String, dynamic>);
  }

  /// Live vehicles of a line. [degraded] is true when the municipality's
  /// live system did not answer (not the same as "no buses running").
  Future<({List<Vehicle> vehicles, bool degraded})> vehicles(String city, String lineId) async {
    final j = await _get('/v1/cities/${seg(city)}/lines/${seg(lineId)}/vehicles') as Map<String, dynamic>;
    return (
      vehicles: ((j['vehicles'] as List?) ?? const []).map((e) => Vehicle.fromJson(e as Map<String, dynamic>)).toList(),
      degraded: j['degraded'] == true,
    );
  }

  Future<List<Alert>> alerts(String city) async {
    final j = await _get('/v1/cities/${seg(city)}/alerts') as List;
    return j.map((e) => Alert.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<List<NearbyVehicle>> nearbyVehicles(String city, double lat, double lon) async {
    final j = await _get('/v1/cities/${seg(city)}/vehicles/nearby', q: {'lat': '$lat', 'lon': '$lon'}) as Map<String, dynamic>;
    return ((j['vehicles'] as List?) ?? const []).map((e) => NearbyVehicle.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<void> _post(String path, Map<String, dynamic> body, {Duration timeout = const Duration(seconds: 20)}) async {
    final http.Response r;
    try {
      r = await _http
          .post(_u(path), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body))
          .timeout(timeout);
    } catch (_) {
      throw ApiException('Gönderilemedi. İnternet bağlantını kontrol et.');
    }
    if (r.statusCode >= 300) {
      String msg = 'Gönderilemedi (${r.statusCode})';
      try {
        msg = (jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>)['error'] as String? ?? msg;
      } catch (_) {}
      throw ApiException(msg, r.statusCode);
    }
  }

  Future<void> report(String city, {required String kind, String? stopId, String? lineId, String? plate, String? note, String? app}) =>
      _post('/v1/cities/${seg(city)}/reports', {
        'kind': kind,
        'stopId': ?stopId,
        'lineId': ?lineId,
        if (plate != null && plate.isNotEmpty) 'plate': plate,
        if (note != null && note.trim().isNotEmpty) 'note': note.trim(),
        'app': ?app,
      });

  Future<void> telemetry(String city, Map<String, int> events) =>
      _post('/v1/telemetry', {'city': city, 'events': events}, timeout: const Duration(seconds: 10));

  /// Place / address search (universities, neighbourhoods, landmarks),
  /// limited to the city. Goes through our server, which asks OSM.
  Future<List<Place>> places(String city, String q) async {
    final j = await _get('/v1/cities/${seg(city)}/places', q: {'q': q}, timeout: const Duration(seconds: 15)) as Map<String, dynamic>;
    return [for (final p in (j['places'] as List? ?? const [])) Place.fromJson(p as Map<String, dynamic>)];
  }

  Future<List<Itinerary>> plan(String city, double fLat, double fLon, double tLat, double tLon) async {
    final j = await _get('/v1/cities/${seg(city)}/plan', q: {
      'from': '$fLat,$fLon',
      'to': '$tLat,$tLon',
    }, timeout: const Duration(seconds: 30)) as Map<String, dynamic>;
    return ((j['itineraries'] as List?) ?? const []).map((e) => Itinerary.fromJson(e as Map<String, dynamic>)).toList();
  }
}
