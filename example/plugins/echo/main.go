package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

// Echo 요청/응답 타입 정의
type EchoRequest struct {
	Message string `json:"message"`
}

type EchoResponse struct {
	Echo string `json:"echo"`
}

// Echo 핸들러 로직
func handleEcho(req EchoRequest) (EchoResponse, bool) {
	// stderr로 로그 출력 (디버깅용)
	fmt.Fprintf(os.Stderr, "Echo Plugin: 메시지 수신: '%s'\n", req.Message)

	// 간단하게 "Echo: " 접두사를 붙여서 반환
	response := EchoResponse{
		Echo: "Echo: " + req.Message,
	}

	return response, false // false = 성공
}

func main() {
	fmt.Fprintln(os.Stderr, "Echo Plugin: 시작됨")

	// stdin/stdout을 통해 호스트와 통신하는 모듈 생성
	module := plugin.New(os.Stdin, os.Stdout)

	// JSON 직렬화/역직렬화 함수 정의
	unmarshalReq := func(data []byte) (EchoRequest, error) {
		var req EchoRequest
		err := json.Unmarshal(data, &req)
		return req, err
	}

	marshalResp := func(resp EchoResponse) ([]byte, error) {
		return json.Marshal(resp)
	}

	// 핸들러 어댑터 생성
	echoAdapter := plugin.NewHandlerAdapter[EchoRequest, EchoResponse](
		"Echo",       // 서비스 이름
		unmarshalReq, // 요청 언마샬링 함수
		marshalResp,  // 응답 마샬링 함수
		handleEcho,   // 실제 핸들러 로직
	)
	// 모듈에 핸들러 등록
	plugin.RegisterHandler(module, "Echo", echoAdapter.ToPluginHandler())

	// Ready 신호 전송 (로더가 첫 번째 메시지를 기다림)
	if err := module.SendReady(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Echo Plugin: ready 메시지 전송 실패: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "Echo Plugin: 요청 대기 중...")

	// 무한 루프로 요청 처리
	if err := module.Listen(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Echo Plugin: 오류 발생: %v\n", err)
		os.Exit(1)
	}
}
