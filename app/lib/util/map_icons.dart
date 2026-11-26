import 'dart:math' as math;
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:maplibre_gl/maplibre_gl.dart';

/// Map icons drawn in code (no image assets): a start marker with an
/// arrow, an end flag, and a small chevron repeated along a route so its
/// direction is obvious. Rendered at 3x and registered with pixelRatio 3.
const _ratio = 3.0;

Future<Uint8List> _png(double w, double h, void Function(ui.Canvas c) paint) async {
  final rec = ui.PictureRecorder();
  final c = ui.Canvas(rec)..scale(_ratio);
  paint(c);
  final img = await rec.endRecording().toImage((w * _ratio).round(), (h * _ratio).round());
  final bd = await img.toByteData(format: ui.ImageByteFormat.png);
  img.dispose();
  return bd!.buffer.asUint8List();
}

/// Green disc with a white arrow pointing up (north); rotate it with the
/// feature's "bearing" so it points along the first segment.
Future<Uint8List> _start() => _png(30, 30, (c) {
      const o = ui.Offset(15, 15);
      c.drawCircle(o, 14, ui.Paint()..color = const ui.Color(0xFFFFFFFF));
      c.drawCircle(o, 11.5, ui.Paint()..color = const ui.Color(0xFF2E7D32));
      final p = ui.Path()
        ..moveTo(15, 6.5)
        ..lineTo(22, 15)
        ..lineTo(17.6, 15)
        ..lineTo(17.6, 23)
        ..lineTo(12.4, 23)
        ..lineTo(12.4, 15)
        ..lineTo(8, 15)
        ..close();
      c.drawPath(p, ui.Paint()..color = const ui.Color(0xFFFFFFFF));
    });

/// Chequered flag on a pole; the pole's foot is the anchor (bottom-left).
Future<Uint8List> _end() => _png(30, 34, (c) {
      final white = ui.Paint()..color = const ui.Color(0xFFFFFFFF);
      final dark = ui.Paint()..color = const ui.Color(0xFF212121);
      final red = ui.Paint()..color = const ui.Color(0xFFD32F2F);
      // white outline so it reads on any map
      c.drawRRect(ui.RRect.fromLTRBR(4, 1, 8.5, 33, const ui.Radius.circular(2)), white);
      c.drawRect(const ui.Rect.fromLTRB(6.5, 2.5, 28.5, 17.5), white);
      c.drawRect(const ui.Rect.fromLTRB(5.2, 2.5, 7.4, 32), dark);
      // chequered cloth 3 x 2
      const x0 = 7.8, y0 = 3.8, cw = 6.6, ch = 6.5;
      for (var r = 0; r < 2; r++) {
        for (var k = 0; k < 3; k++) {
          c.drawRect(ui.Rect.fromLTWH(x0 + k * cw, y0 + r * ch, cw, ch), (r + k).isEven ? red : white);
        }
      }
      c.drawRect(
          const ui.Rect.fromLTWH(x0, y0, 3 * cw, 2 * ch),
          ui.Paint()
            ..color = red.color
            ..style = ui.PaintingStyle.stroke
            ..strokeWidth = 1.2);
    });

/// White chevron pointing right (east): MapLibre rotates line-placed
/// symbols to the line's direction.
Future<Uint8List> _chevron() => _png(12, 12, (c) {
      final p = ui.Path()
        ..moveTo(3, 2)
        ..lineTo(9, 6)
        ..lineTo(3, 10);
      c.drawPath(
          p,
          ui.Paint()
            ..color = const ui.Color(0xFFFFFFFF)
            ..style = ui.PaintingStyle.stroke
            ..strokeWidth = 2.2
            ..strokeCap = ui.StrokeCap.round
            ..strokeJoin = ui.StrokeJoin.round);
    });

/// Registers "route-start", "route-end" and "route-chevron" on the map.
Future<void> addRouteIcons(MapLibreMapController c) async {
  await c.addImage('route-start', await _start(), false, _ratio);
  await c.addImage('route-end', await _end(), false, _ratio);
  await c.addImage('route-chevron', await _chevron(), false, _ratio);
}

/// Compass bearing (degrees, 0 = north) from a to b, both [lat, lon].
double bearing(List<double> a, List<double> b) {
  final f1 = a[0] * math.pi / 180, f2 = b[0] * math.pi / 180;
  final dl = (b[1] - a[1]) * math.pi / 180;
  final y = math.sin(dl) * math.cos(f2);
  final x = math.cos(f1) * math.sin(f2) - math.sin(f1) * math.cos(f2) * math.cos(dl);
  return (math.atan2(y, x) * 180 / math.pi + 360) % 360;
}

/// Bearing from the start of [pts] to the first point at least ~40 m away
/// (the first shape points are often a few metres apart and noisy).
double startBearing(List<List<double>> pts) {
  if (pts.length < 2) return 0;
  final a = pts.first;
  for (final p in pts.skip(1)) {
    final dLat = (p[0] - a[0]) * 111000, dLon = (p[1] - a[1]) * 111000 * math.cos(a[0] * math.pi / 180);
    if (dLat * dLat + dLon * dLon > 1600) return bearing(a, p);
  }
  return bearing(a, pts.last);
}
