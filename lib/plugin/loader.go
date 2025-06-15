package plugin

import (
	"context"
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

	loadCtx    context.Context
	cancelLoad context.CancelFunc
	closed     atomic.Bool
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
	if l.closed.Load() {
		return fmt.Errorf("loader is closed")
	}

	p, err := process.Fork(l.Path)
	if err != nil {
		return fmt.Errorf("failed to fork process: %w", err)
	}
	l.process = p

	mux := multiplexer.NewNode(p.Stdout(), p.Stdin())
	l.multiplexer = mux

	// Create a new context for the loader's internal operations, like the read loop.
	// This allows Close() to cancel it independently of the context passed to Load().
	l.loadCtx, l.cancelLoad = context.WithCancel(context.Background())

	// Use l.loadCtx for ReadMessage so it can be cancelled by l.Close()
	recv, err := l.multiplexer.ReadMessage(l.loadCtx)
	if err != nil {
		// If ReadMessage fails to start, ensure process is cleaned up.
		p.Close()
		// cancelLoad might be nil if ReadMessage failed before it was fully set up.
		if l.cancelLoad != nil {
			l.cancelLoad()
		}
		return fmt.Errorf("failed to read message: %w", err)
	}

	go func() {
		defer func() {
			// This defer runs when the goroutine exits, e.g., when l.loadCtx is cancelled.
			l.requestMutex.Lock()
			for id, ch := range l.pendingRequests {
				close(ch) // Unblock any Call goroutines still waiting
				delete(l.pendingRequests, id)
			}
			l.requestMutex.Unlock()
		}()

		for {
			select {
			case <-l.loadCtx.Done(): // Loader.Close() was called or parent context for Load cancelled
				return
			case mesg, ok := <-recv:
				if !ok { // recv channel was closed
					return
				}

				requestID := mesg.ID

				l.requestMutex.RLock()
				responseChan, exists := l.pendingRequests[requestID]
				l.requestMutex.RUnlock()

				if exists {
					select {
					case responseChan <- mesg.Data:
					case <-l.loadCtx.Done(): // Ensure we don't block if loader is closing
						// responseChan will be closed by the defer above
					default:
						// This case means responseChan was full (buffer 1) and Call wasn't reading.
						// Or Call timed out and removed its entry, but responseChan wasn't closed yet.
						// The response is dropped. Call will time out or has already.
					}
				}
			}
		}
	}()

	return nil
}

// Close shuts down the loader, terminates the plugin process, and cleans up resources.
func (l *Loader) Close() error {
	if !l.closed.CompareAndSwap(false, true) {
		return fmt.Errorf("loader already closed")
	}

	if l.cancelLoad != nil {
		l.cancelLoad() // Signal the Load goroutine to stop
	}

	// The Load goroutine's defer will handle closing pendingRequest channels.

	if l.multiplexer != nil {
		// Multiplexer itself doesn't have a Close. Its operations will fail
		// once the underlying process pipes are closed or its context is cancelled.
	}

	if l.process != nil {
		return l.process.Close()
	}

	return nil
}

func Call[Req, Res any](ctx context.Context, l *Loader, name string, request Req) (*Res, error) {
	if l.closed.Load() {
		return nil, fmt.Errorf("loader is closed")
	}

	if l.multiplexer == nil {
		return nil, fmt.Errorf("loader not loaded or load failed")
	}

	requestPayloadBytes, err := gobEncode(request)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	requestHeader := Header{
		Name:    name,
		IsError: false, // Requests from Call are not errors
		Payload: requestPayloadBytes,
	}

	headerData, err := requestHeader.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to encode header: %w", err)
	}

	l.requestMutex.Lock()
	requestID := l.requestID.Add(1)
	responseChan := make(chan []byte, 1)
	if l.closed.Load() {
		l.requestMutex.Unlock()
		close(responseChan)
		return nil, fmt.Errorf("loader closed before dispatching request")
	}
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
			return nil, fmt.Errorf("response channel closed, loader might be shutting down")
		}

		var responseHeader Header
		if err := responseHeader.UnmarshalBinary(responseData); err != nil {
			return nil, fmt.Errorf("failed to decode response header: %w", err)
		}

		if responseHeader.IsError {
			var errMsg string
			if err := gobDecode(responseHeader.Payload, &errMsg); err != nil {
				return nil, fmt.Errorf("failed to decode error message from plugin (name: %s): %w", name, err)
			}
			return nil, fmt.Errorf("plugin error for service %s: %s", name, errMsg)
		}

		var actualResponse Res
		if err := gobDecode(responseHeader.Payload, &actualResponse); err != nil {
			return nil, fmt.Errorf("failed to decode success response for service %s: %w", name, err)
		}
		return &actualResponse, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
