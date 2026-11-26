import 'package:flutter_test/flutter_test.dart';
import 'package:hibro/util/geo.dart';
import 'package:hibro/util/text.dart';

void main() {
  test('fold matches the server folding', () {
    expect(fold('Üçyol'), 'ucyol');
    expect(fold('ÜÇYOL'), 'ucyol');
    expect(fold('Halkapınar-Konak'), 'halkapinar konak');
    expect(fold('Subay Evleri̇'), 'subay evleri');
    expect(fold('  İSKELE '), 'iskele');
  });

  test('distance', () {
    final d = distanceM(38.4189, 27.1287, 38.4382, 27.1428);
    expect(d, closeTo(2465, 150));
  });

  test('formatDistance', () {
    expect(formatDistance(350), '350 m');
    expect(formatDistance(1530), '1.5 km');
  });
}
