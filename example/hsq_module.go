package main

import (
	"fmt"
	"log"
	"os"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

func echoFunction(input string) (string, error) {
	// HSQ 통신을 통해 받은 메시지를 그대로 반환
	return fmt.Sprintf("HSQ Echo: %s", input), nil
}

func main() {
	fmt.Println("=== HSQ Module Example ===")

	// HSQ 설정 (Host와 동일한 설정 사용)
	hsqConfig := &plugin.HSQConfig{
		SharedMemoryName: "/plugin_hsq_example",
		RingSize:         256,
		MaxMessageSize:   4096,
		IsHost:           false, // Module은 Host가 아님
	}

	// HSQ 옵션으로 Module 생성
	hsqOptions := plugin.WithHSQModule(hsqConfig)
	module, err := plugin.NewWithOptions("hsq_echo", "1.0.0", hsqOptions)
	if err != nil {
		log.Fatalf("Failed to create HSQ module: %v", err)
	}

	// Echo 함수 등록
	if err := module.RegisterFunction("echo", echoFunction); err != nil {
		log.Fatalf("Failed to register echo function: %v", err)
	}

	fmt.Println("HSQ module initialized with echo function")
	fmt.Println("Waiting for HSQ communication...")

	// 모듈 실행
	if err := module.Run(); err != nil {
		log.Fatalf("Module execution failed: %v", err)
	}

	fmt.Println("HSQ module execution completed")
	os.Exit(0)
}
