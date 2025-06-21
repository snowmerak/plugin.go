package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

func main() {
	fmt.Println("=== Simple Plugin Test ===")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test Echo plugin
	fmt.Println("Loading Echo plugin...")
	loader := plugin.NewLoader("./plugins/echo/echo", "echo", "v1.0.0")

	if err := loader.Load(ctx); err != nil {
		log.Fatalf("Failed to load plugin: %v", err)
	}
	defer func() {
		if err := loader.Close(); err != nil {
			log.Printf("Error closing loader: %v", err)
		}
	}()

	// Create JSON adapter
	type EchoRequest struct {
		Message string `json:"message"`
	}
	type EchoResponse struct {
		Echo string `json:"echo"`
	}

	adapter := plugin.NewJSONLoaderAdapter[EchoRequest, EchoResponse](loader)

	// Simple test
	fmt.Println("Calling Echo service...")
	req := EchoRequest{Message: "Hello, Plugin!"}
	resp, err := adapter.Call(ctx, "Echo", req)
	if err != nil {
		log.Fatalf("Call failed: %v", err)
	}

	fmt.Printf("Request: %s\n", req.Message)
	fmt.Printf("Response: %s\n", resp.Echo)
	fmt.Println("Test completed!")
}
