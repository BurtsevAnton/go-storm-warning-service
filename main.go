package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	scale   = 5
	screenW = 800
	screenH = 800

	numObjects     = 2
	numStrikes     = 100
	strikeRadiusKm = 15
	groupSpeed     = 1.0

	updateInterval  = time.Second
	alertResetDelay = 30 * time.Second
	newGroupDelay   = 3 * time.Second
)

var zones = []float64{60, 40, 25, 15, 10, 5}

type UserObject struct {
	ID        uuid.UUID
	UserUUID  uuid.UUID
	Name      string
	Latitude  float64
	Longitude float64
	CreatedAt time.Time
}

type Strike struct {
	lon float64
	lat float64
}

type Game struct {
	objects            []UserObject
	group              []Strike
	frontLon           float64
	frontLat           float64
	startLon           float64
	startLat           float64
	endLon             float64
	endLat             float64
	step               int
	maxSteps           int
	lastUpdate         time.Time
	alerts             map[int]bool
	lastStrikeTime     map[int]time.Time
	groupFinishedTime  time.Time
	waitingForNewGroup bool
	totalGroups        int
}

var sectors = []string{
	"N", "NNE", "NE", "ENE",
	"E", "ESE", "SE", "SSE",
	"S", "SSW", "SW", "WSW",
	"W", "WNW", "NW", "NNW",
}

func deg2rad(deg float64) float64 {
	return deg * math.Pi / 180
}

func polarToLonLat(distance, angle float64) (float64, float64) {
	rad := deg2rad(angle)

	return distance * math.Cos(rad), distance * math.Sin(rad)
}

func dist(lon1, lat1, lon2, lat2 float64) float64 {
	return math.Hypot(lon2-lon1, lat2-lat1)
}

func sectorOf(lon, lat float64) string {
	angle := math.Atan2(-lat, lon) * 180 / math.Pi
	if angle < 0 {
		angle += 360
	}
	sectorIndex := int((angle+11.25)/22.5) % 16

	return sectors[sectorIndex]
}

func generateObjects(n int) []UserObject {
	objs := make([]UserObject, n)

	for i := range objs {
		angle := rand.Float64() * 360
		distance := rand.Float64() * (zones[0] - 5)
		lon, lat := polarToLonLat(distance, angle)
		objs[i] = UserObject{
			ID:        uuid.New(),
			UserUUID:  uuid.New(),
			Name:      fmt.Sprintf("Object %d", i+1),
			Latitude:  lat,
			Longitude: lon,
			CreatedAt: time.Now(),
		}
		fmt.Printf("[OBJECT] %d '%s': (%.2f, %.2f)\n", i+1, objs[i].Name, lon, lat)
	}

	return objs
}

func (g *Game) generatePath() {
	angle := rand.Float64() * 360
	startLon, startLat := polarToLonLat(85+strikeRadiusKm, angle)
	endAngle := math.Mod(angle+180+rand.Float64()*60-30, 360)
	endLon, endLat := polarToLonLat(85+strikeRadiusKm, endAngle)
	steps := int(dist(startLon, startLat, endLon, endLat) / groupSpeed)

	g.frontLon = startLon
	g.frontLat = startLat
	g.startLon, g.startLat = startLon, startLat
	g.endLon, g.endLat = endLon, endLat
	g.step = 0
	g.maxSteps = steps
	g.lastUpdate = time.Now()
	g.waitingForNewGroup = false
	g.totalGroups++

	if g.alerts == nil {
		g.alerts = make(map[int]bool)
	}
	if g.lastStrikeTime == nil {
		g.lastStrikeTime = make(map[int]time.Time)
	}

	fmt.Printf("[STRIKES] Group %d: Start: (%.2f, %.2f), End: (%.2f, %.2f), Steps: %d\n",
		g.totalGroups, startLon, startLat, endLon, endLat, steps)
}

func (g *Game) generateStrikes() {
	group := make([]Strike, numStrikes)

	for i := range group {
		offsetAngle := rand.Float64() * 2 * math.Pi
		offsetDist := rand.Float64() * strikeRadiusKm
		dLon := offsetDist * math.Cos(offsetAngle)
		dLat := offsetDist * math.Sin(offsetAngle)
		group[i] = Strike{g.frontLon + dLon, g.frontLat + dLat}
	}

	g.group = group
}

func kmToScreen(lon, lat float64) (int, int) {
	sLon := int(screenW/2 + lon*scale)
	sLat := int(screenH/2 - lat*scale)

	return sLon, sLat
}

func drawCircle(screen *ebiten.Image, cLon, cLat float64, radiusKm float64, clr color.Color) {
	centerLon := screenW/2 + cLon*scale
	centerLat := screenH/2 - cLat*scale
	radiusPx := radiusKm * scale

	for a := 0.0; a < 2*math.Pi; a += 0.05 {
		lon := centerLon + radiusPx*math.Cos(a)
		lat := centerLat + radiusPx*math.Sin(a)
		screen.Set(int(lon), int(lat), clr)
	}
}

func drawCircleAlert(screen *ebiten.Image, cLon, cLat float64, radiusKm float64, clr color.Color) {
	centerLon := screenW/2 + cLon*scale
	centerLat := screenH/2 - cLat*scale
	radiusPx := radiusKm * scale

	for a := 0.0; a < 2*math.Pi; a += 0.05 {
		lon := centerLon + radiusPx*math.Cos(a)
		lat := centerLat + radiusPx*math.Sin(a)
		screen.Set(int(lon), int(lat), clr)
	}
}

func drawSectors(screen *ebiten.Image, cx, cy, radiusKm float64, clr color.Color) {
	centerX := screenW/2 + cx*scale
	centerY := screenH/2 - cy*scale
	radiusPx := radiusKm * scale

	for i := 0; i < 16; i++ {
		angle := float64(i) * 22.5 * math.Pi / 180
		x := centerX + radiusPx*math.Cos(angle)
		y := centerY + radiusPx*math.Sin(angle)
		vector.StrokeLine(screen, float32(centerX), float32(centerY), float32(x), float32(y), 1, clr, true)
	}
}

func drawSectorLabels(screen *ebiten.Image, cLon, cLat, radiusKm float64) {
	centerLon := screenW/2 + cLon*scale
	centerLat := screenH/2 - cLat*scale
	radiusPx := radiusKm * scale * 1.05

	for i, label := range sectors {
		angle := float64(i) * 22.5 * math.Pi / 180
		lon := centerLon + radiusPx*math.Cos(angle)
		lat := centerLat + radiusPx*math.Sin(angle)
		ebitenutil.DebugPrintAt(screen, label, int(lon)-10, int(lat)-6)
	}
}

func (g *Game) groupCenter() (float64, float64) {
	if len(g.group) == 0 {
		return 0, 0
	}

	var sumLon, sumLat float64

	for _, s := range g.group {
		sumLon += s.lon
		sumLat += s.lat
	}

	n := float64(len(g.group))

	return sumLon / n, sumLat / n
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenW, screenH
}

func main() {
	rand.NewSource(time.Now().UnixNano())

	game := &Game{}
	game.objects = generateObjects(numObjects)
	game.generatePath()

	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowTitle("Storm Alert Simulation")

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}

func (g *Game) Update() error {
	currentTime := time.Now()

	if g.waitingForNewGroup {
		if currentTime.Sub(g.groupFinishedTime) >= newGroupDelay {
			fmt.Printf("[NEW GROUP] Create new lightning group (delay: %.1f сек)\n",
				currentTime.Sub(g.groupFinishedTime).Seconds())
			g.generatePath()
		}

		return nil
	}

	if g.step >= g.maxSteps {
		if !g.waitingForNewGroup {
			fmt.Printf("[GROUP FINISHED] Group %d finished. Waiting for a new group...\n", g.totalGroups)
			g.waitingForNewGroup = true
			g.groupFinishedTime = currentTime
			g.group = nil
		}
		return nil
	}

	if time.Since(g.lastUpdate) < updateInterval {
		return nil
	}

	dLon := (g.endLon - g.startLon) / float64(g.maxSteps)
	dLat := (g.endLat - g.startLat) / float64(g.maxSteps)

	g.lastUpdate = currentTime
	g.frontLon += dLon
	g.frontLat += dLat
	g.step++

	g.generateStrikes()

	for objIdx := range g.objects {
		for zoneIdx := range zones {
			key := objIdx*10 + zoneIdx

			if g.alerts[key] {
				hasCurrentStrikeInZone := false

				for _, s := range g.group {
					d := dist(s.lon, s.lat, g.objects[objIdx].Longitude, g.objects[objIdx].Latitude)
					if d <= zones[zoneIdx] {
						hasCurrentStrikeInZone = true
						g.lastStrikeTime[key] = currentTime

						break
					}
				}

				if !hasCurrentStrikeInZone {
					if lastTime, exists := g.lastStrikeTime[key]; exists {
						if currentTime.Sub(lastTime) >= alertResetDelay {
							fmt.Printf("[RESET] Flag reset for object %s zone %d (%.1f seconds after last strike)\n",
								g.objects[objIdx].Name, zoneIdx+1, currentTime.Sub(lastTime).Seconds())
							delete(g.alerts, key)
							delete(g.lastStrikeTime, key)
						}
					}
				}
			}
		}
	}

	for _, s := range g.group {
		for objIdx, obj := range g.objects {
			d := dist(s.lon, s.lat, obj.Longitude, obj.Latitude)

			for i, r := range zones {
				key := objIdx*10 + i
				if d <= r && !g.alerts[key] {
					sector := sectorOf(s.lon-obj.Longitude, s.lat-obj.Latitude)
					fmt.Printf("[ALERT] Group %d, Object: %s, Zone: %d, Sector: %s, Distance: %.1f km\n",
						g.totalGroups, obj.Name, i+1, sector, d)
					g.alerts[key] = true
					g.lastStrikeTime[key] = currentTime

					break
				}
			}
		}
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 10, G: 10, B: 20, A: 255})

	for objIdx, obj := range g.objects {
		for i, r := range zones {
			key := objIdx*10 + i
			if g.alerts[key] {
				drawCircleAlert(screen, obj.Longitude, obj.Latitude, r, color.RGBA{R: 255, G: 50, B: 50, A: 120})
			} else {
				drawCircle(screen, obj.Longitude, obj.Latitude, r, color.RGBA{R: 100, G: 255, B: 100, A: 40})
			}
		}

		drawSectors(screen, obj.Longitude, obj.Latitude, zones[0], color.RGBA{B: 255, A: 30})
		drawSectorLabels(screen, obj.Longitude, obj.Latitude, zones[0])

		sx, sy := kmToScreen(obj.Longitude, obj.Latitude)
		vector.DrawFilledRect(screen, float32(sx-3), float32(sy-3), 6, 6, color.RGBA{R: 255, A: 255}, true)
	}

	if !g.waitingForNewGroup && g.group != nil {
		for _, s := range g.group {
			sx, sy := kmToScreen(s.lon, s.lat)
			vector.DrawFilledRect(screen, float32(sx-2), float32(sy-2), 4, 4, color.RGBA{R: 255, G: 255, A: 255}, true)
		}
	}

	var msg string

	if g.waitingForNewGroup {
		timeLeft := newGroupDelay - time.Since(g.groupFinishedTime)
		if timeLeft < 0 {
			timeLeft = 0
		}
		msg = fmt.Sprintf("Group %d finished\nWaiting for a new group: %.1f s\nTotal groups: %d",
			g.totalGroups, timeLeft.Seconds(), g.totalGroups)
	} else {
		cx, cy := g.groupCenter()
		msg = fmt.Sprintf("Group %d\nStep: %d / %d\nGroup center: (%.2f, %.2f)\nTotal groups: %d",
			g.totalGroups, g.step, g.maxSteps, cx, cy, g.totalGroups)
	}

	ebitenutil.DebugPrint(screen, msg)
}
