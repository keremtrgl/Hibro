import 'package:flutter_test/flutter_test.dart';
import 'package:geliyo/api/models.dart';

void main() {
  test('parses a live arrival', () {
    final r = ArrivalsResult.fromJson({
      'stop': {'id': 'b:12345'},
      'arrivals': [
        {'lineId': 'b:19', 'line': '19', 'headsign': 'Konak', 'mode': 'bus', 'minutes': 3, 'time': '11:40', 'realtime': true, 'plate': '35DA7284', 'accessible': true, 'stopsAway': 5},
      ],
      'live': true,
      'fetchedAt': '2026-11-26T08:37:44Z',
    });
    expect(r.live, isTrue);
    expect(r.degraded, isFalse);
    expect(r.arrivals.single.minutes, 3);
    expect(r.arrivals.single.accessible, isTrue);
    expect(r.arrivals.single.mode, Mode.bus);
  });

  test('parses a line with shapes and a missing stop list', () {
    final d = LineDetail.fromJson({
      'id': 'b:5', 'short': '5', 'name': 'Narlıdere - Üçkuyular İskele', 'mode': 'bus', 'hasLive': true, 'dayType': 'weekday',
      'directions': [
        {'index': 0, 'headsign': 'Üçkuyular İskele', 'shape': [[38.39, 26.99], [38.40, 27.0]], 'departures': ['06:00', '06:30']},
      ],
    });
    expect(d.line.hasLive, isTrue);
    expect(d.directions.single.stops, isEmpty);
    expect(d.directions.single.shape.first, [38.39, 26.99]);
  });
}
