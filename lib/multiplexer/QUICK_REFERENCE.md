# Multiplexer API Quick Reference

## Quick Start

```go
import "github.com/snowmerak/plugin.go/lib/multiplexer"

// Basic usage with auto-optimization
mux := multiplexer.New(reader, writer)
defer mux.Close()

// Send message
err := mux.WriteMessage(ctx, data)

// Read messages
msgChan, err := mux.ReadMessage(ctx)
for msg := range msgChan {
    // Process msg.Data, msg.Sequence
}

// Get metrics
metrics := mux.GetMetrics()
```

## Factory Functions

| Function | Use Case | Optimization |
|----------|----------|--------------|
| `New(r, w)` | General purpose | Auto-adaptive (Hybrid) |
| `NewForAPI(r, w)` | API responses | Small messages (<8KB) |
| `NewForFileTransfer(r, w)` | File uploads/downloads | Large messages (>8KB) |
| `NewForStreaming(r, w)` | Media streaming | Mixed message sizes |
| `NewForIoT(r, w)` | IoT/sensor data | Small, frequent messages |
| `NewForMemoryConstrained(r, w)` | Embedded systems | Memory efficiency |

## Configuration

```go
config := multiplexer.Config{
    Threshold:      8 * 1024,   // Switch threshold (8KB)
    BufferSize:     64 * 1024,  // Buffer size (64KB)
    MaxMessageSize: 10 * 1024 * 1024, // Max message (10MB)
    EnableMetrics:  true,       // Enable metrics tracking
}
mux := multiplexer.NewWithConfig(reader, writer, config)
```

## Interface Methods

```go
type Multiplexer interface {
    // Send message with auto sequence
    WriteMessage(ctx context.Context, data []byte) error
    
    // Send message with specific sequence
    WriteMessageWithSequence(ctx context.Context, seq uint32, data []byte) error
    
    // Read messages (returns channel)
    ReadMessage(ctx context.Context) (chan *APIMessage, error)
    
    // Clean shutdown
    Close() error
    
    // Get pending message count
    GetPendingMessageCount() int
    
    // Get performance metrics
    GetMetrics() *Metrics
}
```

## Performance Guidance

### Small Messages (<8KB)
- **Best**: `NewForAPI()` or `NewForIoT()`
- **Performance**: Up to 9% faster than baseline
- **Use cases**: JSON APIs, sensor data, status updates

### Large Messages (>8KB)
- **Best**: `NewForFileTransfer()`
- **Performance**: Up to 39% faster, 90% memory savings
- **Use cases**: File uploads, image/video data, large payloads

### Mixed Workloads
- **Best**: `New()` (uses Hybrid)
- **Performance**: Automatically adapts to message size
- **Use cases**: General applications, unknown workload patterns

### Memory Constrained
- **Best**: `NewForMemoryConstrained()`
- **Performance**: Optimized for low memory usage
- **Use cases**: Embedded systems, resource-limited environments

## Metrics

```go
type Metrics struct {
    MessagesWritten  uint64  // Total messages sent
    MessagesRead     uint64  // Total messages received
    BytesWritten     uint64  // Total bytes sent
    BytesRead        uint64  // Total bytes received
    BufferPoolHits   uint64  // Buffer reuse count
    BufferPoolMisses uint64  // Buffer allocation count
    ChunksProcessed  uint64  // Chunk processing count
    OptimizedWrites  uint64  // Optimized path usage
    FastPathWrites   uint64  // Fast path usage
}
```

## Error Handling

```go
// Context cancellation
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

err := mux.WriteMessage(ctx, data)
if err != nil {
    if err == context.DeadlineExceeded {
        // Handle timeout
    } else if err == context.Canceled {
        // Handle cancellation
    } else {
        // Handle other errors
    }
}
```

## Migration from Legacy Code

### From Node
```go
// Old
node := multiplexer.NewNode(reader, writer)
err := node.WriteMessageWithSequence(ctx, seq, data)

// New
mux := multiplexer.NewForAPI(reader, writer)
err := mux.WriteMessageWithSequence(ctx, seq, data)
```

### From OptimizedNode
```go
// Old
node := multiplexer.NewOptimizedNode(reader, writer)
err := node.WriteMessageWithSequenceOptimized(ctx, seq, data)

// New
mux := multiplexer.NewForFileTransfer(reader, writer)
err := mux.WriteMessageWithSequence(ctx, seq, data)
```

### From HybridNode
```go
// Old
node := multiplexer.NewHybridNode(reader, writer)
err := node.WriteMessageWithSequenceHybrid(ctx, seq, data)

// New
mux := multiplexer.New(reader, writer)
err := mux.WriteMessageWithSequence(ctx, seq, data)
```
