package multiplexer

import (
	"bytes"
	"context"
	"fmt"
	"testing"
)

// BenchmarkThresholdOptimization tests different threshold values for the hybrid approach
func BenchmarkThresholdOptimization(b *testing.B) {
	testSizes := []int{
		512,   // 512B
		1024,  // 1KB
		2048,  // 2KB
		4096,  // 4KB
		8192,  // 8KB
		16384, // 16KB
		32768, // 32KB
		65536, // 64KB
	}

	thresholds := []int{
		2 * 1024,  // 2KB
		4 * 1024,  // 4KB
		8 * 1024,  // 8KB (current)
		16 * 1024, // 16KB
		32 * 1024, // 32KB
	}

	for _, threshold := range thresholds {
		for _, size := range testSizes {
			testData := make([]byte, size)
			for i := range testData {
				testData[i] = byte(i % 256)
			}

			sizeLabel := formatSizeForThreshold(size)
			thresholdLabel := formatSizeForThreshold(threshold)

			b.Run(fmt.Sprintf("Threshold_%s_Message_%s", thresholdLabel, sizeLabel), func(b *testing.B) {
				writer := &bytes.Buffer{}
				reader := &bytes.Buffer{}
				node := NewHybridNodeWithThreshold(reader, writer, threshold)
				ctx := context.Background()

				b.SetBytes(int64(len(testData)))
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
}

func formatSizeForThreshold(size int) string {
	switch {
	case size < 1024:
		return fmt.Sprintf("%dB", size)
	case size < 1024*1024:
		return fmt.Sprintf("%dKB", size/1024)
	default:
		return fmt.Sprintf("%dMB", size/(1024*1024))
	}
}

// BenchmarkZeroOverheadComparison tests if we can eliminate the threshold check overhead
func BenchmarkZeroOverheadComparison(b *testing.B) {
	testData := make([]byte, 1024) // 1KB
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	b.Run("Original_FastPath", func(b *testing.B) {
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

	b.Run("Hybrid_WithCheck", func(b *testing.B) {
		writer := &bytes.Buffer{}
		reader := &bytes.Buffer{}
		node := NewHybridNode(reader, writer)
		ctx := context.Background()

		b.SetBytes(int64(len(testData)))
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			writer.Reset()
			if err := node.WriteMessageWithSequenceHybrid(ctx, uint32(i+1), testData); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("DirectFastPath_NoCheck", func(b *testing.B) {
		writer := &bytes.Buffer{}
		reader := &bytes.Buffer{}
		node := NewNode(reader, writer) // Use original node directly
		ctx := context.Background()

		b.SetBytes(int64(len(testData)))
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			writer.Reset()
			// Directly use the original node to measure pure performance
			if err := node.WriteMessageWithSequence(ctx, uint32(i+1), testData); err != nil {
				b.Fatal(err)
			}
		}
	})
}
