import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../state/scope.dart';
import '../util/style.dart';
import '../widgets/common.dart';
import '../api/models.dart';
import 'line_screen.dart';
import 'plan_screen.dart';
import 'stop_screen.dart';

/// Instant, offline search over the downloaded city pack, plus online
/// place/address search ("Kocaeli Üniversitesi", "Kefken") for routing.
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
  Timer? _debounce;
  String _placesFor = '';
  List<Place> _places = const [];
  bool _placesLoading = false;

  void _onChanged(String _) {
    setState(() {});
    _debounce?.cancel();
    final q = _q.text.trim();
    if (q.runes.length < 3) {
      setState(() {
        _places = const [];
        _placesFor = '';
        _placesLoading = false;
      });
      return;
    }
    _debounce = Timer(const Duration(milliseconds: 450), () => _searchPlaces(q));
  }

  Future<void> _searchPlaces(String q) async {
    if (q == _placesFor) return;
    final s = AppScope.read(context);
    setState(() => _placesLoading = true);
    List<Place> res = const [];
    try {
      res = await s.api.places(s.cityId, q);
    } catch (_) {}
    if (!mounted || _q.text.trim() != q) return;
    setState(() {
      _places = res;
      _placesFor = q;
      _placesLoading = false;
    });
  }

  void _goPlace(Place p) {
    AppScope.read(context).track('place_route');
    Navigator.of(context).push(MaterialPageRoute(builder: (_) => PlanScreen(destination: p.toStop())));
  }

  static IconData _placeIcon(String kind) => switch (kind) {
        'Üniversite' || 'Yüksekokul' || 'Okul' => Icons.school_outlined,
        'Havalimanı' => Icons.flight_outlined,
        'İstasyon' || 'Otogar' || 'İskele' => Icons.directions_transit_outlined,
        'Hastane' => Icons.local_hospital_outlined,
        'Köy' || 'Mahalle' || 'Semt' || 'İlçe' || 'Kasaba' => Icons.holiday_village_outlined,
        'Plaj' || 'Park' => Icons.park_outlined,
        'AVM' || 'Mağaza' => Icons.storefront_outlined,
        'Cami' => Icons.mosque_outlined,
        _ => Icons.place_outlined,
      };

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => AppScope.read(context).track('screen_search'));
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _q.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final s = AppScope.of(context);
    final t = Theme.of(context);
    final res = s.search(_q.text);
    final places = _placesFor == _q.text.trim() ? _places : const <Place>[];
    final nothing = res.lines.isEmpty && res.stops.isEmpty && places.isEmpty;
    if (!_trackedEmpty && !_placesLoading && _placesFor.isNotEmpty && nothing) {
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
            hintText: widget.forRoute ? 'Nereye? Yer, adres veya durak' : '${s.city.name}: yer, durak veya hat',
            border: InputBorder.none,
          ),
          onChanged: _onChanged,
        ),
        actions: [
          if (_q.text.isNotEmpty)
            IconButton(icon: const Icon(Icons.close), onPressed: () {
              _q.clear();
              _onChanged('');
            }),
        ],
      ),
      body: _q.text.trim().isEmpty
          ? EmptyState(
              icon: widget.forRoute ? Icons.directions : Icons.search,
              title: widget.forRoute ? 'Nereye gitmek istiyorsun?' : 'Ne arıyorsun?',
              body: widget.forRoute
                  ? 'Gideceğin yeri yaz (ör. üniversite, mahalle, AVM); rota konumundan hesaplanır.'
                  : 'Örnek: "Kocaeli Üniversitesi", "Konak", "5" veya durak numarası.',
            )
          : nothing && !_placesLoading && _placesFor == _q.text.trim()
              ? const EmptyState(icon: Icons.search_off, title: 'Sonuç bulunamadı')
              : ListView(children: [
                  if (places.isNotEmpty || _placesLoading) _section(t, 'Yerler ve adresler'),
                  if (_placesLoading && places.isEmpty)
                    const ListTile(dense: true, leading: SizedBox(width: 24, height: 24, child: CircularProgressIndicator(strokeWidth: 2)), title: Text('Aranıyor…')),
                  for (final p in places.take(widget.forRoute ? 8 : 4))
                    ListTile(
                      leading: CircleAvatar(child: Icon(_placeIcon(p.kind))),
                      title: Text(p.name, maxLines: 1, overflow: TextOverflow.ellipsis),
                      subtitle: Text([if (p.kind.isNotEmpty) p.kind, if (p.detail.isNotEmpty) p.detail].join(' · '), maxLines: 1, overflow: TextOverflow.ellipsis),
                      trailing: const Icon(Icons.directions),
                      onTap: () => _goPlace(p),
                    ),
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

