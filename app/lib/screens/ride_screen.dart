import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:geolocator/geolocator.dart';
import 'package:material_ui/material_ui.dart';

import '../api/models.dart';
import '../state/app_state.dart';
import '../state/scope.dart';
import '../util/geo.dart';
import '../util/text.dart';
import '../widgets/common.dart';
import 'privacy_screen.dart';

/// Removes points within [m] metres of the first and last point so a
/// shared trace never shows where the rider got on or off (home, work).
/// Mirrors server/internal/collect.trimEnds.
List<List<double>> trimTrip(List<List<double>> pts, {double m = 300}) {
  if (pts.isEmpty) return pts;
  final first = pts.first, last = pts.last;
  var start = 0;
  while (start < pts.length && distanceM(first[1], first[2], pts[start][1], pts[start][2]) < m) {
    start++;
  }
  var end = pts.length;
  while (end > start && distanceM(last[1], last[2], pts[end - 1][1], pts[end - 1][2]) < m) {
    end--;
  }
  return pts.sublist(start, end);
}

/// "Bu araca bindim": follows the ride, warns before the rider's stop
/// and, only with permission, shares the trimmed GPS trace.
class RideScreen extends StatefulWidget {
  const RideScreen({super.key, required this.line, required this.direction});
  final Line line;
  final Direction direction;
  @override
  State<RideScreen> createState() => _RideScreenState();
}

class _RideScreenState extends State<RideScreen> {
  static const alarmDistanceM = 350.0;
  static const maxRide = Duration(hours: 2, minutes: 30);

  final List<List<double>> _points = []; // [unix s, lat, lon, accuracy m, speed m/s]
  StreamSubscription<Position>? _sub;
  final _started = DateTime.now();
  Timer? _ticker;
  StopRef? _dest;
  bool _alarmFired = false;
  double _distanceM = 0;
  double _speed = 0;
  String? _error;
  bool _ending = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _start());
    _ticker = Timer.periodic(const Duration(seconds: 1), (_) {
      if (!mounted) return;
      if (DateTime.now().difference(_started) > maxRide) {
        _end(auto: true);
      } else {
        setState(() {});
      }
    });
  }

  @override
  void dispose() {
    _sub?.cancel();
    _ticker?.cancel();
    super.dispose();
  }

  LocationSettings get _settings {
    if (!kIsWeb && defaultTargetPlatform == TargetPlatform.android) {
      // A foreground service keeps tracking (and the stop alarm) alive
      // while the screen is off. Android shows a notification for it.
      return AndroidSettings(
        accuracy: LocationAccuracy.high,
        distanceFilter: 10,
        intervalDuration: const Duration(seconds: 3),
        foregroundNotificationConfig: const ForegroundNotificationConfig(
          notificationTitle: 'Hibro yolculuk modu',
          notificationText: 'İneceğin durağı takip ediyoruz. Bitirmek için uygulamada "İndim"e dokun.',
          notificationChannelName: 'Yolculuk modu',
          enableWakeLock: true,
        ),
      );
    }
    if (!kIsWeb && defaultTargetPlatform == TargetPlatform.iOS) {
      return AppleSettings(
        accuracy: LocationAccuracy.high,
        distanceFilter: 10,
        activityType: ActivityType.automotiveNavigation,
        pauseLocationUpdatesAutomatically: false,
        showBackgroundLocationIndicator: true,
        allowBackgroundLocationUpdates: true,
      );
    }
    return const LocationSettings(accuracy: LocationAccuracy.high, distanceFilter: 10);
  }

  Future<void> _start() async {
    final s = AppScope.read(context);
    s.track('ride_start');
    if (!await s.startLocation()) {
      setState(() => _error = 'Yolculuk modu için konum izni gerekli.');
      return;
    }
    _sub = Geolocator.getPositionStream(locationSettings: _settings).listen(_onPos, onError: (_) {
      if (mounted) setState(() => _error = 'Konum alınamıyor.');
    });
  }

  void _onPos(Position p) {
    if (p.accuracy > 50) return; // too noisy to be useful
    if (_points.isNotEmpty) {
      final last = _points.last;
      _distanceM += distanceM(last[1], last[2], p.latitude, p.longitude);
    }
    if (_points.length < 5000) {
      _points.add([
        p.timestamp.millisecondsSinceEpoch / 1000.0,
        p.latitude,
        p.longitude,
        p.accuracy,
        p.speed < 0 ? 0 : p.speed,
      ]);
    }
    _speed = p.speed < 0 ? 0 : p.speed;
    final d = _dest;
    if (d != null && !_alarmFired && distanceM(p.latitude, p.longitude, d.lat, d.lon) <= alarmDistanceM) {
      _alarmFired = true;
      _alarm();
    }
    if (mounted) setState(() {});
  }

  Future<void> _alarm() async {
    AppScope.read(context).track('ride_alarm');
    SystemSound.play(SystemSoundType.alert);
    for (var i = 0; i < 4; i++) {
      HapticFeedback.heavyImpact();
      await Future<void>.delayed(const Duration(milliseconds: 400));
    }
    if (!mounted) return;
    await showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        icon: const Icon(Icons.notifications_active, size: 40),
        title: const Text('İneceğin durak yaklaşıyor'),
        content: Text('${_dest!.name} durağına yaklaşık ${alarmDistanceM.round()} m kaldı.'),
        actions: [FilledButton(onPressed: () => Navigator.pop(context), child: const Text('Tamam'))],
      ),
    );
  }

  double? get _toDestM {
    final d = _dest;
    if (d == null || _points.isEmpty) return null;
    return distanceM(_points.last[1], _points.last[2], d.lat, d.lon);
  }

  Future<void> _pickDestination() async {
    final s = AppScope.read(context);
    final picked = await showModalBottomSheet<StopRef>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (_) => _DestinationPicker(direction: widget.direction, state: s, from: _points.isEmpty ? null : _points.last),
    );
    if (picked != null) {
      setState(() {
        _dest = picked;
        _alarmFired = false;
      });
    }
  }

  Future<void> _end({bool auto = false}) async {
    if (_ending) return;
    _ending = true;
    await _sub?.cancel();
    _sub = null;
    _ticker?.cancel();
    if (!mounted) return;
    final s = AppScope.read(context);
    s.track('ride_end');
    final trimmed = trimTrip(_points);
    var share = s.shareTrips;
    if (!share && trimmed.length >= 10) {
      share = await _askShare(s) ?? false;
    }
    String msg = auto ? 'Yolculuk modu süre dolduğu için kapandı.' : 'Yolculuk bitti.';
    if (share && trimmed.length >= 10) {
      try {
        await s.api.uploadTrip(
          s.cityId,
          session: s.dailySession,
          lineId: widget.line.id,
          direction: widget.direction.index,
          points: trimmed,
          app: appVersion,
        );
        msg = 'Teşekkürler! Yolculuğun ${widget.line.short} hattının varış tahminlerini iyileştirecek.';
      } catch (e) {
        msg = 'Yolculuk verisi gönderilemedi: $e';
      }
    }
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));
    Navigator.of(context).pop();
  }

  Future<bool?> _askShare(AppState s) {
    var always = false;
    return showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => AlertDialog(
          title: const Text('Bu yolculuğu anonim paylaşır mısın?'),
          content: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
            const Text('Başı ve sonu silinmiş GPS izi, kimliksiz olarak gönderilir ve '
                'otobüslerin gerçek varış sürelerini hesaplamamıza yardım eder.'),
            CheckboxListTile(
              contentPadding: EdgeInsets.zero,
              value: always,
              onChanged: (v) => setLocal(() => always = v ?? false),
              title: const Text('Bundan sonra hep paylaş'),
            ),
            TextButton(
              onPressed: () => Navigator.of(context).push(MaterialPageRoute(builder: (_) => const PrivacyScreen())),
              child: const Text('Nasıl korunuyor?'),
            ),
          ]),
          actions: [
            TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Bu sefer değil')),
            FilledButton(
              onPressed: () {
                if (always) s.setConsent(trips: true);
                Navigator.pop(context, true);
              },
              child: const Text('Paylaş'),
            ),
          ],
        ),
      ),
    );
  }

  String _elapsed() {
    final d = DateTime.now().difference(_started);
    final m = d.inMinutes, sec = d.inSeconds % 60;
    return '${m.toString().padLeft(2, '0')}:${sec.toString().padLeft(2, '0')}';
  }

  @override
  Widget build(BuildContext context) {
    final t = Theme.of(context);
    final to = _toDestM;
    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, _) async {
        if (didPop) return;
        final end = await showDialog<bool>(
          context: context,
          builder: (context) => AlertDialog(
            title: const Text('Yolculuk bitsin mi?'),
            actions: [
              TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Devam et')),
              FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('Bitir')),
            ],
          ),
        );
        if (end == true) _end();
      },
      child: Scaffold(
        appBar: AppBar(
          titleSpacing: 0,
          title: Row(children: [
            LineBadge(text: widget.line.short, mode: widget.line.mode, color: widget.line.color, textColor: widget.line.textColor),
            const SizedBox(width: 10),
            Expanded(child: Text('→ ${widget.direction.headsign}', overflow: TextOverflow.ellipsis)),
          ]),
        ),
        body: ListView(padding: const EdgeInsets.all(16), children: [
          if (_error != null)
            Card(
              color: t.colorScheme.errorContainer,
              child: ListTile(leading: const Icon(Icons.location_off), title: Text(_error!)),
            ),
          Row(children: [
            _Stat(label: 'Süre', value: _elapsed()),
            _Stat(label: 'Yol', value: formatDistance(_distanceM.round())),
            _Stat(label: 'Hız', value: '${(_speed * 3.6).round()} km/s'),
          ]),
          const SizedBox(height: 20),
          Card(
            child: ListTile(
              leading: Icon(_dest == null ? Icons.flag_outlined : Icons.flag, color: t.colorScheme.primary),
              title: Text(_dest?.name ?? 'İneceğin durağı seç'),
              subtitle: Text(_dest == null
                  ? 'Yaklaşınca titreşim ve sesle haber veririz.'
                  : to == null
                      ? 'Konum bekleniyor…'
                      : _alarmFired
                          ? 'Yaklaştın! Kalan: ${formatDistance(to.round())}'
                          : 'Kalan: ${formatDistance(to.round())} · ${alarmDistanceM.round()} m kala uyarırız'),
              trailing: const Icon(Icons.edit_location_alt_outlined),
              onTap: _pickDestination,
            ),
          ),
          const SizedBox(height: 12),
          ListTile(
            leading: Icon(AppScope.of(context).shareTrips ? Icons.cloud_upload_outlined : Icons.cloud_off_outlined),
            title: Text(AppScope.of(context).shareTrips ? 'Yolculuk verisi paylaşılacak' : 'Yolculuk verisi paylaşılmayacak'),
            subtitle: Text('${_points.length} GPS noktası · başı ve sonu silinir'),
            onTap: () => Navigator.of(context).push(MaterialPageRoute(builder: (_) => const PrivacyScreen())),
          ),
          if (!kIsWeb && defaultTargetPlatform == TargetPlatform.android)
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16),
              child: Text(
                'Ekran kapalıyken de takip sürer; bildirim çubuğunda "Hibro yolculuk modu" görünür.',
                style: t.textTheme.bodySmall?.copyWith(color: t.colorScheme.onSurfaceVariant),
              ),
            ),
          const SizedBox(height: 24),
          FilledButton.icon(
            style: FilledButton.styleFrom(minimumSize: const Size.fromHeight(52)),
            onPressed: _ending ? null : () => _end(),
            icon: const Icon(Icons.logout),
            label: const Text('İndim, yolculuğu bitir'),
          ),
        ]),
      ),
    );
  }
}

class _Stat extends StatelessWidget {
  const _Stat({required this.label, required this.value});
  final String label, value;
  @override
  Widget build(BuildContext context) {
    final t = Theme.of(context);
    return Expanded(
      child: Column(children: [
        Text(value, style: t.textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.w700, fontFeatures: const [FontFeature.tabularFigures()])),
        Text(label, style: t.textTheme.labelMedium?.copyWith(color: t.colorScheme.onSurfaceVariant)),
      ]),
    );
  }
}

class _DestinationPicker extends StatefulWidget {
  const _DestinationPicker({required this.direction, required this.state, this.from});
  final Direction direction;
  final AppState state;
  final List<double>? from;
  @override
  State<_DestinationPicker> createState() => _DestinationPickerState();
}

class _DestinationPickerState extends State<_DestinationPicker> {
  String _q = '';

  @override
  Widget build(BuildContext context) {
    final stops = widget.direction.stops;
    List<StopRef> list;
    if (stops.isNotEmpty) {
      final fq = fold(_q);
      list = stops.where((s) => fq.isEmpty || fold(s.name).contains(fq)).toList();
    } else {
      // No published stop order for this line: search all stops instead.
      list = widget.state.search(_q).stops.map((s) => StopRef(s.id, s.name, s.lat, s.lon)).toList();
      final from = widget.from;
      if (from != null) {
        list.sort((a, b) => distanceM(from[1], from[2], a.lat, a.lon).compareTo(distanceM(from[1], from[2], b.lat, b.lon)));
      }
    }
    return SizedBox(
      height: MediaQuery.of(context).size.height * 0.75,
      child: Column(children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 0, 16, 8),
          child: TextField(
            autofocus: stops.isEmpty,
            decoration: const InputDecoration(prefixIcon: Icon(Icons.search), hintText: 'Durak ara', border: OutlineInputBorder()),
            onChanged: (v) => setState(() => _q = v),
          ),
        ),
        if (stops.isEmpty && _q.isEmpty)
          const Padding(padding: EdgeInsets.all(24), child: Text('Bu hat için durak sırası yayınlanmıyor. İneceğin durağın adını yaz.')),
        Expanded(
          child: ListView.builder(
            itemCount: list.length,
            itemBuilder: (context, i) {
              final st = list[i];
              final from = widget.from;
              return ListTile(
                leading: const Icon(Icons.place_outlined),
                title: Text(st.name),
                trailing: from == null ? null : Text(formatDistance(distanceM(from[1], from[2], st.lat, st.lon).round())),
                onTap: () => Navigator.pop(context, st),
              );
            },
          ),
        ),
      ]),
    );
  }
}
