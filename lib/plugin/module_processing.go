// Package plugin provides message listening and processing functionality for Module.
//
// This file contains the core message processing loop that handles incoming
// requests, system messages, and manages the overall request-response cycle.
package plugin

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// Listen starts listening for incoming messages and processes them.
// It handles graceful and force shutdown signals appropriately.
// Automatically sends a ready signal when starting to listen.
func (m *Module) Listen(ctx context.Context) error {
	recv, err := m.multiplexer.ReadMessage(ctx)
	if err != nil {
		return fmt.Errorf("failed to read message: %w", err)
	}

	// Automatically send ready signal after setting up the receiver but before starting to listen
	if err := m.SendReady(ctx); err != nil {
		return fmt.Errorf("failed to send ready signal: %w", err)
	}

	// Create a context that gets cancelled on force shutdown or parent context cancellation
	listenCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Monitor for FORCE shutdown signal only (graceful shutdown is handled in message loop)
	go func() {
		select {
		case <-m.forceShutdownChan:
			cancel()
		case <-ctx.Done():
			cancel()
		}
	}()

	for {
		select {
		case mesg, ok := <-recv:
			if !ok {
				// Channel closed, exit gracefully
				break
			}

			// Check for force shutdown first - immediate exit
			if m.IsForceShutdown() {
				return ctx.Err()
			}

			// Check for shutdown message
			var header Header
			if err := header.UnmarshalBinary(mesg.Data); err == nil {
				if header.Name == "shutdown" {
					m.Shutdown()

					// Send immediate ACK to let host know we received the shutdown signal
					ackHeader := Header{
						Name:        "shutdown_ack",
						IsError:     false,
						MessageType: MessageTypeAck,
						Payload:     []byte("graceful shutdown started, waiting for jobs to complete"),
					}

					if ackData, err := ackHeader.MarshalBinary(); err == nil {
						m.multiplexer.WriteMessageWithSequence(listenCtx, mesg.ID, ackData)
					}

					// Start graceful shutdown process in a goroutine
					go func() {
						// Give a moment for any pending jobs to be added to activeJobs
						time.Sleep(100 * time.Millisecond)

						// Wait for active jobs to complete (no timeout for graceful shutdown)
						done := make(chan struct{})
						go func() {
							m.activeJobs.Wait()
							close(done)
						}()

						select {
						case <-done:
							// All jobs completed
						case <-m.forceShutdownChan:
							// Force shutdown received during graceful shutdown
						}

						// After graceful shutdown is complete, cancel the context to exit the main loop
						cancel()
					}()

					continue // Continue listening for more messages (including force_shutdown)
				} else if header.Name == "force_shutdown" {
					m.ForceShutdown()

					// Send immediate acknowledgment
					ackHeader := Header{
						Name:        "force_shutdown_ack",
						IsError:     false,
						MessageType: MessageTypeAck,
						Payload:     []byte("force shutting down"),
					}

					if ackData, err := ackHeader.MarshalBinary(); err == nil {
						m.multiplexer.WriteMessageWithSequence(listenCtx, mesg.ID, ackData)
					}

					return nil
				} else if header.Name == "request_ready" {
					// Host is requesting a ready signal, send it
					if err := m.SendReady(listenCtx); err == nil {
						// Send acknowledgment that we processed the request_ready
						ackHeader := Header{
							Name:        "request_ready_ack",
							IsError:     false,
							MessageType: MessageTypeAck,
							Payload:     []byte("ready signal sent in response to request"),
						}
						if ackData, err := ackHeader.MarshalBinary(); err == nil {
							m.multiplexer.WriteMessageWithSequence(listenCtx, mesg.ID, ackData)
						}
					}
					continue
				}
			}

			// Reject new requests during graceful shutdown
			if m.IsShutdown() {
				// Send immediate error response
				errorHeader := Header{
					Name:        header.Name,
					IsError:     true,
					MessageType: MessageTypeError,
					Payload:     []byte("service unavailable: graceful shutdown in progress"),
				}
				if errorData, err := errorHeader.MarshalBinary(); err == nil {
					m.multiplexer.WriteMessageWithSequence(listenCtx, mesg.ID, errorData)
				}
				continue // Do not process new requests
			}

			// Process regular messages (only if not shutting down)
			m.activeJobs.Add(1)
			atomic.AddInt64(&m.activeJobCount, 1)
			go func(msg *OldMessage) {
				defer func() {
					m.activeJobs.Done()
					atomic.AddInt64(&m.activeJobCount, -1)
				}()
				m.processMessage(listenCtx, msg)
			}(mesg)

		case <-listenCtx.Done():
			// Context cancelled (shutdown or parent context)

			// Wait for active jobs to complete with timeout
			done := make(chan struct{})
			go func() {
				m.activeJobs.Wait()
				close(done)
			}()

			select {
			case <-done:
				// All active jobs completed
			case <-time.After(5 * time.Second):
				// Shutdown timeout reached
			}

			return listenCtx.Err()
		}
	}
}

// processMessage handles a single message asynchronously.
// Already started tasks are allowed to complete even during graceful shutdown.
func (m *Module) processMessage(ctx context.Context, mesg *OldMessage) {
	// Important: Remove shutdown check so already started tasks can complete
	// Only new requests are blocked in Listen()

	var requestHeader Header
	if err := requestHeader.UnmarshalBinary(mesg.Data); err != nil {
		// Cannot reliably form a response if header is malformed. Just return.
		return
	}

	// Exit immediately only for force shutdown
	if m.IsForceShutdown() {
		responseHeader := Header{
			Name:        requestHeader.Name,
			IsError:     true,
			MessageType: MessageTypeError,
			Payload:     []byte("service unavailable: force shutdown"),
		}
		if responseData, err := responseHeader.MarshalBinary(); err == nil {
			m.multiplexer.WriteMessageWithSequence(ctx, mesg.ID, responseData)
		}
		return
	}

	m.handlerLock.RLock()
	targetHandler, exists := m.handler[requestHeader.Name]
	m.handlerLock.RUnlock()

	var responseHeader Header
	responseHeader.Name = requestHeader.Name

	if !exists {
		errMsg := fmt.Sprintf("no handler registered for service: %s", requestHeader.Name)
		errPayload := []byte(errMsg)
		responseHeader.IsError = true
		responseHeader.MessageType = MessageTypeError
		responseHeader.Payload = errPayload
	} else {
		// Execute handler - already started tasks complete even during graceful shutdown
		appResult, criticalErr := targetHandler(requestHeader.Payload)

		if criticalErr != nil {
			errMsg := fmt.Sprintf("critical internal error processing request for %s: %v", requestHeader.Name, criticalErr)
			errPayload := []byte(errMsg)
			responseHeader.IsError = true
			responseHeader.MessageType = MessageTypeError
			responseHeader.Payload = errPayload
		} else {
			responseHeader.IsError = appResult.IsError
			if appResult.IsError {
				responseHeader.MessageType = MessageTypeError
			} else {
				responseHeader.MessageType = MessageTypeResponse
			}
			responseHeader.Payload = appResult.Payload
		}
	}

	responseData, err := responseHeader.MarshalBinary()
	if err != nil {
		// Cannot marshal response, just return
		return
	}

	// Send response back with the original message ID for proper correlation
	if err := m.multiplexer.WriteMessageWithSequence(ctx, mesg.ID, responseData); err != nil {
		// Log error to stderr if possible, but don't fail the entire listener
	}
}
