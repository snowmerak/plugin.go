package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

// Calculator 요청/응답 타입 정의
type CalculateRequest struct {
	Operation string  `json:"operation"`
	A         float64 `json:"a"`
	B         float64 `json:"b"`
}

type CalculateResponse struct {
	Result float64 `json:"result"`
	Error  string  `json:"error,omitempty"`
}

// Calculator 핸들러 로직
func handleCalculate(req CalculateRequest) (CalculateResponse, bool) {
	fmt.Fprintf(os.Stderr, "Calculator Plugin: 연산 요청: %s %g %g\n",
		req.Operation, req.A, req.B)

	var result float64
	var errMsg string

	switch req.Operation {
	case "add":
		result = req.A + req.B
	case "subtract":
		result = req.A - req.B
	case "multiply":
		result = req.A * req.B
	case "divide":
		if req.B == 0 {
			errMsg = "0으로 나눌 수 없습니다"
		} else {
			result = req.A / req.B
		}
	default:
		errMsg = fmt.Sprintf("지원되지 않는 연산: %s", req.Operation)
	}

	response := CalculateResponse{
		Result: result,
		Error:  errMsg,
	}

	// 에러가 있으면 애플리케이션 에러로 처리하지 않고 응답에 포함
	// 이는 비즈니스 로직 에러와 시스템 에러를 구분하는 예제
	return response, false // 항상 false (성공) - 에러는 응답 객체에서 처리
}

func main() {
	fmt.Fprintln(os.Stderr, "Calculator Plugin: 시작됨")

	// stdin/stdout을 통해 호스트와 통신하는 모듈 생성
	module := plugin.New(os.Stdin, os.Stdout)

	// JSON 직렬화/역직렬화 함수 정의
	unmarshalReq := func(data []byte) (CalculateRequest, error) {
		var req CalculateRequest
		err := json.Unmarshal(data, &req)
		return req, err
	}

	marshalResp := func(resp CalculateResponse) ([]byte, error) {
		return json.Marshal(resp)
	}

	// 핸들러 어댑터 생성
	calcAdapter := plugin.NewHandlerAdapter[CalculateRequest, CalculateResponse](
		"Calculate",     // 서비스 이름
		unmarshalReq,    // 요청 언마샬링 함수
		marshalResp,     // 응답 마샬링 함수
		handleCalculate, // 실제 핸들러 로직
	)

	// 모듈에 핸들러 등록
	plugin.RegisterHandler(module, "Calculate", calcAdapter.ToPluginHandler())

	// Ready 신호 전송 (로더가 첫 번째 메시지를 기다림)
	if err := module.SendReady(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Calculator Plugin: ready 메시지 전송 실패: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "Calculator Plugin: 요청 대기 중...")

	// 무한 루프로 요청 처리
	if err := module.Listen(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Calculator Plugin: 오류 발생: %v\n", err)
		os.Exit(1)
	}
}
