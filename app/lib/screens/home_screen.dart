import 'dart:async';
import 'dart:math' show Point;

import 'package:maplibre_gl/maplibre_gl.dart';
import 'package:material_ui/material_ui.dart';

import '../api/models.dart' as m;
import '../state/app_state.dart';
import '../state/scope.dart';
import '../util/style.dart';
import '../util/text.dart';
import '../widgets/common.dart';
import 'line_screen.dart';
import 'search_screen.dart';
import '../widgets/city_sheet.dart';
import 'stop_screen.dart';

class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key, this.active = true});

  /// False while another tab is shown: live polling pauses.
  final bool active;
  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> with WidgetsBindingObserver {
  MapLibreMapController? _map;
  List<m.NearbyVehicle> _buses = const [];
  Timer? _busTimer;
  bool _resumed = true;
  double _zoom = 12;
  bool _trackedNearbyBuses = false;
  bool _styleReady = false;
  String? _shownCity;
  int _shownStops = -1;
  double _centerLat = 0, _centerLon = 0;
  String? _cameraCity; // city the camera was last moved to
  bool _followMe = false;
  List<NearbyStop> _nearby = const [];
  AppState? _state;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final s = AppScope.of(context);
    if (_state != s) {
      _state?.removeListener(_onState);
      _state = s..addListener(_onState);
      _centerLat = s.city.lat;
      _centerLon = s.city.lon;
      _cameraCity = s.hasCity ? s.cityId : null;
      _recomputeNearby();
    }
  }

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _busTimer = Timer(const Duration(seconds: 3), _pollBuses);
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _busTimer?.cancel();
    _state?.removeListener(_onState);
    super.dispose();
  }

  @override
  void didUpdateWidget(HomeScreen old) {
    super.didUpdateWidget(old);
    if (widget.active && !old.active) _pollBuses();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    _resumed = state == AppLifecycleState.resumed;
    if (_resumed) _pollBuses();
  }

  /// Live buses around the user (or the map centre). Runs only while the
  /// map tab is visible, the app is in front and the map is zoomed in.
  Future<void> _pollBuses() async {
    _busTimer?.cancel();
    final s = _state;
    if (!mounted || s == null) return;
    final canPoll = widget.active && _resumed && s.city.features.liveVehicles && _zoom >= 13;
    if (canPoll) {
      final lat = _followMe && s.myLat != null ? s.myLat! : _centerLat;
      final lon = _followMe && s.myLon != null ? s.myLon! : _centerLon;
      try {
        final v = await s.api.nearbyVehicles(s.cityId, lat, lon);
        if (!mounted) return;
        setState(() => _buses = v);
        if (!_trackedNearbyBuses && v.isNotEmpty) {
          _trackedNearbyBuses = true;
          s.track('nearby_vehicles');
        }
        await _drawBuses();
      } catch (_) {}
    } else if (_buses.isNotEmpty && !s.city.features.liveVehicles) {
      setState(() => _buses = const []);
      await _drawBuses();
    }
    if (mounted) _busTimer = Timer(const Duration(seconds: 15), _pollBuses);
  }

  Future<void> _drawBuses() async {
    final c = _map;
    if (c == null || !_styleReady) return;
    try {
      await c.setGeoJsonSource('buses', {
        'type': 'FeatureCollection',
        'features': [
          for (final b in _buses)
            {
              'type': 'Feature',
              'properties': {
                'fid': '${b.lineId}#${b.id}',
                'line': b.line,
                'color': b.color.length == 6 ? '#${b.color}' : modeHex(b.mode),
              },
              'geometry': {
                'type': 'Point',
                'coordinates': [b.lon, b.lat],
              },
            },
        ],
      });
    } catch (e) {
      debugPrint('hibro buses: $e');
    }
  }

  void _onState() {
    final s = _state!;
    if (s.hasCity && _cameraCity != s.cityId) {
      // City chosen or changed: fly there.
      _cameraCity = s.cityId;
      _centerLat = s.city.lat;
      _centerLon = s.city.lon;
      _map?.moveCamera(CameraUpdate.newLatLngZoom(LatLng(s.city.lat, s.city.lon), s.city.zoom));
    }
    if (_followMe && s.myLat != null) {
      _centerLat = s.myLat!;
      _centerLon = s.myLon!;
    }
    _syncLayers();
    _recomputeNearby();
  }

  void _recomputeNearby() {
    final s = _state;
    if (s == null) return;
    final lat = s.myLat != null && _followMe ? s.myLat! : _centerLat;
    final lon = s.myLat != null && _followMe ? s.myLon! : _centerLon;
    final n = s.nearby(lat, lon, radius: 800, limit: 40);
    if (mounted) setState(() => _nearby = n);
  }

  // ---- map layers ----

  Map<String, dynamic> _stopsGeoJson(List<m.Stop> stops) => {
        'type': 'FeatureCollection',
        'features': [
          for (final st in stops)
            {
              'type': 'Feature',
              'properties': {'sid': st.id, 'mode': st.mode.name, 'name': st.name},
              'geometry': {
                'type': 'Point',
                'coordinates': [st.lon, st.lat],
              },
            },
        ],
      };

  Map<String, dynamic> _meGeoJson() {
    final s = _state!;
    return {
      'type': 'FeatureCollection',
      'features': [
        if (s.myLat != null)
          {
            'type': 'Feature',
            'properties': <String, dynamic>{},
            'geometry': {
              'type': 'Point',
              'coordinates': [s.myLon, s.myLat],
            },
          },
      ],
    };
  }

  Future<void> _onStyleLoaded() async {
    final c = _map;
    if (c == null) return;
    _styleReady = true;
    _shownStops = -1;
    try {
      await c.addGeoJsonSource('stops', _stopsGeoJson(const []), promoteId: 'sid');
      final colorByMode = [
        'match',
        ['get', 'mode'],
        'subway', modeHex(m.Mode.subway),
        'tram', modeHex(m.Mode.tram),
        'rail', modeHex(m.Mode.rail),
        'ferry', modeHex(m.Mode.ferry),
        modeHex(m.Mode.bus),
      ];
      // Rail stations are visible from further out than bus stops.
      await c.addCircleLayer(
        'stops',
        'stops-rail',
        CircleLayerProperties(
          circleColor: colorByMode,
          circleRadius: ['interpolate', ['linear'], ['zoom'], 11, 3.5, 16, 8],
          circleStrokeColor: '#ffffff',
          circleStrokeWidth: 1.5,
        ),
        filter: ['!=', ['get', 'mode'], 'bus'],
        minzoom: 10.5,
      );
      await c.addCircleLayer(
        'stops',
        'stops-bus',
        CircleLayerProperties(
          circleColor: colorByMode,
          circleRadius: ['interpolate', ['linear'], ['zoom'], 14, 3, 17, 7],
          circleStrokeColor: '#ffffff',
          circleStrokeWidth: 1.2,
        ),
        filter: ['==', ['get', 'mode'], 'bus'],
        minzoom: 14,
      );
      await c.addGeoJsonSource('buses', {'type': 'FeatureCollection', 'features': <Map<String, dynamic>>[]}, promoteId: 'fid');
      await c.addCircleLayer(
        'buses',
        'buses',
        const CircleLayerProperties(
          circleColor: ['get', 'color'],
          circleRadius: 12,
          circleStrokeColor: '#ffffff',
          circleStrokeWidth: 2.5,
        ),
      );
      await c.addSymbolLayer(
        'buses',
        'buses-label',
        const SymbolLayerProperties(
          textField: ['get', 'line'],
          textSize: 10.5,
          textColor: '#ffffff',
          textFont: ['Noto Sans Bold'],
          textAllowOverlap: true,
          textIgnorePlacement: true,
        ),
        enableInteraction: false,
      );
      await c.addGeoJsonSource('me', _meGeoJson());
      await c.addCircleLayer(
        'me',
        'me-halo',
        const CircleLayerProperties(circleColor: '#1E88E5', circleOpacity: 0.18, circleRadius: 18),
        enableInteraction: false,
      );
      await c.addCircleLayer(
        'me',
        'me-dot',
        const CircleLayerProperties(circleColor: '#1E88E5', circleRadius: 7, circleStrokeColor: '#ffffff', circleStrokeWidth: 2.5),
        enableInteraction: false,
      );
    } catch (e) {
      // Style reloaded (theme switch): layers are re-added on next load.
      debugPrint('hibro map layers: $e');
    }
    _syncLayers();
    _drawBuses();
  }

  Future<void> _syncLayers() async {
    final c = _map;
    final s = _state;
    if (c == null || s == null || !_styleReady) return;
    try {
      if (_shownStops != s.stops.length || _shownCity != s.cityId) {
        _shownStops = s.stops.length;
        _shownCity = s.cityId;
        await c.setGeoJsonSource('stops', _stopsGeoJson(s.stops));
      }
      await c.setGeoJsonSource('me', _meGeoJson());
    } catch (_) {}
  }

  void _onFeatureTapped(Point<double> point, LatLng coords, String id, String layerId, Annotation? a) {
    if (layerId == 'buses') {
      final lineId = id.split('#').first;
      Navigator.of(context).push(MaterialPageRoute(builder: (_) => LineScreen(lineId: lineId)));
      return;
    }
    if (!layerId.startsWith('stops')) return;
    final stop = _state?.stopById[id];
    if (stop != null) _openStop(stop);
  }

  void _openStop(m.Stop s) {
    Navigator.of(context).push(MaterialPageRoute(builder: (_) => StopScreen(stop: s)));
  }

  void _onCameraIdle() {
    final pos = _map?.cameraPosition;
    if (pos == null) return;
    _centerLat = pos.target.latitude;
    _centerLon = pos.target.longitude;
    final zoomedIn = pos.zoom >= 13 && _zoom < 13;
    _zoom = pos.zoom;
    _recomputeNearby();
    if (zoomedIn) _pollBuses();
  }

  Future<void> _locateMe() async {
    final s = _state!;
    s.track('locate_me');
    final ok = await s.startLocation();
    if (!mounted) return;
    if (!ok) {
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
        content: Text('Konum izni yok. Yakındaki duraklar harita merkezine göre gösteriliyor.'),
      ));
      return;
    }
    // Wait briefly for a first fix.
    for (var i = 0; i < 20 && s.myLat == null; i++) {
      await Future<void>.delayed(const Duration(milliseconds: 150));
    }
    if (s.myLat == null) return;
    _followMe = true;
    _zoom = 16;
    await _map?.animateCamera(CameraUpdate.newLatLngZoom(LatLng(s.myLat!, s.myLon!), 16));
    _recomputeNearby();
    _pollBuses();
  }

  @override
  Widget build(BuildContext context) {
    final s = AppScope.of(context);
    final theme = Theme.of(context);
    final dark = theme.brightness == Brightness.dark;
    return Scaffold(
      body: Stack(children: [
        MapLibreMap(
          key: ValueKey(dark),
          styleString: dark ? mapStyleDark : mapStyleLight,
          initialCameraPosition: CameraPosition(target: LatLng(s.city.lat, s.city.lon), zoom: s.city.zoom),
          onMapCreated: (c) {
            _map = c;
            c.onFeatureTapped.add(_onFeatureTapped);
          },
          onStyleLoadedCallback: _onStyleLoaded,
          onCameraIdle: _onCameraIdle,
          onCameraMoveStarted: (reason) {
            if (reason == CameraMoveReason.gesture) _followMe = false;
          },
          trackCameraPosition: true,
          compassEnabled: false,
          attributionButtonPosition: AttributionButtonPosition.bottomLeft,
          attributionButtonMargins: const Point(8, 120),
        ),
        SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(12, 8, 12, 0),
            child: Column(children: [
              _SearchBar(
                cityName: s.hasCity ? s.city.name : 'Şehir seç',
                onTap: () {
                  if (!s.hasCity) {
                    showCitySheet(context);
                    return;
                  }
                  Navigator.of(context).push(MaterialPageRoute(builder: (_) => const SearchScreen()));
                },
              ),
              if (s.error != null && s.stops.isEmpty)
                Padding(
                  padding: const EdgeInsets.only(top: 8),
                  child: Material(
                    color: theme.colorScheme.errorContainer,
                    borderRadius: BorderRadius.circular(12),
                    child: ListTile(
                      dense: true,
                      leading: Icon(Icons.cloud_off, color: theme.colorScheme.onErrorContainer),
                      title: Text(s.error!, style: TextStyle(color: theme.colorScheme.onErrorContainer)),
                      trailing: TextButton(onPressed: s.refresh, child: const Text('Tekrar dene')),
                    ),
                  ),
                ),
              if (s.loading && s.stops.isEmpty)
                const Padding(padding: EdgeInsets.only(top: 8), child: LinearProgressIndicator()),
              if (s.suggestedCity != null)
                Padding(
                  padding: const EdgeInsets.only(top: 8),
                  child: Material(
                    color: theme.colorScheme.primaryContainer,
                    borderRadius: BorderRadius.circular(12),
                    child: ListTile(
                      dense: true,
                      leading: Icon(Icons.near_me, color: theme.colorScheme.onPrimaryContainer),
                      title: Text('Konumun: ${s.suggestedCity!.name}. Şehri değiştirelim mi?',
                          style: TextStyle(color: theme.colorScheme.onPrimaryContainer)),
                      trailing: Row(mainAxisSize: MainAxisSize.min, children: [
                        TextButton(onPressed: s.dismissSuggestion, child: const Text('Hayır')),
                        FilledButton(onPressed: () => s.selectCity(s.suggestedCity!.id), child: const Text('Değiştir')),
                      ]),
                    ),
                  ),
                ),
            ]),
          ),
        ),
        DraggableScrollableSheet(
          initialChildSize: 0.3,
          minChildSize: 0.12,
          maxChildSize: 0.85,
          snap: true,
          snapSizes: const [0.12, 0.3, 0.85],
          builder: (context, scroll) => _NearbySheet(
            scroll: scroll,
            nearby: _nearby,
            buses: _buses,
            aroundMe: _followMe && s.myLat != null,
            onTap: _openStop,
            onLocate: _locateMe,
          ),
        ),
      ]),
    );
  }
}

class _SearchBar extends StatelessWidget {
  const _SearchBar({required this.cityName, required this.onTap});
  final String cityName;
  final VoidCallback onTap;
  @override
  Widget build(BuildContext context) {
    final t = Theme.of(context);
    return Material(
      elevation: 3,
      color: t.colorScheme.surface,
      borderRadius: BorderRadius.circular(28),
      child: InkWell(
        borderRadius: BorderRadius.circular(28),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
          child: Row(children: [
            Icon(Icons.search, color: t.colorScheme.onSurfaceVariant),
            const SizedBox(width: 12),
            Expanded(
              child: Text('Durak veya hat ara', style: t.textTheme.bodyLarge?.copyWith(color: t.colorScheme.onSurfaceVariant)),
            ),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
              decoration: BoxDecoration(color: t.colorScheme.secondaryContainer, borderRadius: BorderRadius.circular(12)),
              child: Text(cityName, style: t.textTheme.labelMedium?.copyWith(color: t.colorScheme.onSecondaryContainer)),
            ),
          ]),
        ),
      ),
    );
  }
}

class _NearbySheet extends StatelessWidget {
  const _NearbySheet({required this.scroll, required this.nearby, required this.buses, required this.aroundMe, required this.onTap, required this.onLocate});
  final ScrollController scroll;
  final List<NearbyStop> nearby;
  final List<m.NearbyVehicle> buses;
  final bool aroundMe;
  final void Function(m.Stop) onTap;
  final VoidCallback onLocate;

  @override
  Widget build(BuildContext context) {
    final t = Theme.of(context);
    final s = AppScope.of(context);
    return Material(
      elevation: 8,
      color: t.colorScheme.surface,
      borderRadius: const BorderRadius.vertical(top: Radius.circular(20)),
      child: CustomScrollView(controller: scroll, slivers: [
        SliverToBoxAdapter(
          child: Column(children: [
            const SizedBox(height: 8),
            Container(width: 36, height: 4, decoration: BoxDecoration(color: t.colorScheme.outlineVariant, borderRadius: BorderRadius.circular(2))),
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 8, 4),
              child: Row(children: [
                Expanded(
                  child: Text(aroundMe ? 'Yakınımdaki duraklar' : 'Bu bölgedeki duraklar', style: t.textTheme.titleMedium),
                ),
                FilledButton.tonalIcon(
                  onPressed: onLocate,
                  icon: const Icon(Icons.my_location, size: 18),
                  label: const Text('Konumum'),
                ),
              ]),
            ),
            if (buses.isNotEmpty)
              SizedBox(
                height: 44,
                child: ListView(
                  scrollDirection: Axis.horizontal,
                  padding: const EdgeInsets.symmetric(horizontal: 12),
                  children: [
                    Padding(
                      padding: const EdgeInsets.only(right: 8, top: 12),
                      child: Row(children: [
                        const LiveDot(),
                        const SizedBox(width: 6),
                        Text('${buses.length} otobüs yakında', style: t.textTheme.labelMedium),
                      ]),
                    ),
                    for (final b in buses)
                      Padding(
                        padding: const EdgeInsets.only(right: 6),
                        child: ActionChip(
                          visualDensity: VisualDensity.compact,
                          padding: const EdgeInsets.symmetric(horizontal: 4),
                          label: Row(mainAxisSize: MainAxisSize.min, children: [
                            LineBadge(text: b.line, mode: b.mode, color: b.color, dense: true),
                            const SizedBox(width: 6),
                            Text(formatDistance(b.distanceM)),
                          ]),
                          onPressed: () => Navigator.of(context).push(MaterialPageRoute(builder: (_) => LineScreen(lineId: b.lineId))),
                        ),
                      ),
                  ],
                ),
              ),
          ]),
        ),
        if (nearby.isEmpty)
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Text(
                !s.hasCity
                    ? 'Şehir seçilmedi. Arama çubuğundaki "Şehir seç"e dokun.'
                    : s.stops.isEmpty
                        ? 'Şehir verisi yükleniyor…'
                        : 'Yakında durak yok. Haritayı kaydır veya yakınlaştır.',
                textAlign: TextAlign.center,
                style: t.textTheme.bodyMedium?.copyWith(color: t.colorScheme.onSurfaceVariant),
              ),
            ),
          ),
        SliverList.builder(
          itemCount: nearby.length,
          itemBuilder: (context, i) {
            final n = nearby[i];
            final lines = n.stop.lines.map((id) => s.lineById[id]).whereType<m.Line>().toList();
            return ListTile(
              leading: ModeAvatar(mode: n.stop.mode),
              title: Text(n.stop.name, maxLines: 1, overflow: TextOverflow.ellipsis),
              subtitle: lines.isNotEmpty
                  ? Padding(
                      padding: const EdgeInsets.only(top: 4),
                      child: Wrap(spacing: 4, runSpacing: 4, children: [
                        for (final l in lines.take(6)) LineBadge(text: l.short, mode: l.mode, color: l.color, textColor: l.textColor, dense: true),
                        if (lines.length > 6) Text('+${lines.length - 6}', style: t.textTheme.labelSmall),
                      ]),
                    )
                  : Text([modeName(n.stop.mode), if (n.stop.district.isNotEmpty) n.stop.district, if (n.stop.code.isNotEmpty) 'No ${n.stop.code}'].join(' · ')),
              trailing: Text(formatDistance(n.distanceM), style: t.textTheme.labelLarge),
              onTap: () => onTap(n.stop),
            );
          },
        ),
        const SliverToBoxAdapter(child: SizedBox(height: 24)),
      ]),
    );
  }
}
