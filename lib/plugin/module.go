package plugin

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snowmerak/plugin.go/lib/multiplexer"
)

// oldNodeWrapper wraps the original Node to provide our interface.
// This is used for compatibility with the old API.
type oldNodeWrapper struct {
	node *multiplexer.Node
}

// WriteMessage wraps the Node's WriteMessage method.
func (n *oldNodeWrapper) WriteMessage(ctx context.Context, data []byte) error {
	return n.node.WriteMessage(ctx, data)
}

// WriteMessageWithSequence wraps the Node's WriteMessageWithSequence method.
func (n *oldNodeWrapper) WriteMessageWithSequence(ctx context.Context, seq uint32, data []byte) error {
	return n.node.WriteMessageWithSequence(ctx, seq, data)
}

// ReadMessage wraps the Node's ReadMessage and converts to old format.
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

// nodeWrapper wraps the new multiplexer API to provide the old Node interface.
type nodeWrapper struct {
	multiplexer multiplexer.Multiplexer
	sequence    uint32
	mu          sync.Mutex
}

// OldMessage represents a message in the old format for backward compatibility.
type OldMessage struct {
	ID   uint32
	Data []byte
	Type uint8
}

// WriteMessage wraps the new API for writing messages.
func (n *nodeWrapper) WriteMessage(ctx context.Context, data []byte) error {
	return n.multiplexer.WriteMessage(ctx, data)
}

// WriteMessageWithSequence wraps the new API for writing messages with sequence.
func (n *nodeWrapper) WriteMessageWithSequence(ctx context.Context, seq uint32, data []byte) error {
	return n.multiplexer.WriteMessageWithSequence(ctx, seq, data)
}

// ReadMessage wraps the new API and converts to old format.
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

// Header represents the message header containing service name, error status, and payload.
type Header struct {
	Name    string
	IsError bool
	Payload []byte
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
}

// New creates a new Module instance with the specified reader and writer.
// If reader or writer are nil, they default to os.Stdin and os.Stdout respectively.
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
		multiplexer:       &nodeWrapper{multiplexer: mux},
		handler:           make(map[string]Handler),
		shutdownChan:      make(chan struct{}),
		forceShutdownChan: make(chan struct{}),
	}
}

// NewStd creates a new Module instance using standard input and output.
func NewStd() *Module {
	return New(os.Stdin, os.Stdout)
}

// RegisterHandler registers a handler function for the given service name.
// The handler function processes raw byte payloads and returns raw byte responses.
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

// SendReady sends a ready message to indicate the plugin is ready to receive requests.
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

// Shutdown initiates graceful shutdown of the module.
func (m *Module) Shutdown() {
	m.shutdownOnce.Do(func() {
		close(m.shutdownChan)
	})
}

// ForceShutdown initiates immediate shutdown of the module.
func (m *Module) ForceShutdown() {
	m.forceShutdownOnce.Do(func() {
		close(m.forceShutdownChan)
	})
}

// IsShutdown returns true if the module is shutting down (gracefully).
func (m *Module) IsShutdown() bool {
	select {
	case <-m.shutdownChan:
		return true
	default:
		return false
	}
}

// IsForceShutdown returns true if the module is force shutting down.
func (m *Module) IsForceShutdown() bool {
	select {
	case <-m.forceShutdownChan:
		return true
	default:
		return false
	}
}

// getActiveJobCount returns the current number of active jobs.
func (m *Module) getActiveJobCount() int64 {
	return atomic.LoadInt64(&m.activeJobCount)
}

// Listen starts listening for incoming messages and processes them.
// It handles graceful and force shutdown signals appropriately.
// Automatically sends a ready signal when starting to listen.
func (m *Module) Listen(ctx context.Context) error {
	recv, err := m.multiplexer.ReadMessage(ctx)
	if err != nil {
		return fmt.Errorf("failed to read message: %w", err)
	}

	// Automatically send ready signal after setting up the receiver but before starting to listen
	if err := m.SendReady(ctx); err != nil {
		return fmt.Errorf("failed to send ready signal: %w", err)
	}

	// Create a context that gets cancelled on force shutdown or parent context cancellation
	listenCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Monitor for FORCE shutdown signal only (graceful shutdown is handled in message loop)
	go func() {
		select {
		case <-m.forceShutdownChan:
			cancel()
		case <-ctx.Done():
			cancel()
		}
	}()

	for {
		select {
		case mesg, ok := <-recv:
			if !ok {
				// Channel closed, exit gracefully
				break
			}

			// Check for force shutdown first - immediate exit
			if m.IsForceShutdown() {
				return ctx.Err()
			}

			// Check for shutdown message
			var header Header
			if err := header.UnmarshalBinary(mesg.Data); err == nil {
				if header.Name == "shutdown" {
					m.Shutdown()

					// Send immediate ACK to let host know we received the shutdown signal
					ackHeader := Header{
						Name:    "shutdown_ack",
						IsError: false,
						Payload: []byte("graceful shutdown started, waiting for jobs to complete"),
					}

					if ackData, err := ackHeader.MarshalBinary(); err == nil {
						m.multiplexer.WriteMessageWithSequence(listenCtx, mesg.ID, ackData)
					}

					// Start graceful shutdown process in a goroutine
					go func() {
						// Give a moment for any pending jobs to be added to activeJobs
						time.Sleep(100 * time.Millisecond)

						// Wait for active jobs to complete (no timeout for graceful shutdown)
						done := make(chan struct{})
						go func() {
							m.activeJobs.Wait()
							close(done)
						}()

						select {
						case <-done:
							// All jobs completed
						case <-m.forceShutdownChan:
							// Force shutdown received during graceful shutdown
						}

						// After graceful shutdown is complete, cancel the context to exit the main loop
						cancel()
					}()

					continue // Continue listening for more messages (including force_shutdown)
				} else if header.Name == "force_shutdown" {
					m.ForceShutdown()

					// Send immediate acknowledgment
					ackHeader := Header{
						Name:    "force_shutdown_ack",
						IsError: false,
						Payload: []byte("force shutting down"),
					}

					if ackData, err := ackHeader.MarshalBinary(); err == nil {
						m.multiplexer.WriteMessageWithSequence(listenCtx, mesg.ID, ackData)
					}

					return nil
				}
			}

			// Reject new requests during graceful shutdown
			if m.IsShutdown() {
				// Send immediate error response
				errorHeader := Header{
					Name:    header.Name,
					IsError: true,
					Payload: []byte("service unavailable: graceful shutdown in progress"),
				}
				if errorData, err := errorHeader.MarshalBinary(); err == nil {
					m.multiplexer.WriteMessageWithSequence(listenCtx, mesg.ID, errorData)
				}
				continue // Do not process new requests
			}

			// Process regular messages (only if not shutting down)
			m.activeJobs.Add(1)
			atomic.AddInt64(&m.activeJobCount, 1)
			go func(msg *OldMessage) {
				defer func() {
					m.activeJobs.Done()
					atomic.AddInt64(&m.activeJobCount, -1)
				}()
				m.processMessage(listenCtx, msg)
			}(mesg)

		case <-listenCtx.Done():
			// Context cancelled (shutdown or parent context)

			// Wait for active jobs to complete with timeout
			done := make(chan struct{})
			go func() {
				m.activeJobs.Wait()
				close(done)
			}()

			select {
			case <-done:
				// All active jobs completed
			case <-time.After(5 * time.Second):
				// Shutdown timeout reached
			}

			return listenCtx.Err()
		}
	}
}

// processMessage handles a single message asynchronously.
// Already started tasks are allowed to complete even during graceful shutdown.
func (m *Module) processMessage(ctx context.Context, mesg *OldMessage) {
	// Important: Remove shutdown check so already started tasks can complete
	// Only new requests are blocked in Listen()

	var requestHeader Header
	if err := requestHeader.UnmarshalBinary(mesg.Data); err != nil {
		// Cannot reliably form a response if header is malformed. Just return.
		return
	}

	// Exit immediately only for force shutdown
	if m.IsForceShutdown() {
		responseHeader := Header{
			Name:    requestHeader.Name,
			IsError: true,
			Payload: []byte("service unavailable: force shutdown"),
		}
		if responseData, err := responseHeader.MarshalBinary(); err == nil {
			m.multiplexer.WriteMessageWithSequence(ctx, mesg.ID, responseData)
		}
		return
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
		// Execute handler - already started tasks complete even during graceful shutdown
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
		// Cannot marshal response, just return
		return
	}

	// Send response back with the original message ID for proper correlation
	if err := m.multiplexer.WriteMessageWithSequence(ctx, mesg.ID, responseData); err != nil {
		// Log error to stderr if possible, but don't fail the entire listener
	}
}
