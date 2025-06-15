package plugin

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/snowmerak/plugin.go/lib/multiplexer"
)

// gobEncode is a helper function to GOB-encode a value.
func gobEncode(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		return nil, fmt.Errorf("gob encode error: %w", err)
	}
	return buf.Bytes(), nil
}

// gobDecode is a helper function to GOB-decode data into a value.
func gobDecode(data []byte, v any) error {
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(v); err != nil {
		return fmt.Errorf("gob decode error: %w", err)
	}
	return nil
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
	Payload []byte // GOB-encoded payload
	IsError bool   // True if Payload is a GOB-encoded error message string
}

// Handler defines the function signature for registered handlers.
// It returns the GOB-encoded payload (either success data or error string)
// and a boolean indicating if it's an error payload.
// The second error return is for critical errors within the wrapper itself (e.g., GOB encoding failure).
type Handler func(requestPayload []byte) (AppHandlerResult, error)

type Module struct {
	multiplexer *multiplexer.Node
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

	mux := multiplexer.NewNode(reader, writer) // Use passed reader and writer

	return &Module{
		multiplexer: mux,
		handler:     make(map[string]Handler),
	}
}

func RegisterHandler[Req, Res any](m *Module, name string, handler func(Req) (Res, error)) {
	m.handlerLock.Lock()
	defer m.handlerLock.Unlock()

	if _, exists := m.handler[name]; exists {
		panic(fmt.Sprintf("handler for %s already registered", name))
	}

	m.handler[name] = func(requestPayload []byte) (AppHandlerResult, error) {
		var req Req
		if err := gobDecode(requestPayload, &req); err != nil {
			errMsg := fmt.Sprintf("failed to decode request for %s: %v", name, err)
			errPayload, encErr := gobEncode(errMsg)
			if encErr != nil {
				// This is a critical error, as we can't even encode the decode error.
				return AppHandlerResult{}, fmt.Errorf("critical: failed to encode request decode error message: %w", encErr)
			}
			return AppHandlerResult{Payload: errPayload, IsError: true}, nil
		}

		res, appErr := handler(req)
		if appErr != nil {
			// Application-level error from the handler
			errPayload, encErr := gobEncode(appErr.Error())
			if encErr != nil {
				return AppHandlerResult{}, fmt.Errorf("critical: failed to encode application error message: %w", encErr)
			}
			return AppHandlerResult{Payload: errPayload, IsError: true}, nil
		}

		// Success
		successPayload, encErr := gobEncode(res)
		if encErr != nil {
			return AppHandlerResult{}, fmt.Errorf("critical: failed to encode success response: %w", encErr)
		}
		return AppHandlerResult{Payload: successPayload, IsError: false}, nil
	}
}

func (m *Module) Listen(ctx context.Context) error {
	recv, err := m.multiplexer.ReadMessage(ctx)
	if err != nil {
		return fmt.Errorf("failed to read message: %w", err)
	}

	for mesg := range recv {
		var requestHeader Header
		if err := requestHeader.UnmarshalBinary(mesg.Data); err != nil {
			// Cannot reliably form a response if header is malformed. Log and continue or stop.
			// For now, returning error stops the listener.
			// Consider logging: log.Printf("failed to decode header: %v, message ID: %d", err, mesg.ID)
			return fmt.Errorf("failed to decode header: %w", err) // This stops the module
		}

		m.handlerLock.RLock()
		targetHandler, exists := m.handler[requestHeader.Name]
		m.handlerLock.RUnlock()

		var responseHeader Header
		responseHeader.Name = requestHeader.Name

		if !exists {
			errMsg := fmt.Sprintf("no handler registered for service: %s", requestHeader.Name)
			errPayload, encErr := gobEncode(errMsg)
			if encErr != nil {
				// Log this critical internal error, as we can't send it back easily.
				// log.Printf("critical: failed to encode 'handler not found' error for %s: %v", requestHeader.Name, encErr)
				// Potentially continue to next message or stop module if this is considered fatal.
				// For now, let's try to continue to the next message.
				fmt.Fprintf(os.Stderr, "critical: failed to encode 'handler not found' error for %s: %v\n", requestHeader.Name, encErr)
				continue
			}
			responseHeader.IsError = true
			responseHeader.Payload = errPayload
		} else {
			appResult, criticalErr := targetHandler(requestHeader.Payload)
			if criticalErr != nil {
				// A critical error occurred within the handler wrapper (e.g., GOB encoding its own result failed).
				// Log this and decide how to proceed. Sending a generic error back might be an option.
				// log.Printf("critical error in handler for %s: %v", requestHeader.Name, criticalErr)
				// For now, let's try to inform the client if possible.
				errMsg := fmt.Sprintf("critical internal error processing request for %s: %v", requestHeader.Name, criticalErr)
				errPayload, encErr := gobEncode(errMsg)
				if encErr != nil {
					fmt.Fprintf(os.Stderr, "super critical: failed to encode critical error message for %s: %v\n", requestHeader.Name, encErr)
					continue // Give up on this request
				}
				responseHeader.IsError = true
				responseHeader.Payload = errPayload
			} else {
				responseHeader.IsError = appResult.IsError
				responseHeader.Payload = appResult.Payload
			}
		}

		responseData, err := responseHeader.MarshalBinary()
		if err != nil {
			// log.Printf("failed to encode response header for %s: %v", responseHeader.Name, err)
			fmt.Fprintf(os.Stderr, "failed to encode response header for %s: %v\n", responseHeader.Name, err)
			continue // Try to process next message
		}

		if err := m.multiplexer.WriteMessageWithSequence(ctx, mesg.ID, responseData); err != nil {
			// log.Printf("failed to write response for %s: %v", responseHeader.Name, err)
			fmt.Fprintf(os.Stderr, "failed to write response for %s: %v\n", responseHeader.Name, err)
			// This could be a more serious issue, potentially return err to stop Listen
			return fmt.Errorf("failed to write response for %s: %w", responseHeader.Name, err)
		}
	}

	return nil
}
