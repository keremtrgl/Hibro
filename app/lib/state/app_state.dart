import 'dart:async';
import 'dart:convert';
import 'dart:io' show File;
import 'dart:math' show Random;

import 'package:flutter/foundation.dart';
import 'package:geolocator/geolocator.dart';
import 'package:path_provider/path_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../api/client.dart';
import '../api/models.dart';
import '../util/city_detect.dart';
import '../util/geo.dart';
import '../util/text.dart';

const defaultApiUrl = String.fromEnvironment('HIBRO_API', defaultValue: 'https://api.geliyo.site');
const appVersion = 'geliyo-0.4';

/// Cities known at build time, so the map can open instantly on first
/// launch before anything is downloaded.
/// Bounds come from the server (26.11.2026) and are refreshed from
/// /v1/cities at start-up.
final builtInCities = <CityInfo>[
  CityInfo(id: 'izmir', name: 'İzmir', lat: 38.4237, lon: 27.1428, zoom: 12.5, bounds: [37.8192, 26.2848, 39.364, 28.2391]),
  CityInfo(id: 'konya', name: 'Konya', lat: 37.8746, lon: 32.4932, zoom: 12.5, bounds: [37.6213, 32.3097, 38.2523, 32.9269]),
  CityInfo(id: 'gaziantep', name: 'Gaziantep', lat: 37.0662, lon: 37.3833, zoom: 12.5, bounds: [36.7003, 36.5829, 37.4057, 38.0246]),
  CityInfo(id: 'kocaeli', name: 'Kocaeli', lat: 40.7654, lon: 29.9408, zoom: 12, bounds: [40.4096, 29.1875, 41.2092, 30.3661]),
  CityInfo(id: 'istanbul', name: 'İstanbul', lat: 41.0082, lon: 28.9784, zoom: 11.5, bounds: [40.80, 27.97, 41.58, 29.93]),
  // Community-data cities are discovered from the server's /v1/cities.
];

/// Map view of Türkiye, used before a city is chosen.
final turkeyView = CityInfo(id: '', name: 'Türkiye', lat: 39.0, lon: 35.2, zoom: 4.8);

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
  AppState(this.prefs) : api = GeliyoApi(prefs.getString('api') ?? defaultApiUrl) {
    cityId = prefs.getString('city') ?? 'izmir';
    themeMode = ThemeModeX.parse(prefs.getString('theme'));
    favorites = (jsonDecode(prefs.getString('favs') ?? '[]') as List)
        .map((e) => Favorite.fromJson(e as Map<String, dynamic>))
        .toList();
    dataSaver = prefs.getBool('dataSaver') ?? false;
    consentAsked = prefs.getBool('consentAsked') ?? false;
    shareUsage = prefs.getBool('shareUsage') ?? false;
    shareTrips = prefs.getBool('shareTrips') ?? false;
  }

  // ---- privacy choices (all off until the user turns them on) ----

  late bool consentAsked, shareUsage, shareTrips;

  /// "Veri tasarrufu": slower live refresh, no automatic nearby buses.
  late bool dataSaver;

  Future<void> setDataSaver(bool v) async {
    dataSaver = v;
    await prefs.setBool('dataSaver', v);
    notifyListeners();
  }

  /// Polling interval for live data, doubled in data-saver mode.
  Duration liveInterval(int seconds) => Duration(seconds: dataSaver ? seconds * 2 : seconds);

  Future<void> setConsent({bool? usage, bool? trips}) async {
    if (usage != null) {
      shareUsage = usage;
      await prefs.setBool('shareUsage', usage);
      if (!usage) _events.clear();
    }
    if (trips != null) {
      shareTrips = trips;
      await prefs.setBool('shareTrips', trips);
    }
    consentAsked = true;
    await prefs.setBool('consentAsked', true);
    notifyListeners();
  }

  /// Random id that changes every day, used only to group the points of
  /// one uploaded trip. It is never linked to the device or a person.
  String get dailySession {
    final today = DateTime.now().toIso8601String().substring(0, 10);
    if (prefs.getString('sessionDay') != today) {
      final r = Random.secure();
      const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-';
      final id = List.generate(24, (_) => chars[r.nextInt(chars.length)]).join();
      prefs.setString('sessionDay', today);
      prefs.setString('session', id);
    }
    return prefs.getString('session')!;
  }

  // ---- anonymous usage counters ----

  final Map<String, int> _events = {};
  Timer? _flushTimer;

  /// Counts an event locally. Nothing leaves the phone unless the user
  /// enabled "anonim kullanım istatistikleri".
  void track(String event) {
    if (!shareUsage) return;
    _events[event] = (_events[event] ?? 0) + 1;
    _flushTimer ??= Timer(const Duration(seconds: 60), flushEvents);
  }

  Future<void> flushEvents() async {
    _flushTimer?.cancel();
    _flushTimer = null;
    if (!shareUsage || _events.isEmpty) return;
    final batch = Map<String, int>.of(_events);
    _events.clear();
    try {
      await api.telemetry(cityId, batch);
    } catch (_) {
      // Best effort: counters are not worth retrying or storing.
    }
  }

  Future<void> sendReport({required String kind, String? stopId, String? lineId, String? plate, String? note}) async {
    await api.report(cityId, kind: kind, stopId: stopId, lineId: lineId, plate: plate, note: note, app: appVersion);
    track('report_sent');
  }

  final SharedPreferences prefs;
  final GeliyoApi api;

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

  CityInfo get city => !hasCity
      ? turkeyView
      : cityInfo ?? cities.firstWhere((c) => c.id == cityId, orElse: () => builtInCities.first);

  /// False until the user's city is known. Nothing is downloaded before.
  bool get hasCity => prefs.containsKey('city');

  /// A different supported city the user is currently in (travel).
  CityInfo? suggestedCity;
  bool _suggestionDismissed = false;

  Future<void> init() async {
    track('app_open');
    unawaited(_refreshCities());
    if (!hasCity) return; // wait for detection or a manual choice
    await _loadCachedPack();
    // The pack changes rarely; check at most every 6 hours (a check
    // that finds no change costs ~200 bytes thanks to the ETag).
    final last = DateTime.tryParse(prefs.getString('packAt_$cityId') ?? '');
    if (stops.isEmpty || last == null || DateTime.now().difference(last) > const Duration(hours: 6)) {
      unawaited(refresh());
    }
  }

  /// Finds the city from the current location. Returns (city, located):
  /// city is null when the user is outside every supported city.
  Future<({CityInfo? city, bool located})> detectFromLocation() async {
    // Make sure the full city list (with bounds) from the server is here.
    if (cities.every((c) => c.stopCount == 0)) await _refreshCities();
    if (!await startLocation()) return (city: null, located: false);
    for (var i = 0; i < 40 && myLat == null; i++) {
      await Future<void>.delayed(const Duration(milliseconds: 150));
    }
    if (myLat == null) return (city: null, located: false);
    return (city: detectCity(cities, myLat!, myLon!), located: true);
  }

  void dismissSuggestion() {
    suggestedCity = null;
    _suggestionDismissed = true;
    notifyListeners();
  }

  Future<void> _refreshCities() async {
    try {
      final list = await api.cities();
      if (list.isNotEmpty) {
        list.sort((a, b) => fold(a.name).compareTo(fold(b.name)));
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
    if (!hasCity) return;
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
    if (id == cityId && hasCity) return;
    cityId = id;
    suggestedCity = null;
    await prefs.setString('city', id);
    await _deleteOtherPacks(id);
    cityInfo = null;
    stops = const [];
    lines = const [];
    stopById = const {};
    lineById = const {};
    notifyListeners();
    await _loadCachedPack();
    await refresh();
  }

  /// Only the current city's data stays on the phone.
  Future<void> _deleteOtherPacks(String keep) async {
    final ids = {...cities.map((c) => c.id), ...builtInCities.map((c) => c.id)}..remove(keep);
    for (final id in ids) {
      try {
        final f = await _packFile(id);
        if (f != null && await f.exists()) await f.delete();
      } catch (_) {}
      await prefs.remove('etag_$id');
      await prefs.remove('packAt_$id');
    }
  }

  Future<void> setApiUrl(String url) async {
    api.baseUrl = url.trim();
    await prefs.setString('api', api.baseUrl);
    await prefs.remove('etag_$cityId');
    if (hasCity) await refresh();
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
      track('favorite_add');
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
      // Not supported on web, where it throws synchronously.
      try {
        final last = await Geolocator.getLastKnownPosition();
        if (last != null) _setPos(last);
      } catch (_) {}
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
    // Travelling: offer to switch when the user is in another supported city.
    if (hasCity && !_suggestionDismissed) {
      final here = detectCity(cities, p.latitude, p.longitude);
      suggestedCity = here != null && here.id != cityId ? here : null;
    }
    notifyListeners();
  }

  @override
  void dispose() {
    _posSub?.cancel();
    _flushTimer?.cancel();
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
