package main

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"image/color"
)

// Структуры из задачи
type UserObject struct {
	ID        uuid.UUID `json:"id" gorm:"type:char(36);primaryKey"`
	UserUUID  uuid.UUID `json:"user_uuid" gorm:"type:char(36);index"`
	Name      string    `json:"name"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

type Strike struct {
	ID       int64   `json:"id,omitempty"`
	RecordID int64   `json:"-"`
	Time     int64   `json:"time"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Alt      int     `json:"alt"`
	Pol      int     `json:"pol"`
	Mds      int     `json:"mds"`
	Mcg      int     `json:"mcg"`
	Status   int     `json:"-"`
	RegionID int64   `json:"region_id"`
}

// Константы системы предупреждения
const (
	SectorCount = 8 // Количество секторов (N, NE, E, SE, S, SW, W, NW)
)

// Уровни буферных зон (в км)
var BufferZones = []float64{40, 25, 15, 10, 5}

// Направления по секторам
var SectorNames = []string{"С", "СВ", "В", "ЮВ", "Ю", "ЮЗ", "З", "СЗ"}

// Структура для отслеживания состояния объекта
type ObjectState struct {
	Object       UserObject
	ZoneStates   [][]bool      // [зона][сектор] - есть ли активность
	LastActivity [][]time.Time // время последней активности
	AlertLevel   int           // текущий уровень тревоги (индекс зоны)
	AlertSector  int           // сектор, откуда приближается гроза
	LastAlert    time.Time
}

// Система предупреждения
type StormWarningSystem struct {
	Objects map[uuid.UUID]*ObjectState
}

func NewStormWarningSystem() *StormWarningSystem {
	return &StormWarningSystem{
		Objects: make(map[uuid.UUID]*ObjectState),
	}
}

// Добавление объекта для отслеживания
func (sws *StormWarningSystem) AddObject(obj UserObject) {
	state := &ObjectState{
		Object:       obj,
		ZoneStates:   make([][]bool, len(BufferZones)),
		LastActivity: make([][]time.Time, len(BufferZones)),
		AlertLevel:   -1,
		AlertSector:  -1,
	}

	for i := range BufferZones {
		state.ZoneStates[i] = make([]bool, SectorCount)
		state.LastActivity[i] = make([]time.Time, SectorCount)
	}

	sws.Objects[obj.ID] = state
}

// Вычисление расстояния между точками (формула гаверсинуса)
func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371 // Радиус Земли в км

	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

// Вычисление направления (азимута) от объекта к молнии
func calculateBearing(lat1, lon1, lat2, lon2 float64) float64 {
	dLon := (lon2 - lon1) * math.Pi / 180
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180

	y := math.Sin(dLon) * math.Cos(lat2Rad)
	x := math.Cos(lat1Rad)*math.Sin(lat2Rad) - math.Sin(lat1Rad)*math.Cos(lat2Rad)*math.Cos(dLon)

	bearing := math.Atan2(y, x) * 180 / math.Pi
	return math.Mod(bearing+360, 360)
}

// Определение сектора по азимуту
func getSector(bearing float64) int {
	sectorSize := 360.0 / SectorCount
	sector := int((bearing + sectorSize/2) / sectorSize)
	return sector % SectorCount
}

// Обработка порции молний
func (sws *StormWarningSystem) ProcessStrikes(strikes []Strike) []string {
	now := time.Now()
	var alerts []string

	// Сначала очищаем старые состояния (старше 15 минут)
	sws.clearOldActivity(now)

	for _, strike := range strikes {
		for objectID, state := range sws.Objects {
			distance := calculateDistance(
				state.Object.Latitude, state.Object.Longitude,
				strike.Lat, strike.Lon,
			)

			// Проверяем попадание в буферные зоны
			for zoneIdx, zoneRadius := range BufferZones {
				if distance <= zoneRadius {
					bearing := calculateBearing(
						state.Object.Latitude, state.Object.Longitude,
						strike.Lat, strike.Lon,
					)
					sector := getSector(bearing)

					state.ZoneStates[zoneIdx][sector] = true
					state.LastActivity[zoneIdx][sector] = now
					break // Молния попала в эту зону, дальше не проверяем
				}
			}

			// Анализируем приближение грозы
			alert := sws.analyzeApproach(objectID, state, now)
			if alert != "" {
				alerts = append(alerts, alert)
			}
		}
	}

	return alerts
}

// Очистка старой активности
func (sws *StormWarningSystem) clearOldActivity(now time.Time) {
	timeout := 15 * time.Minute

	for _, state := range sws.Objects {
		for zoneIdx := range state.ZoneStates {
			for sectorIdx := range state.ZoneStates[zoneIdx] {
				if now.Sub(state.LastActivity[zoneIdx][sectorIdx]) > timeout {
					state.ZoneStates[zoneIdx][sectorIdx] = false
				}
			}
		}
	}
}

// Анализ приближения грозы
func (sws *StormWarningSystem) analyzeApproach(objectID uuid.UUID, state *ObjectState, now time.Time) string {
	// Находим самую близкую активную зону
	closestActiveZone := -1
	activeSectors := []int{}

	for zoneIdx := len(BufferZones) - 1; zoneIdx >= 0; zoneIdx-- {
		sectorActivity := []int{}
		for sectorIdx, active := range state.ZoneStates[zoneIdx] {
			if active {
				sectorActivity = append(sectorActivity, sectorIdx)
			}
		}

		if len(sectorActivity) > 0 {
			closestActiveZone = zoneIdx
			activeSectors = sectorActivity
			break
		}
	}

	if closestActiveZone == -1 {
		state.AlertLevel = -1
		return ""
	}

	// Генерируем предупреждение при любой активности
	// если прошло достаточно времени с последнего предупреждения
	if len(activeSectors) > 0 {
		mainSector := activeSectors[0]

		// Проверяем, не было ли недавно предупреждения
		timeSinceLastAlert := now.Sub(state.LastAlert)

		// Если это первое предупреждение или прошло достаточно времени
		if state.AlertLevel == -1 || timeSinceLastAlert > 30*time.Second {
			state.LastAlert = now
			state.AlertLevel = closestActiveZone
			state.AlertSector = mainSector

			direction := SectorNames[mainSector]
			distance := BufferZones[closestActiveZone]

			return fmt.Sprintf("К вашему объекту \"%s\" с %s приближается грозовой фронт. Расстояние менее %.0f км.",
				state.Object.Name, direction, distance)
		}
	}

	return ""
}

// Визуализация для тестирования
type Game struct {
	system      *StormWarningSystem
	testCases   []TestCase
	currentTest int
	strikes     []Strike
	alerts      []string
}

type TestCase struct {
	Name     string
	Object   UserObject
	Strikes  []Strike
	Expected string
}

func (g *Game) Update() error {
	// Обновление каждые 2 секунды
	if ebiten.TPS() > 0 && int(time.Now().Unix())%2 == 0 {
		if g.currentTest < len(g.testCases) {
			testCase := g.testCases[g.currentTest]
			g.strikes = testCase.Strikes
			g.alerts = g.system.ProcessStrikes(testCase.Strikes)
			g.currentTest++
		}
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.currentTest == 0 {
		return
	}

	// Константы для визуализации - УВЕЛИЧИВАЕМ МАСШТАБ
	centerX, centerY := 400.0, 300.0
	scale := 0.5 // км на пиксель (было 2.0, теперь 0.5 - в 4 раза больше)

	// Получаем текущий объект
	var currentObject UserObject
	if g.currentTest <= len(g.testCases) {
		currentObject = g.testCases[g.currentTest-1].Object
	}

	// Рисуем буферные зоны
	colors := []color.RGBA{
		{255, 0, 0, 30},   // 40км - красный
		{255, 100, 0, 40}, // 25км - оранжевый
		{255, 200, 0, 50}, // 15км - желтый
		{0, 255, 0, 60},   // 10км - зеленый
		{0, 0, 255, 70},   // 5км - синий
	}

	for i, radius := range BufferZones {
		pixelRadius := float32(radius / scale)
		vector.DrawFilledCircle(screen, float32(centerX), float32(centerY), pixelRadius, colors[i], true)
	}

	// Рисуем линии секторов
	for i := 0; i < SectorCount; i++ {
		angle := float64(i) * 360.0 / SectorCount * math.Pi / 180
		endX := centerX + math.Cos(angle-math.Pi/2)*BufferZones[0]/scale
		endY := centerY + math.Sin(angle-math.Pi/2)*BufferZones[0]/scale
		vector.StrokeLine(screen, float32(centerX), float32(centerY), float32(endX), float32(endY), 1, color.RGBA{0, 0, 0, 100}, true)
	}

	// Рисуем подписи секторов
	for i := 0; i < SectorCount; i++ {
		angle := float64(i) * 360.0 / SectorCount * math.Pi / 180
		labelX := centerX + math.Cos(angle-math.Pi/2)*(BufferZones[0]/scale+20)
		labelY := centerY + math.Sin(angle-math.Pi/2)*(BufferZones[0]/scale+20)
		ebitenutil.DebugPrintAt(screen, SectorNames[i], int(labelX)-10, int(labelY)-10)
	}

	// Рисуем объект - УВЕЛИЧИВАЕМ РАЗМЕР
	vector.DrawFilledCircle(screen, float32(centerX), float32(centerY), 8, color.RGBA{0, 0, 255, 255}, true)

	// Рисуем молнии - УВЕЛИЧИВАЕМ РАЗМЕР
	for _, strike := range g.strikes {
		// Простое приближение для визуализации (игнорируем кривизну Земли)
		deltaLat := strike.Lat - currentObject.Latitude
		deltaLon := strike.Lon - currentObject.Longitude

		// Конвертируем в примерные километры (очень грубо)
		kmLat := deltaLat * 111.0 // примерно 111 км на градус широты
		kmLon := deltaLon * 111.0 * math.Cos(currentObject.Latitude*math.Pi/180)

		strikeX := centerX + kmLon/scale
		strikeY := centerY - kmLat/scale // инвертируем Y для корректного отображения

		vector.DrawFilledCircle(screen, float32(strikeX), float32(strikeY), 6, color.RGBA{255, 255, 0, 255}, true)

		// Добавляем обводку для лучшей видимости
		vector.StrokeCircle(screen, float32(strikeX), float32(strikeY), 6, 2, color.RGBA{255, 165, 0, 255}, true)
	}

	// Отображаем информацию
	testName := ""
	if g.currentTest <= len(g.testCases) {
		testName = g.testCases[g.currentTest-1].Name
	}

	alertText := "Нет предупреждений"
	if len(g.alerts) > 0 {
		alertText = ""
		for i, alert := range g.alerts {
			if i > 0 {
				alertText += "\n"
			}
			alertText += alert
		}
	}

	ebitenutil.DebugPrint(screen, fmt.Sprintf("Тест: %s\nОбъект: %s\nМолний: %d\n\nПредупреждения:\n%s",
		testName, currentObject.Name, len(g.strikes), alertText))
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 800, 600
}

// Функция для создания тестовых случаев
func createTestCases() []TestCase {
	// Базовый объект для тестов (Москва)
	baseObject := UserObject{
		ID:        uuid.New(),
		UserUUID:  uuid.New(),
		Name:      "Тестовый объект",
		Latitude:  55.7558,
		Longitude: 37.6176,
		CreatedAt: time.Now(),
	}

	return []TestCase{
		{
			Name:   "Приближение с севера - зона 40км",
			Object: baseObject,
			Strikes: []Strike{
				{ID: 1, Time: time.Now().Unix(), Lat: 56.1, Lon: 37.6, Alt: 1000},
				{ID: 2, Time: time.Now().Unix(), Lat: 56.0, Lon: 37.65, Alt: 1000},
			},
			Expected: "К вашему объекту \"Тестовый объект\" с С приближается грозовой фронт. Расстояние менее 40 км.",
		},
		{
			Name:   "Приближение с северо-востока - зона 25км",
			Object: baseObject,
			Strikes: []Strike{
				{ID: 3, Time: time.Now().Unix(), Lat: 55.95, Lon: 37.85, Alt: 1000},
				{ID: 4, Time: time.Now().Unix(), Lat: 55.9, Lon: 37.8, Alt: 1000},
			},
			Expected: "К вашему объекту \"Тестовый объект\" с СВ приближается грозовой фронт. Расстояние менее 25 км.",
		},
		{
			Name:   "Приближение с востока - зона 15км",
			Object: baseObject,
			Strikes: []Strike{
				{ID: 5, Time: time.Now().Unix(), Lat: 55.75, Lon: 37.75, Alt: 1000},
			},
			Expected: "К вашему объекту \"Тестовый объект\" с В приближается грозовой фронт. Расстояние менее 15 км.",
		},
		{
			Name:   "Приближение с юго-востока - зона 10км",
			Object: baseObject,
			Strikes: []Strike{
				{ID: 6, Time: time.Now().Unix(), Lat: 55.68, Lon: 37.7, Alt: 1000},
			},
			Expected: "К вашему объекту \"Тестовый объект\" с ЮВ приближается грозовой фронт. Расстояние менее 10 км.",
		},
		{
			Name:   "Приближение с юга - зона 5км",
			Object: baseObject,
			Strikes: []Strike{
				{ID: 7, Time: time.Now().Unix(), Lat: 55.72, Lon: 37.62, Alt: 1000},
			},
			Expected: "К вашему объекту \"Тестовый объект\" с Ю приближается грозовой фронт. Расстояние менее 5 км.",
		},
		{
			Name:   "Приближение с юго-запада",
			Object: baseObject,
			Strikes: []Strike{
				{ID: 8, Time: time.Now().Unix(), Lat: 55.7, Lon: 37.5, Alt: 1000},
			},
			Expected: "К вашему объекту \"Тестовый объект\" с ЮЗ приближается грозовой фронт. Расстояние менее 15 км.",
		},
		{
			Name:   "Приближение с запада",
			Object: baseObject,
			Strikes: []Strike{
				{ID: 9, Time: time.Now().Unix(), Lat: 55.75, Lon: 37.45, Alt: 1000},
			},
			Expected: "К вашему объекту \"Тестовый объект\" с З приближается грозовой фронт. Расстояние менее 25 км.",
		},
		{
			Name:   "Приближение с северо-запада",
			Object: baseObject,
			Strikes: []Strike{
				{ID: 10, Time: time.Now().Unix(), Lat: 55.85, Lon: 37.5, Alt: 1000},
			},
			Expected: "К вашему объекту \"Тестовый объект\" с СЗ приближается грозовой фронт. Расстояние менее 25 км.",
		},
	}
}

// Тестирование без визуализации
func runTests() {
	system := NewStormWarningSystem()
	testCases := createTestCases()

	// Добавляем объект в систему
	system.AddObject(testCases[0].Object)

	fmt.Println("=== ТЕСТИРОВАНИЕ СИСТЕМЫ ПРЕДУПРЕЖДЕНИЯ О ГРОЗАХ ===")

	for i, testCase := range testCases {
		fmt.Printf("Тест %d: %s\n", i+1, testCase.Name)
		fmt.Printf("Объект: %s (%.4f, %.4f)\n", testCase.Object.Name, testCase.Object.Latitude, testCase.Object.Longitude)
		fmt.Printf("Молний: %d\n", len(testCase.Strikes))

		// Показываем детали молний
		for j, strike := range testCase.Strikes {
			distance := calculateDistance(testCase.Object.Latitude, testCase.Object.Longitude, strike.Lat, strike.Lon)
			bearing := calculateBearing(testCase.Object.Latitude, testCase.Object.Longitude, strike.Lat, strike.Lon)
			sector := getSector(bearing)
			fmt.Printf("  Молния %d: (%.4f, %.4f) - расстояние %.1f км, направление %.1f°, сектор %s\n",
				j+1, strike.Lat, strike.Lon, distance, bearing, SectorNames[sector])
		}

		// Обрабатываем молнии
		alerts := system.ProcessStrikes(testCase.Strikes)

		fmt.Printf("Предупреждения: %d\n", len(alerts))
		for _, alert := range alerts {
			fmt.Printf("  ⚠️  %s\n", alert)
		}

		fmt.Printf("Ожидалось: %s\n", testCase.Expected)
		fmt.Println("---")

		// Задержка для имитации времени между проверками
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("\n=== ТЕСТ ПОСЛЕДОВАТЕЛЬНОГО ПРИБЛИЖЕНИЯ ===")

	// Создаем новую систему для теста последовательного приближения
	system2 := NewStormWarningSystem()
	system2.AddObject(testCases[0].Object)

	// Моделируем приближение грозы с севера
	approachingStrikes := [][]Strike{
		{{ID: 100, Time: time.Now().Unix(), Lat: 56.2, Lon: 37.6, Alt: 1000}},        // 50км с севера
		{{ID: 101, Time: time.Now().Unix() + 60, Lat: 56.1, Lon: 37.6, Alt: 1000}},   // 40км с севера
		{{ID: 102, Time: time.Now().Unix() + 120, Lat: 56.0, Lon: 37.6, Alt: 1000}},  // 30км с севера
		{{ID: 103, Time: time.Now().Unix() + 180, Lat: 55.95, Lon: 37.6, Alt: 1000}}, // 25км с севера
		{{ID: 104, Time: time.Now().Unix() + 240, Lat: 55.9, Lon: 37.6, Alt: 1000}},  // 15км с севера
		{{ID: 105, Time: time.Now().Unix() + 300, Lat: 55.85, Lon: 37.6, Alt: 1000}}, // 10км с севера
		{{ID: 106, Time: time.Now().Unix() + 360, Lat: 55.8, Lon: 37.6, Alt: 1000}},  // 5км с севера
	}

	for i, strikes := range approachingStrikes {
		fmt.Printf("Этап %d: ", i+1)
		for _, strike := range strikes {
			distance := calculateDistance(testCases[0].Object.Latitude, testCases[0].Object.Longitude, strike.Lat, strike.Lon)
			fmt.Printf("Молния на расстоянии %.1f км с севера\n", distance)
		}

		alerts := system2.ProcessStrikes(strikes)
		for _, alert := range alerts {
			fmt.Printf("  🚨 %s\n", alert)
		}

		time.Sleep(200 * time.Millisecond)
	}
}

func main() {
	fmt.Println("Выберите режим:")
	fmt.Println("1. Запустить тесты")
	fmt.Println("2. Запустить визуализацию")

	var choice int
	fmt.Scan(&choice)

	if choice == 1 {
		runTests()
		return
	}

	// Создаем систему предупреждения
	system := NewStormWarningSystem()

	// Создаем тестовые случаи
	testCases := createTestCases()

	// Добавляем объект в систему
	system.AddObject(testCases[0].Object)

	// Запускаем визуализацию
	ebiten.SetWindowSize(800, 600)
	ebiten.SetWindowTitle("Storm Warning System Test")

	game := &Game{
		system:    system,
		testCases: testCases,
	}

	if err := ebiten.RunGame(game); err != nil {
		panic(err)
	}
}
