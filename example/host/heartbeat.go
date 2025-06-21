package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

// HeartbeatData represents the data received from heartbeat plugin
type HeartbeatData struct {
	Timestamp time.Time `json:"timestamp"`
	Counter   int       `json:"counter"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
}

// testHeartbeatPlugin demonstrates the heartbeat plugin that sends data every second
func testHeartbeatPlugin(ctx context.Context) error {
	fmt.Println("Starting heartbeat plugin test...")

	// Create and load heartbeat plugin
	loader := plugin.NewLoader("../plugins/heartbeat/heartbeat", "heartbeat", "1.0.0")
	if err := loader.Load(ctx); err != nil {
		return fmt.Errorf("failed to load heartbeat plugin: %w", err)
	}
	defer loader.Close()

	// Register a message handler to receive heartbeat messages
	heartbeatHandler := plugin.NewJSONLoaderMessageHandlerAdapter[HeartbeatData](
		loader,
		"heartbeat",
		func(ctx context.Context, heartbeat HeartbeatData) error {
			fmt.Printf("💓 Heartbeat #%d: %s (Status: %s) at %s\n",
				heartbeat.Counter,
				heartbeat.Message,
				heartbeat.Status,
				heartbeat.Timestamp.Format("15:04:05"))
			return nil
		},
	)
	defer heartbeatHandler.Unregister()

	// Create request senders for testing plugin functionality
	echoSender := plugin.NewJSONLoaderRequestSender[map[string]string, map[string]interface{}](
		loader,
		"echo",
	)

	infoSender := plugin.NewJSONLoaderRequestSender[map[string]string, map[string]interface{}](
		loader,
		"get_info",
	)

	// Give the plugin a moment to initialize and start sending heartbeats
	fmt.Println("🔸 Waiting for heartbeat messages...")
	time.Sleep(3 * time.Second)

	// Test echo functionality
	fmt.Println("\n🔸 Testing echo functionality...")
	echoRequest := map[string]string{
		"message": "Hello from host to heartbeat plugin!",
	}

	echoResponse, err := echoSender.SendRequest(ctx, echoRequest)
	if err != nil {
		log.Printf("Echo request failed: %v", err)
	} else {
		fmt.Printf("✅ Echo response: %s\n", echoResponse["echo_response"])
	}

	// Test info functionality
	fmt.Println("\n🔸 Getting plugin info...")
	infoResponse, err := infoSender.SendRequest(ctx, map[string]string{})
	if err != nil {
		log.Printf("Info request failed: %v", err)
	} else {
		fmt.Printf("📋 Plugin info: %s v%s\n", infoResponse["plugin_name"], infoResponse["version"])
		fmt.Printf("📋 Description: %s\n", infoResponse["description"])
		fmt.Printf("📋 Heartbeat interval: %s\n", infoResponse["heartbeat_interval"])
	}

	// Wait and observe more heartbeats
	fmt.Println("\n⏳ Observing heartbeats for 10 seconds...")
	time.Sleep(10 * time.Second)

	fmt.Println("✅ Heartbeat plugin test completed!")
	return nil
}
