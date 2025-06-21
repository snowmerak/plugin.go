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

	requestID atomic.Uint32 // Changed atomic.Uint64 to atomic.Uint32

	pendingRequests map[uint32]chan []byte // Changed uint64 to uint32
	requestMutex    sync.RWMutex

	loadCtx    context.Context
	cancelLoad context.CancelFunc
	closed     atomic.Bool
	wg         sync.WaitGroup

	// Add process monitoring
	processExited atomic.Bool
}

func NewLoader(path, name, version string) *Loader {
	return &Loader{
		Path:            path,
		Name:            name,
		Version:         version,
		process:         nil,
		multiplexer:     nil,
		pendingRequests: make(map[uint32]chan []byte), // Changed uint64 to uint32
		// wg is initialized to its zero value, which is ready to use.
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

	// Use the provided context as parent, but create a child for internal control
	l.loadCtx, l.cancelLoad = context.WithCancel(ctx)

	recv, err := l.multiplexer.ReadMessage(l.loadCtx)
	if err != nil {
		p.Close()
		l.cancelLoad()
		return fmt.Errorf("failed to read message: %w", err)
	}

	// Start process monitoring
	l.wg.Add(1)
	go l.monitorProcess()

	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		defer func() {
			l.requestMutex.Lock()
			defer l.requestMutex.Unlock()
			for id, ch := range l.pendingRequests {
				// 채널이 이미 데이터를 받았는지 확인하고 안전하게 종료
				select {
				case <-ch:
					// 이미 데이터가 있으면 그대로 둠
				default:
					// 데이터가 없으면 채널을 닫아서 대기 중인 goroutine에 신호
					close(ch)
				}
				delete(l.pendingRequests, id)
			}
		}()

		for {
			select {
			case <-l.loadCtx.Done():
				return
			case mesg, ok := <-recv:
				if !ok {
					return
				}

				requestID := mesg.ID // mesg.ID is now uint32

				l.requestMutex.RLock()
				responseChan, exists := l.pendingRequests[requestID]
				l.requestMutex.RUnlock()

				if exists {
					select {
					case responseChan <- mesg.Data:
						// 성공적으로 전송됨
					case <-l.loadCtx.Done():
						return
					default:
						// 채널이 가득 찬 경우 - 호출자가 타임아웃될 것임
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

	// 1. 먼저 컨텍스트를 취소하여 shutdown 신호 전송
	if l.cancelLoad != nil {
		l.cancelLoad()
	}

	// 2. 메시지 처리 goroutine이 완료될 때까지 대기
	l.wg.Wait()

	// 3. 프로세스 종료 (파이프 닫기 포함)
	var closeErr error
	if l.process != nil {
		closeErr = l.process.Close()
	}

	return closeErr
}

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

	// requestPayload is already []byte, no gobEncode needed.

	requestHeader := Header{
		Name:    name,
		IsError: false,
		Payload: requestPayload, // Use requestPayload directly
	}

	headerData, err := requestHeader.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to encode header: %w", err)
	}

	l.requestMutex.Lock()
	// Double-check closed status while holding lock
	if l.closed.Load() {
		l.requestMutex.Unlock()
		return nil, fmt.Errorf("loader closed before dispatching request")
	}

	requestID := l.generateRequestID()
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
			// The payload is the error data from the plugin, treat as string.
			// No gobDecode needed for errMsg.
			errMsg := string(responseHeader.Payload)
			return nil, fmt.Errorf("plugin error for service %s: %s", name, errMsg)
		}

		// The payload is the success data. No gobDecode needed.
		return responseHeader.Payload, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-l.loadCtx.Done():
		return nil, fmt.Errorf("loader is shutting down")
	}
}

// monitorProcess monitors the plugin process and closes the loader if the process exits
func (l *Loader) monitorProcess() {
	defer l.wg.Done()

	if l.process != nil {
		// Wait for process to exit
		l.process.Wait()

		// Mark process as exited
		l.processExited.Store(true)

		// Close the loader to prevent further operations
		if !l.closed.Load() {
			l.closed.Store(true)
			if l.cancelLoad != nil {
				l.cancelLoad()
			}
		}
	}
}

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

// IsProcessAlive returns true if the plugin process is still running
func (l *Loader) IsProcessAlive() bool {
	return !l.processExited.Load() && !l.closed.Load()
}
