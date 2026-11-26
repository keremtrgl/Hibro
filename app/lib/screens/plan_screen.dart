import 'package:material_ui/material_ui.dart';

import '../api/models.dart';
import '../state/scope.dart';
import '../util/style.dart';
import '../util/text.dart';
import '../widgets/common.dart';

/// Journey planning from the user's location to a stop (needs MOTIS on the server).
class PlanScreen extends StatefulWidget {
  const PlanScreen({super.key, required this.destination});
  final Stop destination;
  @override
  State<PlanScreen> createState() => _PlanScreenState();
}

class _PlanScreenState extends State<PlanScreen> {
  Future<List<Itinerary>>? _future;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _run());
  }

  Future<void> _run() async {
    final s = AppScope.read(context);
    if (s.myLat == null) await s.startLocation();
    for (var i = 0; i < 20 && s.myLat == null; i++) {
      await Future<void>.delayed(const Duration(milliseconds: 150));
    }
    if (!mounted) return;
    final lat = s.myLat ?? s.city.lat, lon = s.myLon ?? s.city.lon;
    s.track('plan_request');
    setState(() => _future = s.api.plan(s.cityId, lat, lon, widget.destination.lat, widget.destination.lon).then((its) {
          if (its.isEmpty) s.track('plan_empty');
          return its;
        }));
  }

  @override
  Widget build(BuildContext context) {
    final s = AppScope.of(context);
    final t = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: Text('→ ${widget.destination.name}')),
      body: FutureBuilder<List<Itinerary>>(
        future: _future,
        builder: (context, snap) {
          if (_future == null || snap.connectionState != ConnectionState.done) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snap.hasError) {
            return EmptyState(icon: Icons.route, title: 'Rota bulunamadı', body: '${snap.error}', action: FilledButton(onPressed: _run, child: const Text('Tekrar dene')));
          }
          final its = snap.data!;
          if (its.isEmpty) {
            return const EmptyState(
              icon: Icons.route,
              title: 'Önümüzdeki 4 saatte uygun bağlantı yok',
              body: 'Hedef bir duraktan çok uzaksa ya da sefer saatleri bitmişse olabilir.',
            );
          }
          return ListView(children: [
            ListTile(
              leading: const Icon(Icons.trip_origin),
              title: Text(s.myLat == null ? 'Şehir merkezinden (konum alınamadı)' : 'Konumundan'),
              subtitle: Text('→ ${widget.destination.name} · şimdi yola çıkarsan'),
            ),
            for (final it in its) _ItineraryCard(it: it),
          ]);
        },
      ),
      backgroundColor: t.colorScheme.surfaceContainerLowest,
    );
  }
}

class _ItineraryCard extends StatelessWidget {
  const _ItineraryCard({required this.it});
  final Itinerary it;

  String _hm(DateTime d) => '${d.hour.toString().padLeft(2, '0')}:${d.minute.toString().padLeft(2, '0')}';

  @override
  Widget build(BuildContext context) {
    final t = Theme.of(context);
    final mins = (it.durationS / 60).round();
    return Card(
      margin: const EdgeInsets.fromLTRB(12, 8, 12, 0),
      child: ExpansionTile(
        title: Row(children: [
          Text('$mins dk', style: t.textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w700)),
          const SizedBox(width: 12),
          Text('${_hm(it.start)} – ${_hm(it.end)}', style: t.textTheme.bodyMedium),
        ]),
        subtitle: Padding(
          padding: const EdgeInsets.only(top: 6),
          child: Wrap(spacing: 4, runSpacing: 4, crossAxisAlignment: WrapCrossAlignment.center, children: [
            for (final l in it.legs)
              l.mode == 'WALK'
                  ? const Icon(Icons.directions_walk, size: 18)
                  : LineBadge(text: l.line.isEmpty ? l.mode : l.line, mode: _mode(l.mode), color: l.color, dense: true),
            Text('  ${it.transfers} aktarma · ${formatDistance(it.walkM)} yürüme', style: t.textTheme.bodySmall),
          ]),
        ),
        children: [
          for (final l in it.legs)
            ListTile(
              dense: true,
              leading: l.mode == 'WALK' ? const Icon(Icons.directions_walk) : Icon(modeIcon(_mode(l.mode)), color: modeColor(_mode(l.mode))),
              title: Text(l.mode == 'WALK' ? '${formatDistance(l.distanceM)} yürü → ${l.to.name}' : '${l.line} ${l.headsign.isEmpty ? '' : '(${l.headsign})'}'),
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

  static Mode _mode(String m) => switch (m) {
        'SUBWAY' || 'METRO' => Mode.subway,
        'TRAM' => Mode.tram,
        'RAIL' || 'REGIONAL_RAIL' || 'SUBURBAN' => Mode.rail,
        'FERRY' => Mode.ferry,
        _ => Mode.bus,
      };
}
