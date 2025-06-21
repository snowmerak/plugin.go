package plugin

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/snowmerak/plugin.go/lib/multiplexer"
)

// oldNodeWrapper wraps the original Node to provide our interface
type oldNodeWrapper struct {
	node *multiplexer.Node
}

// WriteMessage wraps the Node's WriteMessage
func (n *oldNodeWrapper) WriteMessage(ctx context.Context, data []byte) error {
	return n.node.WriteMessage(ctx, data)
}

// WriteMessageWithSequence wraps the Node's WriteMessageWithSequence
func (n *oldNodeWrapper) WriteMessageWithSequence(ctx context.Context, seq uint32, data []byte) error {
	return n.node.WriteMessageWithSequence(ctx, seq, data)
}

// ReadMessage wraps the Node's ReadMessage and converts to old format
func (n *oldNodeWrapper) ReadMessage(ctx context.Context) (chan *OldMessage, error) {
	newCh, err := n.node.ReadMessage(ctx)
	if err != nil {
		return nil, err
	}

	oldCh := make(chan *OldMessage, 100)
	go func() {
		defer close(oldCh)
		for newMsg := range newCh {
			oldMsg := &OldMessage{
				ID:   newMsg.ID,
				Data: newMsg.Data,
				Type: newMsg.Type,
			}
			oldCh <- oldMsg
		}
	}()

	return oldCh, nil
}

// nodeWrapper wraps the new multiplexer API to provide the old Node interface
type nodeWrapper struct {
	multiplexer multiplexer.Multiplexer
	sequence    uint32
	mu          sync.Mutex
}

// Message represents a message in the old format
type OldMessage struct {
	ID   uint32
	Data []byte
	Type uint8
}

// WriteMessage wraps the new API
func (n *nodeWrapper) WriteMessage(ctx context.Context, data []byte) error {
	return n.multiplexer.WriteMessage(ctx, data)
}

// WriteMessageWithSequence wraps the new API
func (n *nodeWrapper) WriteMessageWithSequence(ctx context.Context, seq uint32, data []byte) error {
	return n.multiplexer.WriteMessageWithSequence(ctx, seq, data)
}

// ReadMessage wraps the new API and converts to old format
func (n *nodeWrapper) ReadMessage(ctx context.Context) (chan *OldMessage, error) {
	newCh, err := n.multiplexer.ReadMessage(ctx)
	if err != nil {
		return nil, err
	}

	oldCh := make(chan *OldMessage, 100)
	go func() {
		defer close(oldCh)
		for newMsg := range newCh {
			oldMsg := &OldMessage{
				ID:   newMsg.Sequence,
				Data: newMsg.Data,
				Type: 0x05, // MessageHeaderTypeComplete
			}
			oldCh <- oldMsg
		}
	}()

	return oldCh, nil
}

type Header struct {
	Name    string
	IsError bool
	Payload []byte
}

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

// NodeInterface defines the interface for multiplexer operations
type NodeInterface interface {
	WriteMessage(ctx context.Context, data []byte) error
	WriteMessageWithSequence(ctx context.Context, seq uint32, data []byte) error
	ReadMessage(ctx context.Context) (chan *OldMessage, error)
}

type Module struct {
	multiplexer NodeInterface
	handler     map[string]Handler
	handlerLock sync.RWMutex
}

func New(reader io.Reader, writer io.Writer) *Module {
	if reader == nil {
		reader = os.Stdin
	}

	if writer == nil {
		writer = os.Stdout
	}
	// Use the general multiplexer API which uses HybridNode
	mux := multiplexer.New(reader, writer)

	return &Module{
		multiplexer: &nodeWrapper{multiplexer: mux},
		handler:     make(map[string]Handler),
	}
}

func RegisterHandler(m *Module, name string, handler func(requestPayload []byte) (responsePayload []byte, isAppError bool)) {
	m.handlerLock.Lock()
	defer m.handlerLock.Unlock()

	if _, exists := m.handler[name]; exists {
		panic(fmt.Sprintf("handler for %s already registered", name))
	}

	m.handler[name] = func(requestPayload []byte) (AppHandlerResult, error) {
		// The user-provided handler now directly processes []byte and returns []byte.
		// No GOB decoding of request or GOB encoding of response/error is done here.

		responseBytes, isErr := handler(requestPayload)

		// The 'error' returned by this function is for critical errors within this wrapper itself,
		// which are now minimal as GOB processing is removed.
		return AppHandlerResult{Payload: responseBytes, IsError: isErr}, nil
	}
}

// SendReady sends a ready message to indicate the plugin is ready to receive requests
func (m *Module) SendReady(ctx context.Context) error {
	readyHeader := Header{
		Name:    "ready",
		IsError: false,
		Payload: []byte("ready"),
	}

	readyData, err := readyHeader.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal ready header: %w", err)
	}

	return m.multiplexer.WriteMessage(ctx, readyData)
}

func (m *Module) Listen(ctx context.Context) error {
	recv, err := m.multiplexer.ReadMessage(ctx)
	if err != nil {
		return fmt.Errorf("failed to read message: %w", err)
	}

	for mesg := range recv {
		var requestHeader Header
		if err := requestHeader.UnmarshalBinary(mesg.Data); err != nil {
			// Cannot reliably form a response if header is malformed. Log and continue.
			continue // Continue to the next message
		}

		m.handlerLock.RLock()
		targetHandler, exists := m.handler[requestHeader.Name]
		m.handlerLock.RUnlock()

		var responseHeader Header
		responseHeader.Name = requestHeader.Name

		if !exists {
			errMsg := fmt.Sprintf("no handler registered for service: %s", requestHeader.Name)
			errPayload := []byte(errMsg)
			responseHeader.IsError = true
			responseHeader.Payload = errPayload
		} else {
			appResult, criticalErr := targetHandler(requestHeader.Payload)

			if criticalErr != nil {
				errMsg := fmt.Sprintf("critical internal error processing request for %s: %v", requestHeader.Name, criticalErr)
				errPayload := []byte(errMsg)
				responseHeader.IsError = true
				responseHeader.Payload = errPayload
			} else {
				responseHeader.IsError = appResult.IsError
				responseHeader.Payload = appResult.Payload
			}
		}

		responseData, err := responseHeader.MarshalBinary()
		if err != nil {
			continue // Try to process next message
		}

		if err := m.multiplexer.WriteMessageWithSequence(ctx, mesg.ID, responseData); err != nil {
			return fmt.Errorf("failed to write response for %s: %w", responseHeader.Name, err)
		}
	}

	return nil
}
