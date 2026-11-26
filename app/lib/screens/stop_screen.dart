import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../api/models.dart';
import '../state/scope.dart';
import '../util/style.dart';
import '../widgets/common.dart';
import 'line_screen.dart';
import 'plan_screen.dart';

class StopScreen extends StatefulWidget {
  const StopScreen({super.key, required this.stop});
  final Stop stop;
  @override
  State<StopScreen> createState() => _StopScreenState();
}

class _StopScreenState extends State<StopScreen> with WidgetsBindingObserver {
  ArrivalsResult? _res;
  String? _error;
  bool _loading = false;
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _timer?.cancel();
    super.dispose();
  }

  // Stop polling while the app is in the background: no wasted data or battery.
  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      _load();
    } else {
      _timer?.cancel();
    }
  }

  Future<void> _load() async {
    if (_loading || !mounted) return;
    final s = AppScope.read(context);
    setState(() => _loading = true);
    try {
      final r = await s.api.arrivals(s.cityId, widget.stop.id);
      if (!mounted) return;
      setState(() {
        _res = r;
        _error = null;
      });
    } catch (e) {
      if (mounted) setState(() => _error = e.toString());
    } finally {
      if (mounted) {
        setState(() => _loading = false);
        _timer?.cancel();
        final live = _res?.live == true || s.city.features.liveArrivals && widget.stop.mode == Mode.bus;
        _timer = Timer(Duration(seconds: live ? 15 : 60), _load);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final s = AppScope.of(context);
    final t = Theme.of(context);
    final st = widget.stop;
    final fav = s.isFavorite('stop', st.id);
    final lines = st.lines.map((id) => s.lineById[id]).whereType<Line>().toList();
    final subtitle = [modeName(st.mode), if (st.district.isNotEmpty) st.district, if (st.code.isNotEmpty) 'Durak no ${st.code}'].join(' · ');

    return Scaffold(
      appBar: AppBar(
        title: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text(st.name, maxLines: 1, overflow: TextOverflow.ellipsis),
          Text(subtitle, style: t.textTheme.bodySmall),
        ]),
        actions: [
          IconButton(
            tooltip: fav ? 'Favorilerden çıkar' : 'Favorilere ekle',
            icon: Icon(fav ? Icons.star : Icons.star_border, color: fav ? Colors.amber : null),
            onPressed: () => s.toggleFavorite('stop', st.id, st.name, subtitle),
          ),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: _load,
        child: ListView(children: [
          if (lines.isNotEmpty)
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 0),
              child: Wrap(spacing: 6, runSpacing: 6, children: [
                for (final l in lines)
                  InkWell(
                    borderRadius: BorderRadius.circular(6),
                    onTap: () => _openLine(l.id),
                    child: LineBadge(text: l.short, mode: l.mode, color: l.color, textColor: l.textColor),
                  ),
              ]),
            ),
          if (s.city.features.planner)
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 0),
              child: OutlinedButton.icon(
                onPressed: () => Navigator.of(context).push(MaterialPageRoute(builder: (_) => PlanScreen(destination: st))),
                icon: const Icon(Icons.directions),
                label: const Text('Buraya nasıl giderim?'),
              ),
            ),
          _header(t),
          ..._body(t),
          const SizedBox(height: 32),
        ]),
      ),
    );
  }

  Widget _header(ThemeData t) {
    final r = _res;
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 4),
      child: Row(children: [
        Text(r?.live == true ? 'Canlı varışlar' : 'Sıradaki seferler', style: t.textTheme.titleMedium),
        const SizedBox(width: 8),
        if (r?.live == true) const LiveDot(),
        const Spacer(),
        if (_loading) const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2)),
      ]),
    );
  }

  List<Widget> _body(ThemeData t) {
    final r = _res;
    if (r == null && _error == null) {
      return const [Padding(padding: EdgeInsets.all(40), child: Center(child: CircularProgressIndicator()))];
    }
    if (r == null) {
      return [EmptyState(icon: Icons.cloud_off, title: 'Varış bilgisi alınamadı', body: _error, action: FilledButton(onPressed: _load, child: const Text('Tekrar dene')))];
    }
    final out = <Widget>[];
    if (r.degraded) {
      out.add(Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
        child: Card(
          color: t.colorScheme.tertiaryContainer,
          child: ListTile(
            leading: const Icon(Icons.info_outline),
            title: const Text('Belediyenin canlı sistemi şu an yanıt vermiyor'),
            subtitle: const Text('Birkaç saniye içinde tekrar denenecek.'),
          ),
        ),
      ));
    }
    if (r.arrivals.isEmpty && !r.degraded) {
      out.add(EmptyState(
        icon: Icons.schedule,
        title: r.live || widget.stop.mode == Mode.bus ? 'Yaklaşan araç yok' : 'Önümüzdeki 2 saatte sefer yok',
        body: widget.stop.mode == Mode.bus ? 'Hat sayfalarından sefer saatlerine bakabilirsin.' : null,
      ));
    }
    for (final a in r.arrivals) {
      out.add(_ArrivalTile(a: a, onTap: () => _openLine(a.lineId)));
    }
    if (r.arrivals.isNotEmpty && !r.live) {
      out.add(Padding(
        padding: const EdgeInsets.all(16),
        child: Text('Saatler belediyenin planlı tarifesidir, canlı konum değildir.', style: t.textTheme.bodySmall?.copyWith(color: t.colorScheme.onSurfaceVariant)),
      ));
    }
    return out;
  }

  void _openLine(String id) {
    Navigator.of(context).push(MaterialPageRoute(builder: (_) => LineScreen(lineId: id)));
  }
}

class _ArrivalTile extends StatelessWidget {
  const _ArrivalTile({required this.a, required this.onTap});
  final Arrival a;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final t = Theme.of(context);
    final details = <String>[
      if (a.stopsAway > 0) '${a.stopsAway} durak önce',
      if (a.plate.isNotEmpty) a.plate,
      if (!a.realtime) 'Planlı ${a.time}',
      if (a.isLast) 'Son sefer',
    ];
    return ListTile(
      onTap: onTap,
      leading: LineBadge(text: a.line, mode: a.mode, color: a.color),
      title: Text(a.headsign.isEmpty ? a.line : a.headsign, maxLines: 1, overflow: TextOverflow.ellipsis),
      subtitle: Row(children: [
        if (a.accessible == true) ...[
          Icon(Icons.accessible, size: 16, color: t.colorScheme.primary),
          const SizedBox(width: 4),
        ],
        if (a.bicycle == true) ...[
          Icon(Icons.pedal_bike, size: 16, color: t.colorScheme.primary),
          const SizedBox(width: 4),
        ],
        if (a.electric == true) ...[
          Icon(Icons.electric_bolt, size: 16, color: t.colorScheme.primary),
          const SizedBox(width: 4),
        ],
        Expanded(child: Text(details.join(' · '), maxLines: 1, overflow: TextOverflow.ellipsis)),
      ]),
      trailing: Column(mainAxisAlignment: MainAxisAlignment.center, crossAxisAlignment: CrossAxisAlignment.end, children: [
        Text(
          a.minutes <= 0 ? 'Şimdi' : '${a.minutes} dk',
          style: t.textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.w700,
            color: a.realtime ? const Color(0xFF2E7D32) : t.colorScheme.onSurface,
          ),
        ),
        if (a.realtime) Text(a.time, style: t.textTheme.labelSmall),
      ]),
    );
  }
}
