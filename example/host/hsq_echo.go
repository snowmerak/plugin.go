package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

// testHSQEchoPlugin tests the HSQ Echo plugin functionality using shared memory communication.
func testHSQEchoPlugin(ctx context.Context) error {
	pluginPath := filepath.Join("..", "plugins", "hsq_echo", "hsq_echo")

	// HSQ 설정 - 호스트 모드
	hsqConfig := &plugin.HSQConfig{
		SharedMemoryName: "/plugin_hsq_example",
		RingSize:         256,
		MaxMessageSize:   4096,
		IsHost:           true, // 호스트 모드로 설정
	}

	// HSQ 옵션으로 Loader 생성
	hsqOptions := plugin.WithHSQ(hsqConfig)
	loader := plugin.NewLoaderWithOptions(pluginPath, "hsq_echo", "v1.0.0", hsqOptions)

	if err := loader.Load(ctx); err != nil {
		return fmt.Errorf("failed to load HSQ Echo plugin: %w", err)
	}
	defer func() {
		if err := loader.Close(); err != nil {
			log.Printf("HSQ Echo loader close error: %v", err)
		}
	}()

	fmt.Println("  HSQ Echo plugin loaded successfully!")

	// Raw byte communication test cases
	testCases := []struct {
		name    string
		message string
	}{
		{"Basic HSQ message", "Hello HSQ!"},
		{"Korean message", "안녕하세요, HSQ!"},
		{"Empty message", ""},
		{"Long message", "This is a very long message to test HSQ shared memory communication. " +
			"It should be handled efficiently through the high-speed queue system."},
		{"Special characters", "Hello @#$%^&*() HSQ 测试 🚀"},
	}

	for _, tc := range testCases {
		fmt.Printf("  Test: %s\n", tc.name)

		// HSQ를 통한 Raw byte 통신
		response, err := plugin.Call(ctx, loader, "echo", []byte(tc.message))
		if err != nil {
			fmt.Printf("    Error: %v\n", err)
			continue
		}

		fmt.Printf("    Request: %s\n", tc.message)
		fmt.Printf("    Response: %s\n", string(response))

		// 약간의 지연을 두어 HSQ 통신 안정성 확보
		time.Sleep(100 * time.Millisecond)
	}

	// 성능 테스트
	fmt.Println("  \n=== HSQ Performance Test ===")
	performanceTestMessage := "Performance test message for HSQ communication"
	iterations := 100

	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, err := plugin.Call(ctx, loader, "echo", []byte(performanceTestMessage))
		if err != nil {
			fmt.Printf("    Performance test iteration %d failed: %v\n", i, err)
			break
		}
	}
	duration := time.Since(start)

	fmt.Printf("  Completed %d HSQ echo calls in %v\n", iterations, duration)
	fmt.Printf("  Average call duration: %v\n", duration/time.Duration(iterations))
	fmt.Printf("  Calls per second: %.2f\n", float64(iterations)/duration.Seconds())

	return nil
}
