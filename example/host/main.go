package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

func main() {
	fmt.Println("=== Plugin.go Host Application Started ===")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Echo plugin test
	fmt.Println("\n--- Echo Plugin Test ---")
	if err := testEchoPlugin(ctx); err != nil {
		log.Printf("Echo plugin test failed: %v", err)
	}

	// Calculator plugin test
	fmt.Println("\n--- Calculator Plugin Test ---")
	if err := testCalculatorPlugin(ctx); err != nil {
		log.Printf("Calculator plugin test failed: %v", err)
	}

	// Sleeper plugin test (Graceful Shutdown test)
	fmt.Println("\n--- Sleeper Plugin Test (Graceful Shutdown) ---")
	if err := testSleeperPlugin(ctx); err != nil {
		log.Printf("Sleeper plugin test failed: %v", err)
	}

	// Force Shutdown test
	fmt.Println("\n--- Force Shutdown Test ---")
	if err := testForceShutdown(ctx); err != nil {
		log.Printf("Force shutdown test failed: %v", err)
	}

	fmt.Println("\n=== All Tests Completed ===")
}

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

// testSleeperPlugin tests the Sleeper plugin functionality (Graceful Shutdown test).
func testSleeperPlugin(ctx context.Context) error {
	pluginPath := filepath.Join("..", "plugins", "sleeper", "sleeper")
	loader := plugin.NewLoader(pluginPath, "sleeper", "v1.0.0")

	if err := loader.Load(ctx); err != nil {
		return fmt.Errorf("failed to load Sleeper plugin: %w", err)
	}

	// Create JSON adapter
	type SleeperRequest struct {
		Message   string `json:"message"`
		SleepTime int    `json:"sleep_time"`
	}
	type SleeperResponse struct {
		Message   string `json:"message"`
		SleptTime int    `json:"slept_time"`
	}

	adapter := plugin.NewJSONLoaderAdapter[SleeperRequest, SleeperResponse](loader)

	// Test cases: Start long-running tasks simultaneously
	testCases := []struct {
		name    string
		request SleeperRequest
	}{
		{"5-second task", SleeperRequest{Message: "Task1", SleepTime: 5}},
		{"3-second task", SleeperRequest{Message: "Task2", SleepTime: 3}},
		{"4-second task", SleeperRequest{Message: "Task3", SleepTime: 4}},
		{"6-second task", SleeperRequest{Message: "Task4", SleepTime: 6}},
	}
	fmt.Println("  Starting long-running tasks simultaneously...")

	// Channel to collect results
	results := make(chan string, len(testCases)*2) // Sufficient buffer size

	// Start all tasks simultaneously
	for i, tc := range testCases {
		go func(index int, testCase struct {
			name    string
			request SleeperRequest
		}) {
			fmt.Printf("  Starting task %d (%s)...\n", index+1, testCase.name)
			start := time.Now()
			resp, err := adapter.Call(ctx, "Sleep", testCase.request)
			elapsed := time.Since(start)

			if err != nil {
				results <- fmt.Sprintf("    Failed %s: %v (time: %.1fs)", testCase.name, err, elapsed.Seconds())
			} else {
				results <- fmt.Sprintf("    Completed %s: %s (actual time: %.1fs)",
					testCase.name, resp.Message, elapsed.Seconds())
			}
		}(i, tc)
	}

	// Wait longer for tasks to be actually delivered to plugin
	fmt.Println("  Waiting for tasks to be delivered to plugin...")
	time.Sleep(4 * time.Second) // Wait to ensure tasks are actually started in plugin

	// Key: Start graceful shutdown while tasks are still in progress
	fmt.Println("  Starting Graceful Shutdown test while tasks are in progress")
	shutdownStart := time.Now()

	// Execute graceful shutdown
	if err := loader.Close(); err != nil {
		log.Printf("Sleeper loader close error: %v", err)
	}

	shutdownElapsed := time.Since(shutdownStart)
	fmt.Printf("  Graceful Shutdown completed - total time: %.2fs\n", shutdownElapsed.Seconds())

	// Collect results after shutdown (non-blocking)
	fmt.Println("  Task results:")
	timeout := time.After(1 * time.Second) // Wait only 1 more second after shutdown
	collectedResults := 0

loop:
	for collectedResults < len(testCases) {
		select {
		case result := <-results:
			fmt.Println(result)
			collectedResults++
		case <-timeout:
			fmt.Printf("  Timeout: only %d/%d results collected\n", collectedResults, len(testCases))
			break loop
		}
	}

	// Verify that graceful shutdown worked properly
	expectedMinTime := 3.0 // Should wait at least 3 seconds (shortest task time)

	// Note: loader.Close() only measures ACK waiting time
	// The actual plugin waits for tasks with a 5-second timeout
	fmt.Printf("  Analysis:\n")
	fmt.Printf("    - Host-side shutdown time: %.2fs (ACK waiting time)\n", shutdownElapsed.Seconds())
	fmt.Printf("    - Plugin-side waits for task completion with 5s timeout\n")

	if shutdownElapsed.Seconds() < 0.1 {
		fmt.Printf("  Graceful shutdown confirmed: plugin responded with immediate ACK (waiting for tasks with 5s timeout)\n")
	} else if shutdownElapsed.Seconds() >= expectedMinTime {
		fmt.Printf("  Graceful shutdown properly waited for tasks (%.1fs)\n", shutdownElapsed.Seconds())
	} else {
		fmt.Printf("  Warning: shutdown faster than expected (%.1fs < %.1fs)\n", shutdownElapsed.Seconds(), expectedMinTime)
	}

	return nil
}

// testForceShutdown tests the Force Shutdown functionality.
func testForceShutdown(ctx context.Context) error {
	pluginPath := filepath.Join("..", "plugins", "sleeper", "sleeper")
	loader := plugin.NewLoader(pluginPath, "sleeper", "v1.0.0")

	if err := loader.Load(ctx); err != nil {
		return fmt.Errorf("failed to load Sleeper plugin for force shutdown test: %w", err)
	}

	// Create JSON adapter
	type SleeperRequest struct {
		Message   string `json:"message"`
		SleepTime int    `json:"sleep_time"`
	}
	type SleeperResponse struct {
		Message   string `json:"message"`
		SleptTime int    `json:"slept_time"`
	}

	adapter := plugin.NewJSONLoaderAdapter[SleeperRequest, SleeperResponse](loader)

	// Start very long tasks
	testCases := []struct {
		name    string
		request SleeperRequest
	}{
		{"20-second task", SleeperRequest{Message: "LongTask1", SleepTime: 20}},
		{"15-second task", SleeperRequest{Message: "LongTask2", SleepTime: 15}},
		{"25-second task", SleeperRequest{Message: "LongTask3", SleepTime: 25}},
	}

	fmt.Println("  Starting very long tasks...")

	// Channel to collect results
	results := make(chan string, len(testCases))

	// Start all tasks simultaneously
	for i, tc := range testCases {
		go func(index int, testCase struct {
			name    string
			request SleeperRequest
		}) {
			fmt.Printf("  Starting task %d (%s)...\n", index+1, testCase.name)
			start := time.Now()
			resp, err := adapter.Call(ctx, "Sleep", testCase.request)
			elapsed := time.Since(start)

			if err != nil {
				results <- fmt.Sprintf("    Failed %s: %v (time: %.1fs)", testCase.name, err, elapsed.Seconds())
			} else {
				results <- fmt.Sprintf("    Completed %s: %s (actual time: %.1fs)",
					testCase.name, resp.Message, elapsed.Seconds())
			}
		}(i, tc)
	}

	// Wait for tasks to definitely start
	fmt.Println("  Waiting for tasks to start...")
	time.Sleep(2 * time.Second)

	// Execute Force Shutdown
	fmt.Println("  Executing Force Shutdown! (tasks in progress)")
	forceStart := time.Now()

	// Use ForceClose
	if err := loader.ForceClose(); err != nil {
		log.Printf("Force close error: %v", err)
	}

	forceElapsed := time.Since(forceStart)
	fmt.Printf("  Force Shutdown completed - total time: %.2fs\n", forceElapsed.Seconds())

	// Collect results (short timeout)
	fmt.Println("  Task results:")
	timeout := time.After(1 * time.Second)
	collectedResults := 0

loop:
	for collectedResults < len(testCases) {
		select {
		case result := <-results:
			fmt.Println(result)
			collectedResults++
		case <-timeout:
			fmt.Printf("  Timeout: only %d/%d results collected (interrupted by force shutdown)\n", collectedResults, len(testCases))
			break loop
		}
	}

	// Verify force shutdown
	fmt.Printf("  Analysis:\n")
	fmt.Printf("    - Force shutdown time: %.2fs\n", forceElapsed.Seconds())

	if forceElapsed.Seconds() < 2.0 {
		fmt.Printf("  Force shutdown executed quickly (< 2s)\n")
	} else {
		fmt.Printf("  Force shutdown took longer than expected (%.1fs)\n", forceElapsed.Seconds())
	}

	return nil
}
