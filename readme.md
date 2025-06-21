# plugin.go

`plugin.go` is a Go library designed to facilitate robust communication with external plugin processes. These plugins are separate executables that interact with the host application over standard I/O (stdin/stdout) using a custom multiplexing protocol. This allows for loading Go plugins, or any executable adhering to the protocol, and invoking their functions in a structured manner.

The library provides a clear separation of concerns:
- A `Loader` on the client-side (host application) to manage the lifecycle of a plugin process and send requests.
- A `Module` on the plugin-side (external executable) to register handlers for specific services and process incoming requests.
- Generic `Adapter`s (`LoaderAdapter` for clients, `HandlerAdapter` for plugins) to enable typed communication using serialization formats like JSON or Protocol Buffers, abstracting away the raw byte handling.

## Features

- **Process Management**: Forks and manages external executables as plugins using the `process` package.
- **Multiplexed Communication**: Implements a multiplexing protocol (`multiplexer` package) over stdin/stdout, allowing concurrent request/response cycles between the host and plugin.
- **Request/Response Pattern**: Uses unique sequence IDs for requests to correlate them with their responses.
- **Typed Data Handling**:
    - Client-side: `LoaderAdapter` with `NewJSONLoaderAdapter` and `NewProtobufLoaderAdapter` for making typed calls.
    - Plugin-side: `HandlerAdapter` for creating typed handlers, which can then be registered with the `Module`.
- **Serialization Support**: Built-in support for JSON and Protocol Buffers, with extensibility for custom serializers.
- **Graceful Shutdown**: Mechanisms for context-based cancellation and proper cleanup of resources.
- **Error Propagation**: Clear distinction and propagation of transport errors, plugin execution errors, and serialization/deserialization errors.

## Architecture Overview

The `plugin.go` library establishes a client-server architecture where:
- The **Host Application (Client)** uses `plugin.Loader` to start and manage an external plugin process.
- The **Plugin Process (Server)** uses `plugin.Module` to listen for requests from the host.

**Communication Flow:**
1.  **Loading**: The host application uses `plugin.Loader` to execute the plugin binary. The `Loader` establishes communication pipes using the plugin's stdin and stdout.
2.  **Multiplexing**: All communication passes through `multiplexer.Node` instances on both sides. This node chunks messages, assigns sequence IDs, and handles the low-level protocol to allow multiple "virtual" channels over the single stdin/stdout pair.
3.  **Request (Client to Plugin)**:
    *   The client uses a `LoaderAdapter` (e.g., `JSONLoaderAdapter`) to make a typed call.
    *   The adapter serializes the request object into bytes.
    *   A `plugin.Header` (containing service name, payload) is marshaled.
    *   The `Loader` sends this data through its `multiplexer.Node` to the plugin, using a unique request ID.
4.  **Handling (Plugin Side)**:
    *   The plugin's `Module` listens for messages via its `multiplexer.Node`.
    *   Upon receiving a request, it unmarshals the `plugin.Header`.
    *   It looks up the registered handler for the service name.
    *   A `HandlerAdapter` (if used) deserializes the request payload into a typed object.
    *   The typed handler function is executed.
    *   The `HandlerAdapter` serializes the typed response (or error) back into bytes.
5.  **Response (Plugin to Client)**:
    *   The `Module` marshals a `plugin.Header` for the response (indicating success or error, and containing the response payload).
    *   This is sent back to the client via the `multiplexer.Node`, using the original request ID.
6.  **Receiving Response (Client Side)**:
    *   The `Loader`'s message-reading goroutine receives the response.
    *   It correlates the response to the pending request using the ID.
    *   The `LoaderAdapter` unmarshals the response payload into the expected typed object.
    *   The result (or error) is returned to the original caller.

**Diagrammatic Flow:**
```
+---------------------+      StdIn/StdOut      +----------------------|
| Host Application    |       Pipes via        | Plugin Executable    |
|---------------------|      `process` Pkg     |----------------------|
| `plugin.Loader`     |<--------------------->| `plugin.Module`      |
|  `LoaderAdapter`    |                        |  `HandlerAdapter`    |
|   (JSON/Proto)      |                        |   (JSON/Proto)       |
| `multiplexer.Node`  |----Multiplexed Data--->| `multiplexer.Node`   |
|                     |<---Multiplexed Data----|                      |
+---------------------+                        +----------------------|
```

## Core Components

-   **`lib/process`**:
    *   `Process`: A utility to fork and manage external processes, providing access to their stdin, stdout, and stderr.
-   **`lib/multiplexer`**:
    *   `Node`: Implements the core multiplexing logic. It reads from an `io.Reader` and writes to an `io.Writer`, segmenting messages into frames with headers for type, ID, and length.
    *   `Message`: Represents a unit of data exchanged by the multiplexer.
-   **`lib/plugin`**:
    *   `Loader`: Manages the lifecycle of a plugin. It forks the plugin process, sets up the multiplexer, and provides the `Call` method for sending requests.
    *   `Module`: Used within the plugin executable. It listens for incoming requests via its multiplexer, dispatches them to registered handlers, and sends responses back.
    *   `Header`: Defines the structure of the metadata part of a message, including the service name, an error flag, and the payload. It has `MarshalBinary` and `UnmarshalBinary` methods.
    *   `Serializer[Req, Resp]`: A struct holding functions for `MarshalRequest` and `UnmarshalResponse`. Used by adapters.
    *   `LoaderAdapter[Req, Resp]`: A generic client-side adapter. It takes a `Loader` and a `Serializer` to provide a typed `Call` method.
        *   `NewJSONLoaderAdapter[Req, Resp]`: A constructor for a `LoaderAdapter` pre-configured for JSON.
        *   `NewProtobufLoaderAdapter[Req, Resp]`: A constructor for a `LoaderAdapter` pre-configured for Protocol Buffers. Requires a factory function for the response type.
    *   `HandlerAdapter[Req, Resp]`: A generic plugin-side adapter. It wraps a user's typed handler function, along with unmarshaling and marshaling functions, to produce a raw handler compatible with `Module.RegisterHandler`.
        *   (Implicitly, `NewHandlerAdapter` is the base, and JSON/Protobuf versions use it by providing specific marshal/unmarshal logic).
    *   `RegisterHandler(module, name, handlerFunc)`: Registers a raw byte handler with the module. Typed handlers are typically wrapped by a `HandlerAdapter` first.

## Communication Protocol

The `multiplexer.Node` implements a protocol on top of the I/O streams. Each logical message is framed:

-   **Header (9 bytes total)**:
    *   **Message Type (1 byte)**:
        *   `MessageHeaderTypeStart (0x01)`: Indicates the beginning of a new message sequence.
        *   `MessageHeaderTypeData (0x03)`: Contains a chunk of the message data.
        *   `MessageHeaderTypeEnd (0x02)`: Indicates the end of a message sequence.
        *   `MessageHeaderTypeAbort (0x06)`: Indicates that the message sequence should be aborted (e.g., due to context cancellation).
        *   `MessageHeaderTypeError (0x04)`: Indicates an error within the multiplexer itself.
        *   `MessageHeaderTypeComplete (0x05)`: Used internally by the reader to mark a fully assembled message.
    *   **Frame ID (4 bytes)**: A `uint32` (BigEndian) identifying the message sequence. Allows multiplexing.
    *   **Data Length (4 bytes)**: A `uint32` (BigEndian) specifying the length of the payload *in this specific frame* (for `MessageHeaderTypeData` frames). The `plugin.Header` (service name, etc.) is part of the payload of the first `Data` frame in a logical application message.

-   **Payload (variable length)**: The actual data being sent. For application messages, this payload will first be the marshaled `plugin.Header`, followed by the user's request/response data.

**Message Flow Example (Request):**
1.  Client calls `loader.Call` which uses `multiplexer.WriteMessageWithSequence`.
2.  Multiplexer sends:
    *   Frame 1: `Type=Start, ID=N, Length=0`
    *   Frame 2: `Type=Data, ID=N, Length=X, Payload=chunk1` (Payload contains marshaled `plugin.Header` and part of user data)
    *   ... (more Data frames if needed)
    *   Frame N: `Type=End, ID=N, Length=0`

The plugin's multiplexer reassembles these frames into a complete message for the `plugin.Module`.

## Installation

```bash
go get github.com/snowmerak/plugin.go
```

## Usage Guide

### Complete Working Example

This library includes a complete working example in the `example/` directory demonstrating real-world usage patterns.

#### Building and Running the Examples

1. Build the plugin executables:
   ```bash
   cd example
   go build -o plugins/echo/echo ./plugins/echo
   go build -o plugins/calculator/calculator ./plugins/calculator
   ```

2. Run the host application:
   ```bash
   go run ./host
   ```

### Client-Side (Loading and Calling a Plugin)

Here's the complete example from `example/host/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

func main() {
	fmt.Println("=== Plugin.go Host Application Started ===")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test Echo Plugin
	fmt.Println("\n--- Echo Plugin Test ---")
	if err := testEchoPlugin(ctx); err != nil {
		log.Printf("Echo plugin test failed: %v", err)
	}

	// Test Calculator Plugin
	fmt.Println("\n--- Calculator Plugin Test ---")
	if err := testCalculatorPlugin(ctx); err != nil {
		log.Printf("Calculator plugin test failed: %v", err)
	}

	fmt.Println("\n=== All tests completed ===")
}

// Echo plugin test function
func testEchoPlugin(ctx context.Context) error {
	pluginPath := filepath.Join("..", "plugins", "echo", "echo")
	loader := plugin.NewLoader(pluginPath, "echo", "v1.0.0")

	if err := loader.Load(ctx); err != nil {
		return fmt.Errorf("failed to load Echo plugin: %w", err)
	}
	defer func() {
		if err := loader.Close(); err != nil {
			log.Printf("Error closing Echo loader: %v", err)
		}
	}()

	// Create JSON adapter
	type EchoRequest struct {
		Message string `json:"message"`
	}
	type EchoResponse struct {
		Echo string `json:"echo"`
	}

	adapter := plugin.NewJSONLoaderAdapter[EchoRequest, EchoResponse](loader)

	// Test cases
	testCases := []struct {
		name    string
		request EchoRequest
	}{
		{"Basic message", EchoRequest{Message: "Hello World!"}},
		{"Empty message", EchoRequest{Message: ""}},
		{"Long message", EchoRequest{Message: "This is a very long message to test how the plugin handles longer content."}},
	}

	for _, tc := range testCases {
		fmt.Printf("  Test: %s\n", tc.name)
		resp, err := adapter.Call(ctx, "Echo", tc.request)
		if err != nil {
			fmt.Printf("    Error: %v\n", err)
			continue
		}
		fmt.Printf("    Request: %s\n", tc.request.Message)
		fmt.Printf("    Response: %s\n", resp.Echo)
	}

	return nil
}

// Calculator plugin test function
func testCalculatorPlugin(ctx context.Context) error {
	pluginPath := filepath.Join("..", "plugins", "calculator", "calculator")
	loader := plugin.NewLoader(pluginPath, "calculator", "v1.0.0")

	if err := loader.Load(ctx); err != nil {
		return fmt.Errorf("failed to load Calculator plugin: %w", err)
	}
	defer func() {
		if err := loader.Close(); err != nil {
			log.Printf("Error closing Calculator loader: %v", err)
		}
	}()

	// Create JSON adapter
	type CalculateRequest struct {
		Operation string  `json:"operation"`
		A         float64 `json:"a"`
		B         float64 `json:"b"`
	}
	type CalculateResponse struct {
		Result float64 `json:"result"`
		Error  string  `json:"error,omitempty"`
	}

	adapter := plugin.NewJSONLoaderAdapter[CalculateRequest, CalculateResponse](loader)

	// Test cases
	testCases := []struct {
		name    string
		request CalculateRequest
	}{
		{"Addition", CalculateRequest{Operation: "add", A: 10, B: 5}},
		{"Subtraction", CalculateRequest{Operation: "subtract", A: 10, B: 3}},
		{"Multiplication", CalculateRequest{Operation: "multiply", A: 7, B: 6}},
		{"Division", CalculateRequest{Operation: "divide", A: 20, B: 4}},
		{"Division by zero", CalculateRequest{Operation: "divide", A: 10, B: 0}},
		{"Invalid operation", CalculateRequest{Operation: "invalid", A: 1, B: 2}},
	}

	for _, tc := range testCases {
		fmt.Printf("  Test: %s\n", tc.name)
		resp, err := adapter.Call(ctx, "Calculate", tc.request)
		if err != nil {
			fmt.Printf("    Plugin error: %v\n", err)
			continue
		}

		if resp.Error != "" {
			fmt.Printf("    Calculation error: %s\n", resp.Error)
		} else {
			fmt.Printf("    %g %s %g = %g\n", tc.request.A, tc.request.Operation, tc.request.B, resp.Result)
		}
	}

	return nil
}
```

### Plugin-Side (Implementing the Plugin Executable)

Here's the complete Echo plugin example from `example/plugins/echo/main.go`:

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

// Echo request/response type definitions
type EchoRequest struct {
	Message string `json:"message"`
}

type EchoResponse struct {
	Echo string `json:"echo"`
}

// Echo handler logic
func handleEcho(req EchoRequest) (EchoResponse, bool) {
	// Log to stderr for debugging
	fmt.Fprintf(os.Stderr, "Echo Plugin: Message received: '%s'\n", req.Message)

	// Simply return with "Echo: " prefix
	response := EchoResponse{
		Echo: "Echo: " + req.Message,
	}

	return response, false // false = success
}

func main() {
	fmt.Fprintln(os.Stderr, "Echo Plugin: Started")

	// Create module that communicates with host via stdin/stdout
	module := plugin.NewStd()

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
	echoAdapter := plugin.NewHandlerAdapter[EchoRequest, EchoResponse](
		"Echo",       // Service name
		unmarshalReq, // Request unmarshaling function
		marshalResp,  // Response marshaling function
		handleEcho,   // Actual handler logic
	)

	// Register handler with module
	plugin.RegisterHandler(module, "Echo", echoAdapter.ToPluginHandler())

	// Send ready signal (loader waits for first message)
	if err := module.SendReady(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Echo Plugin: Failed to send ready message: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "Echo Plugin: Waiting for requests...")

	// Process requests in infinite loop
	if err := module.Listen(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Echo Plugin: Error occurred: %v\n", err)
		os.Exit(1)
	}
}
```

And the Calculator plugin example from `example/plugins/calculator/main.go`:

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

// Calculator request/response type definitions
type CalculateRequest struct {
	Operation string  `json:"operation"`
	A         float64 `json:"a"`
	B         float64 `json:"b"`
}

type CalculateResponse struct {
	Result float64 `json:"result"`
	Error  string  `json:"error,omitempty"`
}

// Calculator handler logic
func handleCalculate(req CalculateRequest) (CalculateResponse, bool) {
	fmt.Fprintf(os.Stderr, "Calculator Plugin: Operation request: %s %g %g\n",
		req.Operation, req.A, req.B)

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
			errMsg = "Cannot divide by zero"
		} else {
			result = req.A / req.B
		}
	default:
		errMsg = fmt.Sprintf("Unsupported operation: %s", req.Operation)
	}

	response := CalculateResponse{
		Result: result,
		Error:  errMsg,
	}

	// Don't treat as application error, include errors in response
	// This demonstrates separating business logic errors from system errors
	return response, false // Always false (success) - errors handled in response object
}

func main() {
	fmt.Fprintln(os.Stderr, "Calculator Plugin: Started")

	// Create module that communicates with host via stdin/stdout
	module := plugin.NewStd()

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
	calcAdapter := plugin.NewHandlerAdapter[CalculateRequest, CalculateResponse](
		"Calculate",     // Service name
		unmarshalReq,    // Request unmarshaling function
		marshalResp,     // Response marshaling function
		handleCalculate, // Actual handler logic
	)

	// Register handler with module
	plugin.RegisterHandler(module, "Calculate", calcAdapter.ToPluginHandler())

	// Send ready signal (loader waits for first message)
	if err := module.SendReady(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Calculator Plugin: Failed to send ready message: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "Calculator Plugin: Waiting for requests...")

	// Process requests in infinite loop
	if err := module.Listen(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Calculator Plugin: Error occurred: %v\n", err)
		os.Exit(1)
	}
}
```

## Error Handling

-   **Transport Errors**: If the connection between the host and plugin breaks (e.g., plugin crashes, pipes close), `loader.Call` or `module.Listen` will return errors.
-   **Serialization/Deserialization Errors**:
    *   On the client-side, `LoaderAdapter.Call` can return errors if request marshaling or response unmarshaling fails.
    *   On the plugin-side, `HandlerAdapter` handles marshaling/unmarshaling errors. If they occur, it typically formats an error message and returns it as an application error to the client (i.e., `isAppError = true`).
-   **Application Errors (Plugin Logic)**:
    *   Typed handlers in the plugin (e.g., `func(req Req) (Resp, bool)`) return a boolean flag indicating if an application-specific error occurred.
    *   If this flag is `true`, the `plugin.Header.IsError` field is set to true, and the (marshaled) response payload is treated as an error message by the client.
    *   `loader.Call` on the client-side will return an error formatted like `plugin error for service <name>: <error message from plugin>`.
-   **Service Not Found**: If the client calls a service name not registered in the plugin's `Module`, the `Module` sends back an error, and `loader.Call` returns this error.

## Extensibility

While JSON and Protobuf adapters are provided, you can support other serialization formats:
1.  Implement the `MarshalRequest func(Req) ([]byte, error)` and `UnmarshalResponse func([]byte) (Resp, error)` functions for your chosen format.
2.  On the client-side, create a `plugin.Serializer[Req, Resp]` struct with these functions and use it with `plugin.NewLoaderAdapter[Req, Resp](loader, customSerializer)`.
3.  On the plugin-side, provide your custom unmarshal/marshal functions when creating a `plugin.NewHandlerAdapter` for your typed handlers.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
