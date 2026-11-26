package plan

import "testing"

const sample = `{"itineraries":[{"duration":1500,"startTime":"2026-11-26T09:00:00Z","endTime":"2026-11-26T09:25:00Z","transfers":1,
"legs":[{"mode":"WALK","from":{"name":"START","lat":38.4,"lon":27.1},"to":{"name":"Konak","stopId":"m_2","lat":38.41,"lon":27.12},"startTime":"2026-11-26T09:00:00Z","endTime":"2026-11-26T09:05:00Z","distance":350,"legGeometry":{"points":"abc","precision":7}},
{"mode":"SUBWAY","from":{"name":"Konak","lat":38.41,"lon":27.12},"to":{"name":"Bornova","lat":38.46,"lon":27.21},"startTime":"2026-11-26T09:06:00Z","endTime":"2026-11-26T09:25:00Z","routeShortName":"M1","headsign":"Evka 3","routeColor":"0038EF","intermediateStops":[{"name":"a"},{"name":"b"}],"legGeometry":{"points":"def","precision":7}}]}]}`

func TestConvert(t *testing.T) {
	w, err := DecodeWire([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	its := Convert(w)
	if len(its) != 1 || len(its[0].Legs) != 2 {
		t.Fatalf("%+v", its)
	}
	it := its[0]
	if it.WalkM != 350 || it.Transfers != 1 || it.Legs[1].Line != "M1" || it.Legs[1].NumStops != 3 || it.Legs[0].NumStops != 0 {
		t.Fatalf("%+v", it)
	}
}
