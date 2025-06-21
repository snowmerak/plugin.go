package multiplexer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

// pipeReaderWriter creates a pipe for testing real I/O scenarios
type pipeReaderWriter struct {
	reader *io.PipeReader
	writer *io.PipeWriter
}

func newPipeReaderWriter() *pipeReaderWriter {
	r, w := io.Pipe()
	return &pipeReaderWriter{reader: r, writer: w}
}

func (p *pipeReaderWriter) Read(data []byte) (n int, err error) {
	return p.reader.Read(data)
}

func (p *pipeReaderWriter) Write(data []byte) (n int, err error) {
	return p.writer.Write(data)
}

func (p *pipeReaderWriter) Close() error {
	p.writer.Close()
	return p.reader.Close()
}

// BenchmarkWriteMessage tests the performance of writing messages
func BenchmarkWriteMessage(b *testing.B) {
	writer := &bytes.Buffer{}
	reader := &bytes.Buffer{}
	node := NewNode(reader, writer)
	ctx := context.Background()

	testData := make([]byte, 1024) // 1KB test data
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		writer.Reset()
		if err := node.WriteMessage(ctx, testData); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkWriteMessageLarge tests performance with larger messages
func BenchmarkWriteMessageLarge(b *testing.B) {
	writer := &bytes.Buffer{}
	reader := &bytes.Buffer{}
	node := NewNode(reader, writer)
	ctx := context.Background()

	testData := make([]byte, 1024*1024) // 1MB test data
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		writer.Reset()
		if err := node.WriteMessage(ctx, testData); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkReadMessage tests the performance of reading messages
func BenchmarkReadMessage(b *testing.B) {
	testData := make([]byte, 1024) // 1KB test data
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	// Pre-create message frames
	var messageFrames bytes.Buffer
	for i := 0; i < b.N; i++ {
		frameID := uint32(i + 1)
		messageFrames.Write(createMessageFrame(MessageHeaderTypeStart, frameID, nil))
		messageFrames.Write(createMessageFrame(MessageHeaderTypeData, frameID, testData))
		messageFrames.Write(createMessageFrame(MessageHeaderTypeEnd, frameID, nil))
	}

	b.ResetTimer()
	b.ReportAllocs()

	mock := newMockReaderWriter(messageFrames.Bytes())
	node := NewNode(mock, mock)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ch, err := node.ReadMessage(ctx)
	if err != nil {
		b.Fatal(err)
	}

	received := 0
	for msg := range ch {
		if msg.Type == MessageHeaderTypeComplete {
			received++
			if received >= b.N {
				break
			}
		} else if msg.Type == MessageHeaderTypeError {
			b.Fatalf("Received error: %s", msg.Data)
		}
	}
}

// BenchmarkConcurrentWrites tests performance under concurrent load
func BenchmarkConcurrentWrites(b *testing.B) {
	writer := &bytes.Buffer{}
	reader := &bytes.Buffer{}
	node := NewNode(reader, writer)
	ctx := context.Background()

	testData := make([]byte, 256)
	for i := range testData {
		testData[i] = byte(i)
	}

	numWorkers := 4

	b.ResetTimer()
	b.ReportAllocs()

	var wg sync.WaitGroup
	errChan := make(chan error, b.N)

	work := make(chan struct{}, b.N)
	for i := 0; i < b.N; i++ {
		work <- struct{}{}
	}
	close(work)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range work {
				if err := node.WriteMessage(ctx, testData); err != nil {
					errChan <- err
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		b.Fatal(err)
	}
}

// BenchmarkRoundTrip tests full read-write cycle performance
func BenchmarkRoundTrip(b *testing.B) {
	// Use pipe for realistic I/O
	pipe1 := newPipeReaderWriter()
	pipe2 := newPipeReaderWriter()
	defer pipe1.Close()
	defer pipe2.Close()

	// Node 1 writes to pipe1, reads from pipe2
	node1 := NewNode(pipe2.reader, pipe1.writer)
	// Node 2 reads from pipe1, writes to pipe2
	node2 := NewNode(pipe1.reader, pipe2.writer)

	testData := []byte("benchmark test message")

	b.ResetTimer()
	b.ReportAllocs()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start reading on node2
	ch, err := node2.ReadMessage(ctx)
	if err != nil {
		b.Fatal(err)
	}

	// Start echo goroutine
	go func() {
		for msg := range ch {
			if msg.Type == MessageHeaderTypeComplete {
				// Echo the message back
				if err := node2.WriteMessageWithSequence(ctx, msg.ID, msg.Data); err != nil {
					return
				}
			}
		}
	}()

	// Start reading responses on node1
	respCh, err := node1.ReadMessage(ctx)
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
		// Send message
		frameID := uint32(i + 1)
		if err := node1.WriteMessageWithSequence(ctx, frameID, testData); err != nil {
			b.Fatal(err)
		}

		// Wait for response
		select {
		case resp := <-respCh:
			if resp.Type != MessageHeaderTypeComplete {
				b.Fatalf("Expected complete message, got type %d", resp.Type)
			}
			if resp.ID != frameID {
				b.Fatalf("Expected frame ID %d, got %d", frameID, resp.ID)
			}
		case <-ctx.Done():
			b.Fatal("Timeout waiting for response")
		}
	}
}

// BenchmarkMemoryUsage tests memory efficiency
func BenchmarkMemoryUsage(b *testing.B) {
	sizes := []int{1024, 64 * 1024, 1024 * 1024} // 1KB, 64KB, 1MB

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%dKB", size/1024), func(b *testing.B) {
			writer := &bytes.Buffer{}
			reader := &bytes.Buffer{}
			node := NewNode(reader, writer)
			ctx := context.Background()

			testData := make([]byte, size)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				writer.Reset()
				if err := node.WriteMessage(ctx, testData); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// TestMemoryLeaks verifies that no memory leaks occur during normal operation
func TestMemoryLeaks(t *testing.T) {
	writer := &bytes.Buffer{}
	reader := &bytes.Buffer{}
	node := NewNode(reader, writer)
	ctx := context.Background()

	// Create and send many messages
	numMessages := 1000
	testData := make([]byte, 1024)

	for i := 0; i < numMessages; i++ {
		writer.Reset()
		if err := node.WriteMessage(ctx, testData); err != nil {
			t.Fatal(err)
		}
	}

	// Verify that readBuffer doesn't accumulate
	if count := node.GetPendingMessageCount(); count != 0 {
		t.Errorf("Expected 0 pending messages, got %d - potential memory leak", count)
	}

	// Test cleanup
	if err := node.Close(); err != nil {
		t.Error(err)
	}
}

// TestIntegrationWithRealPipes tests with actual pipes
func TestIntegrationWithRealPipes(t *testing.T) {
	pipe1 := newPipeReaderWriter()
	pipe2 := newPipeReaderWriter()
	defer pipe1.Close()
	defer pipe2.Close()

	node1 := NewNode(pipe2.reader, pipe1.writer)
	node2 := NewNode(pipe1.reader, pipe2.writer)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testMessage := []byte("Integration test message")

	// Start reader on node2
	ch, err := node2.ReadMessage(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Send message from node1
	if err := node1.WriteMessage(ctx, testMessage); err != nil {
		t.Fatal(err)
	}

	// Receive on node2
	select {
	case msg := <-ch:
		if msg.Type != MessageHeaderTypeComplete {
			t.Errorf("Expected complete message, got type %d", msg.Type)
		}
		if !bytes.Equal(msg.Data, testMessage) {
			t.Errorf("Message data mismatch")
		}
	case <-ctx.Done():
		t.Fatal("Timeout waiting for message")
	}
}

// TestStressTest runs a stress test with high message volume
func TestStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	pipe1 := newPipeReaderWriter()
	pipe2 := newPipeReaderWriter()
	defer pipe1.Close()
	defer pipe2.Close()

	node1 := NewNode(pipe2.reader, pipe1.writer)
	node2 := NewNode(pipe1.reader, pipe2.writer)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	numMessages := 10000
	testData := make([]byte, 512)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	// Start receiver
	ch, err := node2.ReadMessage(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Count received messages
	received := make(chan int, 1)
	go func() {
		count := 0
		for msg := range ch {
			if msg.Type == MessageHeaderTypeComplete {
				count++
				if count >= numMessages {
					break
				}
			} else if msg.Type == MessageHeaderTypeError {
				t.Errorf("Received error: %s", msg.Data)
				break
			}
		}
		received <- count
	}()

	// Send messages concurrently
	var wg sync.WaitGroup
	numSenders := 10
	messagesPerSender := numMessages / numSenders

	for i := 0; i < numSenders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < messagesPerSender; j++ {
				if err := node1.WriteMessage(ctx, testData); err != nil {
					t.Errorf("Write error: %v", err)
					return
				}
			}
		}()
	}

	wg.Wait()

	// Wait for all messages to be received
	select {
	case count := <-received:
		if count != numMessages {
			t.Errorf("Expected %d messages, received %d", numMessages, count)
		}
	case <-ctx.Done():
		t.Fatal("Timeout waiting for all messages")
	}
}
