package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

// EchoRequest represents the request structure for the echo service.
type EchoRequest struct {
	Message string `json:"message"`
}

// EchoResponse represents the response structure for the echo service.
type EchoResponse struct {
	Echo string `json:"echo"`
}

// handleEcho processes echo requests and returns the message with "Echo: " prefix.
func handleEcho(req EchoRequest) (EchoResponse, bool) {
	response := EchoResponse{
		Echo: "Echo: " + req.Message,
	}

	return response, false // false = success
}

func main() {
	// Create module for communication with host via stdin/stdout
	module := plugin.New(os.Stdin, os.Stdout)

	// Define JSON serialization/deserialization functions
	unmarshalReq := func(data []byte) (EchoRequest, error) {
		var req EchoRequest
		err := json.Unmarshal(data, &req)
		return req, err
	}

	marshalResp := func(resp EchoResponse) ([]byte, error) {
		return json.Marshal(resp)
	}

	// Create handler adapter
	echoAdapter := plugin.NewHandlerAdapter(
		"Echo",       // Service name
		unmarshalReq, // Request unmarshaling function
		marshalResp,  // Response marshaling function
		handleEcho,   // Actual handler logic
	)

	// Register handler with module
	plugin.RegisterHandler(module, "Echo", echoAdapter.ToPluginHandler())

	// Process requests in infinite loop (ready signal is sent automatically)
	if err := module.Listen(context.Background()); err != nil {
		os.Exit(1)
	}
}
