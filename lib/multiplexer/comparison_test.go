package multiplexer

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"
)

// Performance comparison between original and optimized implementations
func BenchmarkComparison(b *testing.B) {
	sizes := []int{
		1 * 1024,    // 1KB
		64 * 1024,   // 64KB
		1024 * 1024, // 1MB
	}

	for _, size := range sizes {
		testData := make([]byte, size)
		for i := range testData {
			testData[i] = byte(i % 256)
		}

		b.Run(fmt.Sprintf("Original_%dKB", size/1024), func(b *testing.B) {
			writer := &bytes.Buffer{}
			reader := &bytes.Buffer{}
			node := NewNode(reader, writer)
			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				writer.Reset()
				if err := node.WriteMessageWithSequence(ctx, uint32(i+1), testData); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(fmt.Sprintf("Optimized_%dKB", size/1024), func(b *testing.B) {
			writer := &bytes.Buffer{}
			reader := &bytes.Buffer{}
			node := NewOptimizedNode(reader, writer)
			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				writer.Reset()
				if err := node.WriteMessageWithSequenceOptimized(ctx, uint32(i+1), testData); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(fmt.Sprintf("Hybrid_%dKB", size/1024), func(b *testing.B) {
			writer := &bytes.Buffer{}
			reader := &bytes.Buffer{}
			node := NewHybridNode(reader, writer)
			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				writer.Reset()
				if err := node.WriteMessageWithSequenceHybrid(ctx, uint32(i+1), testData); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Measure memory allocation patterns
func BenchmarkMemoryPatterns(b *testing.B) {
	testData := make([]byte, 64*1024) // 64KB
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	b.Run("Original_Memory", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			writer := &bytes.Buffer{}
			reader := &bytes.Buffer{}
			node := NewNode(reader, writer)
			ctx := context.Background()

			if err := node.WriteMessageWithSequence(ctx, uint32(i+1), testData); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Optimized_Memory", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			writer := &bytes.Buffer{}
			reader := &bytes.Buffer{}
			node := NewOptimizedNode(reader, writer)
			ctx := context.Background()

			if err := node.WriteMessageWithSequenceOptimized(ctx, uint32(i+1), testData); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Test concurrent performance
func BenchmarkConcurrentPerformance(b *testing.B) {
	testData := make([]byte, 8*1024) // 8KB
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	b.Run("Original_Concurrent", func(b *testing.B) {
		b.ReportAllocs()
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

	b.Run("Optimized_Concurrent", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			writer := &bytes.Buffer{}
			reader := &bytes.Buffer{}
			node := NewOptimizedNode(reader, writer)
			ctx := context.Background()

			seq := uint32(0)
			for pb.Next() {
				writer.Reset()
				seq++
				if err := node.WriteMessageWithSequenceOptimized(ctx, seq, testData); err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

// Test throughput under sustained load
func BenchmarkThroughput(b *testing.B) {
	testData := make([]byte, 32*1024) // 32KB
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	b.Run("Original_Throughput", func(b *testing.B) {
		writer := &bytes.Buffer{}
		reader := &bytes.Buffer{}
		node := NewNode(reader, writer)
		ctx := context.Background()

		b.SetBytes(int64(len(testData)))
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			writer.Reset()
			if err := node.WriteMessageWithSequence(ctx, uint32(i+1), testData); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Optimized_Throughput", func(b *testing.B) {
		writer := &bytes.Buffer{}
		reader := &bytes.Buffer{}
		node := NewOptimizedNode(reader, writer)
		ctx := context.Background()

		b.SetBytes(int64(len(testData)))
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			writer.Reset()
			if err := node.WriteMessageWithSequenceOptimized(ctx, uint32(i+1), testData); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Profile garbage collection impact
func BenchmarkGCImpact(b *testing.B) {
	testData := make([]byte, 16*1024) // 16KB
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	b.Run("Original_GC", func(b *testing.B) {
		writer := &bytes.Buffer{}
		reader := &bytes.Buffer{}
		node := NewNode(reader, writer)
		ctx := context.Background()

		b.ResetTimer()
		start := time.Now()

		for i := 0; i < b.N; i++ {
			writer.Reset()
			if err := node.WriteMessageWithSequence(ctx, uint32(i+1), testData); err != nil {
				b.Fatal(err)
			}

			// Force GC periodically to measure impact
			if i%100 == 0 {
				// Allow GC to run naturally
				time.Sleep(1 * time.Microsecond)
			}
		}

		duration := time.Since(start)
		b.ReportMetric(float64(duration.Nanoseconds())/float64(b.N), "ns/op")
	})

	b.Run("Optimized_GC", func(b *testing.B) {
		writer := &bytes.Buffer{}
		reader := &bytes.Buffer{}
		node := NewOptimizedNode(reader, writer)
		ctx := context.Background()

		b.ResetTimer()
		start := time.Now()

		for i := 0; i < b.N; i++ {
			writer.Reset()
			if err := node.WriteMessageWithSequenceOptimized(ctx, uint32(i+1), testData); err != nil {
				b.Fatal(err)
			}

			// Force GC periodically to measure impact
			if i%100 == 0 {
				// Allow GC to run naturally
				time.Sleep(1 * time.Microsecond)
			}
		}

		duration := time.Since(start)
		b.ReportMetric(float64(duration.Nanoseconds())/float64(b.N), "ns/op")
	})
}

// BenchmarkHybridComparison tests the hybrid approach
func BenchmarkHybridComparison(b *testing.B) {
	sizes := []int{
		1 * 1024,    // 1KB
		64 * 1024,   // 64KB
		1024 * 1024, // 1MB
	}

	for _, size := range sizes {
		testData := make([]byte, size)
		for i := range testData {
			testData[i] = byte(i % 256)
		}

		b.Run(fmt.Sprintf("Original_%dKB", size/1024), func(b *testing.B) {
			writer := &bytes.Buffer{}
			reader := &bytes.Buffer{}
			node := NewNode(reader, writer)
			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				writer.Reset()
				if err := node.WriteMessageWithSequence(ctx, uint32(i+1), testData); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(fmt.Sprintf("Optimized_%dKB", size/1024), func(b *testing.B) {
			writer := &bytes.Buffer{}
			reader := &bytes.Buffer{}
			node := NewOptimizedNode(reader, writer)
			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				writer.Reset()
				if err := node.WriteMessageWithSequenceOptimized(ctx, uint32(i+1), testData); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(fmt.Sprintf("Hybrid_%dKB", size/1024), func(b *testing.B) {
			writer := &bytes.Buffer{}
			reader := &bytes.Buffer{}
			node := NewHybridNode(reader, writer)
			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				writer.Reset()
				if err := node.WriteMessageWithSequenceHybrid(ctx, uint32(i+1), testData); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkThresholdTuning tests different threshold values for hybrid approach
func BenchmarkThresholdTuning(b *testing.B) {
	thresholds := []int{
		2 * 1024,  // 2KB
		4 * 1024,  // 4KB (default)
		8 * 1024,  // 8KB
		16 * 1024, // 16KB
	}

	testData := make([]byte, 8*1024) // 8KB test data
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	for _, threshold := range thresholds {
		b.Run(fmt.Sprintf("Threshold_%dKB", threshold/1024), func(b *testing.B) {
			writer := &bytes.Buffer{}
			reader := &bytes.Buffer{}
			node := NewHybridNode(reader, writer)
			node.SetFastPathThreshold(threshold)
			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				writer.Reset()
				if err := node.WriteMessageWithSequenceHybrid(ctx, uint32(i+1), testData); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
