import 'package:material_ui/material_ui.dart';
import 'package:url_launcher/url_launcher.dart';

import '../state/app_state.dart';
import '../state/scope.dart';

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
        RadioGroup<String>(
          groupValue: s.cityId,
          onChanged: (v) => v == null ? null : s.selectCity(v),
          child: Column(children: [
            for (final c in s.cities)
              RadioListTile<String>(
                value: c.id,
                title: Text(c.name),
                subtitle: c.stopCount > 0 ? Text('${c.stopCount} durak · ${c.lineCount} hat${c.features.liveArrivals ? ' · canlı takip' : ''}') : null,
              ),
          ]),
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
        _section(t, 'Sunucu'),
        ListTile(
          leading: const Icon(Icons.dns_outlined),
          title: const Text('Hibro sunucu adresi'),
          subtitle: Text(s.api.baseUrl),
          onTap: () => _editServer(context),
        ),
        _section(t, 'Veri kaynakları'),
        for (final src in s.city.sources)
          ListTile(
            leading: const Icon(Icons.dataset_outlined),
            title: Text(src.name),
            subtitle: Text(src.license),
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
          subtitle: Text('Hibro reklam göstermez, izleyici kullanmaz, konumunu kimseyle paylaşmaz. Favorilerin yalnızca bu cihazda tutulur.'),
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

  Future<void> _editServer(BuildContext context) async {
    final s = AppScope.read(context);
    final c = TextEditingController(text: s.api.baseUrl);
    final url = await showDialog<String>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Sunucu adresi'),
        content: TextField(
          controller: c,
          keyboardType: TextInputType.url,
          decoration: const InputDecoration(hintText: 'https://api.ornek.com'),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: const Text('Vazgeç')),
          FilledButton(onPressed: () => Navigator.pop(context, c.text), child: const Text('Kaydet')),
        ],
      ),
    );
    if (url != null && url.trim().isNotEmpty) await s.setApiUrl(url);
  }
}
