package storm

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

// ---------- Domain types ----------

type Point struct {
	Lat, Lon float64
}

type UserObject struct {
	ID        uuid.UUID
	UserUUID  uuid.UUID
	Name      string
	Latitude  float64
	Longitude float64
	CreatedAt time.Time
}

type Strike struct {
	Time int64
	Lat  float64
	Lon  float64
}

// ---------- Warning engine ----------

const (
	earthRadiusKm = 6371.0
)

// 8 направлений
var sectors = []string{
	"север", "северо-восток", "восток", "юго-восток",
	"юг", "юго-запад", "запад", "северо-запад",
}

// радиусы от большего к меньшему!
var levelsKm = []float64{40, 25, 15, 10, 5}

// для каждого уровня считаем «порог» – максимальное расстояние
// (последний 0 км – никогда не используется, просто placeholder)
var thresholds = func() []float64 {
	t := make([]float64, len(levelsKm))
	for i := range levelsKm {
		if i == 0 {
			t[i] = levelsKm[i]
			continue
		}
		t[i] = levelsKm[i]
	}
	return t
}()

// история попаданий в секторы
type SectorState struct {
	Level int // индекс в levelsKm (0..4)
}

type WarningEngine struct {
	obj       *UserObject
	history   map[string]*SectorState // key = sector
	lastWarns map[string]string       // чтобы не спамить одинаковыми
}

func NewWarningEngine(o *UserObject) *WarningEngine {
	return &WarningEngine{
		obj:       o,
		history:   make(map[string]*SectorState),
		lastWarns: make(map[string]string),
	}
}

func (e *WarningEngine) Update(strikes []Strike) map[string]string {
	newWarns := make(map[string]string)

	for _, s := range strikes {
		sec, distKm := e.classifyStrike(s)
		if sec == "" {
			continue // за пределами самого большого круга
		}

		st, ok := e.history[sec]
		if !ok {
			st = &SectorState{Level: 0}
			e.history[sec] = st
		}

		newLevel := 0
		for i, th := range thresholds {
			if distKm <= th {
				newLevel = i
			} else {
				break
			}
		}

		// приближение?
		if newLevel > st.Level {
			st.Level = newLevel
			msg := fmt.Sprintf("К вашему объекту \"%s\" с %s приближается грозовой фронт. Расстояние менее %.0f км.",
				e.obj.Name, sec, levelsKm[newLevel])
			newWarns[sec] = msg
		}
	}

	// merge
	for k, v := range newWarns {
		e.lastWarns[k] = v
	}
	return newWarns
}

// Возвращает сектор и расстояние
func (e *WarningEngine) classifyStrike(s Strike) (sector string, distKm float64) {
	distKm = haversineKm(e.obj.Latitude, e.obj.Longitude, s.Lat, s.Lon)
	if distKm > thresholds[0] {
		return "", distKm
	}
	bearing := bearingDeg(e.obj.Latitude, e.obj.Longitude, s.Lat, s.Lon)
	idx := int(math.Round(bearing/45.0)) % 8
	if idx < 0 {
		idx += 8
	}
	return sectors[idx], distKm
}

// ---------- Geo helpers ----------

func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := deg2rad(lat2 - lat1)
	dLon := deg2rad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(deg2rad(lat1))*math.Cos(deg2rad(lat2))*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKm * c
}

func bearingDeg(lat1, lon1, lat2, lon2 float64) float64 {
	dLon := deg2rad(lon2 - lon1)
	y := math.Sin(dLon) * math.Cos(deg2rad(lat2))
	x := math.Cos(deg2rad(lat1))*math.Sin(deg2rad(lat2)) -
		math.Sin(deg2rad(lat1))*math.Cos(deg2rad(lat2))*math.Cos(dLon)
	return rad2deg(math.Atan2(y, x))
}

func deg2rad(deg float64) float64 { return deg * math.Pi / 180 }
func rad2deg(rad float64) float64 { return rad * 180 / math.Pi }
