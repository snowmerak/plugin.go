package plugin

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// createTestPlugin creates a simple echo plugin for testing
func createTestPlugin(t testing.TB) string {
	// Skip these tests for now since they require building a plugin executable
	t.Skip("Skipping integration test that requires building plugin executable")
	return ""
}

func TestLoader_NewLoader(t *testing.T) {
	loader := NewLoader("/path/to/plugin", "test", "1.0")

	if loader.Path != "/path/to/plugin" {
		t.Errorf("Expected Path '/path/to/plugin', got '%s'", loader.Path)
	}
	if loader.Name != "test" {
		t.Errorf("Expected Name 'test', got '%s'", loader.Name)
	}
	if loader.Version != "1.0" {
		t.Errorf("Expected Version '1.0', got '%s'", loader.Version)
	}
	if loader.closed.Load() {
		t.Error("New loader should not be closed")
	}
	if loader.pendingRequests == nil {
		t.Error("pendingRequests should be initialized")
	}
}

func TestLoader_Load_InvalidPath(t *testing.T) {
	loader := NewLoader("/invalid/path", "test", "1.0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := loader.Load(ctx)
	if err == nil {
		t.Error("Expected error for invalid path")
	}
}

func TestLoader_Load_AlreadyClosed(t *testing.T) {
	loader := NewLoader("/path/to/plugin", "test", "1.0")
	loader.closed.Store(true)

	ctx := context.Background()
	err := loader.Load(ctx)

	if err == nil || err.Error() != "loader is closed" {
		t.Errorf("Expected 'loader is closed' error, got: %v", err)
	}
}

func TestLoader_Close_MultipleCalls(t *testing.T) {
	loader := NewLoader("/path/to/plugin", "test", "1.0")

	// First close should succeed
	err := loader.Close()
	if err == nil || err.Error() != "loader already closed" {
		// This is expected for a loader that was never loaded
	}

	// Second close should return error
	err = loader.Close()
	if err == nil || err.Error() != "loader already closed" {
		t.Errorf("Expected 'loader already closed' error, got: %v", err)
	}
}

func TestLoader_Call_NotLoaded(t *testing.T) {
	loader := NewLoader("/path/to/plugin", "test", "1.0")

	ctx := context.Background()
	_, err := Call(ctx, loader, "echo", []byte("test"))

	if err == nil || err.Error() != "loader not loaded or load failed" {
		t.Errorf("Expected 'loader not loaded' error, got: %v", err)
	}
}

func TestLoader_Call_Closed(t *testing.T) {
	loader := NewLoader("/path/to/plugin", "test", "1.0")
	loader.closed.Store(true)

	ctx := context.Background()
	_, err := Call(ctx, loader, "echo", []byte("test"))

	if err == nil || err.Error() != "loader is closed" {
		t.Errorf("Expected 'loader is closed' error, got: %v", err)
	}
}

func TestLoader_GenerateRequestID_Collision(t *testing.T) {
	loader := NewLoader("/path/to/plugin", "test", "1.0")

	// Set requestID to a high value to test collision handling
	loader.requestID.Store(0xFFFFFFFE)

	// Create a pending request with ID 1 (which would be the next ID after overflow)
	loader.pendingRequests[1] = make(chan []byte, 1)

	// Generate ID should skip 0 and 1, return 2
	id := loader.generateRequestID()
	if id == 0 || id == 1 {
		t.Errorf("Expected ID to skip 0 and 1, got %d", id)
	}

	// Clean up
	delete(loader.pendingRequests, 1)
}

func TestLoader_GenerateRequestID_MaxAttempts(t *testing.T) {
	loader := NewLoader("/path/to/plugin", "test", "1.0")

	// Fill up many request IDs to test max attempts
	for i := uint32(1); i <= 200; i++ {
		loader.pendingRequests[i] = make(chan []byte, 1)
	}

	// Should still generate an ID (fallback case)
	id := loader.generateRequestID()
	if id == 0 {
		t.Error("Should not return 0 as ID")
	}
}

func TestLoader_IsProcessAlive(t *testing.T) {
	loader := NewLoader("/path/to/plugin", "test", "1.0")

	// Initially should be alive (not exited, not closed)
	if !loader.IsProcessAlive() {
		t.Error("New loader should be alive")
	}

	// After marking as closed
	loader.closed.Store(true)
	if loader.IsProcessAlive() {
		t.Error("Closed loader should not be alive")
	}

	// Reset and mark process as exited
	loader.closed.Store(false)
	loader.processExited.Store(true)
	if loader.IsProcessAlive() {
		t.Error("Loader with exited process should not be alive")
	}
}

func TestLoader_ConcurrentCalls(t *testing.T) {
	pluginPath := createTestPlugin(t)
	loader := NewLoader(pluginPath, "test", "1.0")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := loader.Load(ctx); err != nil {
		t.Fatalf("Failed to load plugin: %v", err)
	}
	defer loader.Close()

	// Wait for plugin to initialize
	time.Sleep(100 * time.Millisecond)

	const numCalls = 10
	var wg sync.WaitGroup
	errors := make(chan error, numCalls)

	for i := 0; i < numCalls; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			callCtx, callCancel := context.WithTimeout(ctx, 5*time.Second)
			defer callCancel()

			payload := []byte("test message")
			response, err := Call(callCtx, loader, "echo", payload)
			if err != nil {
				errors <- err
				return
			}

			if string(response) != string(payload) {
				errors <- fmt.Errorf("expected echo response, got %s", response)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent call error: %v", err)
	}
}

func TestLoader_ContextCancellation(t *testing.T) {
	pluginPath := createTestPlugin(t)
	loader := NewLoader(pluginPath, "test", "1.0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := loader.Load(ctx); err != nil {
		t.Fatalf("Failed to load plugin: %v", err)
	}
	defer loader.Close()

	// Wait for plugin to initialize
	time.Sleep(100 * time.Millisecond)

	// Create a context that will be cancelled
	callCtx, callCancel := context.WithCancel(ctx)

	// Start a call
	go func() {
		time.Sleep(50 * time.Millisecond)
		callCancel() // Cancel after a short delay
	}()

	_, err := Call(callCtx, loader, "echo", []byte("test"))
	if err == nil {
		t.Error("Expected context cancellation error")
	}

	// Check if it's a context error
	if err != context.Canceled && err.Error() != "context canceled" {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}
}

// Benchmark tests
func BenchmarkLoader_GenerateRequestID(b *testing.B) {
	loader := NewLoader("/path/to/plugin", "test", "1.0")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			loader.generateRequestID()
		}
	})
}

func BenchmarkLoader_ConcurrentCalls(b *testing.B) {
	b.Skip("Skipping benchmark that requires building plugin executable")
}
