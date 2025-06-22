package plugin

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"unsafe"

	"github.com/snowmerak/plugin.go/lib/hsq/mmap"
	"github.com/snowmerak/plugin.go/lib/hsq/offheap/bufring"
	"github.com/snowmerak/plugin.go/lib/hsq/shm"
)

// HSQAdapter wraps HSQ BufferRing to implement io.Reader and io.Writer
type HSQAdapter struct {
	sendRing    *bufring.BufferRing
	receiveRing *bufring.BufferRing
	sharedMem   *shm.SharedMemory
	memory      []byte
	readBuffer  []byte
	readMutex   sync.Mutex
	writeMutex  sync.Mutex
	closed      bool
	closeMutex  sync.RWMutex
}

// NewHSQAdapter creates a new HSQ-based adapter for plugin communication
func NewHSQAdapter(config *HSQConfig) (*HSQAdapter, error) {
	// Calculate required memory size for two rings (send and receive)
	singleRingSize := bufring.SizeBufferRing(config.RingSize, config.MaxMessageSize)
	totalSize := singleRingSize * 2 // Two rings: send and receive

	// Open or create shared memory
	flags := os.O_RDWR
	if config.IsHost {
		flags |= os.O_CREATE
	}

	sm, err := shm.OpenSharedMemory(config.SharedMemoryName, totalSize, flags, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open shared memory: %w", err)
	}

	// Map shared memory
	memory, err := mmap.Map(sm.FD(), 0, totalSize, mmap.PROT_READ|mmap.PROT_WRITE, mmap.MAP_SHARED)
	if err != nil {
		sm.Close()
		return nil, fmt.Errorf("failed to map memory: %w", err)
	}

	// Initialize ring buffers
	basePtr := uintptr(unsafe.Pointer(&memory[0]))

	var sendOffset, receiveOffset uintptr
	if config.IsHost {
		// Host: send to first ring, receive from second ring
		sendOffset = 0
		receiveOffset = uintptr(singleRingSize)
	} else {
		// Plugin: send to second ring, receive from first ring
		sendOffset = uintptr(singleRingSize)
		receiveOffset = 0
	}

	var sendRing, receiveRing *bufring.BufferRing

	sendRing = bufring.NewBufferRing(basePtr+sendOffset, config.RingSize, config.MaxMessageSize, false)
	receiveRing = bufring.NewBufferRing(basePtr+receiveOffset, config.RingSize, config.MaxMessageSize, false)

	return &HSQAdapter{
		sendRing:    sendRing,
		receiveRing: receiveRing,
		sharedMem:   sm,
		memory:      memory,
		readBuffer:  make([]byte, 0),
	}, nil
}

// Write implements io.Writer interface
func (h *HSQAdapter) Write(data []byte) (n int, err error) {
	h.closeMutex.RLock()
	defer h.closeMutex.RUnlock()

	if h.closed {
		return 0, fmt.Errorf("HSQ adapter is closed")
	}

	h.writeMutex.Lock()
	defer h.writeMutex.Unlock()

	h.sendRing.Send(func(b []byte) []byte {
		if len(data) > len(b) {
			// Split large messages if needed
			copy(b, data[:len(b)])
			return b
		}
		copy(b, data)
		return b[:len(data)]
	})

	return len(data), nil
}

// Read implements io.Reader interface
func (h *HSQAdapter) Read(p []byte) (n int, err error) {
	h.closeMutex.RLock()
	defer h.closeMutex.RUnlock()

	if h.closed {
		return 0, io.EOF
	}

	h.readMutex.Lock()
	defer h.readMutex.Unlock()

	// If we have buffered data, return it first
	if len(h.readBuffer) > 0 {
		n = copy(p, h.readBuffer)
		h.readBuffer = h.readBuffer[n:]
		return n, nil
	}

	// Try to receive data with timeout mechanism
	dataReceived := false
	done := make(chan struct{})

	go func() {
		h.receiveRing.Receive(func(data []byte) {
			h.readBuffer = make([]byte, len(data))
			copy(h.readBuffer, data)
			dataReceived = true
		})
		close(done)
	}()

	// Wait for data with timeout
	select {
	case <-done:
		if dataReceived && len(h.readBuffer) > 0 {
			n = copy(p, h.readBuffer)
			h.readBuffer = h.readBuffer[n:]
			return n, nil
		}
		// No data received, return EOF to indicate no data available
		return 0, io.EOF
	}
}

// Close cleans up resources
func (h *HSQAdapter) Close() error {
	h.closeMutex.Lock()
	defer h.closeMutex.Unlock()

	if h.closed {
		return nil
	}
	h.closed = true

	var errs []error

	if h.memory != nil {
		if err := mmap.UnMap(h.memory); err != nil {
			errs = append(errs, err)
		}
	}

	if h.sharedMem != nil {
		if err := h.sharedMem.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}

	return nil
}

// RealHSQProvider provides actual HSQ shared memory communication
type RealHSQProvider struct {
	config  *HSQConfig
	adapter *HSQAdapter
}

// NewRealHSQProvider creates a new HSQ communication provider with actual implementation
func NewRealHSQProvider(config *HSQConfig) (*RealHSQProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("HSQ config cannot be nil")
	}

	return &RealHSQProvider{
		config: config,
	}, nil
}

// CreateChannel implements CommunicationProvider for HSQ
func (h *RealHSQProvider) CreateChannel(ctx context.Context, path string) (io.Reader, io.Writer, error) {
	adapter, err := NewHSQAdapter(h.config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create HSQ adapter: %w", err)
	}

	h.adapter = adapter
	return adapter, adapter, nil
}

// Close implements CommunicationProvider for HSQ
func (h *RealHSQProvider) Close() error {
	if h.adapter != nil {
		return h.adapter.Close()
	}
	return nil
}
