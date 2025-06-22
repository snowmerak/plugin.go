package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

func echoFunction(input []byte) ([]byte, bool) {
	// HSQ 통신을 통해 받은 메시지를 그대로 반환
	response := fmt.Sprintf("HSQ Echo: %s", string(input))
	return []byte(response), false
}

func main() {
	fmt.Println("=== HSQ Echo Plugin Module ===")

	// HSQ 설정 (Host와 동일한 설정 사용)
	hsqConfig := &plugin.HSQConfig{
		SharedMemoryName: "/plugin_hsq_example",
		RingSize:         256,
		MaxMessageSize:   4096,
		IsHost:           false, // Plugin 모듈이므로 false
	}

	// HSQ 옵션으로 Module 생성
	module, err := plugin.NewFromHSQ(hsqConfig)
	if err != nil {
		log.Fatalf("Failed to create HSQ module: %v", err)
	}

	// Echo 함수 등록
	plugin.RegisterHandler(module, "echo", echoFunction)

	fmt.Println("HSQ module initialized with echo function")
	fmt.Println("Waiting for HSQ communication...")

	// Context 생성
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 종료 신호 처리
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived shutdown signal, closing module...")
		cancel()
	}()

	// 모듈 실행
	if err := module.Listen(ctx); err != nil {
		log.Fatalf("Module listen failed: %v", err)
	}

	fmt.Println("HSQ Echo module shutdown complete")
}
