import 'package:material_ui/material_ui.dart';
import 'package:url_launcher/url_launcher.dart';

import '../state/scope.dart';

/// Plain-language privacy notice shown in the app. The legal texts live
/// in docs/legal/ and must be reviewed by a lawyer before release.
class PrivacyScreen extends StatelessWidget {
  const PrivacyScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final s = AppScope.of(context);
    final t = Theme.of(context);
    Widget h(String text) => Padding(
          padding: const EdgeInsets.fromLTRB(16, 20, 16, 6),
          child: Text(text, style: t.textTheme.titleMedium),
        );
    Widget p(String text) => Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
          child: Text(text, style: t.textTheme.bodyMedium),
        );
    return Scaffold(
      appBar: AppBar(title: const Text('Gizlilik ve veri')),
      body: ListView(children: [
        h('Kısaca'),
        p('Geliyo reklam göstermez ve verini kimseye satmaz. Hesap, telefon numarası veya e-posta istemez. '
            'Aşağıdaki paylaşım tamamen isteğe bağlıdır ve varsayılan olarak kapalıdır.'),
        SwitchListTile(
          value: s.shareUsage,
          onChanged: (v) => s.setConsent(usage: v),
          title: const Text('Anonim kullanım istatistikleri'),
          subtitle: const Text('Hangi ekranların ne kadar kullanıldığına dair günlük toplam sayılar. Konum ve kimlik içermez.'),
        ),
        h('Konumun'),
        p('Konumun, yakındaki durakları ve otobüsleri göstermek için kullanılır. Yakındaki duraklar telefonunda hesaplanır. '
            'Yakındaki otobüsleri sorarken ve rota ararken konum sunucuya gider ama kaydedilmez. Konum geçmişi tutulmaz.'),
        h('Yer ve adres arama'),
        p('Aradığın yer adı (ör. "Kocaeli Üniversitesi") sunucumuz üzerinden OpenStreetMap Nominatim hizmetine gönderilir. '
            'Bu istekte senin konumun, IP adresin veya kimliğin yer almaz; sorgu yalnızca kısa süre önbellekte tutulur.'),
        h('Sorun bildirimleri'),
        p('"Sorun bildir" ile gönderdiğin sorun türü, ilgili durak/hat ve yazdığın not Türkiye\'deki sunucumuzda saklanır (en fazla 1 yıl) '
            've yalnızca veriyi düzeltmek için kullanılır. Konum ve kimlik gönderilmez; notuna kişisel bilgi yazmamanı öneririz.'),
        h('Favoriler ve ayarlar'),
        p('Yalnızca bu telefonda tutulur, sunucuya gönderilmez.'),
        h('Hakların'),
        p('Verilerin kimliğe bağlanmadığı için kişisel bir kaydını tutmuyoruz; paylaşımları istediğin an buradan kapatabilirsin. '
            'KVKK kapsamındaki soruların için uygulama yayınlandığında iletişim adresi burada yer alacak.'),
        ListTile(
          leading: const Icon(Icons.open_in_new),
          title: const Text('Tam gizlilik politikası'),
          onTap: () => launchUrl(Uri.parse('${s.api.baseUrl}/gizlilik'), mode: LaunchMode.externalApplication),
        ),
        const SizedBox(height: 32),
      ]),
    );
  }
}
