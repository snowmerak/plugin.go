// Package plugin provides Module lifecycle management functionality.
//
// This file contains Module creation, initialization, and shutdown management.
// It handles the basic lifecycle operations for plugin modules.
package plugin

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync/atomic"

	"github.com/snowmerak/plugin.go/lib/multiplexer"
)

// New creates a new Module instance with the specified reader and writer using default stdio communication.
// If reader or writer are nil, they default to os.Stdin and os.Stdout respectively.
func New(reader io.Reader, writer io.Writer) *Module {
	return NewWithOptions(reader, writer, DefaultModuleOptions())
}

// NewWithOptions creates a new Module instance with custom communication options.
// If reader or writer are nil, they default to os.Stdin and os.Stdout respectively.
func NewWithOptions(reader io.Reader, writer io.Writer, options *ModuleOptions) *Module {
	if options == nil {
		options = DefaultModuleOptions()
	}

	// Handle custom providers that need to create their own communication channels
	if options.CommunicationType == CommunicationTypeCustom && options.Provider != nil {
		// For custom providers, use the provider to create the communication channel
		providerReader, providerWriter, err := options.Provider.CreateChannel(context.Background(), "")
		if err != nil {
			// If provider fails, fall back to provided reader/writer or stdio
			if reader == nil {
				reader = os.Stdin
			}
			if writer == nil {
				writer = os.Stdout
			}
		} else {
			reader = providerReader
			writer = providerWriter
		}
	} else {
		// For stdio and other types, use provided reader/writer or default to stdio
		if reader == nil {
			reader = os.Stdin
		}

		if writer == nil {
			writer = os.Stdout
		}
	}

	// Use the general multiplexer API which uses HybridNode
	mux := multiplexer.New(reader, writer)

	module := &Module{
		multiplexer:       &nodeWrapper{multiplexer: mux},
		handler:           make(map[string]Handler),
		shutdownChan:      make(chan struct{}),
		forceShutdownChan: make(chan struct{}),
		options:           options,
	}

	// Store provider reference for cleanup
	if options.CommunicationType == CommunicationTypeCustom && options.Provider != nil {
		module.provider = options.Provider
	}

	return module
}

// NewFromHSQ creates a new Module instance using HSQ shared memory communication.
func NewFromHSQ(config *HSQConfig) (*Module, error) {
	options := WithHSQModule(config)

	// For HSQ, we don't use traditional stdin/stdout, so we create a placeholder adapter
	provider, err := NewHSQProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create HSQ provider: %w", err)
	}

	// Create reader/writer from HSQ provider
	reader, writer, err := provider.CreateChannel(context.Background(), "")
	if err != nil {
		return nil, fmt.Errorf("failed to create HSQ communication channel: %w", err)
	}

	module := NewWithOptions(reader, writer, options)
	module.provider = provider

	return module, nil
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
