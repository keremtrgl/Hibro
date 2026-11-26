import 'package:material_ui/material_ui.dart';

import '../state/scope.dart';
import '../widgets/common.dart';
import 'line_screen.dart';
import 'stop_screen.dart';

class FavoritesScreen extends StatelessWidget {
  const FavoritesScreen({super.key});
  @override
  Widget build(BuildContext context) {
    final s = AppScope.of(context);
    final favs = s.cityFavorites;
    return Scaffold(
      appBar: AppBar(title: const Text('Favoriler')),
      body: favs.isEmpty
          ? const EmptyState(
              icon: Icons.star_border,
              title: 'Henüz favori yok',
              body: 'Durak veya hat sayfasındaki yıldıza dokun. Favoriler yalnızca bu telefonda saklanır.',
            )
          : ListView(children: [
              for (final f in favs)
                Dismissible(
                  key: ValueKey('${f.kind}:${f.id}'),
                  direction: DismissDirection.endToStart,
                  background: Container(
                    alignment: Alignment.centerRight,
                    padding: const EdgeInsets.only(right: 20),
                    color: Theme.of(context).colorScheme.errorContainer,
                    child: const Icon(Icons.delete_outline),
                  ),
                  onDismissed: (_) => s.toggleFavorite(f.kind, f.id, f.title, f.subtitle),
                  child: ListTile(
                    leading: Icon(f.kind == 'stop' ? Icons.place_outlined : Icons.route_outlined),
                    title: Text(f.title),
                    subtitle: Text(f.subtitle),
                    trailing: const Icon(Icons.chevron_right),
                    onTap: () {
                      if (f.kind == 'stop') {
                        final st = s.stopById[f.id];
                        if (st != null) Navigator.of(context).push(MaterialPageRoute(builder: (_) => StopScreen(stop: st)));
                      } else {
                        Navigator.of(context).push(MaterialPageRoute(builder: (_) => LineScreen(lineId: f.id)));
                      }
                    },
                  ),
                ),
            ]),
    );
  }
}
