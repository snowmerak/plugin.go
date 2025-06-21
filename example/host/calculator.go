package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

// testCalculatorPlugin tests the Calculator plugin functionality.
func testCalculatorPlugin(ctx context.Context) error {
	pluginPath := filepath.Join("..", "plugins", "calculator", "calculator")
	loader := plugin.NewLoader(pluginPath, "calculator", "v1.0.0")

	if err := loader.Load(ctx); err != nil {
		return fmt.Errorf("failed to load Calculator plugin: %w", err)
	}
	defer func() {
		if err := loader.Close(); err != nil {
			log.Printf("Calculator loader close error: %v", err)
		}
	}()

	// Create JSON adapter
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

	// Test cases
	testCases := []struct {
		name    string
		request CalculateRequest
	}{
		{"Addition", CalculateRequest{Operation: "add", A: 10, B: 5}},
		{"Subtraction", CalculateRequest{Operation: "subtract", A: 10, B: 3}},
		{"Multiplication", CalculateRequest{Operation: "multiply", A: 7, B: 6}},
		{"Division", CalculateRequest{Operation: "divide", A: 20, B: 4}},
		{"Division by zero", CalculateRequest{Operation: "divide", A: 10, B: 0}},
		{"Invalid operation", CalculateRequest{Operation: "invalid", A: 1, B: 2}},
	}

	for _, tc := range testCases {
		fmt.Printf("  Test: %s\n", tc.name)
		resp, err := adapter.Call(ctx, "Calculate", tc.request)
		if err != nil {
			fmt.Printf("    Plugin error: %v\n", err)
			continue
		}

		if resp.Error != "" {
			fmt.Printf("    Calculation error: %s\n", resp.Error)
		} else {
			fmt.Printf("    %g %s %g = %g\n", tc.request.A, tc.request.Operation, tc.request.B, resp.Result)
		}
	}

	return nil
}
