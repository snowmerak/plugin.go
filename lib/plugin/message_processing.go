// Package plugin provides message processing functionality.
// This file contains the main message handling loop and processing logic for incoming messages from plugins.
package plugin

import (
	"context"
	"fmt"
	"time"
)

// handleMessages handles incoming messages from the plugin for request/response communication
func (l *Loader) handleMessages() {
	defer l.wg.Done()
	defer func() {
		l.requestMutex.Lock()
		defer l.requestMutex.Unlock()
		for id, ch := range l.pendingRequests {
			// Check if channel already received data and close safely
			select {
			case <-ch:
				// Data already exists, leave it
			default:
				// No data, close channel to signal waiting goroutines
				close(ch)
			}
			delete(l.pendingRequests, id)
		}
	}()

	// Create a new message reader for handling request/response communication
	recv, err := l.multiplexer.ReadMessage(l.loadCtx)
	if err != nil {
		return
	}

	readyReceived := false

	for {
		select {
		case <-l.loadCtx.Done():
			return
		case mesg, ok := <-recv:
			if !ok {
				return
			}

			// Parse the header first
			var header Header
			if err := header.UnmarshalBinary(mesg.Data); err != nil {
				// Invalid header, skip this message
				continue
			}

			// Handle ready signal first (if not already received)
			if !readyReceived && header.Name == "ready" {
				readyReceived = true
				// Signal that ready is received
				select {
				case l.readySignal <- struct{}{}:
				default:
					// Channel is full, ready signal already sent
				}
				continue
			}

			// Handle built-in shutdown acknowledgments
			if header.Name == "shutdown_ack" {
				select {
				case l.shutdownAck <- struct{}{}:
				default:
					// Channel is full, ack already sent
				}
				continue
			} else if header.Name == "force_shutdown_ack" {
				select {
				case l.forceShutdownAck <- struct{}{}:
				default:
					// Channel is full, ack already sent
				}
				continue
			} else if header.Name == "request_ready_ack" {
				// Handle request_ready acknowledgment
				// This is informational, nothing special to do
				continue
			}

			// Check if this is a response to a pending request
			if mesg.Sequence != 0 {
				// For responses, check if we have a pending request
				if header.MessageType == MessageTypeResponse || header.MessageType == MessageTypeError {
					requestID := mesg.Sequence

					l.requestMutex.RLock()
					responseChan, exists := l.pendingRequests[requestID]
					l.requestMutex.RUnlock()

					if exists {
						select {
						case responseChan <- mesg.Data:
							// Successfully sent response to waiting request
						case <-l.loadCtx.Done():
							return
						default:
							// Channel is full - caller will timeout
						}
						continue
					}
				}
			}

			// Handle module-initiated requests (expects responses)
			if header.MessageType == MessageTypeRequest && mesg.Sequence != 0 {
				if handler, exists := l.getRequestHandler(header.Name); exists {
					// Handle the request asynchronously and send response back to module
					go func(h RequestHandler, hdr Header, seq uint32) {
						ctx, cancel := context.WithTimeout(l.loadCtx, 30*time.Second)
						defer cancel()

						responsePayload, isError, err := h.HandleRequest(ctx, hdr)
						if err != nil {
							// Handler error - send error response
							errorMsg := fmt.Sprintf("Request handler error for '%s': %v", hdr.Name, err)
							l.SendMessageWithSequence(ctx, seq, hdr.Name+"_response", []byte(errorMsg), true)
							return
						}

						// Send successful response back to module
						responseName := hdr.Name + "_response"
						l.SendMessageWithSequence(ctx, seq, responseName, responsePayload, isError)
					}(handler, header, mesg.Sequence)
					continue
				}
			}

			// Handle module-initiated notifications/messages (no response expected)
			if header.MessageType == MessageTypeNotify || header.MessageType == MessageTypeAck {
				if handler, exists := l.getMessageHandler(header.Name); exists {
					// Handle the message asynchronously to avoid blocking the message loop
					go func(h MessageHandler, hdr Header) {
						ctx, cancel := context.WithTimeout(l.loadCtx, 30*time.Second)
						defer cancel()

						if err := h.Handle(ctx, hdr); err != nil {
							// Log handler errors but don't crash the message loop
							// In a production system, you might want to use a proper logger
							fmt.Printf("Message handler error for '%s': %v\n", hdr.Name, err)
						}
					}(handler, header)
				}
			}
			// If no handler is registered, the message is ignored
			// This allows for flexible message handling where not all messages need to be handled
		}
	}
}
