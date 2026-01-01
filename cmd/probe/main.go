package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bingxin666/dn42-globalping/internal/model"
	"github.com/gorilla/websocket"
)

var (
	serverURL = flag.String("server", "ws://localhost:8080/ws/probe", "WebSocket server URL")
	probeName = flag.String("name", "probe-1", "Probe name")
	location  = flag.String("location", "Beijing, China", "Probe location")
	latitude  = flag.Float64("lat", 39.9042, "Latitude")
	longitude = flag.Float64("lon", 116.4074, "Longitude")
)

type ProbeClient struct {
	conn    *websocket.Conn
	probeID string
	sendCh  chan []byte
}

func main() {
	flag.Parse()

	client := &ProbeClient{
		sendCh: make(chan []byte, 256),
	}

	// Connect to server
	if err := client.connect(); err != nil {
		log.Fatal(err)
	}
	defer client.conn.Close()

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start writer goroutine
	go client.writer()

	// Start heartbeat goroutine
	go client.heartbeat()

	// Start reader in main goroutine
	go client.reader()

	// Wait for signal
	<-sigCh
	log.Println("Shutting down...")
}

func (c *ProbeClient) connect() error {
	log.Printf("Connecting to %s", *serverURL)

	conn, _, err := websocket.DefaultDialer.Dial(*serverURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	c.conn = conn

	// Send registration message
	registerMsg := model.Message{
		Type: model.MsgTypeRegister,
		Payload: model.RegisterPayload{
			Name:      *probeName,
			Location:  *location,
			Latitude:  *latitude,
			Longitude: *longitude,
		},
	}
	data, _ := json.Marshal(registerMsg)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("failed to send registration: %w", err)
	}

	// Wait for registration response
	_, message, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read registration response: %w", err)
	}

	var msg model.Message
	if err := json.Unmarshal(message, &msg); err != nil {
		return fmt.Errorf("failed to parse registration response: %w", err)
	}

	if msg.Type == model.MsgTypeRegister {
		payloadBytes, _ := json.Marshal(msg.Payload)
		var payload map[string]string
		json.Unmarshal(payloadBytes, &payload)
		c.probeID = payload["probe_id"]
		log.Printf("Registered with ID: %s", c.probeID)
	}

	return nil
}

func (c *ProbeClient) reader() {
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Connection error: %v", err)
			}
			return
		}

		var msg model.Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		switch msg.Type {
		case model.MsgTypeTask:
			payloadBytes, _ := json.Marshal(msg.Payload)
			var taskPayload model.TaskPayload
			if err := json.Unmarshal(payloadBytes, &taskPayload); err != nil {
				log.Printf("Failed to parse task payload: %v", err)
				continue
			}
			log.Printf("Received task: %s - %s %s", taskPayload.TaskID, taskPayload.Type, taskPayload.Target)
			go c.executeTask(taskPayload)
		}
	}
}

func (c *ProbeClient) writer() {
	for message := range c.sendCh {
		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Printf("Failed to send message: %v", err)
			return
		}
	}
}

func (c *ProbeClient) heartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		msg := model.Message{
			Type:    model.MsgTypeHeartbeat,
			Payload: nil,
		}
		data, _ := json.Marshal(msg)
		c.sendCh <- data
	}
}

func (c *ProbeClient) executeTask(task model.TaskPayload) {
	var cmd *exec.Cmd

	switch task.Type {
	case "ping":
		// Use -c flag for count on Linux/Mac
		args := []string{"-c", "10"}
		if task.Options != "" {
			args = append(args, strings.Fields(task.Options)...)
		}
		args = append(args, task.Target)
		cmd = exec.Command("ping", args...)
	case "traceroute":
		args := []string{}
		if task.Options != "" {
			args = append(args, strings.Fields(task.Options)...)
		}
		args = append(args, task.Target)
		cmd = exec.Command("traceroute", args...)
	case "mtr":
		args := []string{"-r", "-c", "10", "--no-dns"}
		if task.Options != "" {
			args = append(args, strings.Fields(task.Options)...)
		}
		args = append(args, task.Target)
		cmd = exec.Command("mtr", args...)
	default:
		c.sendResult(task.TaskID, "", true, fmt.Sprintf("Unknown task type: %s", task.Type))
		return
	}

	// Create pipe for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.sendResult(task.TaskID, "", true, err.Error())
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		c.sendResult(task.TaskID, "", true, err.Error())
		return
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		c.sendResult(task.TaskID, "", true, err.Error())
		return
	}

	// Read stdout line by line and send
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			c.sendResult(task.TaskID, line, false, "")
		}
	}()

	// Read stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			c.sendResult(task.TaskID, line, false, "")
		}
	}()

	// Wait for command to finish
	err = cmd.Wait()
	if err != nil {
		c.sendResult(task.TaskID, "", true, err.Error())
	} else {
		c.sendResult(task.TaskID, "", true, "")
	}
}

func (c *ProbeClient) sendResult(taskID, line string, isEnd bool, errMsg string) {
	msg := model.Message{
		Type: model.MsgTypeTaskResult,
		Payload: model.TaskResultPayload{
			TaskID: taskID,
			Line:   line,
			IsEnd:  isEnd,
			Error:  errMsg,
		},
	}
	data, _ := json.Marshal(msg)
	c.sendCh <- data
}
