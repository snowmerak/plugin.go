package main

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/snowmerak/plugin.go/lib/multiplexer"
)

func main() {
	// Example 1: Basic usage with auto-optimization
	fmt.Println("=== Example 1: Basic Usage ===")
	basicExample()

	// Example 2: API optimization 
	fmt.Println("\n=== Example 2: API Optimization ===")
	apiExample()

	// Example 3: File transfer optimization
	fmt.Println("\n=== Example 3: File Transfer Optimization ===")
	fileTransferExample()

	// Example 4: Custom configuration
	fmt.Println("\n=== Example 4: Custom Configuration ===")
	customConfigExample()

	// Example 5: Metrics monitoring
	fmt.Println("\n=== Example 5: Metrics Monitoring ===")
	metricsExample()
}

func basicExample() {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}

	// Create a multiplexer with automatic optimization
	mux := multiplexer.New(reader, writer)
	defer mux.Close()

	ctx := context.Background()

	// Send a message
	message := []byte("Hello, World!")
	err := mux.WriteMessage(ctx, message)
	if err != nil {
		log.Printf("Error writing message: %v", err)
		return
	}

	fmt.Printf("Sent message: %s\n", message)
	fmt.Printf("Data written to buffer: %d bytes\n", writer.Len())
}

func apiExample() {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}

	// Create a multiplexer optimized for API responses (small messages)
	mux := multiplexer.NewForAPI(reader, writer)
	defer mux.Close()

	ctx := context.Background()

	// Simulate JSON API response
	jsonResponse := []byte(`{"status": "success", "data": {"user_id": 123, "name": "John Doe"}}`)
	err := mux.WriteMessageWithSequence(ctx, 1001, jsonResponse)
	if err != nil {
		log.Printf("Error writing API response: %v", err)
		return
	}

	fmt.Printf("API Response sent: %s\n", jsonResponse)
	
	// Show metrics
	metrics := mux.GetMetrics()
	fmt.Printf("Messages written: %d\n", metrics.MessagesWritten)
	fmt.Printf("Fast path writes: %d\n", metrics.FastPathWrites)
}

func fileTransferExample() {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}

	// Create a multiplexer optimized for file transfers (large messages)
	mux := multiplexer.NewForFileTransfer(reader, writer)
	defer mux.Close()

	ctx := context.Background()

	// Simulate large file data
	largeFile := make([]byte, 100*1024) // 100KB file
	for i := range largeFile {
		largeFile[i] = byte(i % 256)
	}

	err := mux.WriteMessage(ctx, largeFile)
	if err != nil {
		log.Printf("Error writing file: %v", err)
		return
	}

	fmt.Printf("File transfer completed: %d bytes\n", len(largeFile))
	
	// Show metrics
	metrics := mux.GetMetrics()
	fmt.Printf("Bytes written: %d\n", metrics.BytesWritten)
	fmt.Printf("Optimized writes: %d\n", metrics.OptimizedWrites)
}

func customConfigExample() {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}

	// Create with custom configuration
	config := multiplexer.Config{
		Threshold:      4 * 1024,      // 4KB threshold
		BufferSize:     32 * 1024,     // 32KB buffer
		MaxMessageSize: 5 * 1024 * 1024, // 5MB max message
		EnableMetrics:  true,
	}

	mux := multiplexer.NewWithConfig(reader, writer, config)
	defer mux.Close()

	ctx := context.Background()

	// Send medium-sized message
	mediumMessage := make([]byte, 8*1024) // 8KB message
	for i := range mediumMessage {
		mediumMessage[i] = byte(i % 256)
	}

	err := mux.WriteMessage(ctx, mediumMessage)
	if err != nil {
		log.Printf("Error writing message: %v", err)
		return
	}

	fmt.Printf("Custom config message sent: %d bytes\n", len(mediumMessage))
	fmt.Printf("Pending messages: %d\n", mux.GetPendingMessageCount())
}

func metricsExample() {
	reader := &bytes.Buffer{}
	writer := &bytes.Buffer{}

	// Use general purpose multiplexer
	mux := multiplexer.NewForStreaming(reader, writer)
	defer mux.Close()

	ctx := context.Background()

	// Send various sized messages
	messages := [][]byte{
		[]byte("Small message"),                    // Small
		make([]byte, 16*1024),                     // Medium (16KB)
		make([]byte, 128*1024),                    // Large (128KB)
	}

	for i, msg := range messages {
		// Fill with test data
		for j := range msg {
			msg[j] = byte(j % 256)
		}

		err := mux.WriteMessageWithSequence(ctx, uint32(i+1), msg)
		if err != nil {
			log.Printf("Error writing message %d: %v", i+1, err)
			continue
		}

		fmt.Printf("Message %d sent: %d bytes\n", i+1, len(msg))
	}

	// Display comprehensive metrics
	metrics := mux.GetMetrics()
	fmt.Printf("\n--- Metrics Summary ---\n")
	fmt.Printf("Total messages written: %d\n", metrics.MessagesWritten)
	fmt.Printf("Total bytes written: %d\n", metrics.BytesWritten)
	fmt.Printf("Fast path writes: %d\n", metrics.FastPathWrites)
	fmt.Printf("Optimized writes: %d\n", metrics.OptimizedWrites)
	fmt.Printf("Buffer pool hits: %d\n", metrics.BufferPoolHits)
	fmt.Printf("Buffer pool misses: %d\n", metrics.BufferPoolMisses)
}
