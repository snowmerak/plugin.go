package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

// EchoRequest represents the request structure for the echo service.
type EchoRequest struct {
	Message string `json:"message"`
}

// EchoResponse represents the response structure for the echo service.
type EchoResponse struct {
	Echo string `json:"echo"`
}

// Message types for bidirectional communication
type NotificationMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Time    string `json:"time"`
}

type StatusRequest struct {
	Component string `json:"component"`
}

type StatusResponse struct {
	Component string `json:"component"`
	Status    string `json:"status"`
	Uptime    string `json:"uptime"`
}

// handleEcho processes echo requests and returns the message with "Echo: " prefix.
func handleEcho(req EchoRequest) (EchoResponse, bool) {
	response := EchoResponse{
		Echo: "Echo: " + req.Message,
	}

	return response, false // false = success
}

// handleCommand processes command messages from the loader
func handleCommand(ctx context.Context, module *plugin.Module, command map[string]interface{}) error {
	action, ok := command["action"].(string)
	if !ok {
		return fmt.Errorf("invalid command: missing action")
	}

	fmt.Printf("📨 Module received command: %s\n", action)

	switch action {
	case "start_monitoring":
		// Send a notification back to the loader
		notification := NotificationMessage{
			Type:    "info",
			Message: "Monitoring started successfully",
			Time:    time.Now().Format(time.RFC3339),
		}

		if data, err := json.Marshal(notification); err == nil {
			module.SendMessage(ctx, "notification", data)
		}

	case "update_status":
		component, _ := command["component"].(string)
		status, _ := command["status"].(string)

		// Send a notification about the status update
		notification := NotificationMessage{
			Type:    "status",
			Message: fmt.Sprintf("Component %s status updated to %s", component, status),
			Time:    time.Now().Format(time.RFC3339),
		}

		if data, err := json.Marshal(notification); err == nil {
			module.SendMessage(ctx, "notification", data)
		}
	}

	return nil
}

func main() {
	// Create module for communication with host via stdin/stdout
	module := plugin.New(os.Stdin, os.Stdout)

	// Define JSON serialization/deserialization functions for echo service
	unmarshalReq := func(data []byte) (EchoRequest, error) {
		var req EchoRequest
		err := json.Unmarshal(data, &req)
		return req, err
	}

	marshalResp := func(resp EchoResponse) ([]byte, error) {
		return json.Marshal(resp)
	}

	// Create handler adapter for echo service
	echoAdapter := plugin.NewHandlerAdapter(
		"Echo",       // Service name (changed to lowercase to match the test)
		unmarshalReq, // Request unmarshaling function
		marshalResp,  // Response marshaling function
		handleEcho,   // Actual handler logic
	)

	// Register echo handler with module
	plugin.RegisterHandler(module, "Echo", echoAdapter.ToPluginHandler())

	// Register a handler for command messages from the loader
	plugin.RegisterHandler(module, "command", func(requestPayload []byte) (responsePayload []byte, isAppError bool) {
		var command map[string]interface{}
		if err := json.Unmarshal(requestPayload, &command); err != nil {
			errMsg := fmt.Sprintf("Failed to unmarshal command: %v", err)
			return []byte(errMsg), true
		}

		// Handle the command (this doesn't expect a response, so we just process it)
		ctx := context.Background()
		if err := handleCommand(ctx, module, command); err != nil {
			errMsg := fmt.Sprintf("Command handling failed: %v", err)
			return []byte(errMsg), true
		}

		// Return empty success response for command processing
		return []byte("command processed"), false
	})

	// Send a startup notification
	go func() {
		time.Sleep(500 * time.Millisecond) // Wait a bit for everything to be set up

		notification := NotificationMessage{
			Type:    "info",
			Message: "Echo plugin started and ready for bidirectional communication",
			Time:    time.Now().Format(time.RFC3339),
		}

		if data, err := json.Marshal(notification); err == nil {
			module.SendMessage(context.Background(), "notification", data)
		}
	}()

	// Process requests in infinite loop (ready signal is sent automatically)
	if err := module.Listen(context.Background()); err != nil {
		os.Exit(1)
	}
}
