// Package plugin provides JSON serialization adapters for type-safe plugin communication.
package plugin

import (
	"context"
	"encoding/json"
)

// NewJSONLoaderAdapter creates a LoaderAdapter specialized for JSON serialization.
// Req and Resp are the actual Go types that can be marshaled to/from JSON.
// This adapter automatically handles JSON marshaling of requests and unmarshaling of responses,
// providing a convenient way to work with JSON-based plugin APIs.
func NewJSONLoaderAdapter[Req, Resp any](loader *Loader) *LoaderAdapter[Req, Resp] {
	serializer := Serializer[Req, Resp]{
		MarshalRequest: func(req Req) ([]byte, error) {
			return json.Marshal(req)
		},
		UnmarshalResponse: func(data []byte) (Resp, error) {
			var resp Resp
			// For JSON, if Resp is a pointer type (e.g., *MyStruct),
			// json.Unmarshal will allocate memory if the pointer is nil.
			// If Resp is a value type (e.g., MyStruct), &resp provides the necessary pointer.
			err := json.Unmarshal(data, &resp)
			return resp, err
		},
		// UnmarshalError: func(data []byte) (string, error) {
		//  // Default behavior from loader.Call is string(errorPayload)
		// 	// If errors are JSON objects, unmarshal here:
		// 	// var errObj struct { Error string `json:"error"` }
		// 	// if e := json.Unmarshal(data, &errObj); e == nil {
		// 	// 	return errObj.Error, nil
		// 	// }
		// 	return string(data), nil
		// },
	}
	return NewLoaderAdapter(loader, serializer)
}

// NewJSONLoaderMessageHandlerAdapter creates a LoaderMessageHandlerAdapter specialized for JSON serialization.
// This adapter automatically handles JSON unmarshaling of incoming messages from the module.
func NewJSONLoaderMessageHandlerAdapter[T any](
	loader *Loader,
	serviceName string,
	handlerFunc func(ctx context.Context, message T) error,
) *LoaderMessageHandlerAdapter[T] {
	return NewLoaderMessageHandlerAdapter(
		loader,
		serviceName,
		func(data []byte) (T, error) {
			var msg T
			err := json.Unmarshal(data, &msg)
			return msg, err
		},
		handlerFunc,
	)
}

// NewJSONLoaderRequestHandlerAdapter creates a LoaderRequestHandlerAdapter specialized for JSON serialization.
// This adapter automatically handles JSON unmarshaling of requests and marshaling of responses.
func NewJSONLoaderRequestHandlerAdapter[Req, Resp any](
	loader *Loader,
	serviceName string,
	handlerFunc func(ctx context.Context, request Req) (Resp, bool, error),
) *LoaderRequestHandlerAdapter[Req, Resp] {
	return NewLoaderRequestHandlerAdapter(
		loader,
		serviceName,
		func(data []byte) (Req, error) {
			var req Req
			err := json.Unmarshal(data, &req)
			return req, err
		},
		func(resp Resp) ([]byte, error) {
			return json.Marshal(resp)
		},
		handlerFunc,
	)
}

// NewJSONLoaderMessageSender creates a LoaderMessageSender specialized for JSON serialization.
// This adapter automatically handles JSON marshaling of outgoing messages to the module.
func NewJSONLoaderMessageSender[T any](
	loader *Loader,
	serviceName string,
) *LoaderMessageSender[T] {
	return NewLoaderMessageSender(
		loader,
		serviceName,
		func(msg T) ([]byte, error) {
			return json.Marshal(msg)
		},
	)
}

// NewJSONLoaderRequestSender creates a LoaderRequestSender specialized for JSON serialization.
// This adapter automatically handles JSON marshaling of requests and unmarshaling of responses.
func NewJSONLoaderRequestSender[Req, Resp any](
	loader *Loader,
	serviceName string,
) *LoaderRequestSender[Req, Resp] {
	serializer := Serializer[Req, Resp]{
		MarshalRequest: func(req Req) ([]byte, error) {
			return json.Marshal(req)
		},
		UnmarshalResponse: func(data []byte) (Resp, error) {
			var resp Resp
			err := json.Unmarshal(data, &resp)
			return resp, err
		},
	}
	return NewLoaderRequestSender(loader, serviceName, serializer)
}
