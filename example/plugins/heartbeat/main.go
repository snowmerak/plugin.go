package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

// HeartbeatData represents the data sent every second
type HeartbeatData struct {
	Timestamp time.Time `json:"timestamp"`
	Counter   int       `json:"counter"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
}

func main() {
	// Create a new module
	module := plugin.NewStd()

	// Register handlers for requests from the host
	plugin.RegisterHandler(module, "echo", handleEcho)
	plugin.RegisterHandler(module, "get_info", handleGetInfo)
	plugin.RegisterHandler(module, "stop", handleStop)

	// Start heartbeat goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel to signal when to stop
	stopChan := make(chan struct{})

	// Start heartbeat sender
	go func() {
		sendHeartbeat(ctx, module, stopChan)
	}()

	// Listen for incoming requests
	if err := module.Listen(ctx); err != nil {
		log.Printf("Module listen error: %v", err)
	}

	// Signal heartbeat to stop
	close(stopChan)
}

// sendHeartbeat sends data every second while connected
func sendHeartbeat(ctx context.Context, module *plugin.Module, stopChan <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	counter := 0

	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled, stopping heartbeat")
			return
		case <-stopChan:
			log.Println("Stop signal received, stopping heartbeat")
			return
		case <-ticker.C:
			counter++

			// Create heartbeat data
			heartbeatData := HeartbeatData{
				Timestamp: time.Now(),
				Counter:   counter,
				Status:    "active",
				Message:   fmt.Sprintf("Heartbeat #%d from plugin", counter),
			}

			// Convert to JSON
			data, err := json.Marshal(heartbeatData)
			if err != nil {
				log.Printf("Failed to marshal heartbeat data: %v", err)
				continue
			}

			// Send heartbeat message to host
			if err := module.SendMessage(ctx, "heartbeat", data); err != nil {
				log.Printf("Failed to send heartbeat: %v", err)
				// Continue trying to send heartbeats even if one fails
				continue
			}

			log.Printf("Sent heartbeat #%d at %s", counter, heartbeatData.Timestamp.Format("15:04:05"))
		}
	}
}

// handleEcho handles echo requests from the host
func handleEcho(requestPayload []byte) (responsePayload []byte, isAppError bool) {
	var request map[string]string
	if err := json.Unmarshal(requestPayload, &request); err != nil {
		errorMsg := fmt.Sprintf("Failed to unmarshal echo request: %v", err)
		return []byte(errorMsg), true
	}

	// Echo back the message with additional info
	response := map[string]string{
		"original_message": request["message"],
		"echo_response":    fmt.Sprintf("Echo: %s", request["message"]),
		"timestamp":        time.Now().Format(time.RFC3339),
		"plugin":           "heartbeat",
	}

	data, err := json.Marshal(response)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to marshal echo response: %v", err)
		return []byte(errorMsg), true
	}

	return data, false
}

// handleGetInfo handles info requests from the host
func handleGetInfo(requestPayload []byte) (responsePayload []byte, isAppError bool) {
	info := map[string]interface{}{
		"plugin_name":        "heartbeat",
		"version":            "1.0.0",
		"description":        "A plugin that sends heartbeat data every second",
		"capabilities":       []string{"heartbeat", "echo", "info"},
		"uptime":             time.Now().Format(time.RFC3339),
		"heartbeat_interval": "1s",
	}

	data, err := json.Marshal(info)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to marshal info response: %v", err)
		return []byte(errorMsg), true
	}

	return data, false
}

// handleStop handles stop requests from the host
func handleStop(requestPayload []byte) (responsePayload []byte, isAppError bool) {
	log.Println("Received stop request from host")

	response := map[string]string{
		"message":   "Stop request acknowledged",
		"timestamp": time.Now().Format(time.RFC3339),
		"status":    "stopping",
	}

	data, err := json.Marshal(response)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to marshal stop response: %v", err)
		return []byte(errorMsg), true
	}

	// Note: In a real implementation, you might want to signal the main goroutine to shut down
	// For this example, we'll just acknowledge the stop request

	return data, false
}
