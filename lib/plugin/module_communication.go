// Package plugin provides communication functionality for Module.
//
// This file contains methods for sending messages, requests, and responses
// between the plugin module and the loader/host application.
package plugin

import (
	"context"
	"fmt"
	"time"
)

// SendReady sends a ready message to indicate the plugin is ready to receive requests.
func (m *Module) SendReady(ctx context.Context) error {
	readyHeader := Header{
		Name:        "ready",
		IsError:     false,
		MessageType: MessageTypeAck,
		Payload:     []byte("ready"),
	}

	readyData, err := readyHeader.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal ready header: %w", err)
	}

	return m.multiplexer.WriteMessage(ctx, readyData)
}

// SendMessage sends a message to the loader without expecting a response
func (m *Module) SendMessage(ctx context.Context, name string, payload []byte) error {
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

	return m.multiplexer.WriteMessage(ctx, headerData)
}

// SendErrorMessage sends an error message to the loader
func (m *Module) SendErrorMessage(ctx context.Context, name string, payload []byte) error {
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

	return m.multiplexer.WriteMessage(ctx, headerData)
}

// SendRequest sends a request to the loader and waits for a response
func (m *Module) SendRequest(ctx context.Context, name string, payload []byte) ([]byte, error) {
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

	// Generate a unique sequence ID for this request
	sequenceID := m.generateSequenceID()

	// Create a channel to receive the response
	responseChannel := make(chan []byte, 1)
	defer close(responseChannel)

	// Store the response channel (note: this is a simple implementation;
	// a production version might need a more sophisticated pending request system)

	// Send the request
	if err := m.multiplexer.WriteMessageWithSequence(ctx, sequenceID, headerData); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Since we don't have a built-in response tracking system in Module,
	// we'll implement a simple timeout-based approach
	// In a real implementation, you'd want to add response tracking to Module

	// For now, return an error indicating this feature needs implementation
	return nil, fmt.Errorf("SendRequest not fully implemented - response tracking needed")
}

// generateSequenceID generates a unique sequence ID for requests
func (m *Module) generateSequenceID() uint32 {
	// This is a simple implementation; in production you might want something more sophisticated
	return uint32(time.Now().UnixNano() & 0xFFFFFFFF)
}
