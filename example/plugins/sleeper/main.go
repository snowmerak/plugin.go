package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
)

// Sleeper 요청/응답 타입 정의
type SleeperRequest struct {
	Message   string `json:"message"`
	SleepTime int    `json:"sleep_time"` // 초 단위
}

type SleeperResponse struct {
	Message   string `json:"message"`
	SleptTime int    `json:"slept_time"`
}

// Sleeper 핸들러 로직 - 지정된 시간만큼 sleep
func handleSleeper(req SleeperRequest) (SleeperResponse, bool) {
	start := time.Now()
	fmt.Fprintf(os.Stderr, "Sleeper Plugin: 🏃‍♂️ 요청 처리 시작 - 메시지: '%s', 대기시간: %d초\n", req.Message, req.SleepTime)
	os.Stderr.Sync() // 즉시 출력

	// 지정된 시간만큼 sleep
	sleepDuration := time.Duration(req.SleepTime) * time.Second

	// 0.5초마다 진행 상황 로그
	for i := 0; i < req.SleepTime*2; i++ {
		time.Sleep(500 * time.Millisecond)
		fmt.Fprintf(os.Stderr, "Sleeper Plugin: ⏳ %s 진행 중... (%.1f초/%.0f초)\n",
			req.Message, float64(i+1)*0.5, sleepDuration.Seconds())
		os.Stderr.Sync() // 각 로그 즉시 출력
	}

	elapsed := int(time.Since(start).Seconds())

	fmt.Fprintf(os.Stderr, "Sleeper Plugin: ✅ 작업 완료 - %s, 실제 대기시간: %d초\n", req.Message, elapsed)
	os.Stderr.Sync() // 즉시 출력

	response := SleeperResponse{
		Message:   "완료: " + req.Message,
		SleptTime: elapsed,
	}

	return response, false // false = 성공
}

func main() {
	fmt.Fprintln(os.Stderr, "Sleeper Plugin: 시작됨")

	// stdin/stdout을 통해 호스트와 통신하는 모듈 생성
	module := plugin.New(os.Stdin, os.Stdout)

	// JSON 직렬화/역직렬화 함수 정의
	unmarshalReq := func(data []byte) (SleeperRequest, error) {
		var req SleeperRequest
		err := json.Unmarshal(data, &req)
		return req, err
	}

	marshalResp := func(resp SleeperResponse) ([]byte, error) {
		return json.Marshal(resp)
	}

	// 핸들러 어댑터 생성
	sleeperAdapter := plugin.NewHandlerAdapter[SleeperRequest, SleeperResponse](
		"Sleep",       // 서비스 이름
		unmarshalReq,  // 요청 언마샬링 함수
		marshalResp,   // 응답 마샬링 함수
		handleSleeper, // 실제 핸들러 로직
	)

	// 모듈에 핸들러 등록
	plugin.RegisterHandler(module, "Sleep", sleeperAdapter.ToPluginHandler())

	// Ready 신호 전송 (로더가 첫 번째 메시지를 기다림)
	if err := module.SendReady(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Sleeper Plugin: ready 메시지 전송 실패: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "Sleeper Plugin: 요청 대기 중...")

	// 무한 루프로 요청 처리
	if err := module.Listen(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Sleeper Plugin: 오류 발생: %v\n", err)
		os.Exit(1)
	}
}
