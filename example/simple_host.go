package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

func main() {
	fmt.Println("=== Simple Host Example (Stdio) ===")

	// 기본 stdio 통신으로 Loader 생성
	loader := plugin.NewLoader("./plugins/hsq_echo/hsq_echo", "hsq_echo", "1.0.0")

	if loader == nil {
		log.Fatal("Failed to create loader")
	}

	// 플러그인 로드
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Println("Loading plugin with stdio communication...")
	if err := loader.Load(ctx); err != nil {
		log.Fatalf("Failed to load plugin: %v", err)
	}

	fmt.Println("Plugin loaded successfully!")

	// Echo 테스트
	fmt.Println("\n=== Testing Echo Function ===")
	testMessages := []string{
		"Hello, World!",
		"This is a test message",
	}

	for i, msg := range testMessages {
		result, err := plugin.Call(ctx, loader, "echo", []byte(msg))
		if err != nil {
			fmt.Printf("Error calling echo with message %d: %v\n", i+1, err)
			continue
		}

		fmt.Printf("Message: %s -> Echo: %s\n", msg, string(result))
	}

	// 플러그인 언로드
	fmt.Println("\nUnloading plugin...")
	if err := loader.Close(); err != nil {
		log.Printf("Error unloading plugin: %v", err)
	} else {
		fmt.Println("Plugin unloaded successfully!")
	}
}
