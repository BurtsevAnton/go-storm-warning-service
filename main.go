package main

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"image/color"
	"log"
	"math"
	"math/rand"
	"time"
)

const (
	screenW = 800
	screenH = 800

	numObjects     = 5
	numStrikes     = 20
	strikeRadiusKm = 6
	groupSpeed     = 1.0 // км/шаг

	updateInterval = time.Second
)

var zones = []float64{50, 40, 25, 15, 10, 5} // км

type UserObject struct {
	ID        uuid.UUID
	UserUUID  uuid.UUID
	Name      string
	Latitude  float64 // X
	Longitude float64 // Y
	CreatedAt time.Time
}

type Strike struct {
	X, Y float64
}

type Game struct {
	objects    []UserObject
	group      []Strike
	frontX     float64
	frontY     float64
	startX     float64
	startY     float64
	endX       float64
	endY       float64
	step       int
	maxSteps   int
	lastUpdate time.Time
	alerts     map[int]bool
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

func polarToXY(distance, angle float64) (float64, float64) {
	rad := deg2rad(angle)
	return distance * math.Cos(rad), distance * math.Sin(rad)
}

func dist(x1, y1, x2, y2 float64) float64 {
	return math.Hypot(x2-x1, y2-y1)
}

func sectorOf(x, y float64) string {
	angle := math.Atan2(-y, x) * 180 / math.Pi
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
		x, y := polarToXY(distance, angle)
		objs[i] = UserObject{
			ID:        uuid.New(),
			UserUUID:  uuid.New(),
			Name:      fmt.Sprintf("Object #%d", i+1),
			Latitude:  x,
			Longitude: y,
			CreatedAt: time.Now(),
		}
		fmt.Printf("[OBJECT] #%d '%s': (%.2f, %.2f)\n", i+1, objs[i].Name, x, y)
	}
	return objs
}

func (g *Game) generatePath() {
	angle := rand.Float64() * 360
	startX, startY := polarToXY(55, angle)
	endAngle := math.Mod(angle+180+rand.Float64()*60-30, 360)
	endX, endY := polarToXY(55, endAngle)

	g.frontX = startX
	g.frontY = startY

	steps := int(dist(startX, startY, endX, endY) / groupSpeed)
	g.startX, g.startY = startX, startY
	g.endX, g.endY = endX, endY
	g.step = 0
	g.maxSteps = steps
	g.lastUpdate = time.Now()
	g.alerts = make(map[int]bool)

	fmt.Printf("[STRIKES] Start: (%.2f, %.2f), End: (%.2f, %.2f), Steps: %d\n", startX, startY, endX, endY, steps)
}

func (g *Game) generateStrikes() {
	group := make([]Strike, numStrikes)
	for i := range group {
		offsetAngle := rand.Float64() * 2 * math.Pi
		offsetDist := rand.Float64() * strikeRadiusKm
		dx := offsetDist * math.Cos(offsetAngle)
		dy := offsetDist * math.Sin(offsetAngle)
		group[i] = Strike{g.frontX + dx, g.frontY + dy}
	}
	g.group = group
}

func kmToScreen(x, y float64) (int, int) {
	scale := screenH * 0.5 / zones[0]
	sx := int(screenW/2 + x*scale)
	sy := int(screenH/2 - y*scale)
	return sx, sy
}

func drawCircle(screen *ebiten.Image, cx, cy float64, radiusKm float64, clr color.Color) {
	scale := screenH * 0.5 / zones[0]
	centerX := screenW/2 + cx*scale
	centerY := screenH/2 - cy*scale
	radiusPx := radiusKm * scale
	for a := 0.0; a < 2*math.Pi; a += 0.01 {
		x := centerX + radiusPx*math.Cos(a)
		y := centerY + radiusPx*math.Sin(a)
		screen.Set(int(x), int(y), clr)
	}
}

func drawSectors(screen *ebiten.Image, cx, cy, radiusKm float64, clr color.Color) {
	scale := screenH * 0.5 / zones[0]
	centerX := screenW/2 + cx*scale
	centerY := screenH/2 - cy*scale
	radiusPx := radiusKm * scale
	for i := 0; i < 16; i++ {
		angle := float64(i) * 22.5 * math.Pi / 180
		x := centerX + radiusPx*math.Cos(angle)
		y := centerY + radiusPx*math.Sin(angle)
		ebitenutil.DrawLine(screen, centerX, centerY, x, y, clr)
	}
}

func drawSectorLabels(screen *ebiten.Image, cx, cy, radiusKm float64) {
	scale := screenH * 0.5 / zones[0]
	centerX := screenW/2 + cx*scale
	centerY := screenH/2 - cy*scale
	radiusPx := radiusKm * scale * 1.05
	for i, label := range sectors {
		angle := float64(i) * 22.5 * math.Pi / 180
		x := centerX + radiusPx*math.Cos(angle)
		y := centerY + radiusPx*math.Sin(angle)
		ebitenutil.DebugPrintAt(screen, label, int(x)-10, int(y)-6)
	}
}

func (g *Game) groupCenter() (float64, float64) {
	var sumX, sumY float64
	for _, s := range g.group {
		sumX += s.X
		sumY += s.Y
	}
	n := float64(len(g.group))
	return sumX / n, sumY / n
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenW, screenH
}

func main() {
	rand.Seed(time.Now().UnixNano())

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
	if g.step >= g.maxSteps {
		return nil
	}
	if time.Since(g.lastUpdate) < updateInterval {
		return nil
	}
	g.lastUpdate = time.Now()
	g.step++

	dx := (g.endX - g.startX) / float64(g.maxSteps)
	dy := (g.endY - g.startY) / float64(g.maxSteps)

	g.frontX += dx
	g.frontY += dy

	g.generateStrikes()

	for _, s := range g.group {
		for objIdx, obj := range g.objects {
			d := dist(s.X, s.Y, obj.Latitude, obj.Longitude)
			for i, r := range zones {
				key := objIdx*10 + i
				if d <= r && !g.alerts[key] {
					sector := sectorOf(s.X-obj.Latitude, s.Y-obj.Longitude)
					fmt.Printf("[ALERT] Объект: %s, Уровень: %d, Сектор: %s, Расстояние: %.1f км\n", obj.Name, i+1, sector, d)
					g.alerts[key] = true
					break
				}
			}
		}
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{10, 10, 20, 255})

	for _, obj := range g.objects {
		for _, r := range zones {
			drawCircle(screen, obj.Latitude, obj.Longitude, r, color.RGBA{100, 255, 100, 40})
		}
		//drawSectors(screen, obj.Latitude, obj.Longitude, zones[0], color.RGBA{100, 255, 100, 80})
		drawSectorLabels(screen, obj.Latitude, obj.Longitude, zones[0])

		sx, sy := kmToScreen(obj.Latitude, obj.Longitude)
		ebitenutil.DrawRect(screen, float64(sx-3), float64(sy-3), 6, 6, color.RGBA{255, 0, 0, 255})
	}

	for _, s := range g.group {
		sx, sy := kmToScreen(s.X, s.Y)
		ebitenutil.DrawRect(screen, float64(sx-2), float64(sy-2), 4, 4, color.RGBA{255, 255, 0, 255})
	}

	cx, cy := g.groupCenter()
	msg := fmt.Sprintf("Step: %d / %d\nGroup center: (%.2f, %.2f)", g.step, g.maxSteps, cx, cy)
	ebitenutil.DebugPrint(screen, msg)
}
