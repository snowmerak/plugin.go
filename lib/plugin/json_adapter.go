// Package plugin provides JSON serialization adapters for type-safe plugin communication.
package plugin

import (
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
