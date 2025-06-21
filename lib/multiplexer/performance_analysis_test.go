package multiplexer

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"
)

// BenchmarkPerformanceProfile provides comprehensive performance analysis
func BenchmarkPerformanceProfile(b *testing.B) {
	testSizes := []int{
		512,             // 512B
		1 * 1024,        // 1KB
		4 * 1024,        // 4KB
		16 * 1024,       // 16KB
		64 * 1024,       // 64KB
		256 * 1024,      // 256KB
		1024 * 1024,     // 1MB
		4 * 1024 * 1024, // 4MB
	}

	for _, size := range testSizes {
		testData := make([]byte, size)
		for i := range testData {
			testData[i] = byte(i % 256)
		}

		sizeLabel := formatSizeLabel(size)

		// Original Node
		b.Run(fmt.Sprintf("Original_%s", sizeLabel), func(b *testing.B) {
			benchmarkNodeImplementation(b, func() NodeInterface {
				writer := &bytes.Buffer{}
				reader := &bytes.Buffer{}
				return NewNode(reader, writer)
			}, testData)
		})

		// Optimized Node
		b.Run(fmt.Sprintf("Optimized_%s", sizeLabel), func(b *testing.B) {
			benchmarkNodeImplementation(b, func() NodeInterface {
				writer := &bytes.Buffer{}
				reader := &bytes.Buffer{}
				return NewOptimizedNode(reader, writer)
			}, testData)
		})

		// Hybrid Node
		b.Run(fmt.Sprintf("Hybrid_%s", sizeLabel), func(b *testing.B) {
			benchmarkNodeImplementation(b, func() NodeInterface {
				writer := &bytes.Buffer{}
				reader := &bytes.Buffer{}
				return NewHybridNode(reader, writer)
			}, testData)
		})
	}
}

// NodeInterface provides a common interface for all node types
type NodeInterface interface {
	WriteMessageWithSeq(ctx context.Context, seq uint32, data []byte) error
}

// Adapter functions to match interface
func (n *Node) WriteMessageWithSeq(ctx context.Context, seq uint32, data []byte) error {
	return n.WriteMessageWithSequence(ctx, seq, data)
}

func (n *OptimizedNode) WriteMessageWithSeq(ctx context.Context, seq uint32, data []byte) error {
	return n.WriteMessageWithSequenceOptimized(ctx, seq, data)
}

func (n *HybridNode) WriteMessageWithSeq(ctx context.Context, seq uint32, data []byte) error {
	return n.WriteMessageWithSequenceHybrid(ctx, seq, data)
}

func benchmarkNodeImplementation(b *testing.B, nodeFactory func() NodeInterface, testData []byte) {
	ctx := context.Background()

	b.SetBytes(int64(len(testData)))
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		node := nodeFactory()
		if err := node.WriteMessageWithSeq(ctx, uint32(i+1), testData); err != nil {
			b.Fatal(err)
		}
	}
}

func formatSizeLabel(size int) string {
	switch {
	case size < 1024:
		return fmt.Sprintf("%dB", size)
	case size < 1024*1024:
		return fmt.Sprintf("%dKB", size/1024)
	default:
		return fmt.Sprintf("%dMB", size/(1024*1024))
	}
}

// BenchmarkMemoryUsageProfile analyzes memory usage patterns
func BenchmarkMemoryUsageProfile(b *testing.B) {
	testData := make([]byte, 64*1024) // 64KB
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	b.Run("Original_MemProfile", func(b *testing.B) {
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		writer := &bytes.Buffer{}
		reader := &bytes.Buffer{}
		node := NewNode(reader, writer)
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			writer.Reset()
			if err := node.WriteMessageWithSequence(ctx, uint32(i+1), testData); err != nil {
				b.Fatal(err)
			}
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)
		b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/float64(b.N), "bytes/op")
	})

	b.Run("Hybrid_MemProfile", func(b *testing.B) {
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		writer := &bytes.Buffer{}
		reader := &bytes.Buffer{}
		node := NewHybridNode(reader, writer)
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			writer.Reset()
			if err := node.WriteMessageWithSequenceHybrid(ctx, uint32(i+1), testData); err != nil {
				b.Fatal(err)
			}
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)
		b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/float64(b.N), "bytes/op")
	})
}

// BenchmarkThroughputAnalysis measures sustained throughput
func BenchmarkThroughputAnalysis(b *testing.B) {
	sizes := []int{8 * 1024, 32 * 1024, 128 * 1024} // 8KB, 32KB, 128KB

	for _, size := range sizes {
		testData := make([]byte, size)
		for i := range testData {
			testData[i] = byte(i % 256)
		}

		sizeLabel := formatSizeLabel(size)

		b.Run(fmt.Sprintf("Throughput_Original_%s", sizeLabel), func(b *testing.B) {
			writer := &bytes.Buffer{}
			reader := &bytes.Buffer{}
			node := NewNode(reader, writer)
			ctx := context.Background()

			b.SetBytes(int64(size))
			b.ResetTimer()

			start := time.Now()
			for i := 0; i < b.N; i++ {
				writer.Reset()
				if err := node.WriteMessageWithSequence(ctx, uint32(i+1), testData); err != nil {
					b.Fatal(err)
				}
			}
			duration := time.Since(start)

			totalBytes := int64(b.N) * int64(size)
			throughputMBps := float64(totalBytes) / duration.Seconds() / (1024 * 1024)
			b.ReportMetric(throughputMBps, "MB/s")
		})

		b.Run(fmt.Sprintf("Throughput_Hybrid_%s", sizeLabel), func(b *testing.B) {
			writer := &bytes.Buffer{}
			reader := &bytes.Buffer{}
			node := NewHybridNode(reader, writer)
			ctx := context.Background()

			b.SetBytes(int64(size))
			b.ResetTimer()

			start := time.Now()
			for i := 0; i < b.N; i++ {
				writer.Reset()
				if err := node.WriteMessageWithSequenceHybrid(ctx, uint32(i+1), testData); err != nil {
					b.Fatal(err)
				}
			}
			duration := time.Since(start)

			totalBytes := int64(b.N) * int64(size)
			throughputMBps := float64(totalBytes) / duration.Seconds() / (1024 * 1024)
			b.ReportMetric(throughputMBps, "MB/s")
		})
	}
}

// BenchmarkConcurrencyStress tests performance under concurrent load
func BenchmarkConcurrencyStress(b *testing.B) {
	testData := make([]byte, 16*1024) // 16KB
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	b.Run("Concurrency_Original", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			writer := &bytes.Buffer{}
			reader := &bytes.Buffer{}
			node := NewNode(reader, writer)
			ctx := context.Background()

			seq := uint32(0)
			for pb.Next() {
				writer.Reset()
				seq++
				if err := node.WriteMessageWithSequence(ctx, seq, testData); err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	b.Run("Concurrency_Hybrid", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			writer := &bytes.Buffer{}
			reader := &bytes.Buffer{}
			node := NewHybridNode(reader, writer)
			ctx := context.Background()

			seq := uint32(0)
			for pb.Next() {
				writer.Reset()
				seq++
				if err := node.WriteMessageWithSequenceHybrid(ctx, seq, testData); err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

// BenchmarkLatencyDistribution measures latency characteristics
func BenchmarkLatencyDistribution(b *testing.B) {
	testData := make([]byte, 32*1024) // 32KB
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	b.Run("Latency_Original", func(b *testing.B) {
		writer := &bytes.Buffer{}
		reader := &bytes.Buffer{}
		node := NewNode(reader, writer)
		ctx := context.Background()

		latencies := make([]time.Duration, b.N)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			writer.Reset()
			start := time.Now()
			if err := node.WriteMessageWithSequence(ctx, uint32(i+1), testData); err != nil {
				b.Fatal(err)
			}
			latencies[i] = time.Since(start)
		}

		// Calculate percentiles
		if b.N > 0 {
			// Simple percentile calculation for demonstration
			total := time.Duration(0)
			for _, lat := range latencies {
				total += lat
			}
			avgLatency := total / time.Duration(b.N)
			b.ReportMetric(float64(avgLatency.Nanoseconds()), "avg_ns")
		}
	})

	b.Run("Latency_Hybrid", func(b *testing.B) {
		writer := &bytes.Buffer{}
		reader := &bytes.Buffer{}
		node := NewHybridNode(reader, writer)
		ctx := context.Background()

		latencies := make([]time.Duration, b.N)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			writer.Reset()
			start := time.Now()
			if err := node.WriteMessageWithSequenceHybrid(ctx, uint32(i+1), testData); err != nil {
				b.Fatal(err)
			}
			latencies[i] = time.Since(start)
		}

		if b.N > 0 {
			total := time.Duration(0)
			for _, lat := range latencies {
				total += lat
			}
			avgLatency := total / time.Duration(b.N)
			b.ReportMetric(float64(avgLatency.Nanoseconds()), "avg_ns")
		}
	})
}
