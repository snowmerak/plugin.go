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

### Client-Side (Loading and Calling a Plugin)

The host application uses `plugin.Loader` and a `LoaderAdapter`.

```go
package main

import (
	"context"
	"encoding/json" // For plugin-side example if not using helpers
	"fmt"
	"log"
	"os"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
	// For Protobuf example, you would import your generated protobuf package:
	// "your_project_path/mypb"
	// "google.golang.org/protobuf/proto"
)

// Example JSON types for client and plugin
type MyJSONRequest struct {
	Data string `json:"data"`
}

type MyJSONResponse struct {
	Result string `json:"result"`
}

// Example Protobuf types (assuming these are generated in mypb package)
/*
// In mypb/my.proto (example)
// syntax = "proto3";
// package mypb;
// option go_package = "your_project_path/mypb";
// message MyProtoRequest {
//   string input = 1;
// }
// message MyProtoResponse {
//   string output = 1;
// }

// In Go code (mypb/my.pb.go generated by protoc)
// Ensure these types implement proto.Message
type MyProtoRequest struct {
	// ... generated fields ...
	Input string
}
// ... proto.Message methods for MyProtoRequest ...

type MyProtoResponse struct {
	// ... generated fields ...
	Output string
}
// ... proto.Message methods for MyProtoResponse ...
// func (m *MyProtoResponse) GetOutput() string { if m != nil { return m.Output } return "" }
*/

func main() {
	// Ensure the plugin executable is built and its path is correct.
	// e.g., go build -o ./myplugin ./path/to/plugin/main.go
	loader := plugin.NewLoader("./myplugin", "myplugin", "v1.0.0")

	// Use a context for timeouts and cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := loader.Load(ctx); err != nil {
		log.Fatalf("Failed to load plugin: %v", err)
	}
	defer func() {
		if err := loader.Close(); err != nil {
			log.Printf("Error closing loader: %v", err)
		}
	}()

	// --- Using JSON Adapter ---
	// Note: Corrected to NewJSONLoaderAdapter
	jsonAdapter := plugin.NewJSONLoaderAdapter[MyJSONRequest, MyJSONResponse](loader)
	jsonReq := MyJSONRequest{Data: "Hello JSON from Client"}

	fmt.Println("Client: Calling HandleJSON...")
	jsonResp, err := jsonAdapter.Call(ctx, "HandleJSON", jsonReq)
	if err != nil {
		log.Printf("Client: JSON call error for HandleJSON: %v", err)
	} else {
		fmt.Printf("Client: JSON response from HandleJSON: %+v\n", jsonResp)
	}

	// Example of calling a service that might return an application error
	fmt.Println("Client: Calling HandleJSON with 'error' data...")
	jsonReqError := MyJSONRequest{Data: "error"}
	_, err = jsonAdapter.Call(ctx, "HandleJSON", jsonReqError)
	if err != nil {
		// This error will include the message from the plugin
		log.Printf("Client: Expected JSON call error for HandleJSON (data='error'): %v", err)
	} else {
		fmt.Println("Client: Expected an error for HandleJSON (data='error'), but got none.")
	}


	// --- Using Protobuf Adapter ---
	// (Uncomment and adapt if you have Protobuf definitions)
	/*
	// Note: Corrected to NewProtobufLoaderAdapter
	protoAdapter := plugin.NewProtobufLoaderAdapter[*mypb.MyProtoRequest, *mypb.MyProtoResponse](
		loader,
		func() *mypb.MyProtoResponse { return new(mypb.MyProtoResponse) }, // Factory for response type
	)
	protoReq := &mypb.MyProtoRequest{Input: "Hello Protobuf from Client"}

	fmt.Println("Client: Calling HandleProto...")
	protoResp, err := protoAdapter.Call(ctx, "HandleProto", protoReq)
	if err != nil {
		log.Printf("Client: Protobuf call error for HandleProto: %v", err)
	} else {
		fmt.Printf("Client: Protobuf response from HandleProto: %s\n", protoResp.GetOutput()) // Assuming GetOutput exists
	}
	*/

	// Example of calling a non-existent service
	fmt.Println("Client: Calling NonExistentService...")
	_, err = jsonAdapter.Call(ctx, "NonExistentService", MyJSONRequest{Data: "test"})
	if err != nil {
		log.Printf("Client: Expected error for NonExistentService: %v", err)
	} else {
		fmt.Println("Client: Expected an error for NonExistentService, but got none.")
	}
}
```

### Plugin-Side (Implementing the Plugin Executable)

The plugin executable uses `plugin.Module` and registers handlers. Typed handlers are wrapped using `plugin.HandlerAdapter`.

```go
package main

import (
	"context"
	"encoding/json" // Required for JSON handler adapter
	"fmt"
	"log"
	"os" // For os.Stdin, os.Stdout

	"github.com/snowmerak/plugin.go/lib/plugin"
	// For Protobuf example, you would import your generated protobuf package:
	// "your_project_path/mypb"
	// "google.golang.org/protobuf/proto"
)

// --- JSON Handler Example ---

// MyJSONRequest and MyJSONResponse should match the client's definitions
type MyJSONRequest struct {
	Data string `json:"data"`
}

type MyJSONResponse struct {
	Result string `json:"result"`
}

// Typed JSON handler function
func handleJSONRequestLogic(req MyJSONRequest) (MyJSONResponse, bool) {
	fmt.Fprintf(os.Stderr, "Plugin: Received JSON request for HandleJSON: %+v\n", req)
	if req.Data == "error" {
		// Simulate an application error
		return MyJSONResponse{Result: "Plugin: Simulated JSON error occurred"}, true // true indicates application error
	}
	return MyJSONResponse{Result: "Plugin: Processed JSON data - " + req.Data}, false // false for success
}

// --- Protobuf Handler Example ---
/*
// MyProtoRequest and MyProtoResponse should match the client's definitions
// and implement proto.Message
type MyProtoRequest struct {
	// ... generated fields ...
	Input string
}
// ... proto.Message methods for MyProtoRequest ...
// func (m *MyProtoRequest) GetInput() string { if m != nil { return m.Input } return "" }


type MyProtoResponse struct {
	// ... generated fields ...
	Output string
}
// ... proto.Message methods for MyProtoResponse ...

// Typed Protobuf handler function
func handleProtoRequestLogic(req *mypb.MyProtoRequest) (*mypb.MyProtoResponse, bool) {
	fmt.Fprintf(os.Stderr, "Plugin: Received Proto request for HandleProto: %s\n", req.GetInput())
	if req.GetInput() == "error" {
		return &mypb.MyProtoResponse{Output: "Plugin: Simulated Protobuf error"}, true
	}
	return &mypb.MyProtoResponse{Output: "Plugin says: Hello " + req.GetInput()}, false
}
*/

func main() {
	// Use os.Stdin for reading requests from host, os.Stdout for sending responses to host
	module := plugin.New(os.Stdin, os.Stdout)

	// --- Register JSON Handler ---
	// 1. Define unmarshal function for the request type
	unmarshalJSONReq := func(data []byte) (MyJSONRequest, error) {
		var r MyJSONRequest
		err := json.Unmarshal(data, &r)
		return r, err
	}
	// 2. Define marshal function for the response type
	marshalJSONResp := func(resp MyJSONResponse) ([]byte, error) {
		return json.Marshal(resp)
	}
	// 3. Create a HandlerAdapter
	jsonHandlerAdapter := plugin.NewHandlerAdapter[MyJSONRequest, MyJSONResponse](
		"HandleJSON",             // Service name (for logging/debugging in adapter)
		unmarshalJSONReq,         // Function to unmarshal request bytes
		marshalJSONResp,          // Function to marshal response object
		handleJSONRequestLogic,   // The actual typed handler logic
	)
	// 4. Register the adapted handler with the module
	plugin.RegisterHandler(module, "HandleJSON", jsonHandlerAdapter.ToPluginHandler())


	// --- Register Protobuf Handler ---
	// (Uncomment and adapt if you have Protobuf definitions)
	/*
	// 1. Define unmarshal function for the Protobuf request type
	unmarshalProtoReq := func(data []byte) (*mypb.MyProtoRequest, error) {
		instance := new(mypb.MyProtoRequest) // Or use a factory if Req is an interface/pointer
		err := proto.Unmarshal(data, instance)
		return instance, err
	}
	// 2. Define marshal function for the Protobuf response type
	marshalProtoResp := func(resp *mypb.MyProtoResponse) ([]byte, error) {
		return proto.Marshal(resp)
	}
	// 3. Create a HandlerAdapter for Protobuf
	protoHandlerAdapter := plugin.NewHandlerAdapter[*mypb.MyProtoRequest, *mypb.MyProtoResponse](
		"HandleProto",
		unmarshalProtoReq,
		marshalProtoResp,
		handleProtoRequestLogic,
	)
	// 4. Register the adapted Protobuf handler
	plugin.RegisterHandler(module, "HandleProto", protoHandlerAdapter.ToPluginHandler())
	*/

	fmt.Fprintln(os.Stderr, "Plugin: Started and listening for requests...")
	// Listen for requests indefinitely, or until context is cancelled
	if err := module.Listen(context.Background()); err != nil {
		// Log critical errors that stop the listener
		log.Fatalf("Plugin: Listener error: %v", err)
	}
	fmt.Fprintln(os.Stderr, "Plugin: Listener stopped.")
}

// To build this plugin:
// go build -o ./myplugin ./path/to/plugin/main.go
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
