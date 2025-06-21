package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

// testReadyRequest tests the Ready request functionality by manually controlling plugin loading.
func testReadyRequest(ctx context.Context) error {
	pluginPath := filepath.Join("..", "plugins", "calculator", "calculator")
	loader := plugin.NewLoader(pluginPath, "calculator", "v1.0.0")

	fmt.Println("  Testing ready request functionality...")

	// Load the plugin (this will automatically wait for ready signal with fallback to request)
	if err := loader.Load(ctx); err != nil {
		return fmt.Errorf("failed to load Calculator plugin: %w", err)
	}
	defer func() {
		if err := loader.Close(); err != nil {
			log.Printf("Calculator loader close error: %v", err)
		}
	}()

	// Test that we can manually request ready signal
	fmt.Println("  Manually requesting ready signal...")
	if err := loader.RequestReady(); err != nil {
		return fmt.Errorf("failed to request ready signal: %w", err)
	}

	// Give some time for the ready signal to be processed
	time.Sleep(100 * time.Millisecond)

	// Test that the plugin is still functional after ready request
	type CalculateRequest struct {
		Operation string  `json:"operation"`
		A         float64 `json:"a"`
		B         float64 `json:"b"`
	}
	type CalculateResponse struct {
		Result float64 `json:"result"`
		Error  string  `json:"error,omitempty"`
	}

	adapter := plugin.NewJSONLoaderAdapter[CalculateRequest, CalculateResponse](loader)

	// Simple calculation test to verify functionality
	req := CalculateRequest{
		Operation: "add",
		A:         5,
		B:         3,
	}

	resp, err := adapter.Call(ctx, "Calculate", req)
	if err != nil {
		return fmt.Errorf("calculation failed after ready request: %w", err)
	}

	if resp.Result != 8 {
		return fmt.Errorf("unexpected calculation result: got %f, expected 8", resp.Result)
	}

	fmt.Printf("  Ready request test completed successfully: 5 + 3 = %g\n", resp.Result)
	return nil
}
