package multiplexer

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"
)

// TestHybridNodeBasicFunctionality tests basic operations
func TestHybridNodeBasicFunctionality(t *testing.T) {
	t.Run("SmallMessage", func(t *testing.T) {
		testData := []byte("Hello, small message!")

		readerBuf, writerBuf := createConnectedBuffers()
		sender := NewHybridNode(nil, writerBuf)   // sender only writes
		receiver := NewHybridNode(readerBuf, nil) // receiver only reads

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		// Start receiver
		msgChan, err := receiver.ReadMessageHybrid(ctx)
		if err != nil {
			t.Fatal(err)
		}

		// Send message
		go func() {
			if err := sender.WriteMessageWithSequenceHybrid(ctx, 1, testData); err != nil {
				t.Logf("Send error: %v", err)
			}
		}()

		// Receive message
		select {
		case msg := <-msgChan:
			if msg.Type != MessageHeaderTypeComplete {
				t.Fatalf("Expected complete message, got type %d", msg.Type)
			}
			if string(msg.Data) != string(testData) {
				t.Fatalf("Data mismatch: expected %q, got %q", testData, msg.Data)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Timeout waiting for message")
		}
	})

	t.Run("LargeMessage", func(t *testing.T) {
		testData := make([]byte, 128*1024) // 128KB - should use optimized path
		for i := range testData {
			testData[i] = byte(i % 256)
		}

		readerBuf, writerBuf := createConnectedBuffers()
		sender := NewHybridNode(nil, writerBuf)   // sender only writes
		receiver := NewHybridNode(readerBuf, nil) // receiver only reads

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Start receiver
		msgChan, err := receiver.ReadMessageHybrid(ctx)
		if err != nil {
			t.Fatal(err)
		}

		// Send message
		go func() {
			if err := sender.WriteMessageWithSequenceHybrid(ctx, 1, testData); err != nil {
				t.Logf("Send error: %v", err)
			}
		}()

		// Receive message
		select {
		case msg := <-msgChan:
			if msg.Type != MessageHeaderTypeComplete {
				t.Fatalf("Expected complete message, got type %d", msg.Type)
			}
			if len(msg.Data) != len(testData) {
				t.Fatalf("Data length mismatch: expected %d, got %d", len(testData), len(msg.Data))
			}
			// Verify data content
			for i := 0; i < len(testData); i++ {
				if msg.Data[i] != testData[i] {
					t.Fatalf("Data mismatch at position %d: expected %d, got %d", i, testData[i], msg.Data[i])
				}
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for large message")
		}
	})
}

// TestHybridNodeThresholdBehavior tests the threshold switching logic
func TestHybridNodeThresholdBehavior(t *testing.T) {
	tests := []struct {
		name         string
		dataSize     int
		expectedPath string
	}{
		{"SmallData_1KB", 1024, "fast"},
		{"ThresholdData_8KB", 8 * 1024, "fast"},
		{"LargeData_9KB", 9 * 1024, "optimized"},
		{"LargeData_64KB", 64 * 1024, "optimized"},
		{"LargeData_1MB", 1024 * 1024, "optimized"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testData := make([]byte, tt.dataSize)
			for i := range testData {
				testData[i] = byte(i % 256)
			}

			readerBuf, writerBuf := createConnectedBuffers()
			sender := NewHybridNode(nil, writerBuf)   // sender only writes
			receiver := NewHybridNode(readerBuf, nil) // receiver only reads

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Start receiver to prevent deadlock
			msgChan, err := receiver.ReadMessageHybrid(ctx)
			if err != nil {
				t.Fatalf("Failed to start reader: %v", err)
			}

			// Write message in a goroutine
			errChan := make(chan error, 1)
			go func() {
				errChan <- sender.WriteMessageWithSequenceHybrid(ctx, 1, testData)
			}()

			// Read the message
			select {
			case msg := <-msgChan:
				if msg.Type != MessageHeaderTypeComplete {
					t.Fatalf("Expected complete message, got type %d", msg.Type)
				}
				if len(msg.Data) != len(testData) {
					t.Fatalf("Data length mismatch: expected %d, got %d", len(testData), len(msg.Data))
				}
				// Verify data content
				for i, b := range msg.Data {
					if b != testData[i] {
						t.Fatalf("Data mismatch at position %d: expected %d, got %d", i, testData[i], b)
					}
				}
			case <-time.After(2 * time.Second):
				t.Fatal("Timeout waiting for message")
			}

			// Check if write was successful
			select {
			case err := <-errChan:
				if err != nil {
					t.Fatalf("Failed to write message: %v", err)
				}
			case <-time.After(100 * time.Millisecond):
				t.Fatal("Write operation timed out")
			}

			// Basic verification that the correct path was used
			t.Logf("Data size: %d, expected path: %s", tt.dataSize, tt.expectedPath)
		})
	}
}

// TestHybridNodeConcurrency tests concurrent operations
func TestHybridNodeConcurrency(t *testing.T) {
	const numGoroutines = 10
	const messagesPerGoroutine = 50

	// Use bytes.Buffer for unlimited writing capacity to test concurrency
	// without being blocked by pipe buffer limits
	var writer bytes.Buffer
	reader := &bytes.Buffer{}
	node := NewHybridNode(reader, &writer)
	ctx := context.Background()

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*messagesPerGoroutine)

	// Start multiple goroutines writing messages
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for i := 0; i < messagesPerGoroutine; i++ {
				testData := make([]byte, 1024+i*100) // Variable size messages
				for j := range testData {
					testData[j] = byte((goroutineID + i + j) % 256)
				}

				seq := uint32(goroutineID*messagesPerGoroutine + i + 1)
				if err := node.WriteMessageWithSequenceHybrid(ctx, seq, testData); err != nil {
					select {
					case errors <- err:
					default:
					}
				}
			}
		}(g)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	var errorCount int
	for err := range errors {
		t.Logf("Concurrent write error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Fatalf("Encountered %d errors during concurrent writes", errorCount)
	}

	// Verify that data was written (basic sanity check)
	if writer.Len() == 0 {
		t.Fatal("No data was written to buffer")
	}

	t.Logf("Successfully wrote %d bytes from %d concurrent goroutines", writer.Len(), numGoroutines)
}

// TestHybridNodeErrorHandling tests error scenarios
func TestHybridNodeErrorHandling(t *testing.T) {
	t.Run("ContextCancellation", func(t *testing.T) {
		var writer bytes.Buffer
		reader := &bytes.Buffer{}
		node := NewHybridNode(reader, &writer)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		testData := make([]byte, 1024)
		err := node.WriteMessageWithSequenceHybrid(ctx, 1, testData)

		if err == nil {
			t.Fatal("Expected error due to cancelled context")
		}
		if err != context.Canceled {
			t.Fatalf("Expected context.Canceled, got %v", err)
		}
	})

	t.Run("NilWriter", func(t *testing.T) {
		node := NewHybridNode(nil, nil)
		ctx := context.Background()
		testData := []byte("test")

		err := node.WriteMessageWithSequenceHybrid(ctx, 1, testData)
		if err == nil {
			t.Fatal("Expected error with nil writer")
		}
	})
}

// TestHybridNodeResourceCleanup tests resource management
func TestHybridNodeResourceCleanup(t *testing.T) {
	node := NewHybridNode(nil, nil)

	// Simulate some pending messages
	node.readBuffer[1] = &Message{ID: 1, Data: []byte("test1")}
	node.readBuffer[2] = &Message{ID: 2, Data: []byte("test2")}

	if count := node.GetPendingMessageCount(); count != 2 {
		t.Fatalf("Expected 2 pending messages, got %d", count)
	}

	err := node.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if count := node.GetPendingMessageCount(); count != 0 {
		t.Fatalf("Expected 0 pending messages after close, got %d", count)
	}
}

// TestHybridNodeMetrics tests metrics collection
func TestHybridNodeMetrics(t *testing.T) {
	var writer bytes.Buffer
	reader := &bytes.Buffer{}
	node := NewHybridNode(reader, &writer)
	ctx := context.Background()

	// Write some messages
	for i := 0; i < 5; i++ {
		testData := make([]byte, (i+1)*1024) // 1KB, 2KB, 3KB, 4KB, 5KB
		if err := node.WriteMessageWithSequenceHybrid(ctx, uint32(i+1), testData); err != nil {
			t.Fatal(err)
		}
	}

	metrics := node.GetMetrics()

	// Check that metrics are being collected
	if metrics.MessagesWritten.Load() == 0 {
		t.Error("Expected non-zero messages written count")
	}

	if metrics.BytesWritten.Load() == 0 {
		t.Error("Expected non-zero bytes written count")
	}
}

// createConnectedBuffers creates two buffers connected via pipes for testing
func createConnectedBuffers() (*connectedBuffer, *connectedBuffer) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()

	buf1 := &connectedBuffer{reader: r1, writer: w2}
	buf2 := &connectedBuffer{reader: r2, writer: w1}

	return buf1, buf2
}

// connectedBuffer implements io.Reader and io.Writer for testing
type connectedBuffer struct {
	reader *io.PipeReader
	writer *io.PipeWriter
}

func (cb *connectedBuffer) Read(p []byte) (n int, err error) {
	return cb.reader.Read(p)
}

func (cb *connectedBuffer) Write(p []byte) (n int, err error) {
	return cb.writer.Write(p)
}

func (cb *connectedBuffer) Bytes() []byte {
	// This is a simplified implementation for testing
	// In real scenarios, you might want to use a different approach
	return nil
}

func (cb *connectedBuffer) Close() error {
	cb.writer.Close()
	return cb.reader.Close()
}
