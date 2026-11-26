import 'package:flutter_test/flutter_test.dart';
import 'package:hibro/state/app_state.dart';
import 'package:hibro/util/city_detect.dart';

void main() {
  test('finds the city the user is in', () {
    expect(detectCity(builtInCities, 38.4189, 27.1287)?.id, 'izmir'); // Konak
    expect(detectCity(builtInCities, 38.9590, 27.0890)?.id, 'izmir'); // Aliağa, inside İzmir bounds
    expect(detectCity(builtInCities, 37.8714, 32.5049)?.id, 'konya');
    expect(detectCity(builtInCities, 37.0662, 37.3833)?.id, 'gaziantep');
    expect(detectCity(builtInCities, 40.7654, 29.9408)?.id, 'kocaeli'); // İzmit
    expect(detectCity(builtInCities, 40.8027, 29.4307)?.id, 'kocaeli'); // Gebze
    expect(detectCity(builtInCities, 41.0082, 28.9784)?.id, 'istanbul');
  });

  test('unsupported cities download nothing', () {
    expect(detectCity(builtInCities, 39.9208, 32.8541), isNull); // Ankara
  });
}
