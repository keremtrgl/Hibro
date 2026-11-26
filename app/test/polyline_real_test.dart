import 'package:flutter_test/flutter_test.dart';
import 'package:geliyo/util/polyline.dart';

void main() {
  test('decodes real planner polylines', () {
    for (final s in ['_xuiFgrodDcO~X', 'oxqiF}jydDg@D', 'wyqiFwjydDzm@fkA', '{jpiFo~vdDsN~f@']) {
      final p = decodePolyline(s);
      // ignore: avoid_print
      print('$s -> $p');
      for (final q in p) {
        expect(q[0], inInclusiveRange(38, 39));
        expect(q[1], inInclusiveRange(27, 28));
      }
    }
  });
}
