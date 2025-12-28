package hub

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/bingxin666/dn42-globalping/internal/model"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ProbeConnection represents a connected probe
type ProbeConnection struct {
	ID     string
	Info   model.ProbeInfo
	Conn   *websocket.Conn
	SendCh chan []byte
}

// ClientConnection represents a connected web client
type ClientConnection struct {
	ID     string
	Conn   *websocket.Conn
	SendCh chan []byte
}

// Hub manages all probe and client connections
type Hub struct {
	probes        map[string]*ProbeConnection
	clients       map[string]*ClientConnection
	taskToClient  map[string]string // taskID -> clientID
	taskToProbes  map[string][]string // taskID -> []probeID
	probesMux     sync.RWMutex
	clientsMux    sync.RWMutex
	taskMux       sync.RWMutex
}

// NewHub creates a new Hub
func NewHub() *Hub {
	return &Hub{
		probes:       make(map[string]*ProbeConnection),
		clients:      make(map[string]*ClientConnection),
		taskToClient: make(map[string]string),
		taskToProbes: make(map[string][]string),
	}
}

// RegisterProbe registers a new probe connection
func (h *Hub) RegisterProbe(conn *websocket.Conn, payload model.RegisterPayload) *ProbeConnection {
	h.probesMux.Lock()
	defer h.probesMux.Unlock()

	probeID := uuid.New().String()
	probe := &ProbeConnection{
		ID: probeID,
		Info: model.ProbeInfo{
			ID:        probeID,
			Name:      payload.Name,
			Location:  payload.Location,
			Latitude:  payload.Latitude,
			Longitude: payload.Longitude,
			Status:    "online",
			LastSeen:  time.Now(),
		},
		Conn:   conn,
		SendCh: make(chan []byte, 256),
	}
	h.probes[probeID] = probe

	log.Printf("Probe registered: %s (%s)", payload.Name, probeID)
	h.broadcastProbeList()
	return probe
}

// UnregisterProbe removes a probe connection
func (h *Hub) UnregisterProbe(probeID string) {
	h.probesMux.Lock()
	defer h.probesMux.Unlock()

	if probe, ok := h.probes[probeID]; ok {
		close(probe.SendCh)
		delete(h.probes, probeID)
		log.Printf("Probe unregistered: %s", probeID)
		h.broadcastProbeList()
	}
}

// RegisterClient registers a new web client connection
func (h *Hub) RegisterClient(conn *websocket.Conn) *ClientConnection {
	h.clientsMux.Lock()
	defer h.clientsMux.Unlock()

	clientID := uuid.New().String()
	client := &ClientConnection{
		ID:     clientID,
		Conn:   conn,
		SendCh: make(chan []byte, 256),
	}
	h.clients[clientID] = client

	log.Printf("Client connected: %s", clientID)
	return client
}

// UnregisterClient removes a web client connection
func (h *Hub) UnregisterClient(clientID string) {
	h.clientsMux.Lock()
	defer h.clientsMux.Unlock()

	if client, ok := h.clients[clientID]; ok {
		close(client.SendCh)
		delete(h.clients, clientID)
		log.Printf("Client disconnected: %s", clientID)
	}
}

// GetProbeList returns list of all online probes
func (h *Hub) GetProbeList() []model.ProbeInfo {
	h.probesMux.RLock()
	defer h.probesMux.RUnlock()

	probes := make([]model.ProbeInfo, 0, len(h.probes))
	for _, p := range h.probes {
		probes = append(probes, p.Info)
	}
	return probes
}

// broadcastProbeList sends updated probe list to all clients
func (h *Hub) broadcastProbeList() {
	probes := make([]model.ProbeInfo, 0, len(h.probes))
	for _, p := range h.probes {
		probes = append(probes, p.Info)
	}

	msg := model.Message{
		Type:    model.MsgTypeProbeList,
		Payload: model.ProbeListPayload{Probes: probes},
	}
	data, _ := json.Marshal(msg)

	h.clientsMux.RLock()
	defer h.clientsMux.RUnlock()

	for _, client := range h.clients {
		select {
		case client.SendCh <- data:
		default:
			log.Printf("Client %s send channel full", client.ID)
		}
	}
}

// CreateTask creates a new task and dispatches to probes
func (h *Hub) CreateTask(clientID string, payload model.TaskCreatePayload) string {
	taskID := uuid.New().String()

	h.taskMux.Lock()
	h.taskToClient[taskID] = clientID
	h.taskToProbes[taskID] = payload.ProbeIDs
	h.taskMux.Unlock()

	// Send task to selected probes
	h.probesMux.RLock()
	defer h.probesMux.RUnlock()

	for _, probeID := range payload.ProbeIDs {
		if probe, ok := h.probes[probeID]; ok {
			taskMsg := model.Message{
				Type: model.MsgTypeTask,
				Payload: model.TaskPayload{
					TaskID:  taskID,
					Type:    payload.Type,
					Target:  payload.Target,
					Options: payload.Options,
				},
			}
			data, _ := json.Marshal(taskMsg)
			select {
			case probe.SendCh <- data:
				log.Printf("Task %s sent to probe %s", taskID, probeID)
			default:
				log.Printf("Probe %s send channel full", probeID)
			}
		}
	}

	return taskID
}

// ForwardTaskResult forwards task result from probe to client
func (h *Hub) ForwardTaskResult(result model.TaskResultPayload) {
	h.taskMux.RLock()
	clientID, ok := h.taskToClient[result.TaskID]
	h.taskMux.RUnlock()

	if !ok {
		log.Printf("No client found for task %s", result.TaskID)
		return
	}

	// Get client's send channel while holding the lock
	h.clientsMux.RLock()
	client, clientOk := h.clients[clientID]
	var sendCh chan []byte
	if clientOk {
		sendCh = client.SendCh
	}
	h.clientsMux.RUnlock()

	if !clientOk || sendCh == nil {
		return
	}

	// Get probe name
	h.probesMux.RLock()
	probeName := ""
	if probe, ok := h.probes[result.ProbeID]; ok {
		probeName = probe.Info.Name
	}
	h.probesMux.RUnlock()

	streamPayload := model.TaskStreamPayload{
		TaskID:    result.TaskID,
		ProbeID:   result.ProbeID,
		ProbeName: probeName,
		Line:      result.Line,
		IsEnd:     result.IsEnd,
		Error:     result.Error,
	}

	msg := model.Message{
		Type:    model.MsgTypeTaskStream,
		Payload: streamPayload,
	}
	data, _ := json.Marshal(msg)

	select {
	case sendCh <- data:
	default:
		log.Printf("Client %s send channel full", clientID)
	}
}

// UpdateProbeHeartbeat updates probe's last seen time
func (h *Hub) UpdateProbeHeartbeat(probeID string) {
	h.probesMux.Lock()
	defer h.probesMux.Unlock()

	if probe, ok := h.probes[probeID]; ok {
		probe.Info.LastSeen = time.Now()
	}
}

// SendToClient sends a message to a specific client
func (h *Hub) SendToClient(clientID string, msg model.Message) {
	h.clientsMux.RLock()
	client, ok := h.clients[clientID]
	h.clientsMux.RUnlock()

	if !ok {
		return
	}

	data, _ := json.Marshal(msg)
	select {
	case client.SendCh <- data:
	default:
		log.Printf("Client %s send channel full", clientID)
	}
}

// SendProbeListToClient sends current probe list to a specific client
func (h *Hub) SendProbeListToClient(clientID string) {
	h.probesMux.RLock()
	probes := make([]model.ProbeInfo, 0, len(h.probes))
	for _, p := range h.probes {
		probes = append(probes, p.Info)
	}
	h.probesMux.RUnlock()

	msg := model.Message{
		Type:    model.MsgTypeProbeList,
		Payload: model.ProbeListPayload{Probes: probes},
	}
	h.SendToClient(clientID, msg)
}
