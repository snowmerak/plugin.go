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

type Header[T any] struct {
	Name    string
	Message T
}

func (h *Header[T]) MarshalBinary() ([]byte, error) {
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

	// Write message
	if err := gob.NewEncoder(&buffer).Encode(h.Message); err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}

	return buffer.Bytes(), nil
}

func (h *Header[T]) UnmarshalBinary(data []byte) error {
	buffer := bytes.NewReader(data)

	// Read name length
	var nameLen uint32
	if err := binary.Read(buffer, binary.BigEndian, &nameLen); err != nil {
		return fmt.Errorf("failed to read name length: %w", err)
	}

	// Read name
	nameBytes := make([]byte, nameLen)
	if _, err := buffer.Read(nameBytes); err != nil {
		return fmt.Errorf("failed to read name: %w", err)
	}
	h.Name = string(nameBytes)

	// Read message
	if err := gob.NewDecoder(buffer).Decode(&h.Message); err != nil {
		return fmt.Errorf("failed to decode message: %w", err)
	}

	return nil
}

type Handler func([]byte) ([]byte, error)

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

	mux := multiplexer.NewNode(os.Stdin, os.Stdout)

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

	m.handler[name] = func(data []byte) ([]byte, error) {
		var req Req
		if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&req); err != nil {
			return nil, fmt.Errorf("failed to decode request: %w", err)
		}

		res, err := handler(req)
		if err != nil {
			return nil, fmt.Errorf("handler error: %w", err)
		}

		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(res); err != nil {
			return nil, fmt.Errorf("failed to encode response: %w", err)
		}

		return buf.Bytes(), nil
	}
}

func (m *Module) Listen(ctx context.Context) error {
	recv, err := m.multiplexer.ReadMessage(ctx)
	if err != nil {
		return fmt.Errorf("failed to read message: %w", err)
	}

	for mesg := range recv {
		var header Header[[]byte]
		if err := header.UnmarshalBinary(mesg.Data); err != nil {
			return fmt.Errorf("failed to decode header: %w", err)
		}

		m.handlerLock.RLock()
		handler, exists := m.handler[header.Name]
		m.handlerLock.RUnlock()

		if !exists {
			return fmt.Errorf("no handler registered for service: %s", header.Name)
		}

		response, err := handler(header.Message)
		if err != nil {
			return fmt.Errorf("handler error: %w", err)
		}

		responseHeader := Header[[]byte]{
			Name:    header.Name,
			Message: response,
		}

		responseData, err := responseHeader.MarshalBinary()
		if err != nil {
			return fmt.Errorf("failed to encode response header: %w", err)
		}

		if err := m.multiplexer.WriteMessageWithSequence(ctx, mesg.ID, responseData); err != nil {
			return fmt.Errorf("failed to write response: %w", err)
		}
	}

	return nil
}
