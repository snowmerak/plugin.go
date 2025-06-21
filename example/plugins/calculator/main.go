package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

// CalculateRequest represents the request structure for calculator operations.
type CalculateRequest struct {
	Operation string  `json:"operation"`
	A         float64 `json:"a"`
	B         float64 `json:"b"`
}

// CalculateResponse represents the response structure for calculator operations.
type CalculateResponse struct {
	Result float64 `json:"result"`
	Error  string  `json:"error,omitempty"`
}

// handleCalculate processes calculation requests and returns the result.
// Business logic errors are included in the response rather than treated as application errors.
func handleCalculate(req CalculateRequest) (CalculateResponse, bool) {
	var result float64
	var errMsg string

	switch req.Operation {
	case "add":
		result = req.A + req.B
	case "subtract":
		result = req.A - req.B
	case "multiply":
		result = req.A * req.B
	case "divide":
		if req.B == 0 {
			errMsg = "division by zero is not allowed"
		} else {
			result = req.A / req.B
		}
	default:
		errMsg = fmt.Sprintf("unsupported operation: %s", req.Operation)
	}

	response := CalculateResponse{
		Result: result,
		Error:  errMsg,
	}

	// Always return false (success) - errors are handled in response object
	// This distinguishes between business logic errors and system errors
	return response, false
}

func main() {
	// Create module for communication with host via stdin/stdout
	module := plugin.New(os.Stdin, os.Stdout)

	// Define JSON serialization/deserialization functions
	unmarshalReq := func(data []byte) (CalculateRequest, error) {
		var req CalculateRequest
		err := json.Unmarshal(data, &req)
		return req, err
	}

	marshalResp := func(resp CalculateResponse) ([]byte, error) {
		return json.Marshal(resp)
	}

	// Create handler adapter
	calcAdapter := plugin.NewHandlerAdapter(
		"Calculate",     // Service name
		unmarshalReq,    // Request unmarshaling function
		marshalResp,     // Response marshaling function
		handleCalculate, // Actual handler logic
	)

	// Register handler with module
	plugin.RegisterHandler(module, "Calculate", calcAdapter.ToPluginHandler())

	// Process requests in infinite loop (ready signal is sent automatically)
	if err := module.Listen(context.Background()); err != nil {
		os.Exit(1)
	}
}
