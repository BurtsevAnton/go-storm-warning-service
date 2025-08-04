package storm

import (
	"testing"

	"github.com/google/uuid"
)

func TestSingleSectorApproach(t *testing.T) {
	obj := &UserObject{
		ID:        uuid.New(),
		Name:      "Test Tower",
		Latitude:  50.0,
		Longitude: 30.0,
	}
	eng := NewWarningEngine(obj)

	tests := []struct {
		name    string
		strikes []Strike
		want    string
	}{
		{"level 0", []Strike{{Lat: 50.3, Lon: 30.0}}, ""}, // 33 км, 40-ка ещё нет
		{"level 0 again", []Strike{{Lat: 50.4, Lon: 30.0}}, "К вашему объекту \"Test Tower\" с север приближается грозовой фронт. Расстояние менее 40 км."},
		{"level 1", []Strike{{Lat: 50.25, Lon: 30.0}}, "К вашему объекту \"Test Tower\" с север приближается грозовой фронт. Расстояние менее 25 км."},
		{"level 2", []Strike{{Lat: 50.15, Lon: 30.0}}, "К вашему объекту \"Test Tower\" с север приближается грозовой фронт. Расстояние менее 15 км."},
		{"level 3", []Strike{{Lat: 50.09, Lon: 30.0}}, "К вашему объекту \"Test Tower\" с север приближается грозовой фронт. Расстояние менее 10 км."},
		{"already closer", []Strike{{Lat: 50.05, Lon: 30.0}}, ""}, // уже 10 км, дальше 5 км, но 5-го уровня нет в levelsKm
	}

	for _, tt := range tests {
		gotMap := eng.Update(tt.strikes)
		got := ""
		for _, v := range gotMap {
			got = v
			break
		}
		if got != tt.want {
			t.Errorf("%s: want %q, got %q", tt.name, tt.want, got)
		}
	}
}

func TestDifferentSectors(t *testing.T) {
	obj := &UserObject{
		ID:        uuid.New(),
		Name:      "Test Tower",
		Latitude:  50.0,
		Longitude: 30.0,
	}
	eng := NewWarningEngine(obj)

	eng.Update([]Strike{{Lat: 50.3, Lon: 30.35}}) // NE
	if len(eng.history) != 1 {
		t.Fatalf("expected 1 sector history, got %d", len(eng.history))
	}
	if _, ok := eng.history["восток"]; !ok {
		t.Fatalf("expected east sector")
	}
}
