# DN42 Globalping

A distributed network probing platform similar to Globalping, built with Go backend and Vue 3 frontend.

## Features

- **Multi-probe Support**: Select one or more probe nodes for network testing
- **Map Interface**: Visual representation of probe locations on an interactive map
- **Multiple Tools**: Support for ping, traceroute, and mtr
- **Real-time Streaming**: Results are streamed line-by-line as they become available
- **WebSocket Communication**: Bidirectional real-time communication between server and probes

## Architecture

```
┌─────────────────┐     WebSocket      ┌─────────────────┐     WebSocket      ┌─────────────────┐
│   Web Browser   │ ◄──────────────────►   Go Backend    │ ◄──────────────────►   Probe Node    │
│   (Vue 3)       │                    │   (Gin + WS)    │                    │   (Go)          │
└─────────────────┘                    └─────────────────┘                    └─────────────────┘
```

## Project Structure

```
dn42-globalping/
├── cmd/
│   ├── server/          # Web backend server
│   │   └── main.go
│   └── probe/           # Probe node client
│       └── main.go
├── internal/
│   ├── handler/         # HTTP and WebSocket handlers
│   │   └── handler.go
│   ├── hub/             # Connection management
│   │   └── hub.go
│   └── model/           # Data structures
│       └── model.go
├── web/                 # Vue 3 frontend
│   ├── src/
│   │   ├── App.vue
│   │   └── main.js
│   ├── index.html
│   ├── package.json
│   └── vite.config.js
├── go.mod
├── go.sum
└── README.md
```

## Message Protocol

All messages use JSON format with the following structure:

```json
{
  "type": "message_type",
  "payload": { ... }
}
```

### Message Types

| Type | Direction | Description |
|------|-----------|-------------|
| `register` | Probe → Server | Probe registration |
| `task` | Server → Probe | Task execution request |
| `task_result` | Probe → Server | Task result (streaming) |
| `heartbeat` | Probe → Server | Keep-alive |
| `probe_list` | Server → Client | List of available probes |
| `task_create` | Client → Server | Create new task |
| `task_stream` | Server → Client | Streaming task results |
| `error` | Server → Client | Error message |

## Quick Start

### Prerequisites

- Go 1.21+
- Node.js 18+
- npm

### Build

```bash
# Build Go backend
go mod tidy
go build -o bin/server ./cmd/server
go build -o bin/probe ./cmd/probe

# Build Vue frontend
cd web
npm install
npm run build
cd ..
```

### Run

1. **Start the server:**
```bash
./bin/server
```
Server will start on http://localhost:8080

2. **Start probe nodes (in separate terminals):**
```bash
# Probe 1
./bin/probe -name "Beijing" -location "Beijing, China" -lat 39.9042 -lon 116.4074

# Probe 2
./bin/probe -name "Tokyo" -location "Tokyo, Japan" -lat 35.6762 -lon 139.6503

# Probe 3
./bin/probe -name "New York" -location "New York, USA" -lat 40.7128 -lon -74.0060
```

3. **Open browser:**
Navigate to http://localhost:8080

### Development Mode

For frontend development with hot reload:

```bash
cd web
npm run dev
```

This starts a dev server with proxy to the backend.

## API Endpoints

### REST API

- `GET /api/probes` - List all online probes

### WebSocket Endpoints

- `/ws/probe` - Probe node connection
- `/ws/client` - Web client connection

## Probe Command Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-server` | `ws://localhost:8080/ws/probe` | Server WebSocket URL |
| `-name` | `probe-1` | Probe display name |
| `-location` | `Beijing, China` | Location description |
| `-lat` | `39.9042` | Latitude coordinate |
| `-lon` | `116.4074` | Longitude coordinate |

## License

MIT