import 'dart:async';
import 'dart:convert';
import 'dart:io' show File;

import 'package:flutter/foundation.dart';
import 'package:geolocator/geolocator.dart';
import 'package:path_provider/path_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../api/client.dart';
import '../api/models.dart';
import '../util/geo.dart';
import '../util/text.dart';

const defaultApiUrl = String.fromEnvironment('HIBRO_API', defaultValue: 'http://10.0.2.2:8080');

/// Cities known at build time, so the map can open instantly on first
/// launch before anything is downloaded.
final builtInCities = <CityInfo>[
  CityInfo(id: 'izmir', name: 'İzmir', lat: 38.4237, lon: 27.1428, zoom: 12.5),
  CityInfo(id: 'konya', name: 'Konya', lat: 37.8746, lon: 32.4932, zoom: 12.5),
  CityInfo(id: 'gaziantep', name: 'Gaziantep', lat: 37.0662, lon: 37.3833, zoom: 12.5),
];

class Favorite {
  final String kind; // 'stop' | 'line'
  final String city, id, title, subtitle;
  Favorite(this.kind, this.city, this.id, this.title, this.subtitle);
  Map<String, dynamic> toJson() => {'k': kind, 'c': city, 'i': id, 't': title, 's': subtitle};
  factory Favorite.fromJson(Map<String, dynamic> j) =>
      Favorite(j['k'] as String, j['c'] as String, j['i'] as String, j['t'] as String, j['s'] as String? ?? '');
}

class _Pack {
  final CityInfo city;
  final List<Stop> stops;
  final List<Line> lines;
  _Pack(this.city, this.stops, this.lines);
}

_Pack _parsePack(String body) {
  final j = jsonDecode(body) as Map<String, dynamic>;
  return _Pack(
    CityInfo.fromJson(j['city'] as Map<String, dynamic>),
    (j['stops'] as List).map((e) => Stop.fromJson(e as Map<String, dynamic>, fold: fold)).toList(),
    (j['lines'] as List).map((e) => Line.fromJson(e as Map<String, dynamic>)).toList(),
  );
}

class NearbyStop {
  final Stop stop;
  final int distanceM;
  NearbyStop(this.stop, this.distanceM);
}

class AppState extends ChangeNotifier {
  AppState(this.prefs) : api = HibroApi(prefs.getString('api') ?? defaultApiUrl) {
    cityId = prefs.getString('city') ?? 'izmir';
    themeMode = ThemeModeX.parse(prefs.getString('theme'));
    favorites = (jsonDecode(prefs.getString('favs') ?? '[]') as List)
        .map((e) => Favorite.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  final SharedPreferences prefs;
  final HibroApi api;

  late String cityId;
  late AppThemeMode themeMode;
  late List<Favorite> favorites;

  List<CityInfo> cities = List.of(builtInCities);
  CityInfo? cityInfo;
  List<Stop> stops = const [];
  List<Line> lines = const [];
  Map<String, Stop> stopById = const {};
  Map<String, Line> lineById = const {};
  bool loading = false;
  String? error;
  DateTime? packUpdated;

  double? myLat, myLon;
  StreamSubscription<Position>? _posSub;
  bool locationDenied = false;

  CityInfo get city => cityInfo ?? cities.firstWhere((c) => c.id == cityId, orElse: () => builtInCities.first);

  Future<void> init() async {
    await _loadCachedPack();
    unawaited(refresh());
    unawaited(_refreshCities());
  }

  Future<void> _refreshCities() async {
    try {
      final list = await api.cities();
      if (list.isNotEmpty) {
        cities = list;
        notifyListeners();
      }
    } catch (_) {}
  }

  // ---- offline pack ----

  Future<File?> _packFile(String city) async {
    if (kIsWeb) return null;
    final dir = await getApplicationSupportDirectory();
    return File('${dir.path}/pack_$city.json');
  }

  Future<void> _loadCachedPack() async {
    try {
      final f = await _packFile(cityId);
      if (f == null || !await f.exists()) return;
      _apply(await compute(_parsePack, await f.readAsString()));
      packUpdated = DateTime.tryParse(prefs.getString('packAt_$cityId') ?? '');
    } catch (_) {
      // A corrupt cache is not fatal: the network refresh replaces it.
    }
  }

  void _apply(_Pack p) {
    cityInfo = p.city;
    stops = p.stops;
    lines = p.lines;
    stopById = {for (final s in p.stops) s.id: s};
    lineById = {for (final l in p.lines) l.id: l};
    notifyListeners();
  }

  /// Fetches the city pack; only downloads when the server's copy changed.
  Future<void> refresh() async {
    loading = true;
    error = null;
    notifyListeners();
    final city = cityId;
    try {
      final etag = stops.isEmpty ? null : prefs.getString('etag_$city');
      final res = await api.pack(city, etag: etag);
      if (city != cityId) return;
      if (res != null) {
        _apply(await compute(_parsePack, res.body));
        final f = await _packFile(city);
        await f?.writeAsString(res.body);
        if (res.etag != null) await prefs.setString('etag_$city', res.etag!);
      }
      packUpdated = DateTime.now();
      await prefs.setString('packAt_$city', packUpdated!.toIso8601String());
    } catch (e) {
      if (city == cityId) error = e.toString();
    } finally {
      if (city == cityId) {
        loading = false;
        notifyListeners();
      }
    }
  }

  Future<void> selectCity(String id) async {
    if (id == cityId) return;
    cityId = id;
    await prefs.setString('city', id);
    cityInfo = null;
    stops = const [];
    lines = const [];
    stopById = const {};
    lineById = const {};
    notifyListeners();
    await _loadCachedPack();
    await refresh();
  }

  Future<void> setApiUrl(String url) async {
    api.baseUrl = url.trim();
    await prefs.setString('api', api.baseUrl);
    await prefs.remove('etag_$cityId');
    await refresh();
    await _refreshCities();
  }

  Future<void> setTheme(AppThemeMode m) async {
    themeMode = m;
    await prefs.setString('theme', m.name);
    notifyListeners();
  }

  // ---- favourites ----

  bool isFavorite(String kind, String id) => favorites.any((f) => f.kind == kind && f.id == id && f.city == cityId);

  Future<void> toggleFavorite(String kind, String id, String title, String subtitle) async {
    if (isFavorite(kind, id)) {
      favorites.removeWhere((f) => f.kind == kind && f.id == id && f.city == cityId);
    } else {
      favorites.insert(0, Favorite(kind, cityId, id, title, subtitle));
    }
    await prefs.setString('favs', jsonEncode(favorites.map((f) => f.toJson()).toList()));
    notifyListeners();
  }

  List<Favorite> get cityFavorites => favorites.where((f) => f.city == cityId).toList();

  // ---- location ----

  Future<bool> startLocation() async {
    try {
      if (!await Geolocator.isLocationServiceEnabled()) {
        locationDenied = true;
        notifyListeners();
        return false;
      }
      var perm = await Geolocator.checkPermission();
      if (perm == LocationPermission.denied) perm = await Geolocator.requestPermission();
      if (perm == LocationPermission.denied || perm == LocationPermission.deniedForever) {
        locationDenied = true;
        notifyListeners();
        return false;
      }
      locationDenied = false;
      final last = await Geolocator.getLastKnownPosition().catchError((_) => null);
      if (last != null) _setPos(last);
      _posSub ??= Geolocator.getPositionStream(
        locationSettings: const LocationSettings(accuracy: LocationAccuracy.high, distanceFilter: 15),
      ).listen(_setPos, onError: (_) {});
      return true;
    } catch (_) {
      return false;
    }
  }

  void _setPos(Position p) {
    myLat = p.latitude;
    myLon = p.longitude;
    notifyListeners();
  }

  @override
  void dispose() {
    _posSub?.cancel();
    super.dispose();
  }

  // ---- local queries (offline) ----

  List<NearbyStop> nearby(double lat, double lon, {double radius = 700, int limit = 30}) {
    final out = <NearbyStop>[];
    // Cheap bounding-box filter before the exact distance.
    final dLat = radius / 111320.0;
    for (final s in stops) {
      if ((s.lat - lat).abs() > dLat || (s.lon - lon).abs() > dLat * 1.4) continue;
      final d = distanceM(lat, lon, s.lat, s.lon);
      if (d <= radius) out.add(NearbyStop(s, d.round()));
    }
    out.sort((a, b) => a.distanceM.compareTo(b.distanceM));
    return out.length > limit ? out.sublist(0, limit) : out;
  }

  ({List<Stop> stops, List<Line> lines}) search(String q) {
    final fq = fold(q);
    if (fq.isEmpty) return (stops: const <Stop>[], lines: const <Line>[]);
    final exact = <Line>[], prefix = <Line>[], other = <Line>[];
    for (final l in lines) {
      final fs = fold(l.short);
      if (fs == fq) {
        exact.add(l);
      } else if (fs.startsWith(fq)) {
        prefix.add(l);
      } else if (fold(l.name).contains(fq)) {
        other.add(l);
      }
    }
    final ls = [...exact, ...prefix, ...other].take(20).toList();
    final starts = <Stop>[], contains = <Stop>[];
    for (final s in stops) {
      if (s.folded.startsWith(fq) || s.code == q) {
        starts.add(s);
      } else if (s.folded.contains(fq)) {
        contains.add(s);
      }
      if (starts.length >= 40) break;
    }
    return (stops: [...starts, ...contains].take(40).toList(), lines: ls);
  }
}

enum AppThemeMode { system, light, dark }

extension ThemeModeX on AppThemeMode {
  static AppThemeMode parse(String? s) =>
      AppThemeMode.values.firstWhere((m) => m.name == s, orElse: () => AppThemeMode.system);
}
