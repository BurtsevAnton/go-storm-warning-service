package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCalculateDistance(t *testing.T) {
	// Тест расчета расстояния между Москвой и Санкт-Петербургом
	moscowLat, moscowLon := 55.7558, 37.6176
	spbLat, spbLon := 59.9311, 30.3609

	distance := calculateDistance(moscowLat, moscowLon, spbLat, spbLon)

	// Ожидаем примерно 635 км
	if distance < 630 || distance > 640 {
		t.Errorf("Неверное расстояние между Москвой и СПб: получено %.2f км, ожидалось ~635 км", distance)
	}
}

func TestCalculateBearing(t *testing.T) {
	// Тест расчета направления
	centerLat, centerLon := 55.7558, 37.6176

	testCases := []struct {
		name      string
		lat       float64
		lon       float64
		expected  float64
		tolerance float64
	}{
		{"Север", 56.0, 37.6176, 0, 5},
		{"Восток", 55.7558, 38.0, 90, 5},
		{"Юг", 55.5, 37.6176, 180, 5},
		{"Запад", 55.7558, 37.0, 270, 5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bearing := calculateBearing(centerLat, centerLon, tc.lat, tc.lon)

			// Проверяем с учетом толерантности и кольцевой природы углов
			diff := bearing - tc.expected
			if diff > 180 {
				diff -= 360
			} else if diff < -180 {
				diff += 360
			}

			if abs(diff) > tc.tolerance {
				t.Errorf("Неверное направление для %s: получено %.2f°, ожидалось %.2f°", tc.name, bearing, tc.expected)
			}
		})
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func TestGetSector(t *testing.T) {
	testCases := []struct {
		bearing  float64
		expected int
		name     string
	}{
		{0, 0, "Север"},
		{45, 1, "Северо-восток"},
		{90, 2, "Восток"},
		{135, 3, "Юго-восток"},
		{180, 4, "Юг"},
		{225, 5, "Юго-запад"},
		{270, 6, "Запад"},
		{315, 7, "Северо-запад"},
		{359, 0, "Почти север"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sector := getSector(tc.bearing)
			if sector != tc.expected {
				t.Errorf("Неверный сектор для направления %.2f°: получен %d (%s), ожидался %d (%s)",
					tc.bearing, sector, SectorNames[sector], tc.expected, SectorNames[tc.expected])
			}
		})
	}
}

func TestStormWarningSystem_AddObject(t *testing.T) {
	system := NewStormWarningSystem()

	obj := UserObject{
		ID:        uuid.New(),
		UserUUID:  uuid.New(),
		Name:      "Тестовый объект",
		Latitude:  55.7558,
		Longitude: 37.6176,
		CreatedAt: time.Now(),
	}

	system.AddObject(obj)

	if len(system.Objects) != 1 {
		t.Errorf("Ожидался 1 объект в системе, получено %d", len(system.Objects))
	}

	state, exists := system.Objects[obj.ID]
	if !exists {
		t.Error("Объект не найден в системе")
	}

	if len(state.ZoneStates) != len(BufferZones) {
		t.Errorf("Неверное количество зон: получено %d, ожидалось %d", len(state.ZoneStates), len(BufferZones))
	}

	for i, zoneStates := range state.ZoneStates {
		if len(zoneStates) != SectorCount {
			t.Errorf("Неверное количество секторов в зоне %d: получено %d, ожидалось %d", i, len(zoneStates), SectorCount)
		}
	}
}

func TestStormWarningSystem_ProcessStrikes_SingleStrike(t *testing.T) {
	system := NewStormWarningSystem()

	obj := UserObject{
		ID:        uuid.New(),
		UserUUID:  uuid.New(),
		Name:      "Тестовый объект",
		Latitude:  55.7558,
		Longitude: 37.6176,
		CreatedAt: time.Now(),
	}

	system.AddObject(obj)

	// Молния в 30 км к северу (попадает в зону 40км)
	strike := Strike{
		ID:   1,
		Time: time.Now().Unix(),
		Lat:  56.0,
		Lon:  37.6176,
		Alt:  1000,
	}

	strikes := []Strike{strike}
	alerts := system.ProcessStrikes(strikes)

	// Проверяем, что молния зарегистрирована
	state := system.Objects[obj.ID]
	if !state.ZoneStates[0][0] { // зона 40км, сектор север
		t.Error("Молния не зарегистрирована в северном секторе зоны 40км")
	}

	// Должно быть предупреждение
	if len(alerts) == 0 {
		t.Error("Ожидалось предупреждение, но его нет")
	} else {
		expectedSubstr := "с С приближается"
		if !contains(alerts[0], expectedSubstr) {
			t.Errorf("Предупреждение не содержит ожидаемый текст '%s': %s", expectedSubstr, alerts[0])
		}
	}
}

func TestStormWarningSystem_ProcessStrikes_MultipleZones(t *testing.T) {
	system := NewStormWarningSystem()

	obj := UserObject{
		ID:        uuid.New(),
		UserUUID:  uuid.New(),
		Name:      "Тестовый объект",
		Latitude:  55.7558,
		Longitude: 37.6176,
		CreatedAt: time.Now(),
	}

	system.AddObject(obj)

	testCases := []struct {
		name           string
		strike         Strike
		expectedZone   int
		expectedSector int
		expectAlert    bool
	}{
		{
			name:         "Молния в зоне 40км с севера",
			strike:       Strike{ID: 1, Time: time.Now().Unix(), Lat: 56.1, Lon: 37.6176, Alt: 1000},
			expectedZone: 0, expectedSector: 0, expectAlert: true,
		},
		{
			name:         "Молния в зоне 25км с востока",
			strike:       Strike{ID: 2, Time: time.Now().Unix(), Lat: 55.7558, Lon: 37.85, Alt: 1000},
			expectedZone: 1, expectedSector: 2, expectAlert: true,
		},
		{
			name:         "Молния в зоне 15км с юга",
			strike:       Strike{ID: 3, Time: time.Now().Unix(), Lat: 55.62, Lon: 37.6176, Alt: 1000},
			expectedZone: 2, expectedSector: 4, expectAlert: true,
		},
		{
			name:         "Молния в зоне 10км с запада",
			strike:       Strike{ID: 4, Time: time.Now().Unix(), Lat: 55.7558, Lon: 37.53, Alt: 1000},
			expectedZone: 3, expectedSector: 6, expectAlert: true,
		},
		{
			name:         "Молния в зоне 5км с северо-востока",
			strike:       Strike{ID: 5, Time: time.Now().Unix(), Lat: 55.78, Lon: 37.65, Alt: 1000},
			expectedZone: 4, expectedSector: 1, expectAlert: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Создаем новую систему для каждого теста
			testSystem := NewStormWarningSystem()
			testSystem.AddObject(obj)

			strikes := []Strike{tc.strike}
			alerts := testSystem.ProcessStrikes(strikes)

			state := testSystem.Objects[obj.ID]

			// Проверяем регистрацию в правильной зоне и секторе
			if !state.ZoneStates[tc.expectedZone][tc.expectedSector] {
				t.Errorf("Молния не зарегистрирована в ожидаемой зоне %d, секторе %d", tc.expectedZone, tc.expectedSector)
			}

			// Проверяем наличие предупреждения
			if tc.expectAlert && len(alerts) == 0 {
				t.Error("Ожидалось предупреждение, но его нет")
			} else if !tc.expectAlert && len(alerts) > 0 {
				t.Errorf("Не ожидалось предупреждение, но получено: %v", alerts)
			}

			if len(alerts) > 0 {
				expectedDirection := SectorNames[tc.expectedSector]
				if !contains(alerts[0], expectedDirection) {
					t.Errorf("Предупреждение не содержит ожидаемое направление '%s': %s", expectedDirection, alerts[0])
				}
			}
		})
	}
}

func TestStormWarningSystem_ApproachingStorm(t *testing.T) {
	system := NewStormWarningSystem()

	obj := UserObject{
		ID:        uuid.New(),
		UserUUID:  uuid.New(),
		Name:      "Тестовый объект",
		Latitude:  55.7558,
		Longitude: 37.6176,
		CreatedAt: time.Now(),
	}

	system.AddObject(obj)

	// Моделируем приближение грозы с севера
	strikeSequence := []Strike{
		{ID: 1, Time: time.Now().Unix(), Lat: 56.2, Lon: 37.6176, Alt: 1000},       // ~50км
		{ID: 2, Time: time.Now().Unix() + 60, Lat: 56.1, Lon: 37.6176, Alt: 1000},  // ~40км
		{ID: 3, Time: time.Now().Unix() + 120, Lat: 56.0, Lon: 37.6176, Alt: 1000}, // ~30км
		{ID: 4, Time: time.Now().Unix() + 180, Lat: 55.9, Lon: 37.6176, Alt: 1000}, // ~15км
	}

	var allAlerts []string

	for i, strike := range strikeSequence {
		strikes := []Strike{strike}
		alerts := system.ProcessStrikes(strikes)
		allAlerts = append(allAlerts, alerts...)

		if i == 0 {
			// Первая молния должна дать предупреждение о зоне 40км
			if len(alerts) == 0 {
				t.Error("Ожидалось первое предупреждение")
			}
		}

		// Добавляем задержку для имитации времени
		time.Sleep(10 * time.Millisecond)
	}

	// Проверяем, что было хотя бы одно предупреждение
	if len(allAlerts) == 0 {
		t.Error("Ожидались предупреждения при приближении грозы")
	}

	// Проверяем, что в предупреждениях упоминается северное направление
	foundNorthAlert := false
	for _, alert := range allAlerts {
		if contains(alert, "с С ") {
			foundNorthAlert = true
			break
		}
	}

	if !foundNorthAlert {
		t.Errorf("Не найдено предупреждение о приближении с севера среди: %v", allAlerts)
	}
}

func TestStormWarningSystem_OldActivityCleanup(t *testing.T) {
	system := NewStormWarningSystem()

	obj := UserObject{
		ID:        uuid.New(),
		UserUUID:  uuid.New(),
		Name:      "Тестовый объект",
		Latitude:  55.7558,
		Longitude: 37.6176,
		CreatedAt: time.Now(),
	}

	system.AddObject(obj)

	// Добавляем молнию
	strike := Strike{
		ID:   1,
		Time: time.Now().Unix(),
		Lat:  56.0,
		Lon:  37.6176,
		Alt:  1000,
	}

	strikes := []Strike{strike}
	system.ProcessStrikes(strikes)

	state := system.Objects[obj.ID]

	// Проверяем, что активность зарегистрирована
	if !state.ZoneStates[0][0] {
		t.Error("Активность не зарегистрирована")
	}

	// Устанавливаем время активности в прошлое (более 15 минут назад)
	state.LastActivity[0][0] = time.Now().Add(-20 * time.Minute)

	// Обрабатываем новую порцию молний (пустую)
	system.ProcessStrikes([]Strike{})

	// Проверяем, что старая активность очищена
	if state.ZoneStates[0][0] {
		t.Error("Старая активность не была очищена")
	}
}

func TestStormWarningSystem_NoFalseAlerts(t *testing.T) {
	system := NewStormWarningSystem()

	obj := UserObject{
		ID:        uuid.New(),
		UserUUID:  uuid.New(),
		Name:      "Тестовый объект",
		Latitude:  55.7558,
		Longitude: 37.6176,
		CreatedAt: time.Now(),
	}

	system.AddObject(obj)

	// Молнии далеко от объекта (более 40 км)
	distantStrikes := []Strike{
		{ID: 1, Time: time.Now().Unix(), Lat: 56.5, Lon: 37.6176, Alt: 1000}, // ~80км с севера
		{ID: 2, Time: time.Now().Unix(), Lat: 55.0, Lon: 37.6176, Alt: 1000}, // ~80км с юга
	}

	alerts := system.ProcessStrikes(distantStrikes)

	if len(alerts) > 0 {
		t.Errorf("Не ожидалось предупреждений для далеких молний, но получено: %v", alerts)
	}

	// Проверяем, что активность не зарегистрирована
	state := system.Objects[obj.ID]
	for zoneIdx := range state.ZoneStates {
		for sectorIdx := range state.ZoneStates[zoneIdx] {
			if state.ZoneStates[zoneIdx][sectorIdx] {
				t.Errorf("Неожиданная активность в зоне %d, секторе %d", zoneIdx, sectorIdx)
			}
		}
	}
}

// Вспомогательная функция для проверки содержания строки
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Бенчмарк для проверки производительности
func BenchmarkProcessStrikes(b *testing.B) {
	system := NewStormWarningSystem()

	obj := UserObject{
		ID:        uuid.New(),
		UserUUID:  uuid.New(),
		Name:      "Тестовый объект",
		Latitude:  55.7558,
		Longitude: 37.6176,
		CreatedAt: time.Now(),
	}

	system.AddObject(obj)

	// Создаем набор молний для тестирования
	strikes := make([]Strike, 100)
	for i := range strikes {
		strikes[i] = Strike{
			ID:   int64(i),
			Time: time.Now().Unix(),
			Lat:  55.7558 + float64(i%10)*0.01,
			Lon:  37.6176 + float64(i%10)*0.01,
			Alt:  1000,
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		system.ProcessStrikes(strikes)
	}
}
