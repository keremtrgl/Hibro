import 'package:flutter_test/flutter_test.dart';
import 'package:hibro/state/app_state.dart';
import 'package:hibro/util/city_detect.dart';

void main() {
  test('finds the city the user is in', () {
    expect(detectCity(builtInCities, 38.4189, 27.1287)?.id, 'izmir'); // Konak
    expect(detectCity(builtInCities, 38.9590, 27.0890)?.id, 'izmir'); // Aliağa, inside İzmir bounds
    expect(detectCity(builtInCities, 37.8714, 32.5049)?.id, 'konya');
    expect(detectCity(builtInCities, 37.0662, 37.3833)?.id, 'gaziantep');
  });

  test('unsupported cities download nothing', () {
    expect(detectCity(builtInCities, 40.7654, 29.9408), isNull); // İzmit, Kocaeli
    expect(detectCity(builtInCities, 39.9208, 32.8541), isNull); // Ankara
    expect(detectCity(builtInCities, 41.0082, 28.9784), isNull); // İstanbul
  });
}
