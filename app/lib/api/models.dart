// Data classes mirroring server/internal/model.

double _d(dynamic v) => (v as num?)?.toDouble() ?? 0;
int _i(dynamic v) => (v as num?)?.toInt() ?? 0;
String _s(dynamic v) => v as String? ?? '';
List<String> _ls(dynamic v) => (v as List?)?.map((e) => e as String).toList() ?? const [];

enum Mode { bus, tram, subway, rail, ferry, cable }

Mode parseMode(String? s) => switch (s) {
      'tram' => Mode.tram,
      'subway' => Mode.subway,
      'rail' => Mode.rail,
      'ferry' => Mode.ferry,
      'cable' => Mode.cable,
      _ => Mode.bus,
    };

class Stop {
  final String id, code, name, district;
  final double lat, lon;
  final Mode mode;
  final List<String> lines;
  Stop({
    required this.id,
    required this.name,
    required this.lat,
    required this.lon,
    required this.mode,
    this.code = '',
    this.district = '',
    this.lines = const [],
    this.folded = '',
  });

  final String folded;

  factory Stop.fromJson(Map<String, dynamic> j, {String Function(String)? fold}) {
    final name = _s(j['name']);
    return Stop(
      id: _s(j['id']),
      code: _s(j['code']),
      name: name,
      lat: _d(j['lat']),
      lon: _d(j['lon']),
      mode: parseMode(j['mode'] as String?),
      district: _s(j['district']),
      lines: _ls(j['lines']),
      folded: fold?.call(name) ?? '',
    );
  }
}

class Line {
  final String id, short, name, color, textColor, from, to;
  final Mode mode;
  final bool night, hasLive;
  Line({
    required this.id,
    required this.short,
    required this.name,
    required this.mode,
    this.color = '',
    this.textColor = '',
    this.from = '',
    this.to = '',
    this.night = false,
    this.hasLive = false,
  });
  factory Line.fromJson(Map<String, dynamic> j) => Line(
        id: _s(j['id']),
        short: _s(j['short']),
        name: _s(j['name']),
        mode: parseMode(j['mode'] as String?),
        color: _s(j['color']),
        textColor: _s(j['textColor']),
        from: _s(j['from']),
        to: _s(j['to']),
        night: j['night'] == true,
        hasLive: j['hasLive'] == true,
      );
}

class Features {
  final bool liveArrivals, liveVehicles, alerts, planner;
  const Features({this.liveArrivals = false, this.liveVehicles = false, this.alerts = false, this.planner = false});
  factory Features.fromJson(Map<String, dynamic>? j) => Features(
        liveArrivals: j?['liveArrivals'] == true,
        liveVehicles: j?['liveVehicles'] == true,
        alerts: j?['alerts'] == true,
        planner: j?['planner'] == true,
      );
}

class SourceInfo {
  final String name, license, url, note, prefix;
  final DateTime? updatedAt;
  final bool stale; // outdated fallback data (e.g. Kocaeli 2018 archive)
  final bool unofficial; // community generated, not from the operator
  SourceInfo(this.name, this.license, this.url, this.updatedAt, {this.stale = false, this.unofficial = false, this.note = '', this.prefix = ''});
  factory SourceInfo.fromJson(Map<String, dynamic> j) => SourceInfo(
        _s(j['name']),
        _s(j['license']),
        _s(j['url']),
        DateTime.tryParse(_s(j['updatedAt'])),
        stale: j['stale'] == true,
        unofficial: j['unofficial'] == true,
        note: _s(j['note']),
        prefix: _s(j['prefix']),
      );
}

class CityInfo {
  final String id, name;
  final double lat, lon, zoom;
  final int stopCount, lineCount;
  final Features features;
  final List<SourceInfo> sources;
  final List<double>? bounds; // [minLat, minLon, maxLat, maxLon]

  /// Explanation shown when any of the city's data is outdated.
  /// Warning for an id ("z:12" -> the İZBAN source), or for the whole
  /// city when [id] is null. Only stale/unofficial sources produce one.
  String? staleNote([String? id]) {
    final prefix = id?.split(':').first;
    for (final s in sources) {
      if (prefix != null && s.prefix.isNotEmpty && s.prefix != prefix) continue;
      if (s.stale || s.unofficial) {
        return s.note.isEmpty ? 'Bu şehrin verisi güncel olmayabilir.' : s.note;
      }
    }
    return null;
  }

  bool get isCommunity => sources.isNotEmpty && sources.every((s) => s.unofficial);
  CityInfo({
    required this.id,
    required this.name,
    required this.lat,
    required this.lon,
    required this.zoom,
    this.stopCount = 0,
    this.lineCount = 0,
    this.features = const Features(),
    this.sources = const [],
    this.bounds,
  });
  factory CityInfo.fromJson(Map<String, dynamic> j) {
    final c = (j['center'] as List?) ?? const [0, 0];
    return CityInfo(
      id: _s(j['id']),
      name: _s(j['name']),
      lat: _d(c[0]),
      lon: _d(c[1]),
      zoom: _d(j['zoom']) == 0 ? 12 : _d(j['zoom']),
      stopCount: _i(j['stopCount']),
      lineCount: _i(j['lineCount']),
      features: Features.fromJson(j['features'] as Map<String, dynamic>?),
      sources: ((j['sources'] as List?) ?? const []).map((e) => SourceInfo.fromJson(e as Map<String, dynamic>)).toList(),
      bounds: (j['bounds'] as List?)?.map(_d).toList(),
    );
  }
}

class Arrival {
  final String lineId, line, headsign, color, time, plate;
  final Mode mode;
  final int minutes, distanceM, stopsAway;
  final bool realtime, isLast, crowded, estimated;
  final bool? accessible, bicycle, electric;
  final double lat, lon; // live vehicle position, 0 when unknown
  Arrival.fromJson(Map<String, dynamic> j)
      : lineId = _s(j['lineId']),
        line = _s(j['line']),
        headsign = _s(j['headsign']),
        color = _s(j['color']),
        time = _s(j['time']),
        plate = _s(j['plate']),
        mode = parseMode(j['mode'] as String?),
        minutes = _i(j['minutes']),
        distanceM = _i(j['distanceM']),
        stopsAway = _i(j['stopsAway']),
        realtime = j['realtime'] == true,
        isLast = j['isLast'] == true,
        crowded = j['crowded'] == true,
        estimated = j['estimated'] == true,
        lat = _d(j['lat']),
        lon = _d(j['lon']),
        accessible = j['accessible'] as bool?,
        bicycle = j['bicycle'] as bool?,
        electric = j['electric'] as bool?;
}

class ArrivalsResult {
  final List<Arrival> arrivals;
  final bool live, degraded;
  final DateTime fetchedAt;
  ArrivalsResult(this.arrivals, this.live, this.degraded, this.fetchedAt);
  factory ArrivalsResult.fromJson(Map<String, dynamic> j) => ArrivalsResult(
        ((j['arrivals'] as List?) ?? const []).map((e) => Arrival.fromJson(e as Map<String, dynamic>)).toList(),
        j['live'] == true,
        j['degraded'] == true,
        DateTime.tryParse(_s(j['fetchedAt'])) ?? DateTime.now(),
      );
}

class StopRef {
  final String id, name;
  final double lat, lon;
  StopRef(this.id, this.name, this.lat, this.lon);
  factory StopRef.fromJson(Map<String, dynamic> j) => StopRef(_s(j['id']), _s(j['name']), _d(j['lat']), _d(j['lon']));
}

class Direction {
  final int index;
  final String headsign;
  final List<List<double>> shape; // [lat, lon]
  final List<StopRef> stops;
  final List<String> departures;
  Direction.fromJson(Map<String, dynamic> j)
      : index = _i(j['index']),
        headsign = _s(j['headsign']),
        shape = ((j['shape'] as List?) ?? const []).map((p) => [_d((p as List)[0]), _d(p[1])]).toList(),
        stops = ((j['stops'] as List?) ?? const []).map((e) => StopRef.fromJson(e as Map<String, dynamic>)).toList(),
        departures = _ls(j['departures']);
}

class Alert {
  final String title;
  final List<String> lines, lineIds;
  final DateTime? start, end;
  Alert.fromJson(Map<String, dynamic> j)
      : title = _s(j['title']),
        lines = _ls(j['lines']),
        lineIds = _ls(j['lineIds']),
        start = DateTime.tryParse(_s(j['start'])),
        end = DateTime.tryParse(_s(j['end']));
}

class LineDetail {
  final Line line;
  final List<Direction> directions;
  final List<Alert> alerts;
  final String dayType;
  LineDetail.fromJson(Map<String, dynamic> j)
      : line = Line.fromJson(j),
        directions = ((j['directions'] as List?) ?? const []).map((e) => Direction.fromJson(e as Map<String, dynamic>)).toList(),
        alerts = ((j['alerts'] as List?) ?? const []).map((e) => Alert.fromJson(e as Map<String, dynamic>)).toList(),
        dayType = _s(j['dayType']);
}

class Vehicle {
  final String id;
  final double lat, lon;
  final int direction;
  Vehicle.fromJson(Map<String, dynamic> j)
      : id = _s(j['id']),
        lat = _d(j['lat']),
        lon = _d(j['lon']),
        direction = _i(j['direction']);
}

class Leg {
  final String mode, line, headsign, color, note, polyline;
  final int precision;
  final StopRef from, to;
  final DateTime start, end;
  final int distanceM, numStops;
  Leg.fromJson(Map<String, dynamic> j)
      : mode = _s(j['mode']),
        line = _s(j['line']),
        headsign = _s(j['headsign']),
        color = _s(j['color']),
        note = _s(j['note']),
        polyline = _s(j['polyline']),
        precision = _i(j['precision']),
        from = StopRef.fromJson(j['from'] as Map<String, dynamic>),
        to = StopRef.fromJson(j['to'] as Map<String, dynamic>),
        start = DateTime.parse(_s(j['start'])).toLocal(),
        end = DateTime.parse(_s(j['end'])).toLocal(),
        distanceM = _i(j['distanceM']),
        numStops = _i(j['numStops']);
}

class Itinerary {
  final int durationS, transfers, walkM;
  final DateTime start, end;
  final List<Leg> legs;
  Itinerary.fromJson(Map<String, dynamic> j)
      : durationS = _i(j['durationS']),
        transfers = _i(j['transfers']),
        walkM = _i(j['walkM']),
        start = DateTime.parse(_s(j['start'])).toLocal(),
        end = DateTime.parse(_s(j['end'])).toLocal(),
        legs = ((j['legs'] as List?) ?? const []).map((e) => Leg.fromJson(e as Map<String, dynamic>)).toList();
}

class NearbyVehicle {
  final String id, lineId, line, headsign, color;
  final Mode mode;
  final double lat, lon;
  final int distanceM;
  NearbyVehicle.fromJson(Map<String, dynamic> j)
      : id = _s(j['id']),
        lineId = _s(j['lineId']),
        line = _s(j['line']),
        headsign = _s(j['headsign']),
        color = _s(j['color']),
        mode = parseMode(j['mode'] as String?),
        lat = _d(j['lat']),
        lon = _d(j['lon']),
        distanceM = _i(j['distanceM']);
}

/// Report kinds the server accepts (kept in the app so reporting works
/// with a flaky connection; the server validates again).
const reportKinds = <String, String>{
  'crowded': 'Araç çok kalabalık',
  'no_show': 'Araç gelmedi',
  'ghost': 'Uygulamada görünen araç yok',
  'late': 'Araç geç geldi',
  'early': 'Araç erken geçti',
  'wrong_stop': 'Durak yeri veya adı yanlış',
  'wrong_time': 'Sefer saati yanlış',
  'broken': 'Durak/araç arızalı veya kirli',
  'other': 'Diğer',
};
