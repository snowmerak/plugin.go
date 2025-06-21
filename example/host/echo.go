package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

// testEchoPlugin tests the Echo plugin functionality.
func testEchoPlugin(ctx context.Context) error {
	pluginPath := filepath.Join("..", "plugins", "echo", "echo")
	loader := plugin.NewLoader(pluginPath, "echo", "v1.0.0")

	if err := loader.Load(ctx); err != nil {
		return fmt.Errorf("failed to load Echo plugin: %w", err)
	}
	defer func() {
		if err := loader.Close(); err != nil {
			log.Printf("Echo loader close error: %v", err)
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

	// Test cases
	testCases := []struct {
		name    string
		request EchoRequest
	}{
		{"Basic message", EchoRequest{Message: "Hello!"}},
		{"English message", EchoRequest{Message: "Hello World!"}},
		{"Empty message", EchoRequest{Message: ""}},
		{"Long message", EchoRequest{Message: "This is a very long message. " +
			"Let's test if the plugin can handle long messages properly."}},
	}

	for _, tc := range testCases {
		fmt.Printf("  Test: %s\n", tc.name)
		resp, err := adapter.Call(ctx, "Echo", tc.request)
		if err != nil {
			fmt.Printf("    Error: %v\n", err)
			continue
		}
		fmt.Printf("    Request: %s\n", tc.request.Message)
		fmt.Printf("    Response: %s\n", resp.Echo)
	}

	return nil
}
