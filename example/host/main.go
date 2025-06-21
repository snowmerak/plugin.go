package main

import (
	"context"
	"fmt"
	"log"
	"time"
)

func main() {
	fmt.Println("=== Plugin.go Host Application Started ===")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Echo plugin test
	fmt.Println("\n--- Echo Plugin Test ---")
	if err := testEchoPlugin(ctx); err != nil {
		log.Printf("Echo plugin test failed: %v", err)
	}

	// Calculator plugin test
	fmt.Println("\n--- Calculator Plugin Test ---")
	if err := testCalculatorPlugin(ctx); err != nil {
		log.Printf("Calculator plugin test failed: %v", err)
	}

	// Sleeper plugin test (Graceful Shutdown test)
	fmt.Println("\n--- Sleeper Plugin Test (Graceful Shutdown) ---")
	if err := testSleeperPlugin(ctx); err != nil {
		log.Printf("Sleeper plugin test failed: %v", err)
	}

	// Force Shutdown test
	fmt.Println("\n--- Force Shutdown Test ---")
	if err := testForceShutdown(ctx); err != nil {
		log.Printf("Force shutdown test failed: %v", err)
	}

	// Ready Request test
	fmt.Println("\n--- Ready Request Test ---")
	if err := testReadyRequest(ctx); err != nil {
		log.Printf("Ready request test failed: %v", err)
	}

	fmt.Println("\n=== All Tests Completed ===")
}
