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

/// Thin client for hibro-server. No API keys, no tracking headers.
class HibroApi {
  HibroApi(this.baseUrl, {http.Client? client}) : _http = client ?? http.Client();

  String baseUrl;
  final http.Client _http;

  Uri _u(String path, [Map<String, String>? q]) {
    final base = baseUrl.endsWith('/') ? baseUrl.substring(0, baseUrl.length - 1) : baseUrl;
    return Uri.parse('$base$path').replace(queryParameters: q);
  }

  static String seg(String s) => Uri.encodeComponent(s);

  Future<dynamic> _get(String path, {Map<String, String>? q, Duration timeout = const Duration(seconds: 15)}) async {
    final http.Response r;
    try {
      r = await _http.get(_u(path, q)).timeout(timeout);
    } catch (_) {
      throw ApiException('Sunucuya ulaşılamadı. İnternet bağlantını kontrol et.');
    }
    final body = utf8.decode(r.bodyBytes);
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

  Future<List<Itinerary>> plan(String city, double fLat, double fLon, double tLat, double tLon) async {
    final j = await _get('/v1/cities/${seg(city)}/plan', q: {
      'from': '$fLat,$fLon',
      'to': '$tLat,$tLon',
    }, timeout: const Duration(seconds: 30)) as Map<String, dynamic>;
    return ((j['itineraries'] as List?) ?? const []).map((e) => Itinerary.fromJson(e as Map<String, dynamic>)).toList();
  }
}
