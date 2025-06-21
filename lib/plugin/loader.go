package plugin

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snowmerak/plugin.go/lib/multiplexer"
	"github.com/snowmerak/plugin.go/lib/process"
)

type Loader struct {
	Path    string
	Name    string
	Version string

	process     *process.Process
	multiplexer multiplexer.Multiplexer

	requestID atomic.Uint32 // Changed atomic.Uint64 to atomic.Uint32

	pendingRequests map[uint32]chan []byte // Changed uint64 to uint32
	requestMutex    sync.RWMutex

	loadCtx    context.Context
	cancelLoad context.CancelFunc
	closed     atomic.Bool
	wg         sync.WaitGroup

	// Add process monitoring
	processExited atomic.Bool

	// Add ready signal channel
	readySignal chan struct{}
}

func NewLoader(path, name, version string) *Loader {
	return &Loader{
		Path:            path,
		Name:            name,
		Version:         version,
		process:         nil,
		multiplexer:     nil,
		pendingRequests: make(map[uint32]chan []byte), // Changed uint64 to uint32
		readySignal:     make(chan struct{}, 1),
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

	mux := multiplexer.New(p.Stdout(), p.Stdin())
	l.multiplexer = mux

	// Use the provided context as parent, but create a child for internal control
	l.loadCtx, l.cancelLoad = context.WithCancel(ctx)

	// Start process monitoring
	l.wg.Add(1)
	go l.monitorProcess()

	// Start message handling goroutine FIRST
	l.wg.Add(1)
	go l.handleMessages()

	// Wait for ready signal from plugin
	if err := l.waitForReadySignal(); err != nil {
		p.Close()
		l.cancelLoad()
		return err
	}

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

	// 2. 프로세스를 먼저 종료하여 파이프를 닫음 (고루틴이 블로킹에서 벗어나도록)
	var closeErr error
	if l.process != nil {
		closeErr = l.process.Close()
	}

	// 3. 타임아웃과 함께 고루틴 완료 대기
	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 정상적으로 종료됨
	case <-time.After(2 * time.Second):
		// 타임아웃 - 강제 종료
		fmt.Fprintf(os.Stderr, "Warning: Loader close timed out, some goroutines may not have finished\n")
	}

	return closeErr
}

// Call sends a request to the loaded plugin and waits for a response.
// The request and response are raw byte slices.
// It returns the response payload as []byte. If the plugin indicates an error,
// this error is returned, with the error message being the string representation
// of the plugin's error payload.
func Call(ctx context.Context, l *Loader, name string, requestPayload []byte) ([]byte, error) {
	fmt.Fprintf(os.Stderr, "DEBUG: Call started for service %s with payload length: %d\n", name, len(requestPayload))

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

	fmt.Fprintf(os.Stderr, "DEBUG: Header marshaled successfully, length: %d\n", len(headerData))

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

	fmt.Fprintf(os.Stderr, "DEBUG: Request ID generated: %d\n", requestID)

	defer func() {
		l.requestMutex.Lock()
		delete(l.pendingRequests, requestID)
		l.requestMutex.Unlock()
	}()

	fmt.Fprintf(os.Stderr, "DEBUG: About to write message with sequence %d\n", requestID)
	if err := l.multiplexer.WriteMessageWithSequence(ctx, requestID, headerData); err != nil {
		return nil, fmt.Errorf("failed to write request message: %w", err)
	}

	fmt.Fprintf(os.Stderr, "DEBUG: Message written successfully, waiting for response...\n")

	select {
	case responseData, ok := <-responseChan:
		if !ok {
			return nil, fmt.Errorf("response channel closed, loader shutting down")
		}

		fmt.Fprintf(os.Stderr, "DEBUG: Received response data, length: %d\n", len(responseData))

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
		fmt.Fprintf(os.Stderr, "DEBUG: Call completed successfully for %s\n", name)
		return responseHeader.Payload, nil
	case <-ctx.Done():
		fmt.Fprintf(os.Stderr, "DEBUG: Call context cancelled for %s\n", name)
		return nil, ctx.Err()
	case <-l.loadCtx.Done():
		fmt.Fprintf(os.Stderr, "DEBUG: Loader shutting down during call for %s\n", name)
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

	fmt.Fprintf(os.Stderr, "DEBUG: generateRequestID starting\n")
	for attempt := 0; attempt < maxAttempts; attempt++ {
		id := l.requestID.Add(1)
		fmt.Fprintf(os.Stderr, "DEBUG: generateRequestID attempt %d, generated ID: %d\n", attempt, id)
		if id == 0 {
			// Skip 0 as it might be reserved
			fmt.Fprintf(os.Stderr, "DEBUG: generateRequestID skipping ID 0\n")
			continue
		}

		l.requestMutex.RLock()
		_, exists := l.pendingRequests[id]
		l.requestMutex.RUnlock()

		if !exists {
			fmt.Fprintf(os.Stderr, "DEBUG: generateRequestID found unique ID: %d\n", id)
			return id
		}
		fmt.Fprintf(os.Stderr, "DEBUG: generateRequestID ID %d already exists, retrying\n", id)
	}

	// Fallback: if we can't find a unique ID after maxAttempts, use the current value
	// This should be extremely rare unless there are millions of concurrent requests
	fallbackID := l.requestID.Load()
	fmt.Fprintf(os.Stderr, "DEBUG: generateRequestID using fallback ID: %d\n", fallbackID)
	return fallbackID
}

// IsProcessAlive returns true if the plugin process is still running
func (l *Loader) IsProcessAlive() bool {
	return !l.processExited.Load() && !l.closed.Load()
}

// waitForReadySignal waits for the ready signal from the plugin
func (l *Loader) waitForReadySignal() error {
	// Wait for handleMessages to process the ready signal
	// This is now handled by handleMessages goroutine
	select {
	case <-l.readySignal:
		return nil
	case <-l.loadCtx.Done():
		return fmt.Errorf("context cancelled while waiting for ready signal")
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for ready signal from plugin")
	}
}

// handleMessages handles incoming messages from the plugin for request/response communication
func (l *Loader) handleMessages() {
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

	// Create a new message reader for handling request/response communication
	recv, err := l.multiplexer.ReadMessage(l.loadCtx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "DEBUG: Failed to create ReadMessage channel in handleMessages: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "DEBUG: handleMessages started, listening for responses...\n")

	readyReceived := false

	for {
		select {
		case <-l.loadCtx.Done():
			fmt.Fprintf(os.Stderr, "DEBUG: handleMessages context cancelled\n")
			return
		case mesg, ok := <-recv:
			if !ok {
				fmt.Fprintf(os.Stderr, "DEBUG: handleMessages recv channel closed\n")
				return
			}

			fmt.Fprintf(os.Stderr, "DEBUG: handleMessages received message with sequence %d, data length: %d\n", mesg.Sequence, len(mesg.Data))

			// Handle ready signal first
			if !readyReceived {
				var readyHeader Header
				if err := readyHeader.UnmarshalBinary(mesg.Data); err != nil {
					fmt.Fprintf(os.Stderr, "DEBUG: Failed to parse first message as ready signal: %v\n", err)
					continue
				}

				if readyHeader.Name == "ready" {
					fmt.Fprintf(os.Stderr, "DEBUG: Ready signal received successfully\n")
					readyReceived = true
					// Signal that ready is received
					select {
					case l.readySignal <- struct{}{}:
					default:
						// Channel is full, ready signal already sent
					}
					continue
				} else {
					fmt.Fprintf(os.Stderr, "DEBUG: First message is not ready signal: %s\n", readyHeader.Name)
				}
			}

			// Handle regular request/response messages
			requestID := mesg.Sequence // mesg.Sequence is now uint32

			l.requestMutex.RLock()
			responseChan, exists := l.pendingRequests[requestID]
			l.requestMutex.RUnlock()

			if exists {
				fmt.Fprintf(os.Stderr, "DEBUG: Found pending request for ID %d, forwarding response\n", requestID)
				select {
				case responseChan <- mesg.Data:
					fmt.Fprintf(os.Stderr, "DEBUG: Successfully forwarded response for ID %d\n", requestID)
				case <-l.loadCtx.Done():
					return
				default:
					fmt.Fprintf(os.Stderr, "DEBUG: Response channel full for ID %d\n", requestID)
				}
			} else {
				fmt.Fprintf(os.Stderr, "DEBUG: No pending request found for ID %d\n", requestID)
			}
		}
	}
}
