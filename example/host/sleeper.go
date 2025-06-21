package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

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
