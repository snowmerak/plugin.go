// Package plugin provides utility functions for the plugin system.
// This file contains helper functions for ID generation, ready signal handling, and other utilities.
package plugin

import (
	"context"
	"fmt"
	"time"
)

// generateRequestID generates a unique request ID, avoiding collisions with existing pending requests
func (l *Loader) generateRequestID() uint32 {
	const maxAttempts = 100 // Prevent infinite loop in extreme cases

	for attempt := 0; attempt < maxAttempts; attempt++ {
		id := l.requestID.Add(1)
		if id == 0 {
			// Skip 0 as it might be reserved
			continue
		}

		l.requestMutex.RLock()
		_, exists := l.pendingRequests[id]
		l.requestMutex.RUnlock()

		if !exists {
			return id
		}
	}

	// Fallback: if we can't find a unique ID after maxAttempts, use the current value
	// This should be extremely rare unless there are millions of concurrent requests
	return l.requestID.Load()
}

// waitForReadySignal waits for the ready signal from the plugin
func (l *Loader) waitForReadySignal() error {
	// Wait for handleMessages to process the ready signal
	// This is now handled by handleMessages goroutine
	select {
	case <-l.readySignal:
		return nil
	case <-l.loadCtx.Done():
		return fmt.Errorf("context cancelled while waiting for ready signal")
	case <-time.After(5 * time.Second):
		// If no ready signal received within timeout, try requesting it
		if err := l.RequestReady(); err != nil {
			return fmt.Errorf("timeout waiting for ready signal and failed to request ready: %w", err)
		}
		// Wait a bit more after requesting ready
		select {
		case <-l.readySignal:
			return nil
		case <-l.loadCtx.Done():
			return fmt.Errorf("context cancelled while waiting for ready signal after request")
		case <-time.After(3 * time.Second):
			return fmt.Errorf("timeout waiting for ready signal from plugin even after requesting")
		}
	}
}

// RequestReady sends a request to the plugin asking it to send a ready signal
func (l *Loader) RequestReady() error {
	if l.closed.Load() {
		return fmt.Errorf("loader is closed")
	}

	if l.multiplexer == nil {
		return fmt.Errorf("multiplexer not available")
	}

	requestReadyHeader := Header{
		Name:        "request_ready",
		IsError:     false,
		MessageType: MessageTypeRequest,
		Payload:     []byte("please send ready signal"),
	}

	requestData, err := requestReadyHeader.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal request ready header: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	return l.multiplexer.WriteMessage(ctx, requestData)
}
