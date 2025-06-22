// Package plugin provides Module functionality for plugin development.
//
// This is the main entry point for the Module system, which has been
// reorganized into separate focused files for better maintainability:
//
// - module_types.go - Core types, interfaces, and message definitions
// - module_compat.go - Backward compatibility wrappers for old APIs
// - module_lifecycle.go - Module creation and shutdown management
// - module_handlers.go - Handler registration and management
// - module_processing.go - Message listening and processing loop
// - module_communication.go - External communication methods
//
// All functionality is preserved with the same public APIs for backward compatibility.
package plugin
