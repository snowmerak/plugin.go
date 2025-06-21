package multiplexer

import (
	"bytes"
	"context"
	"fmt"
	"testing"
)

// TestBasicAPIUsage tests the basic API functionality
func TestBasicAPIUsage(t *testing.T) {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}

	mux := New(reader, writer)
	defer mux.Close()

	ctx := context.Background()
	testData := []byte("Hello, World!")

	// Test WriteMessage
	err := mux.WriteMessage(ctx, testData)
	if err != nil {
		t.Fatalf("WriteMessage failed: %v", err)
	}

	if writer.Len() == 0 {
		t.Error("No data written to writer")
	}

	// Test metrics
	metrics := mux.GetMetrics()
	if metrics.MessagesWritten == 0 {
		t.Error("No messages recorded in metrics")
	}
}

// TestAPIOptimizationTargets tests different optimization targets
func TestAPIOptimizationTargets(t *testing.T) {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}

	tests := []struct {
		name    string
		factory func() Multiplexer
	}{
		{"API", func() Multiplexer { return NewForAPI(reader, writer) }},
		{"FileTransfer", func() Multiplexer { return NewForFileTransfer(reader, writer) }},
		{"Streaming", func() Multiplexer { return NewForStreaming(reader, writer) }},
		{"IoT", func() Multiplexer { return NewForIoT(reader, writer) }},
		{"MemoryConstrained", func() Multiplexer { return NewForMemoryConstrained(reader, writer) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := tt.factory()
			defer mux.Close()

			ctx := context.Background()
			testData := []byte("test message")

			err := mux.WriteMessage(ctx, testData)
			if err != nil {
				t.Errorf("WriteMessage failed for %s: %v", tt.name, err)
			}

			// Verify metrics are available
			metrics := mux.GetMetrics()
			if metrics == nil {
				t.Errorf("GetMetrics returned nil for %s", tt.name)
			}
		})
	}
}

// TestAPIWithConfig tests custom configuration
func TestAPIWithConfig(t *testing.T) {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}

	config := Config{
		Threshold:      4 * 1024,
		BufferSize:     32 * 1024,
		MaxMessageSize: 5 * 1024 * 1024,
		EnableMetrics:  true,
	}

	mux := NewWithConfig(reader, writer, config)
	defer mux.Close()

	ctx := context.Background()
	testData := make([]byte, 8*1024) // 8KB - above threshold

	err := mux.WriteMessage(ctx, testData)
	if err != nil {
		t.Fatalf("WriteMessage with config failed: %v", err)
	}

	if writer.Len() == 0 {
		t.Error("No data written with custom config")
	}
}

// TestAPISequenceNumbers tests sequence number handling
func TestAPISequenceNumbers(t *testing.T) {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}

	mux := New(reader, writer)
	defer mux.Close()

	ctx := context.Background()

	// Test WriteMessageWithSequence
	err := mux.WriteMessageWithSequence(ctx, 42, []byte("test"))
	if err != nil {
		t.Fatalf("WriteMessageWithSequence failed: %v", err)
	}

	// Test automatic sequence numbering
	err = mux.WriteMessage(ctx, []byte("auto sequence"))
	if err != nil {
		t.Fatalf("WriteMessage with auto sequence failed: %v", err)
	}

	metrics := mux.GetMetrics()
	if metrics.MessagesWritten < 2 {
		t.Errorf("Expected at least 2 messages written, got %d", metrics.MessagesWritten)
	}
}

// TestAPIContextCancellation tests context cancellation
func TestAPIContextCancellation(t *testing.T) {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}

	mux := New(reader, writer)
	defer mux.Close()

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := mux.WriteMessage(ctx, []byte("should fail"))
	if err == nil {
		t.Error("Expected error with cancelled context, got nil")
	}
}

// TestAPIMetrics tests metrics functionality
func TestAPIMetrics(t *testing.T) {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}

	mux := New(reader, writer)
	defer mux.Close()

	ctx := context.Background()

	// Send some messages
	for i := 0; i < 5; i++ {
		err := mux.WriteMessage(ctx, []byte("test message"))
		if err != nil {
			t.Fatalf("WriteMessage %d failed: %v", i, err)
		}
	}

	metrics := mux.GetMetrics()
	if metrics.MessagesWritten != 5 {
		t.Errorf("Expected 5 messages written, got %d", metrics.MessagesWritten)
	}

	// Metrics should be non-nil and have some reasonable values
	if metrics.FastPathWrites == 0 && metrics.OptimizedWrites == 0 {
		t.Error("Expected either fast path or optimized writes to be non-zero")
	}
}

// TestAPIClose tests proper cleanup
func TestAPIClose(t *testing.T) {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}

	mux := New(reader, writer)

	// Write some data
	ctx := context.Background()
	err := mux.WriteMessage(ctx, []byte("test"))
	if err != nil {
		t.Fatalf("WriteMessage failed: %v", err)
	}

	// Close should not error
	err = mux.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Multiple closes should be safe
	err = mux.Close()
	if err != nil {
		t.Errorf("Second close failed: %v", err)
	}
}

// BenchmarkAPIPerformance benchmarks the API performance
func BenchmarkAPIPerformance(b *testing.B) {
	sizes := []int{1024, 8 * 1024, 64 * 1024}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%dKB", size/1024), func(b *testing.B) {
			reader := &bytes.Buffer{}
			writer := &bytes.Buffer{}
			mux := New(reader, writer)
			defer mux.Close()

			testData := make([]byte, size)
			for i := range testData {
				testData[i] = byte(i % 256)
			}

			ctx := context.Background()
			b.SetBytes(int64(size))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				writer.Reset()
				err := mux.WriteMessage(ctx, testData)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
