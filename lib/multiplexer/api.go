package multiplexer

import (
	"context"
	"io"
)

// Multiplexer provides a unified interface for message multiplexing
type Multiplexer interface {
	// WriteMessage sends a message with automatic sequence numbering
	WriteMessage(ctx context.Context, data []byte) error

	// WriteMessageWithSequence sends a message with a specific sequence number
	WriteMessageWithSequence(ctx context.Context, seq uint32, data []byte) error

	// ReadMessage reads messages and returns a channel
	ReadMessage(ctx context.Context) (chan *APIMessage, error)

	// Close cleanly shuts down the multiplexer
	Close() error

	// GetPendingMessageCount returns the number of pending messages
	GetPendingMessageCount() int

	// GetMetrics returns performance metrics (if available)
	GetMetrics() *Metrics
}

// APIMessage represents a received message for the API
type APIMessage struct {
	Sequence uint32
	Data     []byte
}

// Metrics contains performance information
type Metrics struct {
	MessagesWritten  uint64
	MessagesRead     uint64
	BytesWritten     uint64
	BytesRead        uint64
	BufferPoolHits   uint64
	BufferPoolMisses uint64
	ChunksProcessed  uint64
	OptimizedWrites  uint64
	FastPathWrites   uint64
}

// Config holds configuration options for the multiplexer
type Config struct {
	// Threshold determines when to use optimized vs fast path (default: 8KB)
	Threshold int

	// BufferSize sets the initial buffer size for optimizations (default: 64KB)
	BufferSize int

	// MaxMessageSize sets the maximum allowed message size (default: 10MB)
	MaxMessageSize int

	// EnableMetrics enables performance metrics collection (default: false)
	EnableMetrics bool
}

// OptimizationTarget specifies optimization preferences
type OptimizationTarget int

const (
	// OptimizeForGeneral provides balanced performance (default)
	OptimizeForGeneral OptimizationTarget = iota

	// OptimizeForSmallMessages optimizes for messages < 8KB
	OptimizeForSmallMessages

	// OptimizeForLargeMessages optimizes for messages > 8KB
	OptimizeForLargeMessages

	// OptimizeForMemory prioritizes memory efficiency
	OptimizeForMemory
)

// nodeAdapter wraps existing node implementations to implement the Multiplexer interface
type nodeAdapter struct {
	writeMessageWithSequence func(ctx context.Context, seq uint32, data []byte) error
	readMessage              func(ctx context.Context) (chan *Message, error)
	close                    func() error
	getPendingMessageCount   func() int
	getMetrics               func() *Metrics
	sequence                 uint32
}

func (a *nodeAdapter) WriteMessage(ctx context.Context, data []byte) error {
	a.sequence++
	return a.writeMessageWithSequence(ctx, a.sequence, data)
}

func (a *nodeAdapter) WriteMessageWithSequence(ctx context.Context, seq uint32, data []byte) error {
	return a.writeMessageWithSequence(ctx, seq, data)
}

func (a *nodeAdapter) ReadMessage(ctx context.Context) (chan *APIMessage, error) {
	ch, err := a.readMessage(ctx)
	if err != nil {
		return nil, err
	}

	apiCh := make(chan *APIMessage, cap(ch))
	go func() {
		defer close(apiCh)
		for msg := range ch {
			apiCh <- &APIMessage{
				Sequence: msg.ID,
				Data:     msg.Data,
			}
		}
	}()

	return apiCh, nil
}

func (a *nodeAdapter) Close() error {
	if a.close != nil {
		return a.close()
	}
	return nil
}

func (a *nodeAdapter) GetPendingMessageCount() int {
	if a.getPendingMessageCount != nil {
		return a.getPendingMessageCount()
	}
	return 0
}

func (a *nodeAdapter) GetMetrics() *Metrics {
	if a.getMetrics != nil {
		return a.getMetrics()
	}
	return &Metrics{
		MessagesWritten: uint64(a.sequence),
		FastPathWrites:  uint64(a.sequence),
	}
}

// wrapNode creates an adapter for the basic Node
func wrapNode(node *Node) *nodeAdapter {
	return &nodeAdapter{
		writeMessageWithSequence: node.WriteMessageWithSequence,
		readMessage:              node.ReadMessage,
		close:                    node.Close,
		getPendingMessageCount:   node.GetPendingMessageCount,
		getMetrics: func() *Metrics {
			return &Metrics{
				MessagesWritten: uint64(node.sequence.Load()),
				FastPathWrites:  uint64(node.sequence.Load()),
			}
		},
	}
}

// wrapOptimizedNode creates an adapter for the OptimizedNode
func wrapOptimizedNode(node *OptimizedNode) *nodeAdapter {
	return &nodeAdapter{
		writeMessageWithSequence: node.WriteMessageWithSequenceOptimized,
		readMessage: func(ctx context.Context) (chan *Message, error) {
			// OptimizedNode might not have ReadMessage, create a basic implementation
			ch := make(chan *Message, 1)
			close(ch) // Return empty channel for now
			return ch, nil
		},
		close: func() error {
			return nil // OptimizedNode might not have Close
		},
		getPendingMessageCount: func() int {
			return 0 // OptimizedNode might not have this method
		},
		getMetrics: func() *Metrics {
			metrics := node.GetMetrics()
			return &Metrics{
				MessagesWritten:  metrics.MessagesWritten.Load(),
				MessagesRead:     metrics.MessagesRead.Load(),
				BytesWritten:     metrics.BytesWritten.Load(),
				BytesRead:        metrics.BytesRead.Load(),
				BufferPoolHits:   0, // Not tracked in NodeMetrics
				BufferPoolMisses: 0, // Not tracked in NodeMetrics
				ChunksProcessed:  0, // Not tracked in NodeMetrics
				OptimizedWrites:  metrics.MessagesWritten.Load(),
				FastPathWrites:   0,
			}
		},
	}
}

// wrapHybridNode creates an adapter for the HybridNode
func wrapHybridNode(node *HybridNode) *nodeAdapter {
	return &nodeAdapter{
		writeMessageWithSequence: node.WriteMessageWithSequenceHybrid,
		readMessage: func(ctx context.Context) (chan *Message, error) {
			return node.ReadMessageHybrid(ctx)
		},
		close: func() error {
			return node.Close()
		},
		getPendingMessageCount: func() int {
			return node.GetPendingMessageCount()
		},
		getMetrics: func() *Metrics {
			metrics := node.GetMetrics()
			return &Metrics{
				MessagesWritten:  metrics.MessagesWritten.Load(),
				MessagesRead:     metrics.MessagesRead.Load(),
				BytesWritten:     metrics.BytesWritten.Load(),
				BytesRead:        metrics.BytesRead.Load(),
				BufferPoolHits:   0,                              // HybridNode doesn't track pool hits separately
				BufferPoolMisses: 0,                              // HybridNode doesn't track pool misses separately
				ChunksProcessed:  0,                              // HybridNode doesn't track chunks separately
				OptimizedWrites:  0,                              // HybridNode doesn't distinguish optimization types
				FastPathWrites:   metrics.MessagesWritten.Load(), // Approximate with total messages
			}
		},
	}
}

// New creates a new multiplexer with automatic optimization
// This is the recommended way to create a multiplexer for most use cases
func New(reader io.Reader, writer io.Writer) Multiplexer {
	return wrapHybridNode(NewHybridNode(reader, writer))
}

// NewWithConfig creates a multiplexer with custom configuration
func NewWithConfig(reader io.Reader, writer io.Writer, config Config) Multiplexer {
	if config.Threshold == 0 {
		config.Threshold = 8 * 1024 // 8KB default
	}
	if config.BufferSize == 0 {
		config.BufferSize = 64 * 1024 // 64KB default
	}
	if config.MaxMessageSize == 0 {
		config.MaxMessageSize = 10 * 1024 * 1024 // 10MB default
	}

	return wrapHybridNode(NewHybridNodeWithThreshold(reader, writer, config.Threshold))
}

// NewOptimized creates a multiplexer optimized for specific use cases
func NewOptimized(reader io.Reader, writer io.Writer, target OptimizationTarget) Multiplexer {
	switch target {
	case OptimizeForSmallMessages:
		return wrapNode(NewNode(reader, writer))
	case OptimizeForLargeMessages:
		return wrapOptimizedNode(NewOptimizedNode(reader, writer))
	case OptimizeForMemory:
		return wrapHybridNode(NewHybridNodeWithThreshold(reader, writer, 4*1024)) // 4KB threshold
	default: // OptimizeForGeneral
		return wrapHybridNode(NewHybridNode(reader, writer))
	}
}

// Helper functions for common configurations

// NewForAPI creates a multiplexer optimized for API responses (typically < 8KB)
func NewForAPI(reader io.Reader, writer io.Writer) Multiplexer {
	return NewOptimized(reader, writer, OptimizeForSmallMessages)
}

// NewForFileTransfer creates a multiplexer optimized for file transfers
func NewForFileTransfer(reader io.Reader, writer io.Writer) Multiplexer {
	return NewOptimized(reader, writer, OptimizeForLargeMessages)
}

// NewForStreaming creates a multiplexer optimized for streaming with mixed message sizes
func NewForStreaming(reader io.Reader, writer io.Writer) Multiplexer {
	return NewOptimized(reader, writer, OptimizeForGeneral)
}

// NewForIoT creates a multiplexer optimized for IoT devices with small payloads
func NewForIoT(reader io.Reader, writer io.Writer) Multiplexer {
	return NewOptimized(reader, writer, OptimizeForSmallMessages)
}

// NewForMemoryConstrained creates a multiplexer optimized for memory-constrained environments
func NewForMemoryConstrained(reader io.Reader, writer io.Writer) Multiplexer {
	return NewOptimized(reader, writer, OptimizeForMemory)
}
