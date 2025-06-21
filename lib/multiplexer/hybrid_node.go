package multiplexer

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// HybridNode combines the best of both original and optimized approaches
// Uses fast path for small messages and optimized path for large messages
type HybridNode struct {
	reader io.Reader
	writer io.Writer

	writerLock *sync.Mutex
	readerLock *sync.RWMutex

	readBuffer map[uint32]*Message
	sequence   atomic.Uint32

	// Metrics for monitoring
	metrics NodeMetrics

	// Thresholds for optimization decisions
	fastPathThreshold    int // Messages smaller than this use fast path
	optimizedPathEnabled bool
}

// Threshold constants based on benchmark results
const (
	DefaultFastPathThreshold = 4 * 1024  // 4KB threshold
	SmallBufferSize          = 8 * 1024  // 8KB for small messages
	LargeBufferSize          = 64 * 1024 // 64KB for large messages
)

// Buffer pools with different sizes
var (
	smallBufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, SmallBufferSize)
		},
	}

	largeBufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, LargeBufferSize)
		},
	}

	hybridHeaderPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, MessageHeaderSize)
		},
	}
)

func NewHybridNode(reader io.Reader, writer io.Writer) *HybridNode {
	return &HybridNode{
		reader:               reader,
		writer:               writer,
		writerLock:           &sync.Mutex{},
		readerLock:           &sync.RWMutex{},
		readBuffer:           make(map[uint32]*Message),
		fastPathThreshold:    DefaultFastPathThreshold,
		optimizedPathEnabled: true,
	}
}

// NewHybridNodeWithThreshold creates a new HybridNode with custom threshold
func NewHybridNodeWithThreshold(reader io.Reader, writer io.Writer, threshold int) *HybridNode {
	return &HybridNode{
		reader:               reader,
		writer:               writer,
		writerLock:           &sync.Mutex{},
		readerLock:           &sync.RWMutex{},
		readBuffer:           make(map[uint32]*Message),
		fastPathThreshold:    threshold,
		optimizedPathEnabled: true,
	}
}

// SetFastPathThreshold allows tuning the threshold for performance
func (n *HybridNode) SetFastPathThreshold(threshold int) {
	n.fastPathThreshold = threshold
}

// EnableOptimizedPath allows disabling optimizations for debugging
func (n *HybridNode) EnableOptimizedPath(enabled bool) {
	n.optimizedPathEnabled = enabled
}

func (n *HybridNode) writeHybrid(msgType uint8, frameID uint32, data []byte, useFastPath bool) error {
	n.writerLock.Lock()
	defer n.writerLock.Unlock()

	if n.writer == nil {
		return fmt.Errorf("writer is nil")
	}

	if len(data) > 0xFFFFFFFF {
		return fmt.Errorf("data length exceeds maximum size")
	}

	var header []byte
	var shouldReturnHeader bool

	if useFastPath {
		// Fast path: direct allocation for small messages
		header = make([]byte, MessageHeaderSize)
	} else {
		// Optimized path: use pool for large messages
		header = hybridHeaderPool.Get().([]byte)
		shouldReturnHeader = true
	}

	if shouldReturnHeader {
		defer hybridHeaderPool.Put(header)
	}

	// Build header
	header[0] = msgType
	header[1] = byte(frameID >> 24)
	header[2] = byte(frameID >> 16)
	header[3] = byte(frameID >> 8)
	header[4] = byte(frameID)
	header[5] = byte(len(data) >> 24)
	header[6] = byte(len(data) >> 16)
	header[7] = byte(len(data) >> 8)
	header[8] = byte(len(data))

	if _, err := n.writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	switch msgType {
	case MessageHeaderTypeData:
		if len(data) > 0 {
			if _, err := n.writer.Write(data); err != nil {
				return fmt.Errorf("failed to write data: %w", err)
			}
			n.metrics.BytesWritten.Add(uint64(len(data)))
		}
	}

	return nil
}

func (n *HybridNode) WriteMessageWithSequenceHybrid(ctx context.Context, seq uint32, data []byte) error {
	dataLength := len(data)
	useFastPath := !n.optimizedPathEnabled || dataLength <= n.fastPathThreshold

	done := ctx.Done()

	// Check context before starting
	select {
	case <-done:
		return ctx.Err()
	default:
	}

	// Start message
	if err := n.writeHybrid(MessageHeaderTypeStart, seq, nil, useFastPath); err != nil {
		return fmt.Errorf("failed to write start: %w", err)
	}

	// Check context before processing data
	select {
	case <-done:
		n.writeHybrid(MessageHeaderTypeAbort, seq, nil, useFastPath)
		return ctx.Err()
	default:
	}

	if dataLength > 0 {
		var chunkSize int

		if useFastPath {
			// Fast path: use original chunk size for small messages
			chunkSize = MessageChunkSize // 1KB
		} else {
			// Optimized path: use adaptive chunk size for large messages
			chunkSize = getOptimalChunkSize(dataLength)
		}

		for offset := 0; offset < dataLength; {
			// Check context on each iteration
			select {
			case <-done:
				n.writeHybrid(MessageHeaderTypeAbort, seq, nil, useFastPath)
				return ctx.Err()
			default:
			}

			end := offset + chunkSize
			if end > dataLength {
				end = dataLength
			}

			chunk := data[offset:end]
			if err := n.writeHybrid(MessageHeaderTypeData, seq, chunk, useFastPath); err != nil {
				return fmt.Errorf("failed to write data chunk: %w", err)
			}

			offset = end
		}
	}

	// Final context check before end
	select {
	case <-done:
		n.writeHybrid(MessageHeaderTypeAbort, seq, nil, useFastPath)
		return ctx.Err()
	default:
	}

	// End message
	if err := n.writeHybrid(MessageHeaderTypeEnd, seq, nil, useFastPath); err != nil {
		return fmt.Errorf("failed to write end: %w", err)
	}

	n.metrics.MessagesWritten.Add(1)
	return nil
}

func (n *HybridNode) ReadMessageHybrid(ctx context.Context) (chan *Message, error) {
	const defaultMaxBufferLength = 4096
	ch := make(chan *Message, defaultMaxBufferLength)

	go func() {
		defer close(ch)

		const maxDataLength = 1024 * 1024 * 10 // 10 MB limit

		// Use small buffer initially, will switch to large buffer for big messages
		buffer := smallBufferPool.Get().([]byte)
		defer smallBufferPool.Put(buffer)

		usingLargeBuffer := false
		largeBuffer := ([]byte)(nil)

		headerBuf := make([]byte, MessageHeaderSize)
		done := ctx.Done()

	loop:
		for {
			select {
			case <-done:
				ch <- &Message{Type: MessageHeaderTypeError, Data: []byte("context done")}
				if usingLargeBuffer && largeBuffer != nil {
					largeBufferPool.Put(largeBuffer)
				}
				return
			default:
				// Read header
				if _, err := io.ReadFull(n.reader, headerBuf); err != nil {
					if err == io.EOF {
						if usingLargeBuffer && largeBuffer != nil {
							largeBufferPool.Put(largeBuffer)
						}
						return
					}
					ch <- &Message{Type: MessageHeaderTypeError, Data: []byte(err.Error())}
					continue loop
				}

				// Parse header
				msgType := headerBuf[0]
				frameID := uint32(headerBuf[1])<<24 | uint32(headerBuf[2])<<16 |
					uint32(headerBuf[3])<<8 | uint32(headerBuf[4])
				dataLength := uint32(headerBuf[5])<<24 | uint32(headerBuf[6])<<16 |
					uint32(headerBuf[7])<<8 | uint32(headerBuf[8])

				// Safety check
				if dataLength > maxDataLength {
					ch <- &Message{Type: MessageHeaderTypeError,
						Data: []byte(fmt.Sprintf("data length %d exceeds maximum %d", dataLength, maxDataLength))}
					continue loop
				}

				// Switch to large buffer if needed
				if !usingLargeBuffer && dataLength > uint32(len(buffer)) {
					largeBuffer = largeBufferPool.Get().([]byte)
					usingLargeBuffer = true
				}

				switch msgType {
				case MessageHeaderTypeStart:
					n.readerLock.RLock()
					_, exists := n.readBuffer[frameID]
					n.readerLock.RUnlock()

					if exists {
						ch <- &Message{Type: MessageHeaderTypeError,
							Data: []byte(fmt.Sprintf("frame ID %d already exists", frameID))}
						continue loop
					}

					// Use appropriate initial capacity
					var initialCap int
					if dataLength <= uint32(n.fastPathThreshold) {
						initialCap = min(int(dataLength), SmallBufferSize)
					} else {
						initialCap = min(int(dataLength), LargeBufferSize)
					}

					m := &Message{
						ID:   frameID,
						Type: MessageHeaderTypeStart,
						Data: make([]byte, 0, initialCap),
					}

					n.readerLock.Lock()
					n.readBuffer[frameID] = m
					n.readerLock.Unlock()
					n.metrics.PendingMessages.Add(1)

				case MessageHeaderTypeData:
					if dataLength > 0 {
						var readBuffer []byte

						// Choose appropriate buffer
						if usingLargeBuffer && largeBuffer != nil {
							readBuffer = largeBuffer
						} else {
							readBuffer = buffer
						}

						// Handle cases where data is larger than our buffer
						if int(dataLength) > len(readBuffer) {
							tempBuf := make([]byte, dataLength)
							if _, err := io.ReadFull(n.reader, tempBuf); err != nil {
								if err == io.EOF {
									if usingLargeBuffer && largeBuffer != nil {
										largeBufferPool.Put(largeBuffer)
									}
									return
								}
								ch <- &Message{Type: MessageHeaderTypeError, Data: []byte(err.Error())}
								continue loop
							}

							n.readerLock.RLock()
							m, ok := n.readBuffer[frameID]
							n.readerLock.RUnlock()

							if !ok {
								ch <- &Message{Type: MessageHeaderTypeError,
									Data: []byte(fmt.Sprintf("unknown frame ID: %d", frameID))}
								continue loop
							}

							if len(m.Data)+int(dataLength) > maxDataLength {
								ch <- &Message{Type: MessageHeaderTypeError,
									Data: []byte(fmt.Sprintf("message size would exceed maximum: %d", maxDataLength))}
								n.readerLock.Lock()
								delete(n.readBuffer, frameID)
								n.readerLock.Unlock()
								n.metrics.PendingMessages.Add(^uint64(0))
								continue loop
							}

							m.Data = append(m.Data, tempBuf...)
						} else {
							// Use buffered read
							chunk := readBuffer[:dataLength]
							if _, err := io.ReadFull(n.reader, chunk); err != nil {
								if err == io.EOF {
									if usingLargeBuffer && largeBuffer != nil {
										largeBufferPool.Put(largeBuffer)
									}
									return
								}
								ch <- &Message{Type: MessageHeaderTypeError, Data: []byte(err.Error())}
								continue loop
							}

							n.readerLock.RLock()
							m, ok := n.readBuffer[frameID]
							n.readerLock.RUnlock()

							if !ok {
								ch <- &Message{Type: MessageHeaderTypeError,
									Data: []byte(fmt.Sprintf("unknown frame ID: %d", frameID))}
								continue loop
							}

							if len(m.Data)+int(dataLength) > maxDataLength {
								ch <- &Message{Type: MessageHeaderTypeError,
									Data: []byte(fmt.Sprintf("message size would exceed maximum: %d", maxDataLength))}
								n.readerLock.Lock()
								delete(n.readBuffer, frameID)
								n.readerLock.Unlock()
								n.metrics.PendingMessages.Add(^uint64(0))
								continue loop
							}

							m.Data = append(m.Data, chunk...)
						}
						n.metrics.BytesRead.Add(uint64(dataLength))
					}

				case MessageHeaderTypeEnd:
					n.readerLock.Lock()
					m, ok := n.readBuffer[frameID]
					if ok {
						delete(n.readBuffer, frameID)
					}
					n.readerLock.Unlock()

					if !ok {
						ch <- &Message{Type: MessageHeaderTypeError,
							Data: []byte(fmt.Sprintf("unknown frame ID: %d", frameID))}
						continue loop
					}

					m.Type = MessageHeaderTypeComplete
					ch <- m
					n.metrics.MessagesRead.Add(1)
					n.metrics.PendingMessages.Add(^uint64(0))

				case MessageHeaderTypeAbort:
					n.readerLock.Lock()
					m, ok := n.readBuffer[frameID]
					if ok {
						delete(n.readBuffer, frameID)
					}
					n.readerLock.Unlock()

					if !ok {
						ch <- &Message{Type: MessageHeaderTypeError,
							Data: []byte(fmt.Sprintf("unknown frame ID: %d", frameID))}
						continue loop
					}

					m.Type = MessageHeaderTypeAbort
					ch <- m
					n.metrics.PendingMessages.Add(^uint64(0))

				default:
					ch <- &Message{Type: MessageHeaderTypeError,
						Data: []byte(fmt.Sprintf("unknown message type: %d", msgType))}
					n.metrics.ErrorsEncountered.Add(1)
					continue loop
				}
			}
		}
	}()

	return ch, nil
}

func (n *HybridNode) WriteMessage(ctx context.Context, data []byte) error {
	if err := n.WriteMessageWithSequenceHybrid(ctx, n.sequence.Add(1), data); err != nil {
		return fmt.Errorf("failed to write request message: %w", err)
	}
	return nil
}

func (n *HybridNode) GetMetrics() *NodeMetrics {
	return &n.metrics
}

func (n *HybridNode) GetPendingMessageCount() int {
	n.readerLock.RLock()
	defer n.readerLock.RUnlock()
	return len(n.readBuffer)
}

func (n *HybridNode) Close() error {
	n.readerLock.Lock()
	defer n.readerLock.Unlock()

	count := len(n.readBuffer)
	for frameID := range n.readBuffer {
		delete(n.readBuffer, frameID)
	}

	if count > 0 {
		n.metrics.PendingMessages.Store(0)
	}

	return nil
}
