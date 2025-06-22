// Package plugin provides handler management functionality.
// This file contains functions for registering, unregistering, and managing message and request handlers.
package plugin

import (
	"context"
	"fmt"
)

// RegisterMessageHandler registers a handler for incoming messages from the module with the specified name
func (l *Loader) RegisterMessageHandler(name string, handler MessageHandler) {
	l.handlerMutex.Lock()
	defer l.handlerMutex.Unlock()
	l.messageHandlers[name] = handler
}

// UnregisterMessageHandler removes the handler for the specified message name
func (l *Loader) UnregisterMessageHandler(name string) {
	l.handlerMutex.Lock()
	defer l.handlerMutex.Unlock()
	delete(l.messageHandlers, name)
}

// RegisterMessageHandlerFunc is a convenience method to register a function as a message handler
func (l *Loader) RegisterMessageHandlerFunc(name string, handler func(ctx context.Context, header Header) error) {
	l.RegisterMessageHandler(name, MessageHandlerFunc(handler))
}

// RegisterRequestHandler registers a handler for incoming requests from the module with the specified name
func (l *Loader) RegisterRequestHandler(name string, handler RequestHandler) {
	l.handlerMutex.Lock()
	defer l.handlerMutex.Unlock()
	l.requestHandlers[name] = handler
}

// UnregisterRequestHandler removes the request handler for the specified message name
func (l *Loader) UnregisterRequestHandler(name string) {
	l.handlerMutex.Lock()
	defer l.handlerMutex.Unlock()
	delete(l.requestHandlers, name)
}

// RegisterRequestHandlerFunc is a convenience method to register a function as a request handler
func (l *Loader) RegisterRequestHandlerFunc(name string, handler func(ctx context.Context, header Header) (responsePayload []byte, isError bool, err error)) {
	l.RegisterRequestHandler(name, RequestHandlerFunc(handler))
}

// getRequestHandler safely retrieves a request handler for the given name
func (l *Loader) getRequestHandler(name string) (RequestHandler, bool) {
	l.handlerMutex.RLock()
	defer l.handlerMutex.RUnlock()
	handler, exists := l.requestHandlers[name]
	return handler, exists
}

// getMessageHandler safely retrieves a message handler for the given name
func (l *Loader) getMessageHandler(name string) (MessageHandler, bool) {
	l.handlerMutex.RLock()
	defer l.handlerMutex.RUnlock()
	handler, exists := l.messageHandlers[name]
	return handler, exists
}

// registerBuiltinHandlers registers the built-in message handlers for standard plugin protocol messages
func (l *Loader) registerBuiltinHandlers() {
	// Register handler for informational messages that might be sent by the module
	l.RegisterMessageHandlerFunc("info", func(ctx context.Context, header Header) error {
		// Log informational messages from the module
		fmt.Printf("Module info: %s\n", string(header.Payload))
		return nil
	})

	// Register handler for warning messages
	l.RegisterMessageHandlerFunc("warning", func(ctx context.Context, header Header) error {
		// Log warning messages from the module
		fmt.Printf("Module warning: %s\n", string(header.Payload))
		return nil
	})

	// Register handler for error notifications (not request errors, but module notifications)
	l.RegisterMessageHandlerFunc("error", func(ctx context.Context, header Header) error {
		// Log error messages from the module
		fmt.Printf("Module error notification: %s\n", string(header.Payload))
		return nil
	})

	// Register handler for heartbeat messages
	l.RegisterMessageHandlerFunc("heartbeat", func(ctx context.Context, header Header) error {
		// Handle heartbeat messages - could be used for health monitoring
		// For now, just acknowledge receipt
		return nil
	})

	// Register handler for status updates
	l.RegisterMessageHandlerFunc("status", func(ctx context.Context, header Header) error {
		// Handle status update messages from the module
		fmt.Printf("Module status: %s\n", string(header.Payload))
		return nil
	})
}
