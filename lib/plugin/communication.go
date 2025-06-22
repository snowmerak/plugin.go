// Package plugin provides communication functionality between host and plugin processes.
// This file contains functions for sending messages, requests, and handling responses.
package plugin

import (
	"context"
	"fmt"
)

// Call sends a request to the loaded plugin and waits for a response.
// The request and response are raw byte slices.
// It returns the response payload as []byte. If the plugin indicates an error,
// this error is returned, with the error message being the string representation
// of the plugin's error payload.
func Call(ctx context.Context, l *Loader, name string, requestPayload []byte) ([]byte, error) {
	if l.closed.Load() {
		return nil, fmt.Errorf("loader is closed")
	}

	if l.multiplexer == nil {
		return nil, fmt.Errorf("loader not loaded or load failed")
	}

	requestHeader := Header{
		Name:        name,
		IsError:     false,
		MessageType: MessageTypeRequest,
		Payload:     requestPayload,
	}

	headerData, err := requestHeader.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to encode header: %w", err)
	}

	// Generate request ID first (before acquiring lock)
	requestID := l.generateRequestID()

	l.requestMutex.Lock()
	// Double-check closed status while holding lock
	if l.closed.Load() {
		l.requestMutex.Unlock()
		return nil, fmt.Errorf("loader closed before dispatching request")
	}

	responseChan := make(chan []byte, 1)
	l.pendingRequests[requestID] = responseChan
	l.requestMutex.Unlock()

	defer func() {
		l.requestMutex.Lock()
		delete(l.pendingRequests, requestID)
		l.requestMutex.Unlock()
	}()

	if err := l.multiplexer.WriteMessageWithSequence(ctx, requestID, headerData); err != nil {
		return nil, fmt.Errorf("failed to write request message: %w", err)
	}

	select {
	case responseData, ok := <-responseChan:
		if !ok {
			return nil, fmt.Errorf("response channel closed, loader shutting down")
		}

		var responseHeader Header
		if err := responseHeader.UnmarshalBinary(responseData); err != nil {
			return nil, fmt.Errorf("failed to decode response header: %w", err)
		}

		if responseHeader.IsError {
			errMsg := string(responseHeader.Payload)
			return nil, fmt.Errorf("plugin error for service %s: %s", name, errMsg)
		}

		return responseHeader.Payload, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-l.loadCtx.Done():
		return nil, fmt.Errorf("loader is shutting down")
	}
}

// SendMessage sends a message to the module without expecting a response
func (l *Loader) SendMessage(ctx context.Context, name string, payload []byte) error {
	if l.closed.Load() {
		return fmt.Errorf("loader is closed")
	}

	if l.multiplexer == nil {
		return fmt.Errorf("multiplexer not available")
	}

	header := Header{
		Name:        name,
		IsError:     false,
		MessageType: MessageTypeNotify,
		Payload:     payload,
	}

	headerData, err := header.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal header: %w", err)
	}

	return l.multiplexer.WriteMessage(ctx, headerData)
}

// SendErrorMessage sends an error message to the module
func (l *Loader) SendErrorMessage(ctx context.Context, name string, payload []byte) error {
	if l.closed.Load() {
		return fmt.Errorf("loader is closed")
	}

	if l.multiplexer == nil {
		return fmt.Errorf("multiplexer not available")
	}

	header := Header{
		Name:        name,
		IsError:     true,
		MessageType: MessageTypeError,
		Payload:     payload,
	}

	headerData, err := header.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal header: %w", err)
	}

	return l.multiplexer.WriteMessage(ctx, headerData)
}

// SendRequest sends a request to the module and waits for a response
func (l *Loader) SendRequest(ctx context.Context, name string, payload []byte) ([]byte, error) {
	if l.closed.Load() {
		return nil, fmt.Errorf("loader is closed")
	}

	if l.multiplexer == nil {
		return nil, fmt.Errorf("multiplexer not available")
	}

	header := Header{
		Name:        name,
		IsError:     false,
		MessageType: MessageTypeRequest,
		Payload:     payload,
	}

	headerData, err := header.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal header: %w", err)
	}

	// Generate request ID first (before acquiring lock)
	requestID := l.generateRequestID()

	l.requestMutex.Lock()
	// Double-check closed status while holding lock
	if l.closed.Load() {
		l.requestMutex.Unlock()
		return nil, fmt.Errorf("loader closed before dispatching request")
	}

	responseChan := make(chan []byte, 1)
	l.pendingRequests[requestID] = responseChan
	l.requestMutex.Unlock()

	defer func() {
		l.requestMutex.Lock()
		delete(l.pendingRequests, requestID)
		l.requestMutex.Unlock()
	}()

	if err := l.multiplexer.WriteMessageWithSequence(ctx, requestID, headerData); err != nil {
		return nil, fmt.Errorf("failed to write request message: %w", err)
	}

	select {
	case responseData, ok := <-responseChan:
		if !ok {
			return nil, fmt.Errorf("response channel closed, loader shutting down")
		}

		var responseHeader Header
		if err := responseHeader.UnmarshalBinary(responseData); err != nil {
			return nil, fmt.Errorf("failed to decode response header: %w", err)
		}

		if responseHeader.IsError {
			errMsg := string(responseHeader.Payload)
			return nil, fmt.Errorf("module error for request %s: %s", name, errMsg)
		}

		return responseHeader.Payload, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-l.loadCtx.Done():
		return nil, fmt.Errorf("loader is shutting down")
	}
}

// SendMessageWithSequence sends a message to the module with a specific sequence ID (for responses)
func (l *Loader) SendMessageWithSequence(ctx context.Context, sequenceID uint32, name string, payload []byte, isError bool) error {
	if l.closed.Load() {
		return fmt.Errorf("loader is closed")
	}

	if l.multiplexer == nil {
		return fmt.Errorf("multiplexer not available")
	}

	messageType := MessageTypeResponse
	if isError {
		messageType = MessageTypeError
	}

	header := Header{
		Name:        name,
		IsError:     isError,
		MessageType: messageType,
		Payload:     payload,
	}

	headerData, err := header.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal header: %w", err)
	}

	return l.multiplexer.WriteMessageWithSequence(ctx, sequenceID, headerData)
}
