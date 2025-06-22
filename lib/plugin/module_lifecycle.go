// Package plugin provides Module lifecycle management functionality.
//
// This file contains Module creation, initialization, and shutdown management.
// It handles the basic lifecycle operations for plugin modules.
package plugin

import (
	"io"
	"os"
	"sync/atomic"

	"github.com/snowmerak/plugin.go/lib/multiplexer"
)

// New creates a new Module instance with the specified reader and writer.
// If reader or writer are nil, they default to os.Stdin and os.Stdout respectively.
func New(reader io.Reader, writer io.Writer) *Module {
	if reader == nil {
		reader = os.Stdin
	}

	if writer == nil {
		writer = os.Stdout
	}
	// Use the general multiplexer API which uses HybridNode
	mux := multiplexer.New(reader, writer)

	return &Module{
		multiplexer:       &nodeWrapper{multiplexer: mux},
		handler:           make(map[string]Handler),
		shutdownChan:      make(chan struct{}),
		forceShutdownChan: make(chan struct{}),
	}
}

// NewStd creates a new Module instance using standard input and output.
func NewStd() *Module {
	return New(os.Stdin, os.Stdout)
}

// Shutdown initiates graceful shutdown of the module.
func (m *Module) Shutdown() {
	m.shutdownOnce.Do(func() {
		close(m.shutdownChan)
	})
}

// ForceShutdown initiates immediate shutdown of the module.
func (m *Module) ForceShutdown() {
	m.forceShutdownOnce.Do(func() {
		close(m.forceShutdownChan)
	})
}

// IsShutdown returns true if the module is shutting down (gracefully).
func (m *Module) IsShutdown() bool {
	select {
	case <-m.shutdownChan:
		return true
	default:
		return false
	}
}

// IsForceShutdown returns true if the module is force shutting down.
func (m *Module) IsForceShutdown() bool {
	select {
	case <-m.forceShutdownChan:
		return true
	default:
		return false
	}
}

// getActiveJobCount returns the current number of active jobs.
func (m *Module) getActiveJobCount() int64 {
	return atomic.LoadInt64(&m.activeJobCount)
}
