# Multiplexer Package

High-performance message multiplexing library

## 🚀 Quick Start

```go
package main

import (
    "context"
    "bytes"
    "github.com/snowmerak/plugin.go/lib/multiplexer"
)

func main() {
    // Auto-optimization mode (recommended)
    reader := &bytes.Buffer{}
    writer := &bytes.Buffer{}
    
    mux := multiplexer.New(reader, writer)
    defer mux.Close()
    
    // Send message
    ctx := context.Background()
    data := []byte("Hello, World!")
    err := mux.WriteMessage(ctx, 1, data)
    if err != nil {
        panic(err)
    }
    
    // Receive message
    message, err := mux.ReadMessage(ctx)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Received: %s\n", message.Data)
}
```

## 📊 Performance Characteristics

| Message Size | Performance Improvement | Memory Savings | Recommended Use |
|------------|----------|-------------|-----------|
| < 8KB | Baseline | - | API responses, IoT data |
| 8KB - 64KB | +9.3% | 59% | File chunks, images |
| > 64KB | +39% | 90% | Large files, streaming |

## 🎯 Usage Scenarios

### IoT/Sensor Data
```go
mux := multiplexer.NewOptimized(reader, writer, multiplexer.OptimizeFor.SmallMessages)
```

### File Transfer
```go
mux := multiplexer.NewOptimized(reader, writer, multiplexer.OptimizeFor.LargeMessages)
```

### General Purpose (Auto-optimization)
```go
mux := multiplexer.New(reader, writer) // Recommended
```

## 🔧 Advanced Configuration

```go
// Custom threshold configuration
mux := multiplexer.NewWithConfig(reader, writer, multiplexer.Config{
    Threshold: 16 * 1024, // 16KB
    BufferSize: 64 * 1024, // 64KB
    MaxMessageSize: 10 * 1024 * 1024, // 10MB
})
```

## ⚡ Performance Optimization Tips

1. **Small messages (< 8KB)**: Default mode is optimal
2. **Large messages (> 8KB)**: Automatically optimized
3. **Memory-constrained environments**: Hybrid mode recommended
4. **Throughput priority**: Consider large-only mode

## 📈 Benchmark Results

```bash
# Run benchmark
go test -bench=BenchmarkComparison -benchmem

# Example results:
# BenchmarkComparison/Original_1KB-12     16589593    73.41 ns/op    48 B/op    3 allocs/op
# BenchmarkComparison/Hybrid_64KB-12        667698   1803 ns/op    432 B/op   18 allocs/op  
# BenchmarkComparison/Hybrid_1024KB-12       47683  26139 ns/op   1639 B/op   66 allocs/op
```

## 🛡️ Safety

- ✅ Concurrency Safe (Goroutine-safe)
- ✅ Memory Overflow Prevention (10MB limit)
- ✅ Context Cancellation Support
- ✅ Automatic Resource Cleanup
- ✅ Comprehensive Test Coverage
