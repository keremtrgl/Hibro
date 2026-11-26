import 'package:material_ui/material_ui.dart';

import '../state/scope.dart';
import '../util/style.dart';
import '../widgets/common.dart';
import '../api/models.dart';
import 'line_screen.dart';
import 'plan_screen.dart';
import 'stop_screen.dart';

/// Instant, offline search over the downloaded city pack.
class SearchScreen extends StatefulWidget {
  const SearchScreen({super.key, this.forRoute = false});

  /// When true the user is choosing a destination: tapping a stop plans
  /// a route to it instead of opening the stop.
  final bool forRoute;
  @override
  State<SearchScreen> createState() => _SearchScreenState();
}

class _SearchScreenState extends State<SearchScreen> {
  final _q = TextEditingController();
  bool _trackedEmpty = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => AppScope.read(context).track('screen_search'));
  }

  @override
  void dispose() {
    _q.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final s = AppScope.of(context);
    final t = Theme.of(context);
    final res = s.search(_q.text);
    if (!_trackedEmpty && _q.text.trim().length >= 3 && res.lines.isEmpty && res.stops.isEmpty) {
      _trackedEmpty = true; // counts searches that found nothing: tells us what data is missing
      s.track('search_empty');
    }
    return Scaffold(
      appBar: AppBar(
        titleSpacing: 0,
        title: TextField(
          controller: _q,
          autofocus: true,
          textInputAction: TextInputAction.search,
          decoration: InputDecoration(
            hintText: widget.forRoute ? 'Nereye? Durak adı yaz' : '${s.city.name}: durak adı, durak no veya hat',
            border: InputBorder.none,
          ),
          onChanged: (_) => setState(() {}),
        ),
        actions: [
          if (_q.text.isNotEmpty)
            IconButton(icon: const Icon(Icons.close), onPressed: () => setState(_q.clear)),
        ],
      ),
      body: _q.text.trim().isEmpty
          ? EmptyState(
              icon: widget.forRoute ? Icons.directions : Icons.search,
              title: widget.forRoute ? 'Nereye gitmek istiyorsun?' : 'Ne arıyorsun?',
              body: widget.forRoute ? 'Varış durağını yaz; rota konumundan hesaplanır.' : 'Örnek: "Konak", "Üçyol", "5" veya durak numarası.',
            )
          : (res.lines.isEmpty && res.stops.isEmpty)
              ? const EmptyState(icon: Icons.search_off, title: 'Sonuç bulunamadı')
              : ListView(children: [
                  if (res.lines.isNotEmpty && !widget.forRoute) _section(t, 'Hatlar'),
                  for (final l in widget.forRoute ? const <Line>[] : res.lines)
                    ListTile(
                      leading: LineBadge(text: l.short, mode: l.mode, color: l.color, textColor: l.textColor),
                      title: Text(l.name, maxLines: 1, overflow: TextOverflow.ellipsis),
                      subtitle: Text([modeName(l.mode), if (l.night) 'Gece hattı', if (l.hasLive) 'Canlı takip'].join(' · ')),
                      onTap: () => Navigator.of(context).push(MaterialPageRoute(builder: (_) => LineScreen(lineId: l.id))),
                    ),
                  if (res.stops.isNotEmpty) _section(t, 'Duraklar'),
                  for (final st in res.stops)
                    ListTile(
                      leading: ModeAvatar(mode: st.mode),
                      title: Text(st.name),
                      subtitle: Text([modeName(st.mode), if (st.district.isNotEmpty) st.district, if (st.code.isNotEmpty) 'No ${st.code}'].join(' · ')),
                      trailing: !s.city.features.planner || widget.forRoute
                          ? null
                          : IconButton(
                              tooltip: 'Buraya git',
                              icon: const Icon(Icons.directions),
                              onPressed: () => Navigator.of(context).push(MaterialPageRoute(builder: (_) => PlanScreen(destination: st))),
                            ),
                      onTap: () => Navigator.of(context).push(MaterialPageRoute(
                          builder: (_) => widget.forRoute ? PlanScreen(destination: st) : StopScreen(stop: st))),
                    ),
                ]),
    );
  }

  Widget _section(ThemeData t, String title) => Padding(
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 4),
        child: Text(title, style: t.textTheme.labelLarge?.copyWith(color: t.colorScheme.primary)),
      );
}

