# Storm Alert Simulation

A real-time lightning strike alert system simulation built with Go and Ebiten game engine. The application visualizes moving storm groups and generates alerts when lightning strikes enter predefined zones around monitored objects.

## Features

- **Real-time Storm Simulation**: Lightning groups move across the map with configurable paths and speeds
- **Multi-zone Alert System**: 6 concentric alert zones (50, 40, 25, 15, 10, 5 km) around each object
- **Sector-based Detection**: 16-sector compass system for precise strike location reporting
- **Dynamic Alert Management**: Automatic alert reset after 30 seconds of no strikes in range
- **Visual Interface**: Real-time visualization with color-coded zones and strike indicators

## Configuration

Key parameters can be modified in the constants section:

```go
const (
    numObjects     = 2      // Number of monitored objects
    numStrikes     = 100    // Strikes per group
    strikeRadiusKm = 15     // Strike spread radius
    groupSpeed     = 1.0    // km/step movement speed
)
```

## How it Works

1. **Object Generation**: Randomly places monitoring objects within a 50km radius
2. **Storm Path**: Each lightning group follows a linear path across the map
3. **Strike Generation**: 100 strikes are generated around the group center with 15km spread
4. **Alert Logic**: Monitors distance between strikes and objects, triggering zone-specific alerts
5. **Automatic Reset**: Alerts clear after 30 seconds without strikes in the outermost zone

## Controls

The simulation runs automatically. Each storm group:
- Updates every second
- Moves 1km per step
- Generates new groups after a 3-second delay

## Dependencies

- [Ebiten v2](https://github.com/hajimehoshi/ebiten) - 2D game engine
- [Google UUID](https://github.com/google/uuid) - UUID generation

## Usage

```bash
go mod tidy
go run main.go
```

The application opens an 800x800 pixel window showing the simulation with real-time console logging of alerts and system events.

## Alert Format

Console output includes detailed information:
```
[ALERT] Group 1, Object: Object 1, Zone: 3, Sector: NE, Distance: 22.5 km
[RESET] Flag reset for object Object 1 zone 2 (31.2 seconds left after last strike)
```