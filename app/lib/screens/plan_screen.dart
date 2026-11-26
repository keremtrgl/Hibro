import 'dart:math' as math;

import 'package:maplibre_gl/maplibre_gl.dart';
import 'package:material_ui/material_ui.dart';

import '../api/models.dart';
import '../state/scope.dart';
import '../util/polyline.dart';
import '../util/style.dart';
import '../util/text.dart';
import '../widgets/common.dart';

/// Journey planning from the user's location to a stop or a map point.
/// The selected option is drawn on the map: rides in line colour, walks
/// dashed.
class PlanScreen extends StatefulWidget {
  const PlanScreen({super.key, required this.destination});
  final Stop destination;
  @override
  State<PlanScreen> createState() => _PlanScreenState();
}

class _PlanScreenState extends State<PlanScreen> {
  List<Itinerary>? _its;
  String? _error;
  int _selected = 0;
  double? _fromLat, _fromLon;
  MapLibreMapController? _map;
  bool _styleReady = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _run());
  }

  Future<void> _run() async {
    final s = AppScope.read(context);
    setState(() {
      _its = null;
      _error = null;
    });
    if (s.myLat == null) await s.startLocation();
    for (var i = 0; i < 20 && s.myLat == null; i++) {
      await Future<void>.delayed(const Duration(milliseconds: 150));
    }
    if (!mounted) return;
    _fromLat = s.myLat ?? s.city.lat;
    _fromLon = s.myLon ?? s.city.lon;
    s.track('plan_request');
    try {
      final its = await s.api.plan(s.cityId, _fromLat!, _fromLon!, widget.destination.lat, widget.destination.lon);
      if (its.isEmpty) s.track('plan_empty');
      if (!mounted) return;
      setState(() {
        _its = its;
        _selected = 0;
      });
      _draw();
    } catch (e) {
      if (mounted) setState(() => _error = e.toString());
    }
  }

  Future<void> _onStyleLoaded() async {
    final c = _map;
    if (c == null) return;
    try {
      await c.addGeoJsonSource('plan', {'type': 'FeatureCollection', 'features': <Map<String, dynamic>>[]});
      await c.addLineLayer(
        'plan',
        'plan-walk',
        const LineLayerProperties(lineColor: '#757575', lineWidth: 3.5, lineDasharray: [1, 1.5], lineCap: 'round'),
        filter: ['==', ['get', 'kind'], 'walk'],
        enableInteraction: false,
      );
      await c.addLineLayer(
        'plan',
        'plan-ride-casing',
        const LineLayerProperties(lineColor: '#ffffff', lineWidth: 8, lineJoin: 'round', lineCap: 'round'),
        filter: ['==', ['get', 'kind'], 'ride'],
        enableInteraction: false,
      );
      await c.addLineLayer(
        'plan',
        'plan-ride',
        const LineLayerProperties(lineColor: ['get', 'color'], lineWidth: 5, lineJoin: 'round', lineCap: 'round'),
        filter: ['==', ['get', 'kind'], 'ride'],
        enableInteraction: false,
      );
      await c.addCircleLayer(
        'plan',
        'plan-points',
        const CircleLayerProperties(circleColor: ['get', 'color'], circleRadius: 7, circleStrokeColor: '#ffffff', circleStrokeWidth: 3),
        filter: ['==', ['get', 'kind'], 'point'],
        enableInteraction: false,
      );
      _styleReady = true;
      await _draw();
    } catch (_) {}
  }

  Future<void> _draw() async {
    final c = _map;
    final its = _its;
    if (c == null || !_styleReady || its == null || its.isEmpty) return;
    final it = its[_selected.clamp(0, its.length - 1)];
    final features = <Map<String, dynamic>>[];
    var minLat = 90.0, maxLat = -90.0, minLon = 180.0, maxLon = -180.0;
    void extend(double lat, double lon) {
      minLat = math.min(minLat, lat);
      maxLat = math.max(maxLat, lat);
      minLon = math.min(minLon, lon);
      maxLon = math.max(maxLon, lon);
    }

    for (final l in it.legs) {
      var pts = l.polyline.isEmpty ? <List<double>>[] : decodePolyline(l.polyline, precision: l.precision == 0 ? 5 : l.precision);
      if (pts.length < 2) {
        pts = [
          [l.from.lat, l.from.lon],
          [l.to.lat, l.to.lon],
        ];
      }
      for (final p in pts) {
        extend(p[0], p[1]);
      }
      final walk = l.mode == 'WALK';
      features.add({
        'type': 'Feature',
        'properties': {
          'kind': walk ? 'walk' : 'ride',
          'color': l.color.length == 6 ? '#${l.color}' : modeHex(_mode(l.mode)),
        },
        'geometry': {
          'type': 'LineString',
          'coordinates': [for (final p in pts) [p[1], p[0]]],
        },
      });
    }
    for (final p in [
      [_fromLat ?? it.legs.first.from.lat, _fromLon ?? it.legs.first.from.lon, '#1E88E5'],
      [widget.destination.lat, widget.destination.lon, '#D32F2F'],
    ]) {
      features.add({
        'type': 'Feature',
        'properties': {'kind': 'point', 'color': p[2]},
        'geometry': {
          'type': 'Point',
          'coordinates': [p[1], p[0]],
        },
      });
    }
    try {
      await c.setGeoJsonSource('plan', {'type': 'FeatureCollection', 'features': features});
      if (minLat < maxLat || minLon < maxLon) {
        await c.moveCamera(CameraUpdate.newLatLngBounds(
          LatLngBounds(southwest: LatLng(minLat, minLon), northeast: LatLng(maxLat, maxLon)),
          left: 40, right: 40, top: 40, bottom: 40,
        ));
      }
    } catch (_) {}
  }

  static Mode _mode(String m) => switch (m) {
        'SUBWAY' || 'METRO' => Mode.subway,
        'TRAM' => Mode.tram,
        'RAIL' || 'REGIONAL_RAIL' || 'SUBURBAN' => Mode.rail,
        'FERRY' => Mode.ferry,
        _ => Mode.bus,
      };

  @override
  Widget build(BuildContext context) {
    final s = AppScope.of(context);
    final t = Theme.of(context);
    final dark = t.brightness == Brightness.dark;
    final its = _its;
    Widget body;
    if (_error != null) {
      body = EmptyState(icon: Icons.route, title: 'Rota bulunamadı', body: _error, action: FilledButton(onPressed: _run, child: const Text('Tekrar dene')));
    } else if (its == null) {
      body = const Center(child: CircularProgressIndicator());
    } else if (its.isEmpty) {
      body = const EmptyState(
        icon: Icons.route,
        title: 'Önümüzdeki 4 saatte uygun bağlantı yok',
        body: 'Hedef bir duraktan çok uzaksa ya da sefer saatleri bitmişse olabilir.',
      );
    } else {
      body = ListView(padding: const EdgeInsets.only(bottom: 24), children: [
        ListTile(
          dense: true,
          leading: const Icon(Icons.trip_origin, color: Color(0xFF1E88E5)),
          title: Text(s.myLat == null ? 'Şehir merkezinden (konum alınamadı)' : 'Konumundan, şimdi yola çıkarsan'),
        ),
        for (var i = 0; i < its.length; i++)
          _ItineraryCard(
            it: its[i],
            selected: i == _selected,
            onSelect: () {
              setState(() => _selected = i);
              _draw();
            },
          ),
      ]);
    }
    return Scaffold(
      appBar: AppBar(title: Text('→ ${widget.destination.name}', overflow: TextOverflow.ellipsis)),
      body: Column(children: [
        SizedBox(
          height: MediaQuery.of(context).size.height * 0.32,
          child: MapLibreMap(
            styleString: dark ? mapStyleDark : mapStyleLight,
            initialCameraPosition: CameraPosition(target: LatLng(widget.destination.lat, widget.destination.lon), zoom: 12),
            onMapCreated: (c) => _map = c,
            onStyleLoadedCallback: _onStyleLoaded,
            compassEnabled: false,
            rotateGesturesEnabled: false,
            tiltGesturesEnabled: false,
          ),
        ),
        Expanded(child: body),
      ]),
    );
  }
}

class _ItineraryCard extends StatelessWidget {
  const _ItineraryCard({required this.it, required this.selected, required this.onSelect});
  final Itinerary it;
  final bool selected;
  final VoidCallback onSelect;

  String _hm(DateTime d) => '${d.hour.toString().padLeft(2, '0')}:${d.minute.toString().padLeft(2, '0')}';

  @override
  Widget build(BuildContext context) {
    final t = Theme.of(context);
    final mins = (it.durationS / 60).round();
    final notes = it.legs.where((l) => l.note.isNotEmpty).map((l) => l.note).toSet();
    return Card(
      margin: const EdgeInsets.fromLTRB(12, 8, 12, 0),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: selected ? t.colorScheme.primary : Colors.transparent, width: 2),
      ),
      child: ExpansionTile(
        initiallyExpanded: selected,
        onExpansionChanged: (_) => onSelect(),
        shape: const Border(),
        title: Row(children: [
          Text('$mins dk', style: t.textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w700)),
          const SizedBox(width: 12),
          Text('${_hm(it.start)} – ${_hm(it.end)}', style: t.textTheme.bodyMedium),
        ]),
        subtitle: Padding(
          padding: const EdgeInsets.only(top: 6),
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Wrap(spacing: 4, runSpacing: 4, crossAxisAlignment: WrapCrossAlignment.center, children: [
              for (final l in it.legs)
                l.mode == 'WALK'
                    ? const Icon(Icons.directions_walk, size: 18)
                    : LineBadge(text: l.line.isEmpty ? l.mode : l.line, mode: _PlanScreenState._mode(l.mode), color: l.color, dense: true),
            ]),
            const SizedBox(height: 4),
            Text('${it.transfers} aktarma · ${formatDistance(it.walkM)} yürüme${notes.isEmpty ? '' : ' · ${notes.join(', ')}'}',
                style: t.textTheme.bodySmall),
          ]),
        ),
        children: [
          for (final l in it.legs)
            ListTile(
              dense: true,
              leading: l.mode == 'WALK'
                  ? const Icon(Icons.directions_walk)
                  : Icon(modeIcon(_PlanScreenState._mode(l.mode)), color: modeColor(_PlanScreenState._mode(l.mode))),
              title: Text(l.mode == 'WALK' ? '${formatDistance(l.distanceM)} yürü → ${l.to.name}' : '${l.line}${l.headsign.isEmpty ? '' : ' (${l.headsign})'}'),
              subtitle: l.mode == 'WALK'
                  ? null
                  : Text([
                      '${l.from.name} ${_hm(l.start)} → ${l.to.name} ${_hm(l.end)} · ${l.numStops} durak',
                      if (l.note.isNotEmpty) '⚠ ${l.note}',
                    ].join('\n')),
            ),
        ],
      ),
    );
  }
}
