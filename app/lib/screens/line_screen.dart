import 'dart:async';
import 'dart:math' as math;

import 'package:maplibre_gl/maplibre_gl.dart';
import 'package:material_ui/material_ui.dart';

import '../api/models.dart' as m;
import '../state/scope.dart';
import '../util/style.dart';
import '../widgets/common.dart';
import '../widgets/report_sheet.dart';
import '../widgets/stale_banner.dart';
import 'ride_screen.dart';
import 'stop_screen.dart';

class LineScreen extends StatefulWidget {
  const LineScreen({super.key, required this.lineId});
  final String lineId;
  @override
  State<LineScreen> createState() => _LineScreenState();
}

class _LineScreenState extends State<LineScreen> with WidgetsBindingObserver {
  m.LineDetail? _detail;
  String? _error;
  int _dir = 0;
  List<m.Vehicle> _vehicles = const [];
  bool? _liveOk; // null until the first live answer arrives
  Timer? _timer;
  MapLibreMapController? _map;
  bool _styleReady = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      AppScope.read(context).track('screen_line');
      _load();
    });
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _timer?.cancel();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      _pollVehicles();
    } else {
      _timer?.cancel();
    }
  }

  Future<void> _load() async {
    final s = AppScope.read(context);
    try {
      final d = await s.api.line(s.cityId, widget.lineId);
      if (!mounted) return;
      setState(() {
        _detail = d;
        _error = null;
        _dir = 0;
      });
      await _drawLine();
      _pollVehicles();
    } catch (e) {
      if (mounted) setState(() => _error = e.toString());
    }
  }

  Future<void> _pollVehicles() async {
    _timer?.cancel();
    final d = _detail;
    if (d == null || !d.line.hasLive || !mounted) return;
    final s = AppScope.read(context);
    var retrySoon = false;
    try {
      final r = await s.api.vehicles(s.cityId, widget.lineId);
      if (!mounted) return;
      retrySoon = r.degraded;
      setState(() {
        if (!r.degraded) _vehicles = r.vehicles;
        _liveOk = !r.degraded;
      });
      await _drawVehicles();
    } catch (_) {
      retrySoon = true;
      if (mounted) setState(() => _liveOk ??= false);
    }
    if (mounted) _timer = Timer(retrySoon ? const Duration(seconds: 3) : s.liveInterval(10), _pollVehicles);
  }

  m.Direction? get _direction {
    final d = _detail;
    if (d == null || d.directions.isEmpty) return null;
    return d.directions[_dir.clamp(0, d.directions.length - 1)];
  }

  String get _lineColor {
    final d = _detail;
    if (d == null) return modeHex(m.Mode.bus);
    return d.line.color.length == 6 ? '#${d.line.color}' : modeHex(d.line.mode);
  }

  Future<void> _onStyleLoaded() async {
    final c = _map;
    if (c == null) return;
    try {
      await c.addGeoJsonSource('route', _fc(const []));
      await c.addLineLayer('route', 'route-casing', const LineLayerProperties(lineColor: '#ffffff', lineWidth: 8, lineJoin: 'round', lineCap: 'round'), enableInteraction: false);
      await c.addLineLayer('route', 'route-line', LineLayerProperties(lineColor: _lineColor, lineWidth: 5, lineJoin: 'round', lineCap: 'round'), enableInteraction: false);
      await c.addGeoJsonSource('lstops', _fc(const []), promoteId: 'sid');
      await c.addCircleLayer('lstops', 'lstops', CircleLayerProperties(circleColor: '#ffffff', circleRadius: 4.5, circleStrokeColor: _lineColor, circleStrokeWidth: 2.5));
      await c.addGeoJsonSource('veh', _fc(const []));
      await c.addCircleLayer('veh', 'veh-halo', CircleLayerProperties(circleColor: _lineColor, circleOpacity: 0.25, circleRadius: 14), enableInteraction: false);
      await c.addCircleLayer('veh', 'veh', CircleLayerProperties(circleColor: _lineColor, circleRadius: 8, circleStrokeColor: '#ffffff', circleStrokeWidth: 3), enableInteraction: false);
      _styleReady = true;
      await _drawLine();
      await _drawVehicles();
    } catch (_) {}
  }

  Map<String, dynamic> _fc(List<Map<String, dynamic>> features) => {'type': 'FeatureCollection', 'features': features};

  Future<void> _drawLine() async {
    final c = _map;
    final dir = _direction;
    if (c == null || !_styleReady || dir == null) return;
    try {
      await c.setLayerProperties('route-line', LineLayerProperties(lineColor: _lineColor));
      await c.setGeoJsonSource('route', _fc([
        if (dir.shape.length > 1)
          {
            'type': 'Feature',
            'properties': <String, dynamic>{},
            'geometry': {
              'type': 'LineString',
              'coordinates': [for (final p in dir.shape) [p[1], p[0]]],
            },
          },
      ]));
      await c.setGeoJsonSource('lstops', _fc([
        for (final st in dir.stops)
          {
            'type': 'Feature',
            'properties': {'sid': st.id},
            'geometry': {
              'type': 'Point',
              'coordinates': [st.lon, st.lat],
            },
          },
      ]));
      final pts = dir.shape.isNotEmpty ? dir.shape : [for (final st in dir.stops) [st.lat, st.lon]];
      if (pts.length > 1) {
        var minLat = 90.0, maxLat = -90.0, minLon = 180.0, maxLon = -180.0;
        for (final p in pts) {
          minLat = math.min(minLat, p[0]);
          maxLat = math.max(maxLat, p[0]);
          minLon = math.min(minLon, p[1]);
          maxLon = math.max(maxLon, p[1]);
        }
        await c.moveCamera(CameraUpdate.newLatLngBounds(
          LatLngBounds(southwest: LatLng(minLat, minLon), northeast: LatLng(maxLat, maxLon)),
          left: 40, right: 40, top: 40, bottom: 40,
        ));
      }
    } catch (_) {}
  }

  Future<void> _drawVehicles() async {
    final c = _map;
    if (c == null || !_styleReady) return;
    try {
      await c.setGeoJsonSource('veh', _fc([
        for (final v in _vehicles)
          {
            'type': 'Feature',
            'properties': {'id': v.id},
            'geometry': {
              'type': 'Point',
              'coordinates': [v.lon, v.lat],
            },
          },
      ]));
    } catch (_) {}
  }

  void _onFeatureTapped(math.Point<double> p, LatLng c, String id, String layerId, Annotation? a) {
    if (layerId != 'lstops') return;
    final stop = AppScope.read(context).stopById[id];
    if (stop != null) {
      Navigator.of(context).push(MaterialPageRoute(builder: (_) => StopScreen(stop: stop)));
    }
  }

  @override
  Widget build(BuildContext context) {
    final s = AppScope.of(context);
    final t = Theme.of(context);
    final d = _detail;
    final line = d?.line ?? s.lineById[widget.lineId];
    final fav = s.isFavorite('line', widget.lineId);
    final dark = t.brightness == Brightness.dark;

    return DefaultTabController(
      length: 2,
      child: Scaffold(
        appBar: AppBar(
          titleSpacing: 0,
          title: Row(children: [
            if (line != null) LineBadge(text: line.short, mode: line.mode, color: line.color, textColor: line.textColor),
            const SizedBox(width: 10),
            Expanded(child: Text(line?.name ?? '', maxLines: 1, overflow: TextOverflow.ellipsis)),
          ]),
          actions: [
            if (line != null)
              IconButton(
                tooltip: 'Sorun bildir',
                icon: const Icon(Icons.flag_outlined),
                onPressed: () => showReportSheet(context, line: line),
              ),
            if (line != null)
              IconButton(
                icon: Icon(fav ? Icons.star : Icons.star_border, color: fav ? Colors.amber : null),
                tooltip: fav ? 'Favorilerden çıkar' : 'Favorilere ekle',
                onPressed: () => s.toggleFavorite('line', line.id, '${line.short} ${line.name}', modeName(line.mode)),
              ),
          ],
        ),
        body: _error != null && d == null
            ? EmptyState(icon: Icons.cloud_off, title: 'Hat bilgisi alınamadı', body: _error, action: FilledButton(onPressed: _load, child: const Text('Tekrar dene')))
            : Column(children: [
                SizedBox(
                  height: MediaQuery.of(context).size.height * 0.34,
                  child: Stack(children: [
                    MapLibreMap(
                      key: ValueKey(dark),
                      styleString: dark ? mapStyleDark : mapStyleLight,
                      initialCameraPosition: CameraPosition(target: LatLng(s.city.lat, s.city.lon), zoom: 11),
                      onMapCreated: (c) {
                        _map = c;
                        c.onFeatureTapped.add(_onFeatureTapped);
                      },
                      onStyleLoadedCallback: _onStyleLoaded,
                      compassEnabled: false,
                      rotateGesturesEnabled: false,
                      tiltGesturesEnabled: false,
                    ),
                    if (d?.line.hasLive == true)
                      Positioned(
                        right: 8,
                        top: 8,
                        child: Chip(
                          avatar: const LiveDot(),
                          label: Text(switch (_liveOk) {
                            null => 'Canlı konum alınıyor…',
                            false when _vehicles.isEmpty => 'Canlı veri geçici olarak yok',
                            _ => _vehicles.isEmpty ? 'Şu an seferde araç yok' : 'Canlı: ${_vehicles.length} araç',
                          }),
                          visualDensity: VisualDensity.compact,
                        ),
                      ),
                  ]),
                ),
                if (d == null) const LinearProgressIndicator(),
                StaleBanner(id: widget.lineId),
                if (d != null && d.alerts.isNotEmpty)
                  Material(
                    color: t.colorScheme.tertiaryContainer,
                    child: ExpansionTile(
                      leading: const Icon(Icons.campaign_outlined),
                      title: Text('${d.alerts.length} duyuru', style: TextStyle(color: t.colorScheme.onTertiaryContainer)),
                      children: [for (final a in d.alerts) ListTile(dense: true, title: Text(a.title))],
                    ),
                  ),
                if (d != null && d.directions.length > 1)
                  Padding(
                    padding: const EdgeInsets.fromLTRB(12, 8, 12, 0),
                    child: SegmentedButton<int>(
                      segments: [
                        for (final dd in d.directions)
                          ButtonSegment(value: d.directions.indexOf(dd), label: Text('→ ${dd.headsign}', overflow: TextOverflow.ellipsis)),
                      ],
                      selected: {_dir},
                      showSelectedIcon: false,
                      onSelectionChanged: (v) {
                        setState(() => _dir = v.first);
                        _drawLine();
                      },
                    ),
                  ),
                if (d != null && _direction != null)
                  Padding(
                    padding: const EdgeInsets.fromLTRB(12, 8, 12, 0),
                    child: SizedBox(
                      width: double.infinity,
                      child: FilledButton.tonalIcon(
                        onPressed: () => Navigator.of(context).push(MaterialPageRoute(
                          builder: (_) => RideScreen(line: d.line, direction: _direction!),
                        )),
                        icon: const Icon(Icons.directions_bus_filled_outlined),
                        label: const Text('Bu araca bindim · inilecek durakta uyar'),
                      ),
                    ),
                  ),
                if (d != null)
                  const TabBar(tabs: [Tab(text: 'Sefer saatleri'), Tab(text: 'Duraklar')]),
                if (d != null)
                  Expanded(
                    child: TabBarView(children: [
                      _Timetable(direction: _direction, dayType: d.dayType),
                      _StopList(direction: _direction),
                    ]),
                  ),
              ]),
      ),
    );
  }
}

class _Timetable extends StatelessWidget {
  const _Timetable({required this.direction, required this.dayType});
  final m.Direction? direction;
  final String dayType;

  @override
  Widget build(BuildContext context) {
    final t = Theme.of(context);
    final deps = direction?.departures ?? const [];
    if (deps.isEmpty) {
      return const EmptyState(icon: Icons.event_busy, title: 'Bugün için sefer saati yok');
    }
    final now = TimeOfDay.now();
    final nowMin = now.hour * 60 + now.minute;
    final nextIdx = deps.indexWhere((d) {
      final p = d.split(':');
      final mins = int.parse(p[0]) * 60 + int.parse(p[1]);
      return mins >= nowMin;
    });
    final dayName = switch (dayType) { 'saturday' => 'Cumartesi', 'sunday' => 'Pazar', _ => 'Hafta içi' };
    return ListView(padding: const EdgeInsets.all(16), children: [
      Text('$dayName · ${direction!.headsign} yönü · ilk duraktan kalkış', style: t.textTheme.labelLarge),
      const SizedBox(height: 12),
      Wrap(spacing: 8, runSpacing: 8, children: [
        for (var i = 0; i < deps.length; i++)
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
            decoration: BoxDecoration(
              color: i == nextIdx
                  ? t.colorScheme.primary
                  : (nextIdx >= 0 && i < nextIdx ? t.colorScheme.surfaceContainerHighest.withValues(alpha: 0.5) : t.colorScheme.surfaceContainerHighest),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Text(
              deps[i],
              style: t.textTheme.bodyMedium?.copyWith(
                fontFeatures: const [FontFeature.tabularFigures()],
                color: i == nextIdx ? t.colorScheme.onPrimary : (nextIdx >= 0 && i < nextIdx ? t.colorScheme.outline : null),
                fontWeight: i == nextIdx ? FontWeight.w700 : null,
              ),
            ),
          ),
      ]),
    ]);
  }
}

class _StopList extends StatelessWidget {
  const _StopList({required this.direction});
  final m.Direction? direction;
  @override
  Widget build(BuildContext context) {
    final s = AppScope.of(context);
    final stops = direction?.stops ?? const [];
    if (stops.isEmpty) {
      return const EmptyState(
        icon: Icons.format_list_bulleted,
        title: 'Durak sırası henüz yok',
        body: 'Belediye bu hat için durak sırasını yayınlamıyor. Durağa dokunarak canlı varışları görebilirsin.',
      );
    }
    return ListView.builder(
      itemCount: stops.length,
      itemBuilder: (context, i) {
        final st = stops[i];
        final full = s.stopById[st.id];
        return ListTile(
          dense: true,
          leading: SizedBox(
            width: 24,
            child: Column(children: [
              Expanded(child: Container(width: 3, color: i == 0 ? Colors.transparent : Theme.of(context).colorScheme.outlineVariant)),
              Container(width: 12, height: 12, decoration: BoxDecoration(shape: BoxShape.circle, border: Border.all(width: 2.5, color: Theme.of(context).colorScheme.primary))),
              Expanded(child: Container(width: 3, color: i == stops.length - 1 ? Colors.transparent : Theme.of(context).colorScheme.outlineVariant)),
            ]),
          ),
          title: Text(st.name),
          onTap: full == null ? null : () => Navigator.of(context).push(MaterialPageRoute(builder: (_) => StopScreen(stop: full))),
        );
      },
    );
  }
}
