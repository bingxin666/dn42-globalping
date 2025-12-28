package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/bingxin666/dn42-globalping/internal/hub"
	"github.com/bingxin666/dn42-globalping/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// Handler holds all HTTP and WebSocket handlers
type Handler struct {
	hub *hub.Hub
}

// NewHandler creates a new Handler
func NewHandler(h *hub.Hub) *Handler {
	return &Handler{hub: h}
}

// HandleProbeWS handles WebSocket connections from probe nodes
func (h *Handler) HandleProbeWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade probe connection: %v", err)
		return
	}

	// Wait for registration message
	_, message, err := conn.ReadMessage()
	if err != nil {
		log.Printf("Failed to read registration message: %v", err)
		conn.Close()
		return
	}

	var msg model.Message
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("Failed to parse registration message: %v", err)
		conn.Close()
		return
	}

	if msg.Type != model.MsgTypeRegister {
		log.Printf("Expected register message, got: %s", msg.Type)
		conn.Close()
		return
	}

	// Parse registration payload
	payloadBytes, _ := json.Marshal(msg.Payload)
	var registerPayload model.RegisterPayload
	if err := json.Unmarshal(payloadBytes, &registerPayload); err != nil {
		log.Printf("Failed to parse register payload: %v", err)
		conn.Close()
		return
	}

	probe := h.hub.RegisterProbe(conn, registerPayload)
	defer h.hub.UnregisterProbe(probe.ID)

	// Send probe ID back
	idMsg := model.Message{
		Type:    model.MsgTypeRegister,
		Payload: map[string]string{"probe_id": probe.ID},
	}
	idData, _ := json.Marshal(idMsg)
	conn.WriteMessage(websocket.TextMessage, idData)

	// Start write goroutine
	go h.probeWriter(probe)

	// Read messages from probe
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Probe %s connection error: %v", probe.ID, err)
			}
			break
		}

		var msg model.Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Failed to parse message from probe %s: %v", probe.ID, err)
			continue
		}

		switch msg.Type {
		case model.MsgTypeHeartbeat:
			h.hub.UpdateProbeHeartbeat(probe.ID)
		case model.MsgTypeTaskResult:
			payloadBytes, _ := json.Marshal(msg.Payload)
			var resultPayload model.TaskResultPayload
			if err := json.Unmarshal(payloadBytes, &resultPayload); err != nil {
				log.Printf("Failed to parse task result: %v", err)
				continue
			}
			resultPayload.ProbeID = probe.ID
			h.hub.ForwardTaskResult(resultPayload)
		}
	}
}

// probeWriter handles writing messages to probe
func (h *Handler) probeWriter(probe *hub.ProbeConnection) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-probe.SendCh:
			if !ok {
				probe.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := probe.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := probe.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// HandleClientWS handles WebSocket connections from web clients
func (h *Handler) HandleClientWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade client connection: %v", err)
		return
	}

	client := h.hub.RegisterClient(conn)
	defer h.hub.UnregisterClient(client.ID)

	// Send current probe list
	h.hub.SendProbeListToClient(client.ID)

	// Start write goroutine
	go h.clientWriter(client)

	// Read messages from client
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Client %s connection error: %v", client.ID, err)
			}
			break
		}

		var msg model.Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Failed to parse message from client %s: %v", client.ID, err)
			continue
		}

		switch msg.Type {
		case model.MsgTypeTaskCreate:
			payloadBytes, _ := json.Marshal(msg.Payload)
			var createPayload model.TaskCreatePayload
			if err := json.Unmarshal(payloadBytes, &createPayload); err != nil {
				log.Printf("Failed to parse task create payload: %v", err)
				h.hub.SendToClient(client.ID, model.Message{
					Type:    model.MsgTypeError,
					Payload: model.ErrorPayload{Message: "Invalid task payload"},
				})
				continue
			}

			taskID := h.hub.CreateTask(client.ID, createPayload)
			log.Printf("Task created: %s for client %s", taskID, client.ID)
		case model.MsgTypeProbeList:
			h.hub.SendProbeListToClient(client.ID)
		}
	}
}

// clientWriter handles writing messages to client
func (h *Handler) clientWriter(client *hub.ClientConnection) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-client.SendCh:
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := client.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// GetProbes returns list of all probes (REST API)
func (h *Handler) GetProbes(c *gin.Context) {
	probes := h.hub.GetProbeList()
	c.JSON(http.StatusOK, gin.H{
		"probes": probes,
	})
}
