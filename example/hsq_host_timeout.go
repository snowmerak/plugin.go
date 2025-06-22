package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

func main() {
	fmt.Println("=== HSQ Host Example (Non-blocking) ===")

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

	// 플러그인 로드 with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fmt.Println("Loading plugin with HSQ communication...")

	// 별도 고루틴에서 로드 시도
	loadChan := make(chan error, 1)
	go func() {
		loadChan <- loader.Load(ctx)
	}()

	// 타임아웃 처리
	select {
	case err := <-loadChan:
		if err != nil {
			log.Fatalf("Failed to load plugin: %v", err)
		}
		fmt.Println("Plugin loaded successfully!")
	case <-time.After(3 * time.Second):
		log.Fatal("Plugin loading timed out after 3 seconds")
	}

	// Echo 테스트
	fmt.Println("\n=== Testing Echo Function ===")
	testMessages := []string{
		"Hello, HSQ!",
		"Testing high-speed queue communication",
	}

	for i, msg := range testMessages {
		callCtx, callCancel := context.WithTimeout(context.Background(), 1*time.Second)
		result, err := plugin.Call(callCtx, loader, "echo", []byte(msg))
		callCancel()

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
