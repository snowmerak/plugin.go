// Package plugin provides adapter functionality for type-safe plugin communication.
package plugin

import (
	"context"
	"fmt"
	"log" // Added for HandlerAdapter
)

// Serializer defines the functions for serializing requests and deserializing responses for the client LoaderAdapter.
// It provides type-safe marshaling and unmarshaling of plugin communication data.
type Serializer[Req, Resp any] struct {
	// MarshalRequest converts a typed request into bytes for transmission to the plugin
	MarshalRequest func(Req) ([]byte, error)
	// UnmarshalResponse converts bytes received from the plugin back into a typed response
	UnmarshalResponse func([]byte) (Resp, error)
	// UnmarshalError can be used if plugin error payloads have a specific structure.
	// For now, loader.Call stringifies error payloads, so this might not be strictly needed
	// unless a structured error is preferred by the client.
	// UnmarshalError    func([]byte) (string, error)
}

// LoaderAdapter provides a generic way to call plugin functions with specific request and response types (client-side).
// It wraps a Loader and handles type-safe serialization/deserialization of communication data.
type LoaderAdapter[Req, Resp any] struct {
	loader     *Loader
	serializer Serializer[Req, Resp]
}

// NewLoaderAdapter creates a new generic loader adapter with a given loader and serializer (client-side).
// The adapter provides type-safe communication with plugins by handling serialization automatically.
func NewLoaderAdapter[Req, Resp any](loader *Loader, serializer Serializer[Req, Resp]) *LoaderAdapter[Req, Resp] {
	return &LoaderAdapter[Req, Resp]{
		loader:     loader,
		serializer: serializer,
	}
}

// Call invokes a function on the plugin (client-side).
// It marshals the request, sends it to the plugin, and unmarshals the response into the expected type.
// Returns an error if marshaling, communication, or unmarshaling fails.
func (a *LoaderAdapter[Req, Resp]) Call(ctx context.Context, name string, request Req) (Resp, error) {
	var zeroResp Resp // Zero value of Resp to return on error

	requestBytes, err := a.serializer.MarshalRequest(request)
	if err != nil {
		return zeroResp, fmt.Errorf("loaderadapter: failed to marshal request for %s: %w", name, err)
	}

	// Call the underlying loader's Call method which handles raw byte communication
	responseBytes, err := Call(ctx, a.loader, name, requestBytes)
	if err != nil {
		// err from loader.Call could be a transport error or a plugin-defined error.
		// loader.Call already formats plugin errors like "plugin error for service %s: %s"
		return zeroResp, err
	}

	// If loader.Call was successful, responseBytes contains the actual payload from the plugin.
	// Now, deserialize it into the generic Resp type.
	resp, err := a.serializer.UnmarshalResponse(responseBytes)
	if err != nil {
		return zeroResp, fmt.Errorf("loaderadapter: failed to unmarshal response for %s: %w", name, err)
	}

	return resp, nil
}

// HandlerAdapter provides a generic way to wrap module handlers with specific request and response types (module-side).
// It handles deserialization of incoming requests and serialization of outgoing responses.
type HandlerAdapter[Req, Resp any] struct {
	unmarshalReq func([]byte) (Req, error)
	marshalResp  func(Resp) ([]byte, error)
	typedHandler func(Req) (Resp, bool) // User's handler: takes typed Req, returns typed Resp and appError bool
	serviceName  string
}

// NewHandlerAdapter creates a new generic module adapter (module-side).
// It wraps a typed handler function to work with the plugin module's raw byte interface.
// The typedHandlerFunc should return (response, isAppError) where isAppError indicates
// whether the response represents an application error.
func NewHandlerAdapter[Req, Resp any](
	serviceName string,
	unmarshalReqFunc func([]byte) (Req, error),
	marshalRespFunc func(Resp) ([]byte, error),
	typedHandlerFunc func(Req) (Resp, bool),
) *HandlerAdapter[Req, Resp] {
	return &HandlerAdapter[Req, Resp]{
		unmarshalReq: unmarshalReqFunc,
		marshalResp:  marshalRespFunc,
		typedHandler: typedHandlerFunc,
		serviceName:  serviceName,
	}
}

// ToPluginHandler converts the typed handler into the raw plugin.Handler signature.
// It handles deserialization of the request and serialization of the response using the provided functions.
// The returned plugin.Handler expects raw bytes and returns raw bytes with an error flag.
//
// The returned function unmarshals incoming request bytes into the typed Req, calls the typed handler,
// and marshals the response back to bytes. Any marshaling/unmarshaling errors are returned as
// application errors to be sent back to the client.
func (ha *HandlerAdapter[Req, Resp]) ToPluginHandler() func(requestPayload []byte) (responsePayload []byte, isAppError bool) {
	return func(requestPayload []byte) ([]byte, bool) {
		req, err := ha.unmarshalReq(requestPayload)
		if err != nil {
			errMsg := fmt.Sprintf("handler adapter for %s: failed to unmarshal request: %v", ha.serviceName, err)
			log.Printf("Error: %s. Payload: %x", errMsg, requestPayload)
			// Return this as an application error, so the error message is sent back to the client.
			return []byte(errMsg), true
		}

		respObj, isAppErr := ha.typedHandler(req)

		// If the typedHandler indicates an application error (isAppErr is true),
		// respObj might be an error structure or a regular response structure.
		// In either case, we attempt to marshal it.
		// If isAppErr is false, respObj is the success response.
		marshaledPayload, err := ha.marshalResp(respObj)
		if err != nil {
			// This is an error in marshalling the response (either success or error response from typedHandler).
			// Treat this as an application error to send the marshalling error message back.
			errMsg := fmt.Sprintf("handler adapter for %s: failed to marshal response (isAppErr=%t): %v", ha.serviceName, isAppErr, err)
			log.Printf("Error: %s. Response object: %+v", errMsg, respObj) // Be careful logging potentially large respObj
			return []byte(errMsg), true
		}

		// The isAppErr flag from the typedHandler determines if the marshaledPayload represents an error or success.
		return marshaledPayload, isAppErr
	}
}
