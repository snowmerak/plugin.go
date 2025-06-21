// Package plugin provides loader handler adapter functionality for type-safe bidirectional plugin communication.
package plugin

import (
	"context"
	"fmt"
)

// LoaderMessageHandlerAdapter provides type-safe message handling for the Loader (host-side).
// It handles messages sent from the module to the loader without expecting a response.
type LoaderMessageHandlerAdapter[T any] struct {
	loader       *Loader
	unmarshalMsg func([]byte) (T, error)
	typedHandler func(ctx context.Context, message T) error
	serviceName  string
}

// NewLoaderMessageHandlerAdapter creates a new type-safe message handler adapter for the Loader.
// This allows the loader to handle incoming messages from the module with specific types.
func NewLoaderMessageHandlerAdapter[T any](
	loader *Loader,
	serviceName string,
	unmarshalFunc func([]byte) (T, error),
	handlerFunc func(ctx context.Context, message T) error,
) *LoaderMessageHandlerAdapter[T] {
	adapter := &LoaderMessageHandlerAdapter[T]{
		loader:       loader,
		unmarshalMsg: unmarshalFunc,
		typedHandler: handlerFunc,
		serviceName:  serviceName,
	}

	// Register the handler with the loader
	loader.RegisterMessageHandler(serviceName, adapter)

	return adapter
}

// Handle implements the MessageHandler interface
func (a *LoaderMessageHandlerAdapter[T]) Handle(ctx context.Context, header Header) error {
	// Unmarshal the payload into the typed message
	message, err := a.unmarshalMsg(header.Payload)
	if err != nil {
		return fmt.Errorf("loader message handler adapter for %s: failed to unmarshal message: %w", a.serviceName, err)
	}

	// Call the typed handler
	return a.typedHandler(ctx, message)
}

// Unregister removes this handler from the loader
func (a *LoaderMessageHandlerAdapter[T]) Unregister() {
	a.loader.UnregisterMessageHandler(a.serviceName)
}

// LoaderRequestHandlerAdapter provides type-safe request handling for the Loader (host-side).
// It handles requests sent from the module to the loader that expect a response.
type LoaderRequestHandlerAdapter[Req, Resp any] struct {
	loader       *Loader
	unmarshalReq func([]byte) (Req, error)
	marshalResp  func(Resp) ([]byte, error)
	typedHandler func(ctx context.Context, request Req) (Resp, bool, error) // returns (response, isError, error)
	serviceName  string
}

// NewLoaderRequestHandlerAdapter creates a new type-safe request handler adapter for the Loader.
// This allows the loader to handle incoming requests from the module with specific types and send typed responses.
func NewLoaderRequestHandlerAdapter[Req, Resp any](
	loader *Loader,
	serviceName string,
	unmarshalReqFunc func([]byte) (Req, error),
	marshalRespFunc func(Resp) ([]byte, error),
	handlerFunc func(ctx context.Context, request Req) (Resp, bool, error),
) *LoaderRequestHandlerAdapter[Req, Resp] {
	adapter := &LoaderRequestHandlerAdapter[Req, Resp]{
		loader:       loader,
		unmarshalReq: unmarshalReqFunc,
		marshalResp:  marshalRespFunc,
		typedHandler: handlerFunc,
		serviceName:  serviceName,
	}

	// Register the handler with the loader
	loader.RegisterRequestHandler(serviceName, adapter)

	return adapter
}

// HandleRequest implements the RequestHandler interface
func (a *LoaderRequestHandlerAdapter[Req, Resp]) HandleRequest(ctx context.Context, header Header) (responsePayload []byte, isError bool, err error) {
	// Unmarshal the request payload into the typed request
	request, err := a.unmarshalReq(header.Payload)
	if err != nil {
		errMsg := fmt.Sprintf("loader request handler adapter for %s: failed to unmarshal request: %v", a.serviceName, err)
		return []byte(errMsg), true, nil // Return as application error, not handler error
	}

	// Call the typed handler
	response, isAppError, err := a.typedHandler(ctx, request)
	if err != nil {
		// Handler execution error
		return nil, false, fmt.Errorf("loader request handler adapter for %s: handler execution error: %w", a.serviceName, err)
	}

	// Marshal the response
	responseBytes, err := a.marshalResp(response)
	if err != nil {
		errMsg := fmt.Sprintf("loader request handler adapter for %s: failed to marshal response: %v", a.serviceName, err)
		return []byte(errMsg), true, nil // Return as application error, not handler error
	}

	return responseBytes, isAppError, nil
}

// Unregister removes this handler from the loader
func (a *LoaderRequestHandlerAdapter[Req, Resp]) Unregister() {
	a.loader.UnregisterRequestHandler(a.serviceName)
}

// LoaderMessageSender provides type-safe message sending from the Loader to the Module.
type LoaderMessageSender[T any] struct {
	loader      *Loader
	marshalMsg  func(T) ([]byte, error)
	serviceName string
}

// NewLoaderMessageSender creates a new type-safe message sender for the Loader.
// This allows the loader to send typed messages to the module.
func NewLoaderMessageSender[T any](
	loader *Loader,
	serviceName string,
	marshalFunc func(T) ([]byte, error),
) *LoaderMessageSender[T] {
	return &LoaderMessageSender[T]{
		loader:      loader,
		marshalMsg:  marshalFunc,
		serviceName: serviceName,
	}
}

// Send sends a typed message to the module
func (s *LoaderMessageSender[T]) Send(ctx context.Context, message T) error {
	// Marshal the message
	payload, err := s.marshalMsg(message)
	if err != nil {
		return fmt.Errorf("loader message sender for %s: failed to marshal message: %w", s.serviceName, err)
	}

	// Send the message
	return s.loader.SendMessage(ctx, s.serviceName, payload)
}

// SendError sends a typed error message to the module
func (s *LoaderMessageSender[T]) SendError(ctx context.Context, message T) error {
	// Marshal the message
	payload, err := s.marshalMsg(message)
	if err != nil {
		return fmt.Errorf("loader message sender for %s: failed to marshal error message: %w", s.serviceName, err)
	}

	// Send the error message
	return s.loader.SendErrorMessage(ctx, s.serviceName, payload)
}

// LoaderRequestSender provides type-safe request sending from the Loader to the Module.
type LoaderRequestSender[Req, Resp any] struct {
	loaderAdapter *LoaderAdapter[Req, Resp]
	serviceName   string
}

// NewLoaderRequestSender creates a new type-safe request sender for the Loader.
// This allows the loader to send typed requests to the module and receive typed responses.
func NewLoaderRequestSender[Req, Resp any](
	loader *Loader,
	serviceName string,
	serializer Serializer[Req, Resp],
) *LoaderRequestSender[Req, Resp] {
	return &LoaderRequestSender[Req, Resp]{
		loaderAdapter: NewLoaderAdapter(loader, serializer),
		serviceName:   serviceName,
	}
}

// SendRequest sends a typed request to the module and waits for a typed response
func (s *LoaderRequestSender[Req, Resp]) SendRequest(ctx context.Context, request Req) (Resp, error) {
	return s.loaderAdapter.Call(ctx, s.serviceName, request)
}
