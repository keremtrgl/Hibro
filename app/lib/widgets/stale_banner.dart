import 'package:material_ui/material_ui.dart';

import '../state/scope.dart';

/// Warns that the city's timetable data may be outdated. Shown on stop
/// and line screens; renders nothing for cities with current data.
class StaleBanner extends StatelessWidget {
  const StaleBanner({super.key, this.id});

  /// Stop or line id: the banner only shows when its own source is
  /// outdated or unofficial (İZBAN's old timetable must not mark metro).
  final String? id;

  @override
  Widget build(BuildContext context) {
    final note = AppScope.of(context).city.staleNote(id);
    if (note == null) return const SizedBox.shrink();
    final t = Theme.of(context);
    return Material(
      color: t.colorScheme.tertiaryContainer,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 10, 16, 10),
        child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Icon(Icons.history, size: 20, color: t.colorScheme.onTertiaryContainer),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              '$note Bir hata görürsen "Sorun bildir" ile haber ver.',
              style: t.textTheme.bodySmall?.copyWith(color: t.colorScheme.onTertiaryContainer),
            ),
          ),
        ]),
      ),
    );
  }
}
