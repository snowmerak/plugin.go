package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

func main() {
	fmt.Println("=== Plugin.go 호스트 애플리케이션 시작 ===")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Echo 플러그인 테스트
	fmt.Println("\n--- Echo Plugin 테스트 ---")
	if err := testEchoPlugin(ctx); err != nil {
		log.Printf("Echo 플러그인 테스트 실패: %v", err)
	}

	// Calculator 플러그인 테스트
	fmt.Println("\n--- Calculator Plugin 테스트 ---")
	if err := testCalculatorPlugin(ctx); err != nil {
		log.Printf("Calculator 플러그인 테스트 실패: %v", err)
	}

	fmt.Println("\n=== 모든 테스트 완료 ===")
}

// Echo 플러그인 테스트 함수
func testEchoPlugin(ctx context.Context) error {
	pluginPath := filepath.Join("..", "plugins", "echo", "echo")
	loader := plugin.NewLoader(pluginPath, "echo", "v1.0.0")

	if err := loader.Load(ctx); err != nil {
		return fmt.Errorf("Echo 플러그인 로드 실패: %w", err)
	}
	defer func() {
		if err := loader.Close(); err != nil {
			log.Printf("Echo 로더 닫기 오류: %v", err)
		}
	}()

	// JSON 어댑터 생성
	type EchoRequest struct {
		Message string `json:"message"`
	}
	type EchoResponse struct {
		Echo string `json:"echo"`
	}

	adapter := plugin.NewJSONLoaderAdapter[EchoRequest, EchoResponse](loader)

	// 테스트 케이스들
	testCases := []struct {
		name    string
		request EchoRequest
	}{
		{"기본 메시지", EchoRequest{Message: "안녕하세요!"}},
		{"영어 메시지", EchoRequest{Message: "Hello World!"}},
		{"빈 메시지", EchoRequest{Message: ""}},
		{"긴 메시지", EchoRequest{Message: "이것은 매우 긴 메시지입니다. " +
			"플러그인이 긴 메시지도 잘 처리할 수 있는지 테스트해봅시다."}},
	}

	for _, tc := range testCases {
		fmt.Printf("  테스트: %s\n", tc.name)
		resp, err := adapter.Call(ctx, "Echo", tc.request)
		if err != nil {
			fmt.Printf("    오류: %v\n", err)
			continue
		}
		fmt.Printf("    요청: %s\n", tc.request.Message)
		fmt.Printf("    응답: %s\n", resp.Echo)
	}

	return nil
}

// Calculator 플러그인 테스트 함수
func testCalculatorPlugin(ctx context.Context) error {
	pluginPath := filepath.Join("..", "plugins", "calculator", "calculator")
	loader := plugin.NewLoader(pluginPath, "calculator", "v1.0.0")

	if err := loader.Load(ctx); err != nil {
		return fmt.Errorf("Calculator 플러그인 로드 실패: %w", err)
	}
	defer func() {
		if err := loader.Close(); err != nil {
			log.Printf("Calculator 로더 닫기 오류: %v", err)
		}
	}()

	// JSON 어댑터 생성
	type CalculateRequest struct {
		Operation string  `json:"operation"`
		A         float64 `json:"a"`
		B         float64 `json:"b"`
	}
	type CalculateResponse struct {
		Result float64 `json:"result"`
		Error  string  `json:"error,omitempty"`
	}

	adapter := plugin.NewJSONLoaderAdapter[CalculateRequest, CalculateResponse](loader)

	// 테스트 케이스들
	testCases := []struct {
		name    string
		request CalculateRequest
	}{
		{"덧셈", CalculateRequest{Operation: "add", A: 10, B: 5}},
		{"뺄셈", CalculateRequest{Operation: "subtract", A: 10, B: 3}},
		{"곱셈", CalculateRequest{Operation: "multiply", A: 7, B: 6}},
		{"나눗셈", CalculateRequest{Operation: "divide", A: 20, B: 4}},
		{"0으로 나누기", CalculateRequest{Operation: "divide", A: 10, B: 0}},
		{"잘못된 연산", CalculateRequest{Operation: "invalid", A: 1, B: 2}},
	}

	for _, tc := range testCases {
		fmt.Printf("  테스트: %s\n", tc.name)
		resp, err := adapter.Call(ctx, "Calculate", tc.request)
		if err != nil {
			fmt.Printf("    플러그인 오류: %v\n", err)
			continue
		}

		if resp.Error != "" {
			fmt.Printf("    계산 오류: %s\n", resp.Error)
		} else {
			fmt.Printf("    %g %s %g = %g\n", tc.request.A, tc.request.Operation, tc.request.B, resp.Result)
		}
	}

	return nil
}
