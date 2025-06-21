package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

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
