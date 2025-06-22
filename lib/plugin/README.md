# Plugin Package File Structure

The plugin package has been refactored to improve code organization and maintainability. The original `loader.go` file (800+ lines) has been split into multiple focused files based on functionality.

## File Organization

### Core Files

- **`loader.go`** (15 lines) - Main entry point with documentation about the file structure
- **`types.go`** (75 lines) - Core types, interfaces, and the main Loader struct definition
- **`module.go`** (679 lines) - Plugin module implementation and message types

### Functionality-Specific Files

- **`lifecycle.go`** (214 lines) - Process lifecycle management
  - `NewLoader()` - Constructor
  - `Load()` - Plugin process initialization
  - `Close()` - Graceful shutdown
  - `ForceClose()` - Forced shutdown
  - `monitorProcess()` - Process monitoring
  - `IsProcessAlive()` - Status checking

- **`communication.go`** (230 lines) - Inter-process communication
  - `Call()` - Request/response communication
  - `SendMessage()` - One-way messaging
  - `SendRequest()` - Request with response
  - `SendErrorMessage()` - Error messaging
  - `SendMessageWithSequence()` - Sequenced messaging

- **`handlers.go`** (98 lines) - Handler management
  - Handler registration/unregistration
  - Built-in protocol handlers (info, warning, error, heartbeat, status)
  - Handler retrieval functions

- **`message_processing.go`** (153 lines) - Message processing logic
  - `handleMessages()` - Main message processing loop
  - Protocol message routing
  - Request/response correlation
  - Asynchronous message handling

- **`utils.go`** (86 lines) - Utility functions
  - `generateRequestID()` - Unique ID generation
  - `waitForReadySignal()` - Plugin ready synchronization
  - `RequestReady()` - Ready signal requests

### Adapter Files

- **`adapter.go`** (130 lines) - Generic type-safe adapters
- **`json_adapter.go`** (112 lines) - JSON serialization adapters
- **`protobuf_adapter.go`** (49 lines) - Protocol Buffers adapters
- **`loader_handler_adapter.go`** (187 lines) - Bidirectional handler adapters

### Test Files

- **`loader_test.go`** (254 lines) - Loader functionality tests
- **`module_test.go`** (291 lines) - Module functionality tests
- **`header_test.go`** (276 lines) - Header marshaling/unmarshaling tests
- **`integration_test.go`** (259 lines) - Integration tests

## Module Package Structure

The Module system has been organized into focused files for better maintainability:

### Module Files:
- **`module.go`** (14 lines) - Main entry point with package documentation
- **`module_types.go`** (174 lines) - Core types, interfaces, and message definitions
  - MessageType enum and constants
  - Header struct with binary marshaling/unmarshaling
  - AppHandlerResult, Handler types
  - NodeInterface definition
  - Module struct definition
- **`module_compat.go`** (98 lines) - Backward compatibility wrappers
  - oldNodeWrapper for legacy Node interface
  - nodeWrapper for new multiplexer API
  - OldMessage type for backward compatibility
- **`module_lifecycle.go`** (78 lines) - Module creation and shutdown management
  - New() and NewStd() constructors
  - Shutdown and ForceShutdown methods
  - Status checking methods
- **`module_handlers.go`** (30 lines) - Handler registration and management
  - RegisterHandler function for service registration
- **`module_processing.go`** (249 lines) - Message listening and processing
  - Listen() method with main message loop
  - processMessage() for individual message handling
  - Shutdown signal processing
- **`module_communication.go`** (105 lines) - External communication methods
  - SendReady() for ready signal
  - SendMessage() for notifications
  - SendErrorMessage() for error notifications
  - SendRequest() for request-response pattern

### Benefits of Module Separation:
- **Single Responsibility**: Each file has a clear, focused purpose
- **Easier Maintenance**: Developers can quickly locate relevant code
- **Better Testing**: Focused files enable more targeted unit tests
- **Improved Readability**: Smaller files are easier to understand
- **Reduced Merge Conflicts**: Changes are more likely to be in separate files
- **Backward Compatibility**: All existing APIs remain unchanged

**Total lines reduced from 679 to manageable chunks, with the largest file being only 249 lines.**

## Benefits of This Structure

1. **Improved Maintainability**: Each file has a focused responsibility
2. **Better Code Navigation**: Related functionality is grouped together
3. **Easier Testing**: Functionality can be tested in isolation
4. **Enhanced Readability**: Smaller, focused files are easier to understand
5. **Backward Compatibility**: All public APIs remain unchanged

## File Dependencies

```
types.go (base types and interfaces)
    ↓
lifecycle.go (uses Loader struct)
    ↓
communication.go (uses loaded state)
    ↓
handlers.go (uses communication)
    ↓
message_processing.go (uses handlers)
    ↓
utils.go (supporting functions)
```

All files work together to provide the complete plugin functionality while maintaining clean separation of concerns.
