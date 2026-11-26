import 'package:flutter_test/flutter_test.dart';
import 'package:geliyo/screens/ride_screen.dart';
import 'package:geliyo/util/geo.dart';

void main() {
  test('trimTrip removes the first and last 300 m', () {
    // 60 points, ~50 m apart going north.
    final pts = [for (var i = 0; i < 60; i++) <double>[i * 10.0, 38.40 + i * 0.00045, 27.10, 10, 5]];
    final t = trimTrip(pts);
    expect(t.length, lessThan(60));
    expect(t.length, greaterThan(40));
    expect(distanceM(pts.first[1], pts.first[2], t.first[1], t.first[2]), greaterThanOrEqualTo(300));
    expect(distanceM(pts.last[1], pts.last[2], t.last[1], t.last[2]), greaterThanOrEqualTo(300));
  });

  test('a short ride leaves nothing to share', () {
    final pts = [for (var i = 0; i < 10; i++) <double>[i * 10.0, 38.40 + i * 0.0003, 27.10, 10, 5]];
    expect(trimTrip(pts), isEmpty);
  });
}
