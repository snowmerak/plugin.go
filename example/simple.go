package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

func main() {
	fmt.Println("=== 간단한 Plugin 테스트 ===")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Echo 플러그인 테스트
	fmt.Println("Echo 플러그인 로드 중...")
	loader := plugin.NewLoader("./plugins/echo/echo", "echo", "v1.0.0")

	if err := loader.Load(ctx); err != nil {
		log.Fatalf("플러그인 로드 실패: %v", err)
	}
	defer func() {
		if err := loader.Close(); err != nil {
			log.Printf("로더 닫기 오류: %v", err)
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

	// 간단한 테스트
	fmt.Println("Echo 서비스 호출 중...")
	req := EchoRequest{Message: "Hello, Plugin!"}
	resp, err := adapter.Call(ctx, "Echo", req)
	if err != nil {
		log.Fatalf("호출 실패: %v", err)
	}

	fmt.Printf("요청: %s\n", req.Message)
	fmt.Printf("응답: %s\n", resp.Echo)
	fmt.Println("테스트 완료!")
}
