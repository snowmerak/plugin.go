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
	wg         sync.WaitGroup
}

func NewLoader(path, name, version string) *Loader {
	return &Loader{
		Path:            path,
		Name:            name,
		Version:         version,
		process:         nil,
		multiplexer:     nil,
		pendingRequests: make(map[uint64]chan []byte),
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

				requestID := mesg.ID

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
		IsError: false,
		Payload: requestPayloadBytes,
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

	requestID := l.requestID.Add(1)
	responseChan := make(chan []byte, 1)
	l.pendingRequests[requestID] = responseChan
	l.requestMutex.Unlock()

	defer func() {
		l.requestMutex.Lock()
		delete(l.pendingRequests, requestID)
		l.requestMutex.Unlock()
		// Don't close the channel here - let the read goroutine handle it
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
			var errMsg string
			if err := gobDecode(responseHeader.Payload, &errMsg); err != nil {
				return nil, fmt.Errorf("failed to decode error message from plugin (service: %s): %w", name, err)
			}
			return nil, fmt.Errorf("plugin error for service %s: %s", name, errMsg)
		}

		var actualResponse Res
		if err := gobDecode(responseHeader.Payload, &actualResponse); err != nil {
			return nil, fmt.Errorf("failed to decode response for service %s: %w", name, err)
		}
		return &actualResponse, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-l.loadCtx.Done():
		return nil, fmt.Errorf("loader is shutting down")
	}
}
