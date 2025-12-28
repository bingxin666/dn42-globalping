package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type MessageType string

const (
	// Probe to Server
	MsgRegisterProbe  MessageType = "register_probe"
	MsgProbeHeartbeat MessageType = "probe_heartbeat"
	MsgTaskResult     MessageType = "task_result"
	MsgTaskComplete   MessageType = "task_complete"

	// Server to Probe
	MsgTaskAssignment MessageType = "task_assignment"
)

type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type ProbeInfo struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Location string  `json:"location"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
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

type Probe struct {
	info       ProbeInfo
	serverURL  string
	conn       *websocket.Conn
	send       chan Message
	done       chan struct{}
	activeTasks map[string]context.CancelFunc
}

func NewProbe(serverURL string, info ProbeInfo) *Probe {
	if info.ID == "" {
		info.ID = uuid.New().String()
	}
	return &Probe{
		info:        info,
		serverURL:   serverURL,
		send:        make(chan Message, 256),
		done:        make(chan struct{}),
		activeTasks: make(map[string]context.CancelFunc),
	}
}

func (p *Probe) Connect() error {
	var err error
	p.conn, _, err = websocket.DefaultDialer.Dial(p.serverURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Register probe
	payload, _ := json.Marshal(p.info)
	if err := p.conn.WriteJSON(Message{
		Type:    MsgRegisterProbe,
		Payload: payload,
	}); err != nil {
		return fmt.Errorf("failed to register: %w", err)
	}

	log.Printf("Probe %s registered successfully", p.info.Name)
	return nil
}

func (p *Probe) Run() {
	go p.writer()
	go p.heartbeat()
	p.reader()
}

func (p *Probe) reader() {
	defer func() {
		close(p.done)
		p.conn.Close()
	}()

	for {
		var msg Message
		err := p.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Read error: %v", err)
			}
			break
		}

		switch msg.Type {
		case MsgTaskAssignment:
			var task TaskAssignment
			if err := json.Unmarshal(msg.Payload, &task); err != nil {
				log.Printf("Failed to unmarshal task: %v", err)
				continue
			}
			go p.executeTask(task)
		}
	}
}

func (p *Probe) writer() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-p.send:
			if !ok {
				p.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := p.conn.WriteJSON(msg); err != nil {
				log.Printf("Write error: %v", err)
				return
			}
		case <-ticker.C:
			if err := p.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-p.done:
			return
		}
	}
}

func (p *Probe) heartbeat() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.send <- Message{
				Type:    MsgProbeHeartbeat,
				Payload: json.RawMessage("{}"),
			}
		case <-p.done:
			return
		}
	}
}

func (p *Probe) executeTask(task TaskAssignment) {
	log.Printf("Executing task %s: %s %s", task.TaskID, task.Tool, task.Target)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	p.activeTasks[task.TaskID] = cancel
	defer delete(p.activeTasks, task.TaskID)

	var cmd *exec.Cmd
	switch task.Tool {
	case "ping":
		cmd = exec.CommandContext(ctx, "ping", "-c", "4", task.Target)
	case "traceroute":
		cmd = exec.CommandContext(ctx, "traceroute", task.Target)
	case "mtr":
		cmd = exec.CommandContext(ctx, "mtr", "-r", "-c", "10", task.Target)
	default:
		p.sendOutput(task.TaskID, fmt.Sprintf("Unknown tool: %s", task.Tool), true)
		p.sendComplete(task.TaskID, 1)
		return
	}

	// Check if command exists
	toolPath, err := exec.LookPath(task.Tool)
	if err != nil {
		p.sendOutput(task.TaskID, fmt.Sprintf("Tool %s not found. Please install it.", task.Tool), true)
		p.sendComplete(task.TaskID, 127)
		return
	}
	log.Printf("Using tool at: %s", toolPath)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		p.sendOutput(task.TaskID, fmt.Sprintf("Failed to create stdout pipe: %v", err), true)
		p.sendComplete(task.TaskID, 1)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		p.sendOutput(task.TaskID, fmt.Sprintf("Failed to create stderr pipe: %v", err), true)
		p.sendComplete(task.TaskID, 1)
		return
	}

	if err := cmd.Start(); err != nil {
		p.sendOutput(task.TaskID, fmt.Sprintf("Failed to start command: %v", err), true)
		p.sendComplete(task.TaskID, 1)
		return
	}

	// Read stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			p.sendOutput(task.TaskID, line, false)
		}
	}()

	// Read stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			p.sendOutput(task.TaskID, line, true)
		}
	}()

	exitCode := 0
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	p.sendComplete(task.TaskID, exitCode)
	log.Printf("Task %s completed with exit code %d", task.TaskID, exitCode)
}

func (p *Probe) sendOutput(taskID, line string, isError bool) {
	output := TaskOutput{
		TaskID:  taskID,
		ProbeID: p.info.ID,
		Line:    line,
		IsError: isError,
	}
	payload, _ := json.Marshal(output)
	p.send <- Message{
		Type:    MsgTaskResult,
		Payload: payload,
	}
}

func (p *Probe) sendComplete(taskID string, exitCode int) {
	complete := TaskComplete{
		TaskID:   taskID,
		ProbeID:  p.info.ID,
		ExitCode: exitCode,
	}
	payload, _ := json.Marshal(complete)
	p.send <- Message{
		Type:    MsgTaskComplete,
		Payload: payload,
	}
}

func main() {
	serverURL := flag.String("server", "ws://localhost:8080/ws/probe", "Server WebSocket URL")
	name := flag.String("name", "", "Probe name")
	location := flag.String("location", "", "Probe location")
	lat := flag.Float64("lat", 0, "Latitude")
	lng := flag.Float64("lng", 0, "Longitude")
	flag.Parse()

	if *name == "" {
		hostname, _ := os.Hostname()
		*name = hostname
		if *name == "" {
			*name = "probe-" + strings.Split(uuid.New().String(), "-")[0]
		}
	}

	if *location == "" {
		*location = "Unknown"
	}

	info := ProbeInfo{
		Name:     *name,
		Location: *location,
		Lat:      *lat,
		Lng:      *lng,
	}

	probe := NewProbe(*serverURL, info)

	for {
		err := probe.Connect()
		if err != nil {
			log.Printf("Connection failed: %v. Retrying in 5s...", err)
			time.Sleep(5 * time.Second)
			continue
		}

		probe.Run()
		log.Println("Disconnected. Reconnecting in 5s...")
		time.Sleep(5 * time.Second)
		probe = NewProbe(*serverURL, info)
	}
}
