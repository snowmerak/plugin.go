package multiplexer

import (
	"bytes"
	"context"
	"fmt"
	"testing"
)

// BenchmarkOptimizedWriteMessage tests the performance of optimized writing
func BenchmarkOptimizedWriteMessage(b *testing.B) {
	writer := &bytes.Buffer{}
	reader := &bytes.Buffer{}
	node := NewOptimizedNode(reader, writer)
	ctx := context.Background()

	testData := make([]byte, 1024) // 1KB test data
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		writer.Reset()
		if err := node.WriteMessageWithSequenceOptimized(ctx, uint32(i+1), testData); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkOptimizedWriteMessageLarge tests large message performance
func BenchmarkOptimizedWriteMessageLarge(b *testing.B) {
	writer := &bytes.Buffer{}
	reader := &bytes.Buffer{}
	node := NewOptimizedNode(reader, writer)
	ctx := context.Background()

	// Create 1MB test data
	testData := make([]byte, 1024*1024)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		writer.Reset()
		if err := node.WriteMessageWithSequenceOptimized(ctx, uint32(i+1), testData); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAdaptiveChunking tests different chunk sizes
func BenchmarkAdaptiveChunking(b *testing.B) {
	sizes := []int{
		1 * 1024,    // 1KB
		4 * 1024,    // 4KB
		16 * 1024,   // 16KB
		64 * 1024,   // 64KB
		256 * 1024,  // 256KB
		1024 * 1024, // 1MB
	}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%dKB", size/1024), func(b *testing.B) {
			writer := &bytes.Buffer{}
			reader := &bytes.Buffer{}
			node := NewOptimizedNode(reader, writer)
			ctx := context.Background()

			testData := make([]byte, size)
			for i := range testData {
				testData[i] = byte(i % 256)
			}

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
}

// BenchmarkBufferPoolEfficiency tests buffer pool effectiveness
func BenchmarkBufferPoolEfficiency(b *testing.B) {
	testData := make([]byte, 32*1024) // 32KB
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	b.ResetTimer()
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
}
