package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

func main() {
	fmt.Println("=== HSQ Host Example (Small Size) ===")

	// HSQ 설정 - 더 작은 크기로 시도
	hsqConfig := &plugin.HSQConfig{
		SharedMemoryName: "/plugin_hsq_small",
		RingSize:         16,   // 256 -> 16
		MaxMessageSize:   1024, // 4096 -> 1024
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

	fmt.Println("Loading plugin with HSQ communication (small size)...")

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
	msg := "Hello, Small HSQ!"

	callCtx, callCancel := context.WithTimeout(context.Background(), 1*time.Second)
	result, err := plugin.Call(callCtx, loader, "echo", []byte(msg))
	callCancel()

	if err != nil {
		fmt.Printf("Error calling echo: %v\n", err)
	} else {
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
