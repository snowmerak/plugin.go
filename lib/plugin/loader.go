// filepath: /Users/gwon-yongmin/GitHub/snowmerak/plugin.go/lib/plugin/loader.go
package plugin

// This file serves as the main entry point for the loader package.
// The actual implementations have been split into separate files:
//
// - types.go: Core types, interfaces, and the Loader struct
// - lifecycle.go: NewLoader, Load, Close, ForceClose, and process monitoring
// - communication.go: Call, SendMessage, SendRequest, and related communication functions
// - handlers.go: Handler registration and built-in handlers
// - message_processing.go: Message handling loop and processing logic
// - utils.go: Utility functions like generateRequestID, waitForReadySignal
//
// This separation improves code organization and maintainability by grouping
// related functionality together while maintaining backward compatibility.
