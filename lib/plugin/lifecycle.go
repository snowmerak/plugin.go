// Package plugin provides lifecycle management functionality for plugin processes.
// This file contains functions for creating, loading, closing, and monitoring plugin processes.
package plugin

import (
	"context"
	"fmt"
	"time"

	"github.com/snowmerak/plugin.go/lib/multiplexer"
	"github.com/snowmerak/plugin.go/lib/process"
)

// NewLoader creates a new Loader instance with the specified path, name, and version.
func NewLoader(path, name, version string) *Loader {
	l := &Loader{
		Path:             path,
		Name:             name,
		Version:          version,
		process:          nil,
		multiplexer:      nil,
		pendingRequests:  make(map[uint32]chan []byte), // Changed uint64 to uint32
		readySignal:      make(chan struct{}, 1),
		shutdownAck:      make(chan struct{}, 1),
		forceShutdownAck: make(chan struct{}, 1),
		messageHandlers:  make(map[string]MessageHandler),
		requestHandlers:  make(map[string]RequestHandler),
		// wg is initialized to its zero value, which is ready to use.
	}

	// Register built-in message handlers immediately
	l.registerBuiltinHandlers()

	return l
}

// Load starts the plugin process and establishes communication.
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
// It attempts graceful shutdown first, then waits for completion.
func (l *Loader) Close() error {
	if !l.closed.CompareAndSwap(false, true) {
		return fmt.Errorf("loader already closed")
	}

	var closeErr error

	// 1. Send graceful shutdown signal to plugin if multiplexer is available
	if l.multiplexer != nil && l.loadCtx != nil {
		shutdownHeader := Header{
			Name:        "shutdown",
			IsError:     false,
			MessageType: MessageTypeRequest,
			Payload:     []byte("graceful shutdown"),
		}

		if shutdownData, err := shutdownHeader.MarshalBinary(); err == nil {
			// Try to send shutdown signal with a short timeout
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer shutdownCancel()

			if err := l.multiplexer.WriteMessage(shutdownCtx, shutdownData); err == nil {
				// Wait for shutdown acknowledgment (no timeout)
				<-l.shutdownAck
			}
		}
	}

	// 2. Cancel the load context
	if l.cancelLoad != nil {
		l.cancelLoad()
	}

	// 3. Close the process (this will close pipes and signal goroutines to exit)
	if l.process != nil {
		closeErr = l.process.Close()
	}

	// 4. Wait for goroutines to complete with timeout
	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines completed
	case <-time.After(2 * time.Second):
		// Close timed out, some goroutines may not have finished
	}

	return closeErr
}

// ForceClose forcibly shuts down the loader without waiting for graceful shutdown.
// This method should be used when immediate termination is required.
func (l *Loader) ForceClose() error {
	if !l.closed.CompareAndSwap(false, true) {
		return fmt.Errorf("loader already closed")
	}

	var closeErr error

	// 1. Send force shutdown signal to plugin if multiplexer is available
	if l.multiplexer != nil && l.loadCtx != nil {
		forceShutdownHeader := Header{
			Name:        "force_shutdown",
			IsError:     false,
			MessageType: MessageTypeRequest,
			Payload:     []byte("force shutdown"),
		}

		if shutdownData, err := forceShutdownHeader.MarshalBinary(); err == nil {
			// Try to send force shutdown signal with a short timeout
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer shutdownCancel()

			if err := l.multiplexer.WriteMessage(shutdownCtx, shutdownData); err == nil {
				// Wait for force shutdown acknowledgment with short timeout
				select {
				case <-l.forceShutdownAck:
					// Received force shutdown acknowledgment
				case <-time.After(500 * time.Millisecond):
					// Force shutdown ack timeout, proceeding anyway
				}
			}
		}
	}

	// 2. Cancel the load context immediately
	if l.cancelLoad != nil {
		l.cancelLoad()
	}

	// 3. Close the process immediately (this will force terminate)
	if l.process != nil {
		closeErr = l.process.Close()
	}

	// 4. Wait for goroutines to complete with shorter timeout
	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines completed (force)
	case <-time.After(500 * time.Millisecond):
		// Force close timed out quickly - some goroutines may still be running
	}

	return closeErr
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

// IsProcessAlive returns true if the plugin process is still running
func (l *Loader) IsProcessAlive() bool {
	return !l.processExited.Load() && !l.closed.Load()
}
