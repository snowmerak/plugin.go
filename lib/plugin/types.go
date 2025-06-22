// Package plugin provides core types and interfaces for the plugin system.
// This file contains the fundamental types, interfaces, and the main Loader struct definition.
package plugin

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/snowmerak/plugin.go/lib/multiplexer"
	"github.com/snowmerak/plugin.go/lib/process"
)

// MessageHandler defines a function type for handling incoming messages from the module
type MessageHandler interface {
	Handle(ctx context.Context, header Header) error
}

// RequestHandler defines a function type for handling requests from the module that expect a response
type RequestHandler interface {
	HandleRequest(ctx context.Context, header Header) (responsePayload []byte, isError bool, err error)
}

// MessageHandlerFunc is a convenience type for converting functions to MessageHandler
type MessageHandlerFunc func(ctx context.Context, header Header) error

// Handle implements MessageHandler interface
func (f MessageHandlerFunc) Handle(ctx context.Context, header Header) error {
	return f(ctx, header)
}

// RequestHandlerFunc is a convenience type for converting functions to RequestHandler
type RequestHandlerFunc func(ctx context.Context, header Header) (responsePayload []byte, isError bool, err error)

// HandleRequest implements RequestHandler interface
func (f RequestHandlerFunc) HandleRequest(ctx context.Context, header Header) (responsePayload []byte, isError bool, err error) {
	return f(ctx, header)
}

// Loader manages the lifecycle of a plugin process and provides communication capabilities.
type Loader struct {
	Path    string
	Name    string
	Version string

	process     *process.Process
	multiplexer multiplexer.Multiplexer

	requestID atomic.Uint32

	pendingRequests map[uint32]chan []byte
	requestMutex    sync.RWMutex

	loadCtx    context.Context
	cancelLoad context.CancelFunc
	closed     atomic.Bool
	wg         sync.WaitGroup

	// Process monitoring
	processExited atomic.Bool

	// Ready signal channel
	readySignal chan struct{}

	// Shutdown acknowledgment channel
	shutdownAck chan struct{}

	// Force shutdown acknowledgment channel
	forceShutdownAck chan struct{}

	// Message handlers for incoming messages from module
	messageHandlers map[string]MessageHandler
	handlerMutex    sync.RWMutex

	// Request handlers for incoming requests from module that expect responses
	requestHandlers map[string]RequestHandler

	// Communication options and provider
	options  *LoaderOptions
	provider CommunicationProvider
}
