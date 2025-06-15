package plugin

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/snowmerak/plugin.go/lib/multiplexer"
	"github.com/snowmerak/plugin.go/lib/process"
)

type Loader struct {
	Path    string
	Name    string
	Version string

	process     *process.Process
	multiplexer *multiplexer.Node

	requestID atomic.Uint64

	pendingRequests map[uint64]chan []byte
	requestMutex    sync.RWMutex
}

func NewLoader(path, name, version string) *Loader {
	return &Loader{
		Path:            path,
		Name:            name,
		Version:         version,
		process:         nil,
		multiplexer:     nil,
		pendingRequests: make(map[uint64]chan []byte),
	}
}

func (l *Loader) Load(ctx context.Context) error {
	p, err := process.Fork(l.Path)
	if err != nil {
		return fmt.Errorf("failed to fork process: %w", err)
	}
	l.process = p

	mux := multiplexer.NewNode(p.Stdout(), p.Stdin())
	l.multiplexer = mux

	recv, err := l.multiplexer.ReadMessage(ctx)
	if err != nil {
		return fmt.Errorf("failed to read message: %w", err)
	}

	go func() {
		for mesg := range recv {
			requestID := mesg.ID

			l.requestMutex.RLock()
			responseChan, exists := l.pendingRequests[requestID]
			l.requestMutex.RUnlock()

			if exists {
				select {
				case responseChan <- mesg.Data:
				default:
				}
			}
		}
	}()

	return nil
}

func Call[Req, Res any](ctx context.Context, l *Loader, name string, request Req) (*Res, error) {
	requestBuffer := bytes.NewBuffer(nil)
	if err := gob.NewEncoder(requestBuffer).Encode(request); err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	header := Header[[]byte]{
		Name:    name,
		Message: requestBuffer.Bytes(),
	}

	headerData, err := header.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to encode header: %w", err)
	}

	l.requestMutex.Lock()
	requestID := l.requestID.Add(1)
	responseChan := make(chan []byte, 1)
	l.pendingRequests[requestID] = responseChan
	l.requestMutex.Unlock()

	defer func() {
		l.requestMutex.Lock()
		delete(l.pendingRequests, requestID)
		close(responseChan)
		l.requestMutex.Unlock()
	}()

	if err := l.multiplexer.WriteMessageWithSequence(ctx, requestID, headerData); err != nil {
		return nil, fmt.Errorf("failed to write request message: %w", err)
	}

	select {
	case responseData := <-responseChan:
		var responseHeader Header[[]byte]
		if err := responseHeader.UnmarshalBinary(responseData); err != nil {
			return nil, fmt.Errorf("failed to decode response header: %w", err)
		}

		var response Res
		if err := gob.NewDecoder(bytes.NewReader(responseHeader.Message)).Decode(&response); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		return &response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
