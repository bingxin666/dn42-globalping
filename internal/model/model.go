package model

import "time"

// ProbeInfo represents a probe node's registration information
type ProbeInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Location  string    `json:"location"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Status    string    `json:"status"` // online, offline
	LastSeen  time.Time `json:"last_seen"`
}

// MessageType defines the type of WebSocket message
type MessageType string

const (
	MsgTypeRegister    MessageType = "register"
	MsgTypeTask        MessageType = "task"
	MsgTypeTaskResult  MessageType = "task_result"
	MsgTypeHeartbeat   MessageType = "heartbeat"
	MsgTypeProbeList   MessageType = "probe_list"
	MsgTypeTaskCreate  MessageType = "task_create"
	MsgTypeTaskStream  MessageType = "task_stream"
	MsgTypeTaskEnd     MessageType = "task_end"
	MsgTypeError       MessageType = "error"
)

// Message is the base WebSocket message structure
type Message struct {
	Type    MessageType `json:"type"`
	Payload interface{} `json:"payload"`
}

// RegisterPayload is sent by probe to register with server
type RegisterPayload struct {
	Name      string  `json:"name"`
	Location  string  `json:"location"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// TaskPayload is sent by server to probe to execute a task
type TaskPayload struct {
	TaskID  string `json:"task_id"`
	Type    string `json:"type"` // ping, traceroute, mtr
	Target  string `json:"target"`
	Options string `json:"options,omitempty"`
}

// TaskResultPayload is sent by probe to server with task results
type TaskResultPayload struct {
	TaskID  string `json:"task_id"`
	ProbeID string `json:"probe_id"`
	Line    string `json:"line"`
	IsEnd   bool   `json:"is_end"`
	Error   string `json:"error,omitempty"`
}

// TaskCreatePayload is sent by web client to create a new task
type TaskCreatePayload struct {
	ProbeIDs []string `json:"probe_ids"`
	Type     string   `json:"type"` // ping, traceroute, mtr
	Target   string   `json:"target"`
	Options  string   `json:"options,omitempty"`
}

// TaskStreamPayload is sent to web client with streaming results
type TaskStreamPayload struct {
	TaskID    string `json:"task_id"`
	ProbeID   string `json:"probe_id"`
	ProbeName string `json:"probe_name"`
	Line      string `json:"line"`
	IsEnd     bool   `json:"is_end"`
	Error     string `json:"error,omitempty"`
}

// ProbeListPayload contains the list of available probes
type ProbeListPayload struct {
	Probes []ProbeInfo `json:"probes"`
}

// ErrorPayload contains error information
type ErrorPayload struct {
	Message string `json:"message"`
}
