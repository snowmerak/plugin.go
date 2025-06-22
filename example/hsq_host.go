package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

func main() {
	fmt.Println("=== HSQ Host Example ===")

	// HSQ 설정
	hsqConfig := &plugin.HSQConfig{
		SharedMemoryName: "/plugin_hsq_example",
		RingSize:         256,
		MaxMessageSize:   4096,
		IsHost:           true,
	}

	// HSQ 옵션으로 Loader 생성
	hsqOptions := plugin.WithHSQ(hsqConfig)
	loader := plugin.NewLoaderWithOptions("./plugins/hsq_echo/hsq_echo", "hsq_echo", "1.0.0", hsqOptions)

	if loader == nil {
		log.Fatal("Failed to create HSQ loader")
	}

	// 플러그인 로드
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Println("Loading plugin with HSQ communication...")
	if err := loader.Load(ctx); err != nil {
		log.Fatalf("Failed to load plugin: %v", err)
	}

	fmt.Println("Plugin loaded successfully!")

	// Echo 테스트
	fmt.Println("\n=== Testing Echo Function ===")
	testMessages := []string{
		"Hello, HSQ!",
		"This is a test message",
		"Testing high-speed queue communication",
		"한글 메시지도 테스트",
	}
	for i, msg := range testMessages {
		result, err := plugin.Call(ctx, loader, "echo", []byte(msg))
		if err != nil {
			fmt.Printf("Error calling echo with message %d: %v\n", i+1, err)
			continue
		}

		fmt.Printf("Message: %s -> Echo: %s\n", msg, string(result))
	}

	// 성능 테스트
	fmt.Println("\n=== Performance Test ===")
	startTime := time.Now()
	iterations := 1000

	for i := 0; i < iterations; i++ {
		msg := fmt.Sprintf("Performance test message %d", i)
		_, err := plugin.Call(ctx, loader, "echo", []byte(msg))
		if err != nil {
			fmt.Printf("Error in performance test iteration %d: %v\n", i, err)
			break
		}
	}

	elapsed := time.Since(startTime)
	fmt.Printf("Performance test completed: %d calls in %v\n", iterations, elapsed)
	fmt.Printf("Average per call: %v\n", elapsed/time.Duration(iterations))

	// 플러그인 언로드
	fmt.Println("\nUnloading plugin...")
	if err := loader.Close(); err != nil {
		log.Printf("Error unloading plugin: %v", err)
	} else {
		fmt.Println("Plugin unloaded successfully!")
	}
}
