import 'package:material_ui/material_ui.dart';

import '../api/models.dart';

Color hexColor(String hex, Color fallback) {
  final h = hex.replaceAll('#', '');
  if (h.length != 6) return fallback;
  final v = int.tryParse(h, radix: 16);
  return v == null ? fallback : Color(0xFF000000 | v);
}

Color modeColor(Mode m) => switch (m) {
      Mode.subway => const Color(0xFF0038EF),
      Mode.tram => const Color(0xFFAD1457),
      Mode.rail => const Color(0xFF2E7D32),
      Mode.ferry => const Color(0xFF00838F),
      Mode.cable => const Color(0xFF6A1B9A),
      Mode.bus => const Color(0xFF1565C0),
    };

String modeHex(Mode m) => '#${(modeColor(m).toARGB32() & 0xFFFFFF).toRadixString(16).padLeft(6, '0')}';

IconData modeIcon(Mode m) => switch (m) {
      Mode.subway => Icons.subway,
      Mode.tram => Icons.tram,
      Mode.rail => Icons.train,
      Mode.ferry => Icons.directions_boat,
      Mode.cable => Icons.airline_seat_recline_normal,
      Mode.bus => Icons.directions_bus,
    };

String modeName(Mode m) => switch (m) {
      Mode.subway => 'Metro',
      Mode.tram => 'Tramvay',
      Mode.rail => 'Banliyö',
      Mode.ferry => 'Vapur',
      Mode.cable => 'Teleferik',
      Mode.bus => 'Otobüs',
    };

const mapStyleLight = 'https://tiles.openfreemap.org/styles/positron';
const mapStyleDark = 'https://tiles.openfreemap.org/styles/dark';
