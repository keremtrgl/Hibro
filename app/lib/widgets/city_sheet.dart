import 'package:material_ui/material_ui.dart';

import '../state/scope.dart';

/// Lets the user pick a city by hand. [reason] explains why we ask
/// (location off, or the user's city is not supported yet).
Future<void> showCitySheet(BuildContext context, {String? reason}) {
  return showModalBottomSheet<void>(
    context: context,
    showDragHandle: true,
    isScrollControlled: true,
    builder: (context) {
      final s = AppScope.of(context);
      final t = Theme.of(context);
      return SafeArea(
        child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 4),
            child: Text('Şehrini seç', style: t.textTheme.titleLarge),
          ),
          if (reason != null)
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 0, 20, 8),
              child: Text(reason, style: t.textTheme.bodyMedium?.copyWith(color: t.colorScheme.onSurfaceVariant)),
            ),
          for (final c in s.cities)
            ListTile(
              leading: const Icon(Icons.location_city),
              title: Text(c.name),
              subtitle: c.stopCount > 0
                  ? Text('${c.stopCount} durak · ${c.lineCount} hat${c.features.liveArrivals ? ' · canlı takip' : ''}')
                  : null,
              onTap: () {
                s.selectCity(c.id);
                Navigator.pop(context);
              },
            ),
          Padding(
            padding: const EdgeInsets.fromLTRB(20, 8, 20, 16),
            child: Text(
              'Şehrin listede yok mu? Belediyesi açık veri yayınladıkça ekliyoruz. '
              'Yalnızca seçtiğin şehrin verisi telefonuna iner.',
              style: t.textTheme.bodySmall?.copyWith(color: t.colorScheme.onSurfaceVariant),
            ),
          ),
        ]),
      );
    },
  );
}
