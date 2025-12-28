package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type MessageType string

const (
	// Client to Server
	MsgRegisterProbe  MessageType = "register_probe"
	MsgProbeHeartbeat MessageType = "probe_heartbeat"
	MsgTaskResult     MessageType = "task_result"
	MsgListProbes     MessageType = "list_probes"
	MsgExecuteTask    MessageType = "execute_task"
	MsgTaskComplete   MessageType = "task_complete"

	// Server to Client
	MsgProbesList     MessageType = "probes_list"
	MsgTaskAssignment MessageType = "task_assignment"
	MsgTaskOutput     MessageType = "task_output"
	MsgError          MessageType = "error"
)

type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type ProbeInfo struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Location string    `json:"location"`
	Lat      float64   `json:"lat"`
	Lng      float64   `json:"lng"`
	Status   string    `json:"status"`
	LastSeen time.Time `json:"last_seen"`
}

type TaskRequest struct {
	ProbeIDs []string          `json:"probe_ids"`
	Tool     string            `json:"tool"` // ping, traceroute, mtr
	Target   string            `json:"target"`
	Options  map[string]string `json:"options,omitempty"`
}

type TaskAssignment struct {
	TaskID  string            `json:"task_id"`
	Tool    string            `json:"tool"`
	Target  string            `json:"target"`
	Options map[string]string `json:"options,omitempty"`
}

type TaskOutput struct {
	TaskID  string `json:"task_id"`
	ProbeID string `json:"probe_id"`
	Line    string `json:"line"`
	IsError bool   `json:"is_error,omitempty"`
}

type TaskComplete struct {
	TaskID   string `json:"task_id"`
	ProbeID  string `json:"probe_id"`
	ExitCode int    `json:"exit_code"`
}

type Server struct {
	probes     map[string]*ProbeConnection
	webClients map[string]*WebConnection
	tasks      map[string]*Task
	probesMu   sync.RWMutex
	webMu      sync.RWMutex
	tasksMu    sync.RWMutex
}

type ProbeConnection struct {
	ID   string
	Info ProbeInfo
	Conn *websocket.Conn
	Send chan Message
}

type WebConnection struct {
	ID   string
	Conn *websocket.Conn
	Send chan Message
}

type Task struct {
	ID       string
	Request  TaskRequest
	WebConn  *WebConnection
	ProbeIDs map[string]bool
}

func NewServer() *Server {
	return &Server{
		probes:     make(map[string]*ProbeConnection),
		webClients: make(map[string]*WebConnection),
		tasks:      make(map[string]*Task),
	}
}

func (s *Server) handleProbeConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade probe connection: %v", err)
		return
	}

	probeConn := &ProbeConnection{
		Conn: conn,
		Send: make(chan Message, 256),
	}

	go s.probeWriter(probeConn)
	s.probeReader(probeConn)
}

func (s *Server) probeReader(pc *ProbeConnection) {
	defer func() {
		if pc.ID != "" {
			s.probesMu.Lock()
			delete(s.probes, pc.ID)
			s.probesMu.Unlock()
			log.Printf("Probe %s disconnected", pc.ID)
		}
		pc.Conn.Close()
		close(pc.Send)
	}()

	for {
		var msg Message
		err := pc.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Probe read error: %v", err)
			}
			break
		}

		switch msg.Type {
		case MsgRegisterProbe:
			var info ProbeInfo
			if err := json.Unmarshal(msg.Payload, &info); err != nil {
				log.Printf("Failed to unmarshal probe info: %v", err)
				continue
			}
			info.Status = "online"
			info.LastSeen = time.Now()
			pc.ID = info.ID
			pc.Info = info

			s.probesMu.Lock()
			s.probes[pc.ID] = pc
			s.probesMu.Unlock()

			log.Printf("Probe registered: %s (%s)", info.ID, info.Name)

		case MsgProbeHeartbeat:
			if pc.ID != "" {
				s.probesMu.Lock()
				if probe, ok := s.probes[pc.ID]; ok {
					probe.Info.LastSeen = time.Now()
				}
				s.probesMu.Unlock()
			}

		case MsgTaskResult:
			var output TaskOutput
			if err := json.Unmarshal(msg.Payload, &output); err != nil {
				log.Printf("Failed to unmarshal task result: %v", err)
				continue
			}
			output.ProbeID = pc.ID

			// Forward to web client
			s.tasksMu.RLock()
			task, ok := s.tasks[output.TaskID]
			s.tasksMu.RUnlock()

			if ok && task.WebConn != nil {
				payload, _ := json.Marshal(output)
				task.WebConn.Send <- Message{
					Type:    MsgTaskOutput,
					Payload: payload,
				}
			}

		case MsgTaskComplete:
			var complete TaskComplete
			if err := json.Unmarshal(msg.Payload, &complete); err != nil {
				log.Printf("Failed to unmarshal task complete: %v", err)
				continue
			}
			complete.ProbeID = pc.ID

			// Forward to web client
			s.tasksMu.Lock()
			task, ok := s.tasks[complete.TaskID]
			if ok {
				delete(task.ProbeIDs, pc.ID)
				if len(task.ProbeIDs) == 0 {
					delete(s.tasks, complete.TaskID)
				}
			}
			s.tasksMu.Unlock()

			if ok && task.WebConn != nil {
				payload, _ := json.Marshal(complete)
				task.WebConn.Send <- Message{
					Type:    MsgTaskComplete,
					Payload: payload,
				}
			}
		}
	}
}

func (s *Server) probeWriter(pc *ProbeConnection) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-pc.Send:
			if !ok {
				pc.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := pc.Conn.WriteJSON(msg); err != nil {
				log.Printf("Probe write error: %v", err)
				return
			}
		case <-ticker.C:
			if err := pc.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleWebConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade web connection: %v", err)
		return
	}

	webConn := &WebConnection{
		ID:   uuid.New().String(),
		Conn: conn,
		Send: make(chan Message, 256),
	}

	s.webMu.Lock()
	s.webClients[webConn.ID] = webConn
	s.webMu.Unlock()

	go s.webWriter(webConn)
	s.webReader(webConn)
}

func (s *Server) webReader(wc *WebConnection) {
	defer func() {
		s.webMu.Lock()
		delete(s.webClients, wc.ID)
		s.webMu.Unlock()
		wc.Conn.Close()
		close(wc.Send)
		log.Printf("Web client %s disconnected", wc.ID)
	}()

	for {
		var msg Message
		err := wc.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Web read error: %v", err)
			}
			break
		}

		switch msg.Type {
		case MsgListProbes:
			s.probesMu.RLock()
			probes := make([]ProbeInfo, 0, len(s.probes))
			for _, pc := range s.probes {
				probes = append(probes, pc.Info)
			}
			s.probesMu.RUnlock()

			payload, _ := json.Marshal(probes)
			wc.Send <- Message{
				Type:    MsgProbesList,
				Payload: payload,
			}

		case MsgExecuteTask:
			var req TaskRequest
			if err := json.Unmarshal(msg.Payload, &req); err != nil {
				log.Printf("Failed to unmarshal task request: %v", err)
				continue
			}

			taskID := uuid.New().String()
			task := &Task{
				ID:       taskID,
				Request:  req,
				WebConn:  wc,
				ProbeIDs: make(map[string]bool),
			}

			// Assign task to requested probes
			s.probesMu.RLock()
			for _, probeID := range req.ProbeIDs {
				if pc, ok := s.probes[probeID]; ok {
					task.ProbeIDs[probeID] = true

					assignment := TaskAssignment{
						TaskID:  taskID,
						Tool:    req.Tool,
						Target:  req.Target,
						Options: req.Options,
					}
					payload, _ := json.Marshal(assignment)
					pc.Send <- Message{
						Type:    MsgTaskAssignment,
						Payload: payload,
					}
				}
			}
			s.probesMu.RUnlock()

			if len(task.ProbeIDs) > 0 {
				s.tasksMu.Lock()
				s.tasks[taskID] = task
				s.tasksMu.Unlock()
			}
		}
	}
}

func (s *Server) webWriter(wc *WebConnection) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-wc.Send:
			if !ok {
				wc.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := wc.Conn.WriteJSON(msg); err != nil {
				log.Printf("Web write error: %v", err)
				return
			}
		case <-ticker.C:
			if err := wc.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func main() {
	server := NewServer()

	http.HandleFunc("/ws/probe", server.handleProbeConnection)
	http.HandleFunc("/ws/web", server.handleWebConnection)

	// Serve static files
	fs := http.FileServer(http.Dir("./frontend/dist"))
	http.Handle("/", fs)

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
