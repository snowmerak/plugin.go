# Plugin.go Examples

This directory contains practical working examples demonstrating the `plugin.go` library usage.

## 📁 Structure

- `simple.go` - Basic Echo plugin usage example
- `host/` - Host application that loads and invokes multiple plugins
- `plugins/` - Various plugin implementation examples
  - `echo/` - Simple echo plugin
  - `calculator/` - JSON-based calculator plugin
  - `sleeper/` - Sleep operation plugin with timeout handling

## 🚀 Quick Start

1. **Build all plugins:**
   ```bash
   cd example
   go build -o plugins/echo/echo ./plugins/echo
   go build -o plugins/calculator/calculator ./plugins/calculator
   go build -o plugins/sleeper/sleeper ./plugins/sleeper
   ```

2. **Run simple example:**
   ```bash
   go run simple.go
   ```

3. **Run comprehensive host application:**
   ```bash
   go run ./host
   ```

## 📖 Example Descriptions

### `simple.go`
- **Purpose**: Most basic plugin usage demonstration
- **Features**: 
  - Loads and invokes Echo plugin
  - Demonstrates JSON adapter usage
  - Shows basic plugin lifecycle management
- **Use Case**: Getting started with plugin.go

### Echo Plugin (`plugins/echo/`)
- **Purpose**: Simple message echoing service
- **Features**:
  - Returns input message with "Echo: " prefix
  - JSON-based communication
  - Demonstrates basic plugin development structure
- **API**: 
  - Input: `{"message": "hello"}`
  - Output: `{"echo": "Echo: hello"}`

### Calculator Plugin (`plugins/calculator/`)
- **Purpose**: Mathematical operations service
- **Features**:
  - Supports basic math operations (add, subtract, multiply, divide)
  - JSON-based request/response
  - Comprehensive error handling
- **API**:
  - Input: `{"operation": "add", "a": 5, "b": 3}`
  - Output: `{"result": 8}`
  - Operations: `add`, `subtract`, `multiply`, `divide`

### Sleeper Plugin (`plugins/sleeper/`)
- **Purpose**: Demonstrates long-running operations and timeout handling
- **Features**:
  - Simulates work by sleeping for specified duration
  - Shows context cancellation support
  - Timeout and graceful shutdown handling
- **API**:
  - Input: `{"message": "work", "sleep_time": 2}`
  - Output: `{"message": "work completed", "slept_time": 2}`

### Host Application (`host/`)
- **Purpose**: Real-world application scenario demonstration
- **Features**:
  - Loads and manages multiple plugins simultaneously
  - Demonstrates plugin independence and isolation
  - Shows proper resource management and cleanup
  - Includes force shutdown and timeout scenarios
- **Components**:
  - `main.go` - Main application entry point
  - `echo.go` - Echo plugin integration
  - `calculator.go` - Calculator plugin integration
  - `sleeper.go` - Sleeper plugin integration
  - `force_shutdown.go` - Shutdown handling demonstration

## 🔧 Building and Running

### Prerequisites
- Go 1.19 or later
- Unix-like environment (Linux, macOS)

### Build Commands
```bash
# Build individual plugins
go build -o plugins/echo/echo ./plugins/echo
go build -o plugins/calculator/calculator ./plugins/calculator
go build -o plugins/sleeper/sleeper ./plugins/sleeper

# Or build all at once
make build-examples  # if Makefile exists
```

### Running Examples
```bash
# Simple echo test
go run simple.go

# Comprehensive test suite
go run ./host

# Individual plugin tests (if needed)
./plugins/echo/echo
./plugins/calculator/calculator
./plugins/sleeper/sleeper
```

## 🎯 Learning Path

1. **Start with `simple.go`** - Understand basic plugin loading and invocation
2. **Examine plugin implementations** - See how plugins are structured
3. **Run host application** - Experience multi-plugin scenarios
4. **Modify examples** - Experiment with your own plugin logic

## 🔍 Key Concepts Demonstrated

- **Plugin Lifecycle**: Loading, invoking, and cleanup
- **JSON Communication**: Standardized request/response format
- **Context Management**: Timeout and cancellation handling
- **Error Handling**: Graceful error propagation and recovery
- **Concurrency**: Multiple plugins running independently
- **Resource Management**: Proper cleanup and shutdown procedures
