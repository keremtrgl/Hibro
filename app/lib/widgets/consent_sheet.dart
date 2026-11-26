import 'package:material_ui/material_ui.dart';

import '../screens/privacy_screen.dart';
import '../state/scope.dart';

/// Asked once, on first launch. Both choices default to off; the user
/// can change them later in Ayarlar → Gizlilik ve veri.
Future<void> showConsentSheet(BuildContext context) {
  return showModalBottomSheet<void>(
    context: context,
    isDismissible: false,
    enableDrag: false,
    isScrollControlled: true,
    builder: (_) => const _ConsentSheet(),
  );
}

class _ConsentSheet extends StatefulWidget {
  const _ConsentSheet();
  @override
  State<_ConsentSheet> createState() => _ConsentSheetState();
}

class _ConsentSheetState extends State<_ConsentSheet> {
  bool _usage = false, _trips = false;

  @override
  Widget build(BuildContext context) {
    final t = Theme.of(context);
    final s = AppScope.read(context);
    return SafeArea(
      child: SingleChildScrollView(
        padding: const EdgeInsets.fromLTRB(20, 24, 20, 16),
        child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
          Icon(Icons.volunteer_activism_outlined, size: 40, color: t.colorScheme.primary),
          const SizedBox(height: 12),
          Text('Geliyo\'ya hoş geldin', style: t.textTheme.headlineSmall),
          const SizedBox(height: 8),
          Text(
            'Reklam yok, hesap yok. Belediye verisi çoğu zaman eksik ya da hatalı; '
            'istersen anonim katkınla daha doğru varış süreleri sunabiliriz. İkisi de isteğe bağlı.',
            style: t.textTheme.bodyMedium,
          ),
          const SizedBox(height: 8),
          SwitchListTile(
            contentPadding: EdgeInsets.zero,
            value: _usage,
            onChanged: (v) => setState(() => _usage = v),
            title: const Text('Anonim kullanım istatistikleri'),
            subtitle: const Text('Ekran ve özellik kullanım sayıları. Konum ve kimlik yok.'),
          ),
          SwitchListTile(
            contentPadding: EdgeInsets.zero,
            value: _trips,
            onChanged: (v) => setState(() => _trips = v),
            title: const Text('Yolculuk verisi'),
            subtitle: const Text('Sadece "Bu araca bindim" dediğinde, başı ve sonu silinmiş GPS izi.'),
          ),
          TextButton(
            onPressed: () => Navigator.of(context).push(MaterialPageRoute(builder: (_) => const PrivacyScreen())),
            child: const Text('Ayrıntılar: Gizlilik ve veri'),
          ),
          const SizedBox(height: 8),
          Row(children: [
            Expanded(
              child: OutlinedButton(
                onPressed: () {
                  s.setConsent(usage: false, trips: false);
                  Navigator.pop(context);
                },
                child: const Text('Hiçbirini paylaşma'),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: FilledButton(
                onPressed: () {
                  s.setConsent(usage: _usage, trips: _trips);
                  Navigator.pop(context);
                },
                child: const Text('Kaydet'),
              ),
            ),
          ]),
        ]),
      ),
    );
  }
}
