package main

import (
	"github.com/BurtsevAnton/go-storm-warning-service/ki/storm"
	"image/color"
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	screenW, screenH = 800, 600
	scale            = 0.5 // 1° ≈ 111 km
)

type Game struct {
	obj  *storm.UserObject
	eng  *storm.WarningEngine
	stks []storm.Strike
}

func (g *Game) Update() error {
	// имитация новых молний
	if ebiten.IsKeyPressed(ebiten.KeySpace) && len(g.stks) < 20 {
		// случайная молния в радиусе 50 км от объекта
		g.stks = append(g.stks, storm.Strike{
			Lat:  g.obj.Latitude + (randFloat()-0.5)*0.9,
			Lon:  g.obj.Longitude + (randFloat()-0.5)*0.9,
			Time: time.Now().Unix(),
		})
	}
	g.eng.Update(g.stks)
	return nil
}

func randFloat() float64 { return float64(time.Now().UnixNano()) / 1e18 }

func (g *Game) Draw(screen *ebiten.Image) {
	cx, cy := g.latLonToScreen(g.obj.Latitude, g.obj.Longitude)
	// рисуем круги уровней
	for _, rKm := range []float64{40, 25, 15, 10, 5} {
		r := rKm * scale * 1.11 // 1° = 111 km
		col := color.RGBA{255, 0, 0, 30}
		vector.StrokeCircle(screen, float32(cx), float32(cy), float32(r), 1, col, false)
	}
	// рисуем молнии
	for _, s := range g.stks {
		x, y := g.latLonToScreen(s.Lat, s.Lon)
		vector.DrawFilledRect(screen, float32(x), float32(y), 3, 3, color.RGBA{255, 255, 0, 255}, false)
	}
	// объект
	vector.DrawFilledRect(screen, float32(cx), float32(cy), 6, 6, color.RGBA{0, 255, 0, 255}, false)
}

func (g *Game) latLonToScreen(lat, lon float64) (float64, float64) {
	return screenW/2 + (lon-g.obj.Longitude)*111*scale,
		screenH/2 - (lat-g.obj.Latitude)*111*scale
}

func (g *Game) Layout(_, _ int) (int, int) { return screenW, screenH }

func main() {
	obj := &storm.UserObject{
		Name:      "Demo Tower",
		Latitude:  50.0,
		Longitude: 30.0,
	}
	g := &Game{
		obj: obj,
		eng: storm.NewWarningEngine(obj),
	}
	ebiten.SetWindowSize(screenW, screenH)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
