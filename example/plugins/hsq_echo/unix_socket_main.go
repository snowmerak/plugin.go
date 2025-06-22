package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

func echoFunction(input []byte) ([]byte, bool) {
	// Unix socket 통신을 통해 받은 메시지를 그대로 반환
	response := fmt.Sprintf("Unix Socket Echo: %s", string(input))
	return []byte(response), false
}

func main() {
	// Unix socket 설정 (Host와 동일한 설정 사용)
	socketPath := filepath.Join("/tmp", "plugin_socket_example")
	socketConfig := &plugin.UnixSocketConfig{
		SocketPath: socketPath,
		IsServer:   false, // Module은 클라이언트
	}

	// Unix socket 옵션으로 Module 생성
	socketOptions := plugin.WithUnixSocketModule(socketConfig)
	module := plugin.NewWithOptions(nil, nil, socketOptions)

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
