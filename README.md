# DN42 Globalping - Distributed Network Probe Platform

A distributed network probing platform similar to Globalping, built with Go backend and Vue 3 frontend.

## Features

- 🌍 Web-based interface with interactive map for probe selection
- 🔍 Support for multiple network diagnostic tools: ping, traceroute, mtr
- 📡 Real-time streaming output (line-by-line, similar to terminal)
- 🔌 WebSocket-based bidirectional communication
- 🐳 Docker support for easy deployment

## Architecture

```
┌─────────────┐          WebSocket         ┌──────────┐
│   Browser   │◄──────────────────────────►│  Backend │
│  (Vue 3)    │     (Web Client API)       │   (Go)   │
└─────────────┘                             └────┬─────┘
                                                 │
                                                 │ WebSocket
                                                 │ (Probe API)
                                                 │
                        ┌────────────────────────┴────────────────┐
                        │                                         │
                   ┌────▼─────┐                            ┌─────▼────┐
                   │  Probe 1 │                            │ Probe N  │
                   │   (Go)   │         ...                │   (Go)   │
                   └──────────┘                            └──────────┘
```

## Project Structure

```
dn42-globalping/
├── backend/                 # Go backend server
│   ├── main.go             # WebSocket server implementation
│   └── go.mod              # Go dependencies
├── probe/                   # Go probe client
│   ├── main.go             # Probe implementation with tool execution
│   └── go.mod              # Go dependencies
├── frontend/                # Vue 3 frontend
│   └── index.html          # Single-page application
├── Dockerfile.backend       # Backend container
├── Dockerfile.probe         # Probe container
├── docker-compose.yml       # Multi-probe deployment
└── README.md               # This file
```

## Quick Start

### Option 1: Using Docker Compose (Recommended)

1. Start all services:
```bash
docker-compose up -d
```

2. Access the web interface at http://localhost:8080

3. The system will start with 3 sample probes in different locations

### Option 2: Manual Build and Run

#### Backend Server

```bash
cd backend
go mod download
go run main.go
```

The server will start on port 8080.

#### Probe Nodes

In separate terminals, start probe nodes:

```bash
cd probe
go mod download

# Probe 1 - US East
go run main.go -server=ws://localhost:8080/ws/probe \
  -name="US-East-1" \
  -location="Virginia, USA" \
  -lat=37.5407 \
  -lng=-77.4360

# Probe 2 - EU West
go run main.go -server=ws://localhost:8080/ws/probe \
  -name="EU-West-1" \
  -location="London, UK" \
  -lat=51.5074 \
  -lng=-0.1278

# Probe 3 - Asia
go run main.go -server=ws://localhost:8080/ws/probe \
  -name="Asia-East-1" \
  -location="Tokyo, Japan" \
  -lat=35.6762 \
  -lng=139.6503
```

#### Frontend

For manual run, the backend serves the frontend from `frontend/dist/` directory:

```bash
mkdir -p frontend/dist
cp frontend/index.html frontend/dist/
```

Then access http://localhost:8080

## Usage

1. **Open the web interface** at http://localhost:8080

2. **View available probes** in the sidebar and on the map

3. **Select probes** by clicking on them (sidebar or map markers)

4. **Choose a tool**: ping, traceroute, or mtr

5. **Enter a target**: hostname or IP address (e.g., google.com or 8.8.8.8)

6. **Click Execute** to run the task

7. **View real-time output** in the terminal-style output panel

## WebSocket Message Protocol

### Message Structure

```json
{
  "type": "message_type",
  "payload": { /* type-specific data */ }
}
```

### Probe → Server Messages

- `register_probe`: Register a new probe
- `probe_heartbeat`: Keep-alive message
- `task_result`: Stream a line of output
- `task_complete`: Task execution completed

### Server → Probe Messages

- `task_assignment`: Execute a network diagnostic task

### Web Client → Server Messages

- `list_probes`: Request list of available probes
- `execute_task`: Request task execution on selected probes

### Server → Web Client Messages

- `probes_list`: List of available probes
- `task_output`: Real-time output from probe
- `task_complete`: Task completion notification

## Probe Configuration

Probes accept the following command-line flags:

- `-server`: WebSocket server URL (default: `ws://localhost:8080/ws/probe`)
- `-name`: Probe display name
- `-location`: Human-readable location
- `-lat`: Latitude coordinate
- `-lng`: Longitude coordinate

Example:
```bash
./probe -server=ws://backend:8080/ws/probe \
  -name="Custom-Probe" \
  -location="San Francisco, USA" \
  -lat=37.7749 \
  -lng=-122.4194
```

## Security Notes

⚠️ **Important Security Considerations:**

1. Probes only execute `ping`, `traceroute`, and `mtr` commands
2. No shell access or arbitrary command execution
3. Commands are executed with context timeout (5 minutes max)
4. Target validation should be added for production use
5. Add authentication/authorization for production deployment
6. Use TLS/WSS for production WebSocket connections

## Requirements

### Backend & Probe
- Go 1.21 or higher
- Network tools installed (ping, traceroute, mtr)

### Frontend
- Modern web browser with WebSocket support
- JavaScript enabled

### Docker
- Docker Engine 20.10+
- Docker Compose 2.0+

## Development

### Adding a New Tool

1. Edit `probe/main.go` in the `executeTask` function
2. Add a new case in the switch statement
3. Ensure the tool is available in the probe environment
4. Update the frontend tool selector in `frontend/index.html`

### Customizing the Frontend

The frontend is a single-page Vue 3 application. Edit `frontend/index.html` to:
- Modify UI styling
- Add new features
- Change map providers or styling

## Troubleshooting

### Probes not appearing

- Check probe logs: `docker-compose logs probe-us-east`
- Verify WebSocket connection URL
- Ensure backend is running and accessible

### Tasks not executing

- Verify tools are installed in probe container
- Check probe logs for errors
- Ensure target is reachable from probe's network

### Frontend connection issues

- Check browser console for WebSocket errors
- Verify backend WebSocket endpoint is accessible
- Try using `ws://` instead of `wss://` for local development

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.