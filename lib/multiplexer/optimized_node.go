package multiplexer

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// bufferPool provides reusable byte slices to reduce allocations
var bufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 64*1024) // 64KB default buffer
	},
}

// headerPool provides reusable header buffers
var headerPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, MessageHeaderSize)
	},
}

// OptimizedNode is an optimized version of Node with better memory management
type OptimizedNode struct {
	reader io.Reader
	writer io.Writer

	writerLock *sync.Mutex
	readerLock *sync.RWMutex

	readBuffer map[uint32]*Message
	sequence   atomic.Uint32

	// Metrics for monitoring
	metrics NodeMetrics
}

type NodeMetrics struct {
	MessagesWritten   atomic.Uint64
	MessagesRead      atomic.Uint64
	BytesWritten      atomic.Uint64
	BytesRead         atomic.Uint64
	ErrorsEncountered atomic.Uint64
	PendingMessages   atomic.Uint64
}

func NewOptimizedNode(reader io.Reader, writer io.Writer) *OptimizedNode {
	return &OptimizedNode{
		reader:     reader,
		writer:     writer,
		writerLock: &sync.Mutex{},
		readerLock: &sync.RWMutex{},
		readBuffer: make(map[uint32]*Message),
	}
}

// getOptimalChunkSize returns the optimal chunk size based on data length
func getOptimalChunkSize(dataLength int) int {
	switch {
	case dataLength <= 4*1024: // <= 4KB
		return 1024 // 1KB chunks
	case dataLength <= 64*1024: // <= 64KB
		return 4 * 1024 // 4KB chunks
	case dataLength <= 1024*1024: // <= 1MB
		return 16 * 1024 // 16KB chunks
	default:
		return 64 * 1024 // 64KB chunks for large messages
	}
}

func (n *OptimizedNode) writeOptimized(msgType uint8, frameID uint32, data []byte) error {
	n.writerLock.Lock()
	defer n.writerLock.Unlock()

	if n.writer == nil {
		return fmt.Errorf("writer is nil")
	}

	if len(data) > 0xFFFFFFFF {
		return fmt.Errorf("data length exceeds maximum size")
	}

	// Get header buffer from pool
	header := headerPool.Get().([]byte)
	defer headerPool.Put(header)

	// Build header in-place
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

func (n *OptimizedNode) WriteMessageWithSequenceOptimized(ctx context.Context, seq uint32, data []byte) error {
	done := ctx.Done()

	// Start message
	if err := n.writeOptimized(MessageHeaderTypeStart, seq, nil); err != nil {
		return fmt.Errorf("failed to write start: %w", err)
	}

	// Check context before processing data
	select {
	case <-done:
		n.writeOptimized(MessageHeaderTypeAbort, seq, nil)
		return ctx.Err()
	default:
	}

	if len(data) > 0 {
		// Use optimal chunk size
		chunkSize := getOptimalChunkSize(len(data))

		for offset := 0; offset < len(data); {
			// Check context on each iteration
			select {
			case <-done:
				n.writeOptimized(MessageHeaderTypeAbort, seq, nil)
				return ctx.Err()
			default:
			}

			end := offset + chunkSize
			if end > len(data) {
				end = len(data)
			}

			chunk := data[offset:end]
			if err := n.writeOptimized(MessageHeaderTypeData, seq, chunk); err != nil {
				return fmt.Errorf("failed to write data chunk: %w", err)
			}

			offset = end
		}
	}

	// Final context check before end
	select {
	case <-done:
		n.writeOptimized(MessageHeaderTypeAbort, seq, nil)
		return ctx.Err()
	default:
	}

	// End message
	if err := n.writeOptimized(MessageHeaderTypeEnd, seq, nil); err != nil {
		return fmt.Errorf("failed to write end: %w", err)
	}

	n.metrics.MessagesWritten.Add(1)
	return nil
}

func (n *OptimizedNode) ReadMessageOptimized(ctx context.Context) (chan *Message, error) {
	const defaultMaxBufferLength = 4096
	ch := make(chan *Message, defaultMaxBufferLength)

	go func() {
		defer close(ch)

		const (
			maxDataLength = 1024 * 1024 * 10 // 10 MB limit
		)

		// Get buffer from pool
		buffer := bufferPool.Get().([]byte)
		defer bufferPool.Put(buffer)

		headerBuf := make([]byte, MessageHeaderSize)
		done := ctx.Done()

	loop:
		for {
			select {
			case <-done:
				ch <- &Message{Type: MessageHeaderTypeError, Data: []byte("context done")}
				return
			default:
				// Read header
				if _, err := io.ReadFull(n.reader, headerBuf); err != nil {
					if err == io.EOF {
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

					m := &Message{
						ID:   frameID,
						Type: MessageHeaderTypeStart,
						Data: make([]byte, 0, min(int(dataLength), len(buffer))),
					}

					n.readerLock.Lock()
					n.readBuffer[frameID] = m
					n.readerLock.Unlock()
					n.metrics.PendingMessages.Add(1)

				case MessageHeaderTypeData:
					if dataLength > 0 {
						// Ensure buffer is large enough
						if int(dataLength) > len(buffer) {
							// For very large chunks, allocate temporary buffer
							tempBuf := make([]byte, dataLength)
							if _, err := io.ReadFull(n.reader, tempBuf); err != nil {
								if err == io.EOF {
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

							// Memory safety check
							if len(m.Data)+int(dataLength) > maxDataLength {
								ch <- &Message{Type: MessageHeaderTypeError,
									Data: []byte(fmt.Sprintf("message size would exceed maximum: %d", maxDataLength))}
								n.readerLock.Lock()
								delete(n.readBuffer, frameID)
								n.readerLock.Unlock()
								n.metrics.PendingMessages.Add(^uint64(0)) // Subtract 1
								continue loop
							}

							m.Data = append(m.Data, tempBuf...)
						} else {
							// Use pooled buffer for normal sized chunks
							chunk := buffer[:dataLength]
							if _, err := io.ReadFull(n.reader, chunk); err != nil {
								if err == io.EOF {
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
								n.metrics.PendingMessages.Add(^uint64(0)) // Subtract 1
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
					n.metrics.PendingMessages.Add(^uint64(0)) // Subtract 1

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
					n.metrics.PendingMessages.Add(^uint64(0)) // Subtract 1

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

func (n *OptimizedNode) GetMetrics() NodeMetrics {
	return n.metrics
}

func (n *OptimizedNode) Close() error {
	n.readerLock.Lock()
	defer n.readerLock.Unlock()

	// Clear all pending messages
	count := len(n.readBuffer)
	for frameID := range n.readBuffer {
		delete(n.readBuffer, frameID)
	}

	// Update metrics
	if count > 0 {
		n.metrics.PendingMessages.Store(0)
	}

	return nil
}
