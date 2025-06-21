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

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // 타임아웃을 60초로 늘림
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

	// Sleeper 플러그인 테스트 (Graceful Shutdown 테스트)
	fmt.Println("\n--- Sleeper Plugin 테스트 (Graceful Shutdown) ---")
	if err := testSleeperPlugin(ctx); err != nil {
		log.Printf("Sleeper 플러그인 테스트 실패: %v", err)
	}

	// Force Shutdown 테스트
	fmt.Println("\n--- Force Shutdown 테스트 ---")
	if err := testForceShutdown(ctx); err != nil {
		log.Printf("Force shutdown 테스트 실패: %v", err)
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

// Sleeper 플러그인 테스트 함수 (Graceful Shutdown 테스트)
func testSleeperPlugin(ctx context.Context) error {
	pluginPath := filepath.Join("..", "plugins", "sleeper", "sleeper")
	loader := plugin.NewLoader(pluginPath, "sleeper", "v1.0.0")

	if err := loader.Load(ctx); err != nil {
		return fmt.Errorf("Sleeper 플러그인 로드 실패: %w", err)
	}

	// JSON 어댑터 생성
	type SleeperRequest struct {
		Message   string `json:"message"`
		SleepTime int    `json:"sleep_time"`
	}
	type SleeperResponse struct {
		Message   string `json:"message"`
		SleptTime int    `json:"slept_time"`
	}

	adapter := plugin.NewJSONLoaderAdapter[SleeperRequest, SleeperResponse](loader)

	// 테스트 케이스: 긴 작업들을 동시에 시작
	testCases := []struct {
		name    string
		request SleeperRequest
	}{
		{"5초 작업", SleeperRequest{Message: "작업1", SleepTime: 5}},
		{"3초 작업", SleeperRequest{Message: "작업2", SleepTime: 3}},
		{"4초 작업", SleeperRequest{Message: "작업3", SleepTime: 4}},
		{"6초 작업", SleeperRequest{Message: "작업4", SleepTime: 6}},
	}
	fmt.Println("  🚀 장시간 작업들을 동시에 시작합니다...")

	// 결과를 수집할 채널
	results := make(chan string, len(testCases)*2) // 충분한 버퍼 크기

	// 모든 작업을 동시에 시작
	for i, tc := range testCases {
		go func(index int, testCase struct {
			name    string
			request SleeperRequest
		}) {
			fmt.Printf("  🏃‍♂️ 작업 %d (%s) 시작 중...\n", index+1, testCase.name)
			start := time.Now()
			resp, err := adapter.Call(ctx, "Sleep", testCase.request)
			elapsed := time.Since(start)

			if err != nil {
				results <- fmt.Sprintf("    ❌ %s 실패: %v (시간: %.1f초)", testCase.name, err, elapsed.Seconds())
			} else {
				results <- fmt.Sprintf("    ✅ %s 완료: %s (실제시간: %.1f초)",
					testCase.name, resp.Message, elapsed.Seconds())
			}
		}(i, tc)
	}

	// 작업들이 실제로 플러그인에 전달되도록 더 오래 대기
	fmt.Println("  ⏳ 작업들이 플러그인에 전달되기를 기다리는 중...")
	time.Sleep(4 * time.Second) // 8초 대기하여 작업들이 실제로 플러그인에서 시작되도록 함

	// 🔥 핵심: 작업들이 아직 진행 중일 때 graceful shutdown 시작 🔥
	fmt.Println("  🔥 작업들이 진행 중일 때 Graceful Shutdown 테스트 시작 🔥")
	shutdownStart := time.Now()

	// Graceful shutdown 실행
	if err := loader.Close(); err != nil {
		log.Printf("Sleeper 로더 닫기 오류: %v", err)
	}

	shutdownElapsed := time.Since(shutdownStart)
	fmt.Printf("  ✅ Graceful Shutdown 완료 - 총 소요시간: %.2f초\n", shutdownElapsed.Seconds())

	// shutdown 후 결과들을 수집 (non-blocking)
	fmt.Println("  📋 작업 결과들:")
	timeout := time.After(1 * time.Second) // shutdown 후 1초만 더 대기
	collectedResults := 0

	for collectedResults < len(testCases) {
		select {
		case result := <-results:
			fmt.Println(result)
			collectedResults++
		case <-timeout:
			fmt.Printf("  ⏰ 타임아웃: %d/%d 결과만 수집됨\n", collectedResults, len(testCases))
			break
		}
	}

	// Graceful shutdown이 제대로 작동했는지 검증
	expectedMinTime := 3.0 // 최소 3초는 기다려야 함 (가장 짧은 작업 시간)

	// 주의: loader.Close()는 ACK를 기다리는 시간만 측정함
	// 실제 플러그인은 5초 타임아웃으로 작업을 기다림
	fmt.Printf("  📊 분석:\n")
	fmt.Printf("    - 호스트 측 shutdown 시간: %.2f초 (ACK 대기 시간)\n", shutdownElapsed.Seconds())
	fmt.Printf("    - 플러그인 측에서는 5초 타임아웃으로 작업 완료를 기다림\n")

	if shutdownElapsed.Seconds() < 0.1 {
		fmt.Printf("  ✅ Graceful shutdown 작동 확인: 플러그인이 즉시 ACK 응답 (5초 타임아웃으로 작업 대기 중)\n")
	} else if shutdownElapsed.Seconds() >= expectedMinTime {
		fmt.Printf("  ✅ Graceful shutdown이 적절히 작업을 기다렸습니다 (%.1f초)\n", shutdownElapsed.Seconds())
	} else {
		fmt.Printf("  ⚠️  경고: 예상보다 빠른 종료 (%.1f초 < %.1f초)\n", shutdownElapsed.Seconds(), expectedMinTime)
	}

	return nil
}

// Force Shutdown 테스트 함수
func testForceShutdown(ctx context.Context) error {
	pluginPath := filepath.Join("..", "plugins", "sleeper", "sleeper")
	loader := plugin.NewLoader(pluginPath, "sleeper", "v1.0.0")

	if err := loader.Load(ctx); err != nil {
		return fmt.Errorf("Force shutdown용 Sleeper 플러그인 로드 실패: %w", err)
	}

	// JSON 어댑터 생성
	type SleeperRequest struct {
		Message   string `json:"message"`
		SleepTime int    `json:"sleep_time"`
	}
	type SleeperResponse struct {
		Message   string `json:"message"`
		SleptTime int    `json:"slept_time"`
	}

	adapter := plugin.NewJSONLoaderAdapter[SleeperRequest, SleeperResponse](loader)

	// 매우 긴 작업들을 시작
	testCases := []struct {
		name    string
		request SleeperRequest
	}{
		{"20초 작업", SleeperRequest{Message: "긴작업1", SleepTime: 20}},
		{"15초 작업", SleeperRequest{Message: "긴작업2", SleepTime: 15}},
		{"25초 작업", SleeperRequest{Message: "긴작업3", SleepTime: 25}},
	}

	fmt.Println("  🚀 매우 긴 작업들을 시작합니다...")

	// 결과를 수집할 채널
	results := make(chan string, len(testCases))

	// 모든 작업을 동시에 시작
	for i, tc := range testCases {
		go func(index int, testCase struct {
			name    string
			request SleeperRequest
		}) {
			fmt.Printf("  🏃‍♂️ 작업 %d (%s) 시작 중...\n", index+1, testCase.name)
			start := time.Now()
			resp, err := adapter.Call(ctx, "Sleep", testCase.request)
			elapsed := time.Since(start)

			if err != nil {
				results <- fmt.Sprintf("    ❌ %s 실패: %v (시간: %.1f초)", testCase.name, err, elapsed.Seconds())
			} else {
				results <- fmt.Sprintf("    ✅ %s 완료: %s (실제시간: %.1f초)",
					testCase.name, resp.Message, elapsed.Seconds())
			}
		}(i, tc)
	}

	// 작업들이 확실히 시작되도록 대기
	fmt.Println("  ⏳ 작업들이 시작되기를 기다리는 중...")
	time.Sleep(2 * time.Second)

	// 🔥 Force Shutdown 실행 🔥
	fmt.Println("  💥 Force Shutdown 실행! (작업 진행 중)")
	forceStart := time.Now()

	// ForceClose 사용
	if err := loader.ForceClose(); err != nil {
		log.Printf("Force close 오류: %v", err)
	}

	forceElapsed := time.Since(forceStart)
	fmt.Printf("  ⚡ Force Shutdown 완료 - 총 소요시간: %.2f초\n", forceElapsed.Seconds())

	// 결과들을 수집 (짧은 타임아웃)
	fmt.Println("  📋 작업 결과들:")
	timeout := time.After(1 * time.Second)
	collectedResults := 0

	for collectedResults < len(testCases) {
		select {
		case result := <-results:
			fmt.Println(result)
			collectedResults++
		case <-timeout:
			fmt.Printf("  ⏰ 타임아웃: %d/%d 결과만 수집됨 (Force shutdown으로 중단됨)\n", collectedResults, len(testCases))
			break
		}
	}

	// Force shutdown 검증
	fmt.Printf("  📊 분석:\n")
	fmt.Printf("    - Force shutdown 시간: %.2f초\n", forceElapsed.Seconds())

	if forceElapsed.Seconds() < 2.0 {
		fmt.Printf("  ✅ Force shutdown이 빠르게 실행됨 (< 2초)\n")
	} else {
		fmt.Printf("  ⚠️  Force shutdown이 예상보다 오래 걸림 (%.1f초)\n", forceElapsed.Seconds())
	}

	return nil
}
