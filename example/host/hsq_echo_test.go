package main

import (
	"context"
	"fmt"
	"log"
	"time"
)

func main() {
	fmt.Println("=== HSQ Echo Plugin Test ===")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := testHSQEchoPlugin(ctx); err != nil {
		log.Fatalf("HSQ Echo plugin test failed: %v", err)
	}

	fmt.Println("HSQ Echo test completed successfully!")
}
