package plugin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"
)

// mockReadWriter implements io.Reader and io.Writer for testing
type mockReadWriter struct {
	readData  []byte
	readPos   int
	writeData bytes.Buffer
	readErr   error
	writeErr  error
}

func newMockReadWriter(data []byte) *mockReadWriter {
	return &mockReadWriter{readData: data}
}

func (m *mockReadWriter) Read(p []byte) (n int, err error) {
	if m.readErr != nil {
		return 0, m.readErr
	}
	if m.readPos >= len(m.readData) {
		return 0, io.EOF
	}
	n = copy(p, m.readData[m.readPos:])
	m.readPos += n
	return n, nil
}

func (m *mockReadWriter) Write(p []byte) (n int, err error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	return m.writeData.Write(p)
}

func (m *mockReadWriter) SetReadError(err error) {
	m.readErr = err
}

func (m *mockReadWriter) SetWriteError(err error) {
	m.writeErr = err
}

func (m *mockReadWriter) GetWrittenData() []byte {
	return m.writeData.Bytes()
}

func TestModule_New(t *testing.T) {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}

	module := New(reader, writer)

	if module == nil {
		t.Fatal("New() returned nil")
	}
	if module.multiplexer == nil {
		t.Error("Module multiplexer should not be nil")
	}
	if module.handler == nil {
		t.Error("Module handler map should not be nil")
	}
}

func TestRegisterHandler_Simple(t *testing.T) {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}
	module := New(reader, writer)

	// Register a simple echo handler
	RegisterHandler(module, "echo", func(payload []byte) ([]byte, bool) {
		return payload, false
	})

	// Check if handler was registered
	if module.handler["echo"] == nil {
		t.Error("Echo handler should be registered")
	}
}

func TestRegisterHandler_Multiple(t *testing.T) {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}
	module := New(reader, writer)

	// Register multiple handlers
	RegisterHandler(module, "echo", func(payload []byte) ([]byte, bool) {
		return payload, false
	})

	RegisterHandler(module, "upper", func(payload []byte) ([]byte, bool) {
		return bytes.ToUpper(payload), false
	})

	RegisterHandler(module, "error", func(payload []byte) ([]byte, bool) {
		return []byte("error occurred"), true
	})

	// Check all handlers
	if len(module.handler) != 3 {
		t.Errorf("Expected 3 handlers, got %d", len(module.handler))
	}

	expectedHandlers := []string{"echo", "upper", "error"}
	for _, name := range expectedHandlers {
		if module.handler[name] == nil {
			t.Errorf("Handler %s should be registered", name)
		}
	}
}

func TestRegisterHandler_Override(t *testing.T) {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}
	module := New(reader, writer)

	// This should register successfully
	RegisterHandler(module, "test", func(payload []byte) ([]byte, bool) {
		return []byte("first"), false
	})

	// Second registration should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for duplicate handler registration")
		}
	}()

	RegisterHandler(module, "test", func(payload []byte) ([]byte, bool) {
		return []byte("second"), false
	})
}

func TestModule_Listen_ContextCancellation(t *testing.T) {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}
	module := New(reader, writer)

	RegisterHandler(module, "echo", func(payload []byte) ([]byte, bool) {
		return payload, false
	})

	ctx, cancel := context.WithCancel(context.Background())

	// Start module in goroutine
	done := make(chan error, 1)
	go func() {
		done <- module.Listen(ctx)
	}()

	// Cancel context after short delay
	time.Sleep(10 * time.Millisecond)
	cancel()

	// Wait for module to finish
	select {
	case err := <-done:
		// Listen should return when context is cancelled, error can be nil or context.Canceled
		if err != nil && err != context.Canceled {
			t.Errorf("Expected nil or context.Canceled, got %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Module.Listen did not exit after context cancellation")
	}
}

func TestAppHandlerResult(t *testing.T) {
	testCases := []struct {
		name   string
		result AppHandlerResult
	}{
		{
			name: "Success result",
			result: AppHandlerResult{
				Payload: []byte("success"),
				IsError: false,
			},
		},
		{
			name: "Error result",
			result: AppHandlerResult{
				Payload: []byte("error message"),
				IsError: true,
			},
		},
		{
			name: "Empty payload",
			result: AppHandlerResult{
				Payload: []byte{},
				IsError: false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.result.Payload == nil {
				tc.result.Payload = []byte{}
			}

			// Just verify the structure is as expected
			if tc.result.IsError && string(tc.result.Payload) == "success" {
				t.Error("Error result should not have success payload")
			}
			if !tc.result.IsError && string(tc.result.Payload) == "error message" {
				t.Error("Success result should not have error message")
			}
		})
	}
}

func TestHandler_DirectCall(t *testing.T) {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}
	module := New(reader, writer)

	// Register echo handler
	RegisterHandler(module, "echo", func(payload []byte) ([]byte, bool) {
		return payload, false
	})

	// Register error handler
	RegisterHandler(module, "error", func(payload []byte) ([]byte, bool) {
		return []byte("test error"), true
	})

	// Test echo handler directly
	result, err := module.handler["echo"]([]byte("test"))
	if err != nil {
		t.Errorf("Echo handler should not return error: %v", err)
	}
	if result.IsError {
		t.Error("Echo handler should not return error result")
	}
	if string(result.Payload) != "test" {
		t.Errorf("Expected 'test', got '%s'", result.Payload)
	}

	// Test error handler directly
	result, err = module.handler["error"]([]byte("test"))
	if err != nil {
		t.Errorf("Error handler should not return critical error: %v", err)
	}
	if !result.IsError {
		t.Error("Error handler should return error result")
	}
	if string(result.Payload) != "test error" {
		t.Errorf("Expected 'test error', got '%s'", result.Payload)
	}
}

// Benchmark tests
func BenchmarkRegisterHandler(b *testing.B) {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		module := New(reader, writer)
		name := fmt.Sprintf("handler_%d", i)
		RegisterHandler(module, name, func(payload []byte) ([]byte, bool) {
			return payload, false
		})
	}
}

func BenchmarkHandler_Execution(b *testing.B) {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}
	module := New(reader, writer)

	RegisterHandler(module, "bench", func(payload []byte) ([]byte, bool) {
		// Simple echo handler
		return payload, false
	})

	payload := []byte("benchmark test data")
	handler := module.handler["bench"]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := handler(payload)
		if err != nil {
			b.Fatalf("Handler execution failed: %v", err)
		}
	}
}
