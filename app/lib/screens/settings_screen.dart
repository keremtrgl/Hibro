import 'package:material_ui/material_ui.dart';
import 'package:url_launcher/url_launcher.dart';

import '../state/app_state.dart';
import '../state/scope.dart';
import 'privacy_screen.dart';
import '../widgets/city_sheet.dart';

class SettingsScreen extends StatelessWidget {
  const SettingsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final s = AppScope.of(context);
    final t = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('Ayarlar')),
      body: ListView(children: [
        _section(t, 'Şehir'),
        ListTile(
          leading: const Icon(Icons.location_city),
          title: Text(s.hasCity ? s.city.name : 'Şehir seçilmedi'),
          subtitle: Text(s.hasCity
              ? '${s.city.stopCount} durak · ${s.city.lineCount} hat${s.city.isCommunity ? ' · topluluk verisi' : ''}'
              : '${s.cities.length} şehir destekleniyor'),
          trailing: const Text('Değiştir'),
          onTap: () => showCitySheet(context),
        ),
        if (s.packUpdated != null)
          ListTile(
            leading: const Icon(Icons.offline_pin_outlined),
            title: const Text('Çevrimdışı veri'),
            subtitle: Text('Son güncelleme: ${_fmt(s.packUpdated!)}'),
            trailing: s.loading
                ? const SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2))
                : IconButton(icon: const Icon(Icons.refresh), onPressed: s.refresh),
          ),
        _section(t, 'Görünüm'),
        RadioGroup<AppThemeMode>(
          groupValue: s.themeMode,
          onChanged: (v) => v == null ? null : s.setTheme(v),
          child: const Column(children: [
            RadioListTile(value: AppThemeMode.system, title: Text('Sistem ayarı')),
            RadioListTile(value: AppThemeMode.light, title: Text('Açık')),
            RadioListTile(value: AppThemeMode.dark, title: Text('Koyu')),
          ]),
        ),
        _section(t, 'Veri kullanımı'),
        SwitchListTile(
          secondary: const Icon(Icons.data_saver_on_outlined),
          value: s.dataSaver,
          onChanged: s.setDataSaver,
          title: const Text('Veri tasarrufu'),
          subtitle: const Text('Canlı bilgiler daha seyrek yenilenir, yakındaki otobüsler yalnızca "Konumum"a basınca yüklenir.'),
        ),
        ListTile(
          leading: const Icon(Icons.swap_vert),
          title: const Text('Bu oturumda Geliyo sunucusundan'),
          subtitle: Text('${(s.api.bytesReceived / 1024).toStringAsFixed(1)} KB indirildi · harita karoları telefonda 150 MB\'a kadar saklanır'),
        ),
        _section(t, 'Gizlilik ve veri'),
        ListTile(
          leading: const Icon(Icons.privacy_tip_outlined),
          title: const Text('Paylaşım tercihlerin'),
          subtitle: Text(s.shareUsage ? 'Anonim kullanım istatistikleri: açık' : 'Anonim kullanım istatistikleri: kapalı'),
          trailing: const Icon(Icons.chevron_right),
          onTap: () => Navigator.of(context).push(MaterialPageRoute(builder: (_) => const PrivacyScreen())),
        ),
        _section(t, 'Veri kaynakları'),
        for (final src in s.city.sources)
          ListTile(
            leading: const Icon(Icons.dataset_outlined),
            title: Text(src.name),
            subtitle: Text([src.license, if (src.note.isNotEmpty) src.note].join('\n')),
            trailing: src.url.isEmpty ? null : const Icon(Icons.open_in_new, size: 18),
            onTap: src.url.isEmpty ? null : () => launchUrl(Uri.parse(src.url), mode: LaunchMode.externalApplication),
          ),
        const ListTile(
          leading: Icon(Icons.map_outlined),
          title: Text('Harita'),
          subtitle: Text('© OpenStreetMap katkıcıları · OpenFreeMap'),
        ),
        _section(t, 'Hakkında'),
        const ListTile(
          leading: Icon(Icons.block),
          title: Text('Reklam yok. Hiçbir zaman.'),
          subtitle: Text('Geliyo reklam göstermez, izleyici kullanmaz, konumunu kimseyle paylaşmaz. Favorilerin yalnızca bu cihazda tutulur.'),
        ),
        const SizedBox(height: 24),
      ]),
    );
  }

  static String _fmt(DateTime d) =>
      '${d.day.toString().padLeft(2, '0')}.${d.month.toString().padLeft(2, '0')}.${d.year} ${d.hour.toString().padLeft(2, '0')}:${d.minute.toString().padLeft(2, '0')}';

  Widget _section(ThemeData t, String title) => Padding(
        padding: const EdgeInsets.fromLTRB(16, 20, 16, 4),
        child: Text(title, style: t.textTheme.labelLarge?.copyWith(color: t.colorScheme.primary)),
      );

}
