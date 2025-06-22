// Package plugin provides handler registration and management for Module.
//
// This file contains functionality for registering and managing
// service handlers that process incoming requests from the loader.
package plugin

import (
	"fmt"
)

// RegisterHandler registers a handler function for the given service name.
// The handler function processes raw byte payloads and returns raw byte responses.
func RegisterHandler(m *Module, name string, handler func(requestPayload []byte) (responsePayload []byte, isAppError bool)) {
	m.handlerLock.Lock()
	defer m.handlerLock.Unlock()

	if _, exists := m.handler[name]; exists {
		panic(fmt.Sprintf("handler for %s already registered", name))
	}

	m.handler[name] = func(requestPayload []byte) (AppHandlerResult, error) {
		// The user-provided handler now directly processes []byte and returns []byte.
		// No GOB decoding of request or GOB encoding of response/error is done here.
		responseBytes, isErr := handler(requestPayload)

		// The 'error' returned by this function is for critical errors within this wrapper itself,
		// which are now minimal as GOB processing is removed.
		return AppHandlerResult{Payload: responseBytes, IsError: isErr}, nil
	}
}
