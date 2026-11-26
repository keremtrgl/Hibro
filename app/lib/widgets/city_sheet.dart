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
        child: ConstrainedBox(
          constraints: BoxConstraints(maxHeight: MediaQuery.of(context).size.height * 0.85),
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
            Flexible(
              child: ListView(shrinkWrap: true, children: [
                for (final c in s.cities)
                  ListTile(
                    leading: const Icon(Icons.location_city),
                    title: Text(c.name),
                    subtitle: c.stopCount > 0
                        ? Text([
                            '${c.stopCount} durak · ${c.lineCount} hat',
                            if (c.features.liveArrivals) 'canlı takip',
                            if (c.isCommunity) 'topluluk verisi',
                          ].join(' · '))
                        : null,
                    selected: s.hasCity && c.id == s.cityId,
                    onTap: () {
                      s.selectCity(c.id);
                      Navigator.pop(context);
                    },
                  ),
              ]),
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
        ),
      );
    },
  );
}
