package multiplexer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

// mockReaderWriter implements io.Reader and io.Writer for testing
type mockReaderWriter struct {
	readData  []byte
	readPos   int
	writeData bytes.Buffer
	readError error
	mu        sync.Mutex
}

func newMockReaderWriter(data []byte) *mockReaderWriter {
	return &mockReaderWriter{
		readData: data,
	}
}

func (m *mockReaderWriter) Read(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.readError != nil {
		return 0, m.readError
	}

	if m.readPos >= len(m.readData) {
		return 0, io.EOF
	}

	n = copy(p, m.readData[m.readPos:])
	m.readPos += n
	return n, nil
}

func (m *mockReaderWriter) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeData.Write(p)
}

func (m *mockReaderWriter) GetWrittenData() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeData.Bytes()
}

func (m *mockReaderWriter) SetReadError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readError = err
}

// Helper function to create a message frame
func createMessageFrame(msgType uint8, frameID uint32, data []byte) []byte {
	header := make([]byte, MessageHeaderSize)
	header[0] = msgType
	header[1] = byte(frameID >> 24)
	header[2] = byte(frameID >> 16)
	header[3] = byte(frameID >> 8)
	header[4] = byte(frameID)
	header[5] = byte(len(data) >> 24)
	header[6] = byte(len(data) >> 16)
	header[7] = byte(len(data) >> 8)
	header[8] = byte(len(data))

	result := make([]byte, 0, len(header)+len(data))
	result = append(result, header...)
	result = append(result, data...)
	return result
}

func TestNewNode(t *testing.T) {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}

	node := NewNode(reader, writer)

	if node.reader != reader {
		t.Error("Reader not set correctly")
	}
	if node.writer != writer {
		t.Error("Writer not set correctly")
	}
	if node.readBuffer == nil {
		t.Error("ReadBuffer not initialized")
	}
	if node.writerLock == nil {
		t.Error("WriterLock not initialized")
	}
	if node.readerLock == nil {
		t.Error("ReaderLock not initialized")
	}
}

func TestWriteMessage(t *testing.T) {
	writer := &bytes.Buffer{}
	reader := &bytes.Buffer{}
	node := NewNode(reader, writer)

	ctx := context.Background()
	testData := []byte("Hello, World!")

	err := node.WriteMessage(ctx, testData)
	if err != nil {
		t.Fatalf("WriteMessage failed: %v", err)
	}

	written := writer.Bytes()
	if len(written) == 0 {
		t.Fatal("No data written")
	}

	// Check that sequence number is incremented
	err = node.WriteMessage(ctx, testData)
	if err != nil {
		t.Fatalf("Second WriteMessage failed: %v", err)
	}

	if node.sequence.Load() != 2 {
		t.Errorf("Expected sequence to be 2, got %d", node.sequence.Load())
	}
}

func TestWriteMessageWithSequence(t *testing.T) {
	writer := &bytes.Buffer{}
	reader := &bytes.Buffer{}
	node := NewNode(reader, writer)

	ctx := context.Background()
	testData := []byte("Test message for sequence")
	frameID := uint32(12345)

	err := node.WriteMessageWithSequence(ctx, frameID, testData)
	if err != nil {
		t.Fatalf("WriteMessageWithSequence failed: %v", err)
	}

	written := writer.Bytes()

	// Should have at least: START frame + DATA frame(s) + END frame
	if len(written) < MessageHeaderSize*3 {
		t.Errorf("Expected at least %d bytes, got %d", MessageHeaderSize*3, len(written))
	}

	// Parse the first frame (START)
	if written[0] != MessageHeaderTypeStart {
		t.Errorf("Expected START frame, got %d", written[0])
	}

	// Check frame ID in START frame
	startFrameID := uint32(written[1])<<24 | uint32(written[2])<<16 | uint32(written[3])<<8 | uint32(written[4])
	if startFrameID != frameID {
		t.Errorf("Expected frame ID %d, got %d", frameID, startFrameID)
	}
}

func TestWriteMessageWithCancellation(t *testing.T) {
	writer := &bytes.Buffer{}
	reader := &bytes.Buffer{}
	node := NewNode(reader, writer)

	ctx, cancel := context.WithCancel(context.Background())
	testData := make([]byte, MessageChunkSize*3) // Large enough to require multiple chunks
	frameID := uint32(999)

	// Cancel immediately to test abort path
	cancel()

	err := node.WriteMessageWithSequence(ctx, frameID, testData)
	if err == nil {
		t.Fatal("Expected error due to cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func TestReadMessage_SimpleMessage(t *testing.T) {
	testData := []byte("Hello from multiplexer!")
	frameID := uint32(1)

	// Create a complete message sequence
	var buffer bytes.Buffer
	buffer.Write(createMessageFrame(MessageHeaderTypeStart, frameID, nil))
	buffer.Write(createMessageFrame(MessageHeaderTypeData, frameID, testData))
	buffer.Write(createMessageFrame(MessageHeaderTypeEnd, frameID, nil))

	mock := newMockReaderWriter(buffer.Bytes())
	node := NewNode(mock, mock)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ch, err := node.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	// Should receive one complete message
	select {
	case msg := <-ch:
		if msg == nil {
			t.Fatal("Received nil message")
		}
		if msg.Type != MessageHeaderTypeComplete {
			t.Errorf("Expected Complete message, got type %d", msg.Type)
		}
		if msg.ID != frameID {
			t.Errorf("Expected frame ID %d, got %d", frameID, msg.ID)
		}
		if !bytes.Equal(msg.Data, testData) {
			t.Errorf("Expected data %s, got %s", testData, msg.Data)
		}
	case <-ctx.Done():
		t.Fatal("Timeout waiting for message")
	}
}

func TestReadMessage_ChunkedMessage(t *testing.T) {
	testData := []byte("This is a longer message that will be split into multiple chunks for testing")
	frameID := uint32(2)
	chunkSize := 20

	var buffer bytes.Buffer
	buffer.Write(createMessageFrame(MessageHeaderTypeStart, frameID, nil))

	// Split data into chunks
	for i := 0; i < len(testData); i += chunkSize {
		end := i + chunkSize
		if end > len(testData) {
			end = len(testData)
		}
		chunk := testData[i:end]
		buffer.Write(createMessageFrame(MessageHeaderTypeData, frameID, chunk))
	}

	buffer.Write(createMessageFrame(MessageHeaderTypeEnd, frameID, nil))

	mock := newMockReaderWriter(buffer.Bytes())
	node := NewNode(mock, mock)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ch, err := node.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	select {
	case msg := <-ch:
		if msg.Type != MessageHeaderTypeComplete {
			t.Errorf("Expected Complete message, got type %d", msg.Type)
		}
		if !bytes.Equal(msg.Data, testData) {
			t.Errorf("Data mismatch. Expected length %d, got %d", len(testData), len(msg.Data))
		}
	case <-ctx.Done():
		t.Fatal("Timeout waiting for message")
	}
}

func TestReadMessage_MultipleMessages(t *testing.T) {
	frameID1 := uint32(10)
	frameID2 := uint32(20)
	data1 := []byte("First message")
	data2 := []byte("Second message")

	var buffer bytes.Buffer

	// First message
	buffer.Write(createMessageFrame(MessageHeaderTypeStart, frameID1, nil))
	buffer.Write(createMessageFrame(MessageHeaderTypeData, frameID1, data1))
	buffer.Write(createMessageFrame(MessageHeaderTypeEnd, frameID1, nil))

	// Second message
	buffer.Write(createMessageFrame(MessageHeaderTypeStart, frameID2, nil))
	buffer.Write(createMessageFrame(MessageHeaderTypeData, frameID2, data2))
	buffer.Write(createMessageFrame(MessageHeaderTypeEnd, frameID2, nil))

	mock := newMockReaderWriter(buffer.Bytes())
	node := NewNode(mock, mock)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ch, err := node.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	// Collect messages
	var messages []*Message
	for i := 0; i < 2; i++ {
		select {
		case msg := <-ch:
			if msg.Type == MessageHeaderTypeComplete {
				messages = append(messages, msg)
			}
		case <-ctx.Done():
			t.Fatal("Timeout waiting for messages")
		}
	}

	if len(messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(messages))
	}

	// Check first message
	if messages[0].ID != frameID1 || !bytes.Equal(messages[0].Data, data1) {
		t.Error("First message mismatch")
	}

	// Check second message
	if messages[1].ID != frameID2 || !bytes.Equal(messages[1].Data, data2) {
		t.Error("Second message mismatch")
	}
}

func TestReadMessage_AbortMessage(t *testing.T) {
	frameID := uint32(99)
	testData := []byte("This message will be aborted")

	var buffer bytes.Buffer
	buffer.Write(createMessageFrame(MessageHeaderTypeStart, frameID, nil))
	buffer.Write(createMessageFrame(MessageHeaderTypeData, frameID, testData))
	buffer.Write(createMessageFrame(MessageHeaderTypeAbort, frameID, nil))

	mock := newMockReaderWriter(buffer.Bytes())
	node := NewNode(mock, mock)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ch, err := node.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	select {
	case msg := <-ch:
		if msg.Type != MessageHeaderTypeAbort {
			t.Errorf("Expected Abort message, got type %d", msg.Type)
		}
		if msg.ID != frameID {
			t.Errorf("Expected frame ID %d, got %d", frameID, msg.ID)
		}
	case <-ctx.Done():
		t.Fatal("Timeout waiting for abort message")
	}

	// Check that the message was cleaned up from readBuffer
	if node.GetPendingMessageCount() != 0 {
		t.Error("Expected readBuffer to be clean after abort")
	}
}

func TestReadMessage_ErrorCases(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectError string
	}{
		{
			name: "Unknown message type",
			data: createMessageFrame(0xFF, 1, []byte("test")),
		},
		{
			name: "Data without start",
			data: createMessageFrame(MessageHeaderTypeData, 1, []byte("test")),
		},
		{
			name: "End without start",
			data: createMessageFrame(MessageHeaderTypeEnd, 1, nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockReaderWriter(tt.data)
			node := NewNode(mock, mock)

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			ch, err := node.ReadMessage(ctx)
			if err != nil {
				t.Fatalf("ReadMessage setup failed: %v", err)
			}

			select {
			case msg := <-ch:
				if msg.Type != MessageHeaderTypeError {
					t.Errorf("Expected error message, got type %d", msg.Type)
				}
			case <-ctx.Done():
				t.Fatal("Timeout waiting for error message")
			}
		})
	}
}

func TestReadMessage_ContextCancellation(t *testing.T) {
	// Create an infinite reader that never returns
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}
	node := NewNode(reader, writer)

	ctx, cancel := context.WithCancel(context.Background())

	ch, err := node.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	// Cancel context immediately
	cancel()

	select {
	case msg := <-ch:
		if msg.Type != MessageHeaderTypeError {
			t.Errorf("Expected error message due to cancellation, got type %d", msg.Type)
		}
		if !bytes.Equal(msg.Data, []byte("context done")) {
			t.Errorf("Expected 'context done' message, got %s", msg.Data)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Should receive cancellation message quickly")
	}
}

func TestReadMessage_MaxDataLengthExceeded(t *testing.T) {
	frameID := uint32(1)

	// Create a frame with data length that exceeds maximum
	header := make([]byte, MessageHeaderSize)
	header[0] = MessageHeaderTypeStart
	header[1] = byte(frameID >> 24)
	header[2] = byte(frameID >> 16)
	header[3] = byte(frameID >> 8)
	header[4] = byte(frameID)

	// Set data length to exceed maximum (more than 10MB)
	exceedingLength := uint32(1024 * 1024 * 11) // 11MB
	header[5] = byte(exceedingLength >> 24)
	header[6] = byte(exceedingLength >> 16)
	header[7] = byte(exceedingLength >> 8)
	header[8] = byte(exceedingLength)

	mock := newMockReaderWriter(header)
	node := NewNode(mock, mock)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ch, err := node.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	select {
	case msg := <-ch:
		if msg.Type != MessageHeaderTypeError {
			t.Errorf("Expected error message, got type %d", msg.Type)
		}
		if !bytes.Contains(msg.Data, []byte("exceeds maximum")) {
			t.Errorf("Expected 'exceeds maximum' error, got %s", msg.Data)
		}
	case <-ctx.Done():
		t.Fatal("Timeout waiting for error message")
	}
}

func TestReadMessage_DuplicateFrameID(t *testing.T) {
	frameID := uint32(5)

	var buffer bytes.Buffer
	// Start first message
	buffer.Write(createMessageFrame(MessageHeaderTypeStart, frameID, nil))
	// Try to start another message with same frame ID
	buffer.Write(createMessageFrame(MessageHeaderTypeStart, frameID, nil))

	mock := newMockReaderWriter(buffer.Bytes())
	node := NewNode(mock, mock)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ch, err := node.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	// Should receive error for duplicate frame ID
	select {
	case msg := <-ch:
		if msg.Type != MessageHeaderTypeError {
			t.Errorf("Expected error message, got type %d", msg.Type)
		}
		if !bytes.Contains(msg.Data, []byte("already exists")) {
			t.Errorf("Expected 'already exists' error, got %s", msg.Data)
		}
	case <-ctx.Done():
		t.Fatal("Timeout waiting for error message")
	}
}

func TestCloseAndGetPendingMessageCount(t *testing.T) {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}
	node := NewNode(reader, writer)

	// Add some pending messages manually
	node.readerLock.Lock()
	node.readBuffer[1] = &Message{ID: 1, Type: MessageHeaderTypeStart}
	node.readBuffer[2] = &Message{ID: 2, Type: MessageHeaderTypeStart}
	node.readerLock.Unlock()

	if count := node.GetPendingMessageCount(); count != 2 {
		t.Errorf("Expected 2 pending messages, got %d", count)
	}

	err := node.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if count := node.GetPendingMessageCount(); count != 0 {
		t.Errorf("Expected 0 pending messages after close, got %d", count)
	}
}

func TestConcurrentWriteMessages(t *testing.T) {
	writer := &bytes.Buffer{}
	reader := &bytes.Buffer{}
	node := NewNode(reader, writer)

	ctx := context.Background()
	numGoroutines := 10
	messagesPerGoroutine := 5

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*messagesPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				data := []byte(fmt.Sprintf("Message from goroutine %d, iteration %d", id, j))
				if err := node.WriteMessage(ctx, data); err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent write error: %v", err)
	}

	expectedSequence := uint32(numGoroutines * messagesPerGoroutine)
	if node.sequence.Load() != expectedSequence {
		t.Errorf("Expected sequence %d, got %d", expectedSequence, node.sequence.Load())
	}
}

func TestReadMessageWithReadError(t *testing.T) {
	mock := newMockReaderWriter([]byte{})
	mock.SetReadError(errors.New("read error"))
	node := NewNode(mock, mock)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ch, err := node.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	select {
	case msg := <-ch:
		if msg.Type != MessageHeaderTypeError {
			t.Errorf("Expected error message, got type %d", msg.Type)
		}
		if !bytes.Contains(msg.Data, []byte("read error")) {
			t.Errorf("Expected 'read error', got %s", msg.Data)
		}
	case <-ctx.Done():
		t.Fatal("Timeout waiting for error message")
	}
}
