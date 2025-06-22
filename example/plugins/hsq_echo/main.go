package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

func echoFunction(input []byte) ([]byte, bool) {
	// HSQ 통신을 통해 받은 메시지를 그대로 반환
	response := fmt.Sprintf("HSQ Echo: %s", string(input))
	return []byte(response), false
}

func main() {
	// stdio 통신으로 Module 생성 (HSQ 대신)
	module := plugin.NewStd()

	// Echo 함수 등록
	plugin.RegisterHandler(module, "echo", echoFunction)

	// Context 생성
	ctx := context.Background()

	// 모듈 실행 (Listen 사용)
	if err := module.Listen(ctx); err != nil {
		log.Fatalf("Module listen failed: %v", err)
	}

	os.Exit(0)
}
