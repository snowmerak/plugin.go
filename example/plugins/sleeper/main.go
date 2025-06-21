package main

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

// SleeperRequest represents the request structure for sleep operations.
type SleeperRequest struct {
	Message   string `json:"message"`
	SleepTime int    `json:"sleep_time"` // Time in seconds
}

// SleeperResponse represents the response structure for sleep operations.
type SleeperResponse struct {
	Message   string `json:"message"`
	SleptTime int    `json:"slept_time"`
}

// handleSleeper processes sleep requests and sleeps for the specified duration.
func handleSleeper(req SleeperRequest) (SleeperResponse, bool) {
	start := time.Now()

	// Sleep for the specified duration
	sleepDuration := time.Duration(req.SleepTime) * time.Second
	time.Sleep(sleepDuration)

	elapsed := int(time.Since(start).Seconds())

	response := SleeperResponse{
		Message:   "Completed: " + req.Message,
		SleptTime: elapsed,
	}

	return response, false // false = success
}

func main() {
	// Create module for communication with host via stdin/stdout
	module := plugin.New(os.Stdin, os.Stdout)

	// Define JSON serialization/deserialization functions
	unmarshalReq := func(data []byte) (SleeperRequest, error) {
		var req SleeperRequest
		err := json.Unmarshal(data, &req)
		return req, err
	}

	marshalResp := func(resp SleeperResponse) ([]byte, error) {
		return json.Marshal(resp)
	}

	// Create handler adapter
	sleeperAdapter := plugin.NewHandlerAdapter[SleeperRequest, SleeperResponse](
		"Sleep",       // Service name
		unmarshalReq,  // Request unmarshaling function
		marshalResp,   // Response marshaling function
		handleSleeper, // Actual handler logic
	)

	// Register handler with module
	plugin.RegisterHandler(module, "Sleep", sleeperAdapter.ToPluginHandler())

	// Send ready signal (loader waits for first message)
	if err := module.SendReady(context.Background()); err != nil {
		os.Exit(1)
	}

	// Process requests in infinite loop
	if err := module.Listen(context.Background()); err != nil {
		os.Exit(1)
	}
}
