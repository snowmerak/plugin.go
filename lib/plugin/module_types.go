// Package plugin provides types and interfaces for the Module functionality.
//
// This file contains the core type definitions, constants, and interfaces
// used by the Module system for plugin communication and message handling.
package plugin

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
)

// MessageType represents the type of message being sent
type MessageType uint8

const (
	MessageTypeRequest  MessageType = 0x01 // Request message (expects response)
	MessageTypeResponse MessageType = 0x02 // Response message (response to request)
	MessageTypeNotify   MessageType = 0x03 // Notification message (no response expected)
	MessageTypeAck      MessageType = 0x04 // Acknowledgment message
	MessageTypeError    MessageType = 0x05 // Error message
)

// String returns the string representation of MessageType
func (mt MessageType) String() string {
	switch mt {
	case MessageTypeRequest:
		return "Request"
	case MessageTypeResponse:
		return "Response"
	case MessageTypeNotify:
		return "Notify"
	case MessageTypeAck:
		return "Ack"
	case MessageTypeError:
		return "Error"
	default:
		return "Unknown"
	}
}

// Header represents the message header containing service name, error status, and payload.
type Header struct {
	Name        string
	IsError     bool
	MessageType MessageType
	Payload     []byte
}

// MarshalBinary encodes the header into binary format.
func (h *Header) MarshalBinary() ([]byte, error) {
	var buffer bytes.Buffer

	nameBytes := []byte(h.Name)
	nameLen := uint32(len(nameBytes))

	// Write name length
	if err := binary.Write(&buffer, binary.BigEndian, nameLen); err != nil {
		return nil, fmt.Errorf("failed to write name length: %w", err)
	}

	// Write name
	if _, err := buffer.Write(nameBytes); err != nil {
		return nil, fmt.Errorf("failed to write name: %w", err)
	}

	// Write IsError
	var isErrorByte byte
	if h.IsError {
		isErrorByte = 1
	}
	if err := binary.Write(&buffer, binary.BigEndian, isErrorByte); err != nil {
		return nil, fmt.Errorf("failed to write IsError flag: %w", err)
	}

	// Write MessageType
	if err := binary.Write(&buffer, binary.BigEndian, uint8(h.MessageType)); err != nil {
		return nil, fmt.Errorf("failed to write message type: %w", err)
	}

	// Write Payload length
	payloadLen := uint32(len(h.Payload))
	if err := binary.Write(&buffer, binary.BigEndian, payloadLen); err != nil {
		return nil, fmt.Errorf("failed to write payload length: %w", err)
	}

	// Write Payload
	if _, err := buffer.Write(h.Payload); err != nil {
		return nil, fmt.Errorf("failed to write payload: %w", err)
	}

	return buffer.Bytes(), nil
}

// UnmarshalBinary decodes the header from binary format.
func (h *Header) UnmarshalBinary(data []byte) error {
	buffer := bytes.NewReader(data)

	// Read name length
	var nameLen uint32
	if err := binary.Read(buffer, binary.BigEndian, &nameLen); err != nil {
		return fmt.Errorf("failed to read name length: %w", err)
	}

	// Read name
	nameBytes := make([]byte, nameLen)
	if _, err := io.ReadFull(buffer, nameBytes); err != nil { // Use io.ReadFull for precise reads
		return fmt.Errorf("failed to read name: %w", err)
	}
	h.Name = string(nameBytes)

	// Read IsError
	var isErrorByte byte
	if err := binary.Read(buffer, binary.BigEndian, &isErrorByte); err != nil {
		return fmt.Errorf("failed to read IsError flag: %w", err)
	}
	h.IsError = isErrorByte == 1

	// Read MessageType
	var messageTypeByte uint8
	if err := binary.Read(buffer, binary.BigEndian, &messageTypeByte); err != nil {
		return fmt.Errorf("failed to read message type: %w", err)
	}
	h.MessageType = MessageType(messageTypeByte)

	// Read Payload length
	var payloadLen uint32
	if err := binary.Read(buffer, binary.BigEndian, &payloadLen); err != nil {
		return fmt.Errorf("failed to read payload length: %w", err)
	}

	// Read Payload
	h.Payload = make([]byte, payloadLen)
	if _, err := io.ReadFull(buffer, h.Payload); err != nil { // Use io.ReadFull
		return fmt.Errorf("failed to read payload: %w", err)
	}

	return nil
}

// AppHandlerResult holds the result of an application handler execution.
type AppHandlerResult struct {
	Payload []byte // Raw payload
	IsError bool   // True if Payload is an error payload
}

// Handler defines the function signature for registered handlers.
// It returns the raw payload (either success data or error data)
// and a boolean indicating if it's an error payload.
// The second error return is for critical errors within the wrapper itself.
type Handler func(requestPayload []byte) (AppHandlerResult, error)

// NodeInterface defines the interface for multiplexer operations.
type NodeInterface interface {
	WriteMessage(ctx context.Context, data []byte) error
	WriteMessageWithSequence(ctx context.Context, seq uint32, data []byte) error
	ReadMessage(ctx context.Context) (chan *OldMessage, error)
}

// Module represents a plugin module that handles incoming requests and dispatches them to registered handlers.
type Module struct {
	multiplexer       NodeInterface
	handler           map[string]Handler
	handlerLock       sync.RWMutex
	shutdownChan      chan struct{}
	forceShutdownChan chan struct{}
	shutdownOnce      sync.Once
	forceShutdownOnce sync.Once
	activeJobs        sync.WaitGroup
	activeJobCount    int64 // atomic counter for active jobs

	// Communication options and provider
	options  *ModuleOptions
	provider CommunicationProvider
}
