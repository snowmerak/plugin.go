package main

import (
	"context"
	"fmt"
	"log"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

func main() {
	fmt.Println("=== 디버깅: 단계별 플러그인 테스트 ===")

	ctx := context.Background()

	// 1. 로더 생성
	fmt.Println("1. 로더 생성 중...")
	loader := plugin.NewLoader("./plugins/echo/echo", "echo", "v1.0.0")

	// 2. 플러그인 로드
	fmt.Println("2. 플러그인 로드 중...")
	if err := loader.Load(ctx); err != nil {
		log.Fatalf("플러그인 로드 실패: %v", err)
	}
	defer func() {
		fmt.Println("5. 로더 종료 중...")
		if err := loader.Close(); err != nil {
			log.Printf("로더 닫기 오류: %v", err)
		}
	}()

	fmt.Println("3. 플러그인 로드 성공!")

	// 3. 간단한 바이트 요청 보내기 (JSON 어댑터 사용하지 않고)
	fmt.Println("4. 직접 Call 함수로 요청 보내기...")
	requestJSON := `{"message":"Hello World"}`

	response, err := plugin.Call(ctx, loader, "Echo", []byte(requestJSON))
	if err != nil {
		log.Fatalf("호출 실패: %v", err)
	}

	fmt.Printf("응답 수신: %s\n", string(response))
	fmt.Println("테스트 완료!")
}
