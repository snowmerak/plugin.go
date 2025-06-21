package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

// ExamplePlugin demonstrates bidirectional communication
type ExamplePlugin struct {
	loader *plugin.Loader
}

// TaskRequest represents a task that the loader sends to the plugin
type TaskRequest struct {
	ID         string                 `json:"id"`
	Operation  string                 `json:"operation"`
	Parameters map[string]interface{} `json:"parameters"`
}

// TaskResponse represents the plugin's response to a task
type TaskResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result"`
	Error  string      `json:"error,omitempty"`
}

// StatusUpdate represents status updates sent from plugin to loader
type StatusUpdate struct {
	PluginID string `json:"plugin_id"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	Progress int    `json:"progress"`
}

func main() {
	// Create and start the plugin
	plugin := &ExamplePlugin{}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := plugin.start(ctx); err != nil {
		log.Fatalf("Failed to start plugin: %v", err)
	}
}

func (p *ExamplePlugin) start(ctx context.Context) error {
	// Create a new loader (simulating the host application)
	loader, err := plugin.NewLoader()
	if err != nil {
		return fmt.Errorf("failed to create loader: %w", err)
	}
	defer loader.Close()

	p.loader = loader

	// Register handlers for messages from plugins
	loader.RegisterMessageHandler("status_update", p.handleStatusUpdate)
	loader.RegisterMessageHandler("progress", p.handleProgress)
	loader.RegisterRequestHandler("data_request", p.handleDataRequest)

	// Start a mock plugin process (in real scenario, this would be a separate executable)
	go p.simulatePlugin(ctx)

	// Simulate periodic tasks
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	taskID := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			taskID++
			if err := p.sendTask(fmt.Sprintf("task_%d", taskID)); err != nil {
				log.Printf("Failed to send task: %v", err)
			}

			if taskID >= 5 {
				log.Println("Demo completed successfully!")
				return nil
			}
		}
	}
}

func (p *ExamplePlugin) sendTask(taskID string) error {
	task := TaskRequest{
		ID:        taskID,
		Operation: "process_data",
		Parameters: map[string]interface{}{
			"input": fmt.Sprintf("data_for_%s", taskID),
			"config": map[string]interface{}{
				"timeout": 5,
				"retries": 3,
			},
		},
	}

	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	log.Printf("Sending task: %s", taskID)

	// Send as a request expecting a response
	response, err := p.loader.SendRequest(context.Background(), "execute_task", data)
	if err != nil {
		return fmt.Errorf("failed to send task request: %w", err)
	}

	var taskResponse TaskResponse
	if err := json.Unmarshal(response, &taskResponse); err != nil {
		return fmt.Errorf("failed to unmarshal task response: %w", err)
	}

	if taskResponse.Error != "" {
		log.Printf("Task %s failed: %s", taskID, taskResponse.Error)
	} else {
		log.Printf("Task %s completed successfully: %v", taskID, taskResponse.Result)
	}

	return nil
}

func (p *ExamplePlugin) handleStatusUpdate(data []byte) error {
	var status StatusUpdate
	if err := json.Unmarshal(data, &status); err != nil {
		return fmt.Errorf("failed to unmarshal status update: %w", err)
	}

	log.Printf("Plugin status update: %s - %s (%d%%)", status.Status, status.Message, status.Progress)
	return nil
}

func (p *ExamplePlugin) handleProgress(data []byte) error {
	var progress struct {
		TaskID   string `json:"task_id"`
		Progress int    `json:"progress"`
	}

	if err := json.Unmarshal(data, &progress); err != nil {
		return fmt.Errorf("failed to unmarshal progress: %w", err)
	}

	log.Printf("Task %s progress: %d%%", progress.TaskID, progress.Progress)
	return nil
}

func (p *ExamplePlugin) handleDataRequest(data []byte) ([]byte, error) {
	var request struct {
		Type       string                 `json:"type"`
		Parameters map[string]interface{} `json:"parameters"`
	}

	if err := json.Unmarshal(data, &request); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data request: %w", err)
	}

	log.Printf("Handling data request: %s", request.Type)

	// Simulate providing requested data
	response := map[string]interface{}{
		"type":      request.Type,
		"data":      fmt.Sprintf("mock_data_for_%s", request.Type),
		"timestamp": time.Now().Unix(),
	}

	return json.Marshal(response)
}

// simulatePlugin simulates a plugin process that communicates bidirectionally
func (p *ExamplePlugin) simulatePlugin(ctx context.Context) {
	module := plugin.NewModule()

	// Register handlers for messages from the loader
	module.RegisterHandler("execute_task", p.handleExecuteTask)
	module.RegisterHandler("heartbeat", p.handleHeartbeat)

	// Send initial status
	statusData, _ := json.Marshal(StatusUpdate{
		PluginID: "example_plugin",
		Status:   "ready",
		Message:  "Plugin initialized successfully",
		Progress: 100,
	})

	if err := module.SendMessage("status_update", statusData); err != nil {
		log.Printf("Failed to send initial status: %v", err)
	}

	// Simulate periodic heartbeat and status updates
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Send heartbeat status
			statusData, _ := json.Marshal(StatusUpdate{
				PluginID: "example_plugin",
				Status:   "active",
				Message:  "Plugin is running normally",
				Progress: 100,
			})

			if err := module.SendMessage("status_update", statusData); err != nil {
				log.Printf("Failed to send heartbeat status: %v", err)
			}

			// Sometimes request data from the loader
			if time.Now().Unix()%10 == 0 {
				requestData, _ := json.Marshal(map[string]interface{}{
					"type": "configuration",
					"parameters": map[string]interface{}{
						"section": "database",
					},
				})

				response, err := module.SendRequest("data_request", requestData)
				if err != nil {
					log.Printf("Failed to request data: %v", err)
				} else {
					log.Printf("Received data from loader: %s", string(response))
				}
			}
		}
	}
}

func (p *ExamplePlugin) handleExecuteTask(data []byte) ([]byte, error) {
	var task TaskRequest
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	log.Printf("Plugin executing task: %s (%s)", task.ID, task.Operation)

	// Send progress updates
	for progress := 0; progress <= 100; progress += 25 {
		progressData, _ := json.Marshal(map[string]interface{}{
			"task_id":  task.ID,
			"progress": progress,
		})

		// Use the global module instance to send progress
		module := plugin.NewModule()
		if err := module.SendMessage("progress", progressData); err != nil {
			log.Printf("Failed to send progress: %v", err)
		}

		time.Sleep(100 * time.Millisecond) // Simulate work
	}

	// Simulate task execution
	result := map[string]interface{}{
		"processed_data": fmt.Sprintf("Result for %s", task.Parameters["input"]),
		"execution_time": "100ms",
		"status":         "completed",
	}

	response := TaskResponse{
		ID:     task.ID,
		Result: result,
	}

	return json.Marshal(response)
}

func (p *ExamplePlugin) handleHeartbeat(data []byte) ([]byte, error) {
	log.Println("Plugin received heartbeat")

	response := map[string]interface{}{
		"status":    "alive",
		"timestamp": time.Now().Unix(),
		"uptime":    "active",
	}

	return json.Marshal(response)
}
