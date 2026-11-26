import 'package:flutter_test/flutter_test.dart';
import 'package:hibro/util/polyline.dart';

void main() {
  test('decodes Google example polyline', () {
    final pts = decodePolyline('_p~iF~ps|U_ulLnnqC_mqNvxq`@');
    expect(pts.length, 3);
    expect(pts[0][0], closeTo(38.5, 1e-6));
    expect(pts[0][1], closeTo(-120.2, 1e-6));
    expect(pts[2][1], closeTo(-126.453, 1e-6));
  });
}
