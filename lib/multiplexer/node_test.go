package multiplexer_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/snowmerak/plugin.go/lib/multiplexer"
)

func TestNewNode(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()

	node := multiplexer.NewNode(reader, writer)
	if node == nil {
		t.Fatal("NewNode returned nil")
	}
}

func TestNode_WriteResponseMessage(t *testing.T) {
	tests := []struct {
		name    string
		seq     uint64
		data    []byte
		wantErr bool
		timeout time.Duration
	}{
		{
			name:    "empty data",
			seq:     1,
			data:    []byte{},
			wantErr: false,
			timeout: time.Second,
		},
		{
			name:    "small data",
			seq:     2,
			data:    []byte("hello world"),
			wantErr: false,
			timeout: time.Second,
		},
		{
			name:    "large data",
			seq:     3,
			data:    make([]byte, 2048), // Larger than chunk size
			wantErr: false,
			timeout: time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, writer := io.Pipe()
			defer reader.Close()
			defer writer.Close()

			node := multiplexer.NewNode(reader, writer)
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			// Write message in goroutine
			errCh := make(chan error, 1)
			go func() {
				errCh <- node.WriteResponseMessage(ctx, tt.seq, tt.data)
			}()

			// Read and verify the message
			messageCh, err := node.ReadMessage(ctx)
			if err != nil {
				t.Fatalf("ReadMessage failed: %v", err)
			}

			var receivedMessage *multiplexer.Message
			select {
			case msg := <-messageCh:
				if msg.Type == multiplexer.MessageHeaderTypeComplete {
					receivedMessage = msg
				}
			case <-time.After(tt.timeout):
				t.Fatal("timeout waiting for message")
			}

			// Check write result
			select {
			case err := <-errCh:
				if (err != nil) != tt.wantErr {
					t.Errorf("WriteResponseMessage() error = %v, wantErr %v", err, tt.wantErr)
				}
			case <-time.After(tt.timeout):
				if !tt.wantErr {
					t.Fatal("timeout waiting for write completion")
				}
			}

			if !tt.wantErr && receivedMessage != nil {
				if receivedMessage.ID != tt.seq {
					t.Errorf("Expected sequence %d, got %d", tt.seq, receivedMessage.ID)
				}
				if len(receivedMessage.Data) != len(tt.data) {
					t.Errorf("Expected data length %d, got %d", len(tt.data), len(receivedMessage.Data))
				}
			}
		})
	}
}

func TestNode_WriteRequestMessage(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()

	node := multiplexer.NewNode(reader, writer)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	data := []byte("test request")

	// Write message in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- node.WriteRequestMessage(ctx, data)
	}()

	// Read the message
	messageCh, err := node.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	select {
	case msg := <-messageCh:
		if msg.Type == multiplexer.MessageHeaderTypeComplete {
			if len(msg.Data) != len(data) {
				t.Errorf("Expected data length %d, got %d", len(data), len(msg.Data))
			}
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("WriteRequestMessage() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for write completion")
	}
}

func TestNode_ReadMessage_ContextCancellation(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()

	node := multiplexer.NewNode(reader, writer)
	ctx, cancel := context.WithCancel(context.Background())

	messageCh, err := node.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	// Cancel context immediately
	cancel()

	select {
	case msg := <-messageCh:
		if msg.Type != multiplexer.MessageHeaderTypeError {
			t.Errorf("Expected error message, got type %d", msg.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for error message")
	}
}

func TestNode_ReadMessage_EOF(t *testing.T) {
	reader, writer := io.Pipe()
	node := multiplexer.NewNode(reader, writer)
	ctx := context.Background()

	messageCh, err := node.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	// Close writer to simulate EOF
	writer.Close()

	// Should not receive any messages and channel should close
	select {
	case msg, ok := <-messageCh:
		if ok {
			t.Errorf("Expected channel to close, but received message: %+v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for channel to close")
	}
}

func TestNode_WriteResponseMessage_ContextCancellation(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()

	node := multiplexer.NewNode(reader, writer)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context before writing
	cancel()

	err := node.WriteResponseMessage(ctx, 1, []byte("test data"))
	if err == nil {
		t.Error("Expected error when context is cancelled")
	}
}

func TestNode_ConcurrentReadWrite(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()

	node := multiplexer.NewNode(reader, writer)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const numMessages = 10
	done := make(chan bool, 2)

	// Start reader
	go func() {
		defer func() { done <- true }()
		messageCh, err := node.ReadMessage(ctx)
		if err != nil {
			t.Errorf("ReadMessage failed: %v", err)
			return
		}

		receivedCount := 0
		for msg := range messageCh {
			if msg.Type == multiplexer.MessageHeaderTypeComplete {
				receivedCount++
				if receivedCount == numMessages {
					return
				}
			}
		}
	}()

	// Start writer
	go func() {
		defer func() { done <- true }()
		for i := 0; i < numMessages; i++ {
			data := []byte("message " + string(rune('0'+i)))
			if err := node.WriteRequestMessage(ctx, data); err != nil {
				t.Errorf("WriteRequestMessage failed: %v", err)
				return
			}
		}
	}()

	// Wait for both goroutines to complete
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("timeout waiting for concurrent operations")
		}
	}
}

func TestNode_InvalidMessageType(t *testing.T) {
	reader, writer := io.Pipe()
	defer writer.Close()

	node := multiplexer.NewNode(reader, writer)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Write invalid message header
	go func() {
		defer reader.Close()
		invalidHeader := make([]byte, 9)
		invalidHeader[0] = 0xFF // Invalid message type
		writer.Write(invalidHeader)
	}()

	messageCh, err := node.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	select {
	case msg := <-messageCh:
		if msg.Type != multiplexer.MessageHeaderTypeError {
			t.Errorf("Expected error message for invalid type, got type %d", msg.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for error message")
	}
}
