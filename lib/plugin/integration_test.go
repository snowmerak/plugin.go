package plugin

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// TestMessageTypes tests that different message types are handled correctly
func TestMessageTypes(t *testing.T) {
	tests := []struct {
		name        string
		messageType MessageType
		expected    string
	}{
		{"Request", MessageTypeRequest, "request"},
		{"Response", MessageTypeResponse, "response"},
		{"Notify", MessageTypeNotify, "notify"},
		{"Ack", MessageTypeAck, "ack"},
		{"Error", MessageTypeError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := Header{
				Name:        "test",
				MessageType: tt.messageType,
				IsError:     false,
				Payload:     []byte("test payload"),
			}

			data, err := header.MarshalBinary()
			if err != nil {
				t.Fatalf("Failed to marshal header: %v", err)
			}

			var unmarshaled Header
			if err := unmarshaled.UnmarshalBinary(data); err != nil {
				t.Fatalf("Failed to unmarshal header: %v", err)
			}

			if unmarshaled.MessageType != tt.messageType {
				t.Errorf("Expected MessageType %v, got %v", tt.messageType, unmarshaled.MessageType)
			}
		})
	}
}

// TestBuiltinHandlers tests that builtin handlers are registered correctly
func TestBuiltinHandlers(t *testing.T) {
	loader := NewLoader("test_path", "test_plugin", "1.0.0")
	defer loader.Close()

	// Check that builtin handlers are registered
	builtinHandlers := []string{"info", "warning", "error", "heartbeat", "status"}

	for _, handlerName := range builtinHandlers {
		loader.handlerMutex.RLock()
		_, exists := loader.messageHandlers[handlerName]
		loader.handlerMutex.RUnlock()

		if !exists {
			t.Errorf("Builtin handler '%s' not registered", handlerName)
		}
	}
}

// TestHandlerOverride tests that handlers can be overridden
func TestHandlerOverride(t *testing.T) {
	loader := NewLoader("test_path", "test_plugin", "1.0.0")
	defer loader.Close()

	var called bool

	// Override a builtin handler
	loader.RegisterMessageHandlerFunc("info", func(ctx context.Context, header Header) error {
		called = true
		return nil
	})

	// Simulate calling the handler
	loader.handlerMutex.RLock()
	handler, exists := loader.messageHandlers["info"]
	loader.handlerMutex.RUnlock()

	if !exists {
		t.Fatal("Handler not found")
	}

	header := Header{
		Name:        "info",
		MessageType: MessageTypeNotify,
		IsError:     false,
		Payload:     []byte("test message"),
	}

	err := handler.Handle(context.Background(), header)
	if err != nil {
		t.Fatalf("Failed to call handler: %v", err)
	}

	if !called {
		t.Error("Custom handler was not called")
	}
}

// TestConcurrentHandlerCalls tests that handlers can be called concurrently
func TestConcurrentHandlerCalls(t *testing.T) {
	loader := NewLoader("test_path", "test_plugin", "1.0.0")
	defer loader.Close()

	var callCount int32
	var mu sync.Mutex

	loader.RegisterMessageHandlerFunc("concurrent_test", func(ctx context.Context, header Header) error {
		mu.Lock()
		callCount++
		mu.Unlock()
		time.Sleep(10 * time.Millisecond) // Simulate some work
		return nil
	})

	// Make concurrent calls
	const numCalls = 10
	var wg sync.WaitGroup
	wg.Add(numCalls)

	for i := 0; i < numCalls; i++ {
		go func() {
			defer wg.Done()

			loader.handlerMutex.RLock()
			handler, exists := loader.messageHandlers["concurrent_test"]
			loader.handlerMutex.RUnlock()

			if exists {
				header := Header{
					Name:        "concurrent_test",
					MessageType: MessageTypeNotify,
					IsError:     false,
					Payload:     []byte("test"),
				}
				handler.Handle(context.Background(), header)
			}
		}()
	}

	wg.Wait()

	mu.Lock()
	if callCount != numCalls {
		t.Errorf("Expected %d calls, got %d", numCalls, callCount)
	}
	mu.Unlock()
}

// TestLoaderRegistration tests handler registration and unregistration
func TestLoaderRegistration(t *testing.T) {
	loader := NewLoader("test_path", "test_plugin", "1.0.0")
	defer loader.Close()

	// Test message handler registration
	loader.RegisterMessageHandlerFunc("test_message", func(ctx context.Context, header Header) error {
		return nil
	})

	loader.handlerMutex.RLock()
	_, exists := loader.messageHandlers["test_message"]
	loader.handlerMutex.RUnlock()

	if !exists {
		t.Error("Message handler not registered")
	}

	// Test request handler registration
	loader.RegisterRequestHandlerFunc("test_request", func(ctx context.Context, header Header) ([]byte, bool, error) {
		return []byte("response"), false, nil
	})

	loader.handlerMutex.RLock()
	_, exists = loader.requestHandlers["test_request"]
	loader.handlerMutex.RUnlock()

	if !exists {
		t.Error("Request handler not registered")
	}

	// Test unregistration
	loader.UnregisterMessageHandler("test_message")

	loader.handlerMutex.RLock()
	_, exists = loader.messageHandlers["test_message"]
	loader.handlerMutex.RUnlock()

	if exists {
		t.Error("Message handler not unregistered")
	}
}

// TestModuleHandlerRegistration tests module handler registration
func TestModuleHandlerRegistration(t *testing.T) {
	module := NewStd()

	// Test handler registration
	RegisterHandler(module, "test_service", func(requestPayload []byte) (responsePayload []byte, isAppError bool) {
		var request map[string]interface{}
		if err := json.Unmarshal(requestPayload, &request); err != nil {
			return []byte("error"), true
		}

		response := map[string]interface{}{
			"result": "success",
			"echo":   request,
		}

		data, _ := json.Marshal(response)
		return data, false
	})

	// Check that handler is registered
	module.handlerLock.RLock()
	_, exists := module.handler["test_service"]
	module.handlerLock.RUnlock()

	if !exists {
		t.Error("Handler not registered in module")
	}

	// Test handler call
	request := map[string]interface{}{
		"action": "test",
		"data":   "sample",
	}
	requestData, _ := json.Marshal(request)

	module.handlerLock.RLock()
	handler := module.handler["test_service"]
	module.handlerLock.RUnlock()

	result, err := handler(requestData)
	if err != nil {
		t.Fatalf("Handler call failed: %v", err)
	}

	if result.IsError {
		t.Error("Handler returned error when it shouldn't")
	}

	var response map[string]interface{}
	if err := json.Unmarshal(result.Payload, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["result"] != "success" {
		t.Errorf("Expected result 'success', got %v", response["result"])
	}
}
