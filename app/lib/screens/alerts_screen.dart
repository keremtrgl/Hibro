import 'package:material_ui/material_ui.dart';

import '../api/models.dart';
import '../state/scope.dart';
import '../widgets/common.dart';
import 'line_screen.dart';

class AlertsScreen extends StatefulWidget {
  const AlertsScreen({super.key});
  @override
  State<AlertsScreen> createState() => _AlertsScreenState();
}

class _AlertsScreenState extends State<AlertsScreen> {
  Future<List<Alert>>? _future;
  String? _city;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final s = AppScope.of(context);
    final key = s.hasCity ? s.cityId : null; // nothing is fetched before a city is chosen
    if (_city != key) {
      _city = key;
      _future = key == null ? null : s.api.alerts(key);
    }
  }

  @override
  Widget build(BuildContext context) {
    final s = AppScope.of(context);
    final t = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('Duyurular')),
      body: RefreshIndicator(
        onRefresh: () async {
          if (!s.hasCity) return;
          setState(() => _future = s.api.alerts(s.cityId));
          await _future;
        },
        child: FutureBuilder<List<Alert>>(
          future: _future,
          builder: (context, snap) {
            if (_future == null) {
              return ListView(children: const [EmptyState(icon: Icons.location_city, title: 'Önce şehrini seç')]);
            }
            if (snap.connectionState != ConnectionState.done) return const Center(child: CircularProgressIndicator());
            if (snap.hasError) {
              return ListView(children: [EmptyState(icon: Icons.cloud_off, title: 'Duyurular alınamadı', body: '${snap.error}')]);
            }
            final list = snap.data!;
            if (list.isEmpty) {
              return ListView(children: [
                EmptyState(
                  icon: Icons.check_circle_outline,
                  title: 'Aktif duyuru yok',
                  body: s.city.features.alerts ? null : '${s.city.name} belediyesi duyuruları açık veri olarak yayınlamıyor.',
                ),
              ]);
            }
            return ListView.separated(
              padding: const EdgeInsets.symmetric(vertical: 8),
              itemCount: list.length,
              separatorBuilder: (_, _) => const Divider(height: 1),
              itemBuilder: (context, i) {
                final a = list[i];
                final range = [
                  if (a.start != null) _fmt(a.start!),
                  if (a.end != null) _fmt(a.end!),
                ].join(' – ');
                return Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
                  child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                    Text(_sentence(a.title), style: t.textTheme.bodyLarge),
                    if (range.isNotEmpty) ...[
                      const SizedBox(height: 4),
                      Text(range, style: t.textTheme.bodySmall?.copyWith(color: t.colorScheme.onSurfaceVariant)),
                    ],
                    if (a.lineIds.isNotEmpty) ...[
                      const SizedBox(height: 8),
                      Wrap(spacing: 6, runSpacing: 6, children: [
                        for (var j = 0; j < a.lineIds.length; j++)
                          InkWell(
                            onTap: () => Navigator.of(context).push(MaterialPageRoute(builder: (_) => LineScreen(lineId: a.lineIds[j]))),
                            child: LineBadge(text: a.lines[j], mode: s.lineById[a.lineIds[j]]?.mode ?? Mode.bus, dense: true),
                          ),
                      ]),
                    ],
                  ]),
                );
              },
            );
          },
        ),
      ),
    );
  }

  static String _two(int n) => n.toString().padLeft(2, '0');
  static String _fmt(DateTime d) {
    final l = d.toLocal();
    return '${_two(l.day)}.${_two(l.month)}.${l.year} ${_two(l.hour)}:${_two(l.minute)}';
  }

  /// Municipal alerts are often ALL CAPS; make them readable.
  static String _sentence(String s) {
    final letters = s.replaceAll(RegExp(r'[^A-Za-zÇĞİÖŞÜçğıöşü]'), '');
    if (letters.isEmpty || letters != letters.toUpperCase()) return s;
    final lower = s.replaceAll('I', 'ı').replaceAll('İ', 'i').toLowerCase();
    return lower[0].toUpperCase() + lower.substring(1);
  }
}
