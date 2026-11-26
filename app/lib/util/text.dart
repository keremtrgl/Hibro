/// Turkish-aware search folding: "Üçyol", "ucyol" and "UÇYOL" match.
/// Mirrors server/internal/textnorm.Fold so offline and online search agree.
String fold(String s) {
  const map = {
    'ı': 'i', 'İ': 'i', 'I': 'i', 'ş': 's', 'Ş': 's', 'ğ': 'g', 'Ğ': 'g',
    'ü': 'u', 'Ü': 'u', 'ö': 'o', 'Ö': 'o', 'ç': 'c', 'Ç': 'c',
    'â': 'a', 'î': 'i', 'û': 'u', '̇': '',
  };
  final b = StringBuffer();
  var space = false;
  for (final r in s.runes) {
    var ch = String.fromCharCode(r);
    ch = map[ch] ?? ch.toLowerCase();
    if (ch.isEmpty) continue;
    final c = ch.codeUnitAt(0);
    final isAlnum = (c >= 48 && c <= 57) || (c >= 97 && c <= 122) || c > 127;
    if (isAlnum) {
      b.write(ch);
      space = false;
    } else if (!space) {
      b.write(' ');
      space = true;
    }
  }
  return b.toString().trim();
}

String formatDistance(int m) {
  if (m < 1000) return '$m m';
  return '${(m / 1000).toStringAsFixed(1)} km';
}
