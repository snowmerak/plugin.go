# plugin.go (한국어)

`plugin.go`는 외부 플러그인 프로세스와의 강력한 통신을 용이하게 하도록 설계된 Go 라이브러리입니다. 이러한 플러그인은 표준 I/O(stdin/stdout)를 통해 사용자 정의 멀티플렉싱 프로토콜을 사용하여 호스트 애플리케이션과 상호 작용하는 별도의 실행 파일입니다. 이를 통해 Go 플러그인 또는 프로토콜을 준수하는 모든 실행 파일을 로드하고 구조화된 방식으로 해당 기능을 호출할 수 있습니다.

이 라이브러리는 다음과 같은 명확한 관심사 분리를 제공합니다:
- 클라이언트 측(호스트 애플리케이션)의 `Loader`는 플러그인 프로세스의 수명 주기를 관리하고 요청을 보냅니다.
- 플러그인 측(외부 실행 파일)의 `Module`은 특정 서비스에 대한 핸들러를 등록하고 들어오는 요청을 처리합니다.
- 제네릭 `Adapter`(`LoaderAdapter`는 클라이언트용, `HandlerAdapter`는 플러그인용)는 JSON 또는 프로토콜 버퍼와 같은 직렬화 형식을 사용하여 타입이 지정된 통신을 가능하게 하여 원시 바이트 처리를 추상화합니다.

## 기능

- **프로세스 관리**: `process` 패키지를 사용하여 외부 실행 파일을 플러그인으로 포크하고 관리합니다.
- **멀티플렉싱 통신**: stdin/stdout을 통해 멀티플렉싱 프로토콜(`multiplexer` 패키지)을 구현하여 호스트와 플러그인 간의 동시 요청/응답 주기를 허용합니다.
- **요청/응답 패턴**: 요청에 고유한 시퀀스 ID를 사용하여 응답과 연관시킵니다.
- **타입이 지정된 데이터 처리**:
    - 클라이언트 측: 타입이 지정된 호출을 위한 `NewJSONLoaderAdapter` 및 `NewProtobufLoaderAdapter`가 있는 `LoaderAdapter`.
    - 플러그인 측: 타입이 지정된 핸들러를 생성하기 위한 `HandlerAdapter`. 그런 다음 `Module`에 등록할 수 있습니다.
- **직렬화 지원**: JSON 및 프로토콜 버퍼에 대한 기본 지원과 사용자 정의 직렬 변환기를 위한 확장성을 제공합니다.
- **정상 종료**: 컨텍스트 기반 취소 및 리소스의 적절한 정리를 위한 메커니즘.
- **오류 전파**: 전송 오류, 플러그인 실행 오류 및 직렬화/역직렬화 오류의 명확한 구분 및 전파.

## 아키텍처 개요

`plugin.go` 라이브러리는 다음과 같은 클라이언트-서버 아키텍처를 구축합니다:
- **호스트 애플리케이션(클라이언트)**는 `plugin.Loader`를 사용하여 외부 플러그인 프로세스를 시작하고 관리합니다.
- **플러그인 프로세스(서버)**는 `plugin.Module`을 사용하여 호스트로부터의 요청을 수신합니다.

**통신 흐름:**
1.  **로딩**: 호스트 애플리케이션은 `plugin.Loader`를 사용하여 플러그인 바이너리를 실행합니다. `Loader`는 플러그인의 stdin 및 stdout을 사용하여 통신 파이프를 설정합니다.
2.  **멀티플렉싱**: 모든 통신은 양쪽의 `multiplexer.Node` 인스턴스를 통과합니다. 이 노드는 메시지를 청크로 나누고, 시퀀스 ID를 할당하며, 단일 stdin/stdout 쌍을 통해 여러 "가상" 채널을 허용하기 위해 저수준 프로토콜을 처리합니다.
3.  **요청 (클라이언트에서 플러그인으로)**:
    *   클라이언트는 `LoaderAdapter`(예: `JSONLoaderAdapter`)를 사용하여 타입이 지정된 호출을 합니다.
    *   어댑터는 요청 객체를 바이트로 직렬화합니다.
    *   `plugin.Header`(서비스 이름, 페이로드 포함)가 마샬링됩니다.
    *   `Loader`는 고유한 요청 ID를 사용하여 `multiplexer.Node`를 통해 이 데이터를 플러그인으로 보냅니다.
4.  **처리 (플러그인 측)**:
    *   플러그인의 `Module`은 `multiplexer.Node`를 통해 메시지를 수신합니다.
    *   요청을 받으면 `plugin.Header`를 언마샬링합니다.
    *   서비스 이름에 대해 등록된 핸들러를 찾습니다.
    *   `HandlerAdapter`(사용된 경우)는 요청 페이로드를 타입이 지정된 객체로 역직렬화합니다.
    *   타입이 지정된 핸들러 함수가 실행됩니다.
    *   `HandlerAdapter`는 타입이 지정된 응답(또는 오류)을 다시 바이트로 직렬화합니다.
5.  **응답 (플러그인에서 클라이언트로)**:
    *   `Module`은 응답에 대한 `plugin.Header`를 마샬링합니다(성공 또는 오류 표시, 응답 페이로드 포함).
    *   이것은 원래 요청 ID를 사용하여 `multiplexer.Node`를 통해 클라이언트로 다시 전송됩니다.
6.  **응답 수신 (클라이언트 측)**:
    *   `Loader`의 메시지 읽기 고루틴이 응답을 수신합니다.
    *   ID를 사용하여 응답을 보류 중인 요청과 연관시킵니다.
    *   `LoaderAdapter`는 응답 페이로드를 예상되는 타입이 지정된 객체로 언마샬링합니다.
    *   결과(또는 오류)가 원래 호출자에게 반환됩니다.

**흐름도:**
```
+---------------------+      StdIn/StdOut      +----------------------|
| 호스트 애플리케이션   |       `process` Pkg    | 플러그인 실행 파일    |
| (Host Application)  |       를 통한 파이프   | (Plugin Executable)  |
|---------------------|      (Pipes via)       |----------------------|
| `plugin.Loader`     |<--------------------->| `plugin.Module`      |
|  `LoaderAdapter`    |                        |  `HandlerAdapter`    |
|   (JSON/Proto)      |                        |   (JSON/Proto)       |
| `multiplexer.Node`  |----멀티플렉싱된 데이터--->| `multiplexer.Node`   |
|                     |<---멀티플렉싱된 데이터----|                      |
+---------------------+                        +----------------------|
```

## 핵심 구성 요소

-   **`lib/process`**:
    *   `Process`: 외부 프로세스를 포크하고 관리하여 stdin, stdout 및 stderr에 대한 액세스를 제공하는 유틸리티입니다.
-   **`lib/multiplexer`**:
    *   `Node`: 핵심 멀티플렉싱 로직을 구현합니다. `io.Reader`에서 읽고 `io.Writer`에 쓰며, 메시지를 타입, ID 및 길이에 대한 헤더가 있는 프레임으로 분할합니다.
    *   `Message`: 멀티플렉서에 의해 교환되는 데이터 단위를 나타냅니다.
-   **`lib/plugin`**:
    *   `Loader`: 플러그인의 수명 주기를 관리합니다. 플러그인 프로세스를 포크하고, 멀티플렉서를 설정하며, 요청을 보내기 위한 `Call` 메서드를 제공합니다.
    *   `Module`: 플러그인 실행 파일 내에서 사용됩니다. 멀티플렉서를 통해 들어오는 요청을 수신하고, 등록된 핸들러에 디스패치하며, 응답을 다시 보냅니다.
    *   `Header`: 서비스 이름, 오류 플래그 및 페이로드를 포함하는 메시지의 메타데이터 부분 구조를 정의합니다. `MarshalBinary` 및 `UnmarshalBinary` 메서드가 있습니다.
    *   `Serializer[Req, Resp]`: `MarshalRequest` 및 `UnmarshalResponse` 함수를 보유하는 구조체입니다. 어댑터에서 사용됩니다.
    *   `LoaderAdapter[Req, Resp]`: 제네릭 클라이언트 측 어댑터입니다. 타입이 지정된 `Call` 메서드를 제공하기 위해 `Loader`와 `Serializer`를 사용합니다.
        *   `NewJSONLoaderAdapter[Req, Resp]`: JSON용으로 미리 구성된 `LoaderAdapter`의 생성자입니다.
        *   `NewProtobufLoaderAdapter[Req, Resp]`: 프로토콜 버퍼용으로 미리 구성된 `LoaderAdapter`의 생성자입니다. 응답 타입에 대한 팩토리 함수가 필요합니다.
    *   `HandlerAdapter[Req, Resp]`: 제네릭 플러그인 측 어댑터입니다. 사용자의 타입이 지정된 핸들러 함수와 언마샬링 및 마샬링 함수를 래핑하여 `Module.RegisterHandler`와 호환되는 원시 핸들러를 생성합니다.
        *   (암시적으로 `NewHandlerAdapter`가 기본이며, JSON/Protobuf 버전은 특정 마샬/언마샬 로직을 제공하여 이를 사용합니다.)
    *   `RegisterHandler(module, name, handlerFunc)`: 원시 바이트 핸들러를 모듈에 등록합니다. 타입이 지정된 핸들러는 일반적으로 먼저 `HandlerAdapter`로 래핑됩니다.

## 통신 프로토콜

`multiplexer.Node`는 I/O 스트림 위에 프로토콜을 구현합니다. 각 논리적 메시지는 프레임화됩니다:

-   **헤더 (총 9바이트)**:
    *   **메시지 타입 (1바이트)**:
        *   `MessageHeaderTypeStart (0x01)`: 새 메시지 시퀀스의 시작을 나타냅니다.
        *   `MessageHeaderTypeData (0x03)`: 메시지 데이터의 청크를 포함합니다.
        *   `MessageHeaderTypeEnd (0x02)`: 메시지 시퀀스의 끝을 나타냅니다.
        *   `MessageHeaderTypeAbort (0x06)`: 메시지 시퀀스를 중단해야 함을 나타냅니다(예: 컨텍스트 취소로 인해).
        *   `MessageHeaderTypeError (0x04)`: 멀티플렉서 자체 내의 오류를 나타냅니다.
        *   `MessageHeaderTypeComplete (0x05)`: 완전히 조립된 메시지를 표시하기 위해 리더가 내부적으로 사용합니다.
    *   **프레임 ID (4바이트)**: 메시지 시퀀스를 식별하는 `uint32` (BigEndian)입니다. 멀티플렉싱을 허용합니다.
    *   **데이터 길이 (4바이트)**: *이 특정 프레임*의 페이로드 길이를 지정하는 `uint32` (BigEndian)입니다 (`MessageHeaderTypeData` 프레임용). `plugin.Header`(서비스 이름 등)는 논리적 애플리케이션 메시지의 첫 번째 `Data` 프레임 페이로드의 일부입니다.

-   **페이로드 (가변 길이)**: 전송되는 실제 데이터입니다. 애플리케이션 메시지의 경우 이 페이로드는 먼저 마샬링된 `plugin.Header`가 되고 그 뒤에 사용자의 요청/응답 데이터가 옵니다.

**메시지 흐름 예제 (요청):**
1.  클라이언트는 `multiplexer.WriteMessageWithSequence`를 사용하는 `loader.Call`을 호출합니다.
2.  멀티플렉서가 다음을 전송합니다:
    *   프레임 1: `Type=Start, ID=N, Length=0`
    *   프레임 2: `Type=Data, ID=N, Length=X, Payload=chunk1` (페이로드에는 마샬링된 `plugin.Header`와 사용자 데이터의 일부가 포함됨)
    *   ... (필요한 경우 더 많은 데이터 프레임)
    *   프레임 N: `Type=End, ID=N, Length=0`

플러그인의 멀티플렉서는 이러한 프레임을 `plugin.Module`을 위한 완전한 메시지로 재조립합니다.

## 설치

```bash
go get github.com/snowmerak/plugin.go
```

## 사용 가이드

### 클라이언트 측 (플러그인 로드 및 호출)

호스트 애플리케이션은 `plugin.Loader`와 `LoaderAdapter`를 사용합니다.

```go
package main

import (
	"context"
	"encoding/json" // 플러그인 측 예제에서 헬퍼를 사용하지 않는 경우
	"fmt"
	"log"
	"os"
	"time"

	"github.com/snowmerak/plugin.go/lib/plugin"
	// 프로토콜 버퍼 예제의 경우 생성된 프로토콜 버퍼 패키지를 가져옵니다:
	// "your_project_path/mypb"
	// "google.golang.org/protobuf/proto"
)

// 클라이언트 및 플러그인용 예제 JSON 타입
type MyJSONRequest struct {
	Data string `json:"data"`
}

type MyJSONResponse struct {
	Result string `json:"result"`
}

// 예제 프로토콜 버퍼 타입 (mypb 패키지에서 생성되었다고 가정)
/*
// mypb/my.proto (예제)
// syntax = "proto3";
// package mypb;
// option go_package = "your_project_path/mypb";
// message MyProtoRequest {
//   string input = 1;
// }
// message MyProtoResponse {
//   string output = 1;
// }

// Go 코드 (protoc에 의해 생성된 mypb/my.pb.go)
// 이 타입들이 proto.Message를 구현하는지 확인
type MyProtoRequest struct {
	// ... 생성된 필드 ...
	Input string
}
// ... MyProtoRequest에 대한 proto.Message 메서드 ...

type MyProtoResponse struct {
	// ... 생성된 필드 ...
	Output string
}
// ... MyProtoResponse에 대한 proto.Message 메서드 ...
// func (m *MyProtoResponse) GetOutput() string { if m != nil { return m.Output } return "" }
*/

func main() {
	// 플러그인 실행 파일이 빌드되었고 경로가 올바른지 확인합니다.
	// 예: go build -o ./myplugin ./path/to/plugin/main.go
	loader := plugin.NewLoader("./myplugin", "myplugin", "v1.0.0")

	// 타임아웃 및 취소를 위해 컨텍스트 사용
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := loader.Load(ctx); err != nil {
		log.Fatalf("플러그인 로드 실패: %v", err)
	}
	defer func() {
		if err := loader.Close(); err != nil {
			log.Printf("로더 닫기 오류: %v", err)
		}
	}()

	// --- JSON 어댑터 사용 ---
	// 참고: NewJSONLoaderAdapter로 수정됨
	jsonAdapter := plugin.NewJSONLoaderAdapter[MyJSONRequest, MyJSONResponse](loader)
	jsonReq := MyJSONRequest{Data: "클라이언트에서 온 JSON 안녕하세요"}

	fmt.Println("클라이언트: HandleJSON 호출 중...")
	jsonResp, err := jsonAdapter.Call(ctx, "HandleJSON", jsonReq)
	if err != nil {
		log.Printf("클라이언트: HandleJSON에 대한 JSON 호출 오류: %v", err)
	} else {
		fmt.Printf("클라이언트: HandleJSON의 JSON 응답: %+v\n", jsonResp)
	}

	// 애플리케이션 오류를 반환할 수 있는 서비스 호출 예제
	fmt.Println("클라이언트: 'error' 데이터로 HandleJSON 호출 중...")
	jsonReqError := MyJSONRequest{Data: "error"}
	_, err = jsonAdapter.Call(ctx, "HandleJSON", jsonReqError)
	if err != nil {
		// 이 오류에는 플러그인의 메시지가 포함됩니다.
		log.Printf("클라이언트: HandleJSON에 대한 예상된 JSON 호출 오류 (data='error'): %v", err)
	} else {
		fmt.Println("클라이언트: HandleJSON (data='error')에 대한 오류가 예상되었지만 아무것도 받지 못했습니다.")
	}


	// --- 프로토콜 버퍼 어댑터 사용 ---
	// (프로토콜 버퍼 정의가 있는 경우 주석 해제 및 수정)
	/*
	// 참고: NewProtobufLoaderAdapter로 수정됨
	protoAdapter := plugin.NewProtobufLoaderAdapter[*mypb.MyProtoRequest, *mypb.MyProtoResponse](
		loader,
		func() *mypb.MyProtoResponse { return new(mypb.MyProtoResponse) }, // 응답 타입용 팩토리
	)
	protoReq := &mypb.MyProtoRequest{Input: "클라이언트에서 온 프로토콜 버퍼 안녕하세요"}

	fmt.Println("클라이언트: HandleProto 호출 중...")
	protoResp, err := protoAdapter.Call(ctx, "HandleProto", protoReq)
	if err != nil {
		log.Printf("클라이언트: HandleProto에 대한 프로토콜 버퍼 호출 오류: %v", err)
	} else {
		fmt.Printf("클라이언트: HandleProto의 프로토콜 버퍼 응답: %s\n", protoResp.GetOutput()) // GetOutput이 있다고 가정
	}
	*/

	// 존재하지 않는 서비스 호출 예제
	fmt.Println("클라이언트: NonExistentService 호출 중...")
	_, err = jsonAdapter.Call(ctx, "NonExistentService", MyJSONRequest{Data: "test"})
	if err != nil {
		log.Printf("클라이언트: NonExistentService에 대한 예상된 오류: %v", err)
	} else {
		fmt.Println("클라이언트: NonExistentService에 대한 오류가 예상되었지만 아무것도 받지 못했습니다.")
	}
}
```

### 플러그인 측 (플러그인 실행 파일 구현)

플러그인 실행 파일은 `plugin.Module`을 사용하고 핸들러를 등록합니다. 타입이 지정된 핸들러는 `plugin.HandlerAdapter`를 사용하여 래핑됩니다.

```go
package main

import (
	"context"
	"encoding/json" // JSON 핸들러 어댑터에 필요
	"fmt"
	"log"
	"os" // os.Stdin, os.Stdout용

	"github.com/snowmerak/plugin.go/lib/plugin"
	// 프로토콜 버퍼 예제의 경우 생성된 프로토콜 버퍼 패키지를 가져옵니다:
	// "your_project_path/mypb"
	// "google.golang.org/protobuf/proto"
)

// --- JSON 핸들러 예제 ---

// MyJSONRequest 및 MyJSONResponse는 클라이언트의 정의와 일치해야 합니다.
type MyJSONRequest struct {
	Data string `json:"data"`
}

type MyJSONResponse struct {
	Result string `json:"result"`
}

// 타입이 지정된 JSON 핸들러 함수
func handleJSONRequestLogic(req MyJSONRequest) (MyJSONResponse, bool) {
	fmt.Fprintf(os.Stderr, "플러그인: HandleJSON에 대한 JSON 요청 수신: %+v\n", req)
	if req.Data == "error" {
		// 애플리케이션 오류 시뮬레이션
		return MyJSONResponse{Result: "플러그인: 시뮬레이션된 JSON 오류 발생"}, true // true는 애플리케이션 오류를 나타냄
	}
	return MyJSONResponse{Result: "플러그인: JSON 데이터 처리됨 - " + req.Data}, false // false는 성공을 나타냄
}

// --- 프로토콜 버퍼 핸들러 예제 ---
/*
// MyProtoRequest 및 MyProtoResponse는 클라이언트의 정의와 일치해야 하며
// proto.Message를 구현해야 합니다.
type MyProtoRequest struct {
	// ... 생성된 필드 ...
	Input string
}
// ... MyProtoRequest에 대한 proto.Message 메서드 ...
// func (m *MyProtoRequest) GetInput() string { if m != nil { return m.Input } return "" }


type MyProtoResponse struct {
	// ... 생성된 필드 ...
	Output string
}
// ... MyProtoResponse에 대한 proto.Message 메서드 ...

// 타입이 지정된 프로토콜 버퍼 핸들러 함수
func handleProtoRequestLogic(req *mypb.MyProtoRequest) (*mypb.MyProtoResponse, bool) {
	fmt.Fprintf(os.Stderr, "플러그인: HandleProto에 대한 프로토콜 버퍼 요청 수신: %s\n", req.GetInput())
	if req.GetInput() == "error" {
		return &mypb.MyProtoResponse{Output: "플러그인: 시뮬레이션된 프로토콜 버퍼 오류"}, true
	}
	return &mypb.MyProtoResponse{Output: "플러그인이 말합니다: 안녕하세요 " + req.GetInput()}, false
}
*/

func main() {
	// 호스트로부터 요청을 읽기 위해 os.Stdin 사용, 호스트로 응답을 보내기 위해 os.Stdout 사용
	module := plugin.New(os.Stdin, os.Stdout)

	// --- JSON 핸들러 등록 ---
	// 1. 요청 타입에 대한 언마샬 함수 정의
	unmarshalJSONReq := func(data []byte) (MyJSONRequest, error) {
		var r MyJSONRequest
		err := json.Unmarshal(data, &r)
		return r, err
	}
	// 2. 응답 타입에 대한 마샬 함수 정의
	marshalJSONResp := func(resp MyJSONResponse) ([]byte, error) {
		return json.Marshal(resp)
	}
	// 3. HandlerAdapter 생성
	jsonHandlerAdapter := plugin.NewHandlerAdapter[MyJSONRequest, MyJSONResponse](
		"HandleJSON",             // 서비스 이름 (어댑터에서 로깅/디버깅용)
		unmarshalJSONReq,         // 요청 바이트를 언마샬하는 함수
		marshalJSONResp,          // 응답 객체를 마샬하는 함수
		handleJSONRequestLogic,   // 실제 타입이 지정된 핸들러 로직
	)
	// 4. 조정된 핸들러를 모듈에 등록
	plugin.RegisterHandler(module, "HandleJSON", jsonHandlerAdapter.ToPluginHandler())


	// --- 프로토콜 버퍼 핸들러 등록 ---
	// (프로토콜 버퍼 정의가 있는 경우 주석 해제 및 수정)
	/*
	// 1. 프로토콜 버퍼 요청 타입에 대한 언마샬 함수 정의
	unmarshalProtoReq := func(data []byte) (*mypb.MyProtoRequest, error) {
		instance := new(mypb.MyProtoRequest) // 또는 Req가 인터페이스/포인터인 경우 팩토리 사용
		err := proto.Unmarshal(data, instance)
		return instance, err
	}
	// 2. 프로토콜 버퍼 응답 타입에 대한 마샬 함수 정의
	marshalProtoResp := func(resp *mypb.MyProtoResponse) ([]byte, error) {
		return proto.Marshal(resp)
	}
	// 3. 프로토콜 버퍼용 HandlerAdapter 생성
	protoHandlerAdapter := plugin.NewHandlerAdapter[*mypb.MyProtoRequest, *mypb.MyProtoResponse](
		"HandleProto",
		unmarshalProtoReq,
		marshalProtoResp,
		handleProtoRequestLogic,
	)
	// 4. 조정된 프로토콜 버퍼 핸들러 등록
	plugin.RegisterHandler(module, "HandleProto", protoHandlerAdapter.ToPluginHandler())
	*/

	fmt.Fprintln(os.Stderr, "플러그인: 시작되었으며 요청 대기 중...")
	// 컨텍스트가 취소될 때까지 또는 무기한으로 요청 수신
	if err := module.Listen(context.Background()); err != nil {
		// 리스너를 중지시키는 치명적인 오류 기록
		log.Fatalf("플러그인: 리스너 오류: %v", err)
	}
	fmt.Fprintln(os.Stderr, "플러그인: 리스너 중지됨.")
}

// 이 플러그인을 빌드하려면:
// go build -o ./myplugin ./path/to/plugin/main.go
```

## 오류 처리

-   **전송 오류**: 호스트와 플러그인 간의 연결이 끊어지면(예: 플러그인 충돌, 파이프 닫힘), `loader.Call` 또는 `module.Listen`이 오류를 반환합니다.
-   **직렬화/역직렬화 오류**:
    *   클라이언트 측에서 `LoaderAdapter.Call`은 요청 마샬링 또는 응답 언마샬링이 실패하면 오류를 반환할 수 있습니다.
    *   플러그인 측에서 `HandlerAdapter`는 마샬링/언마샬링 오류를 처리합니다. 이러한 오류가 발생하면 일반적으로 오류 메시지를 포맷하여 클라이언트에 애플리케이션 오류로 반환합니다(즉, `isAppError = true`).
-   **애플리케이션 오류 (플러그인 로직)**:
    *   플러그인의 타입이 지정된 핸들러(예: `func(req Req) (Resp, bool)`)는 애플리케이션별 오류가 발생했는지 여부를 나타내는 부울 플래그를 반환합니다.
    *   이 플래그가 `true`이면 `plugin.Header.IsError` 필드가 true로 설정되고 (마샬링된) 응답 페이로드는 클라이언트에 의해 오류 메시지로 처리됩니다.
    *   클라이언트 측의 `loader.Call`은 `plugin error for service <name>: <error message from plugin>`과 같이 포맷된 오류를 반환합니다.
-   **서비스를 찾을 수 없음**: 클라이언트가 플러그인의 `Module`에 등록되지 않은 서비스 이름을 호출하면 `Module`은 오류를 다시 보내고 `loader.Call`은 이 오류를 반환합니다.

## 확장성

JSON 및 프로토콜 버퍼 어댑터가 제공되지만 다른 직렬화 형식을 지원할 수 있습니다:
1.  선택한 형식에 대해 `MarshalRequest func(Req) ([]byte, error)` 및 `UnmarshalResponse func([]byte) (Resp, error)` 함수를 구현합니다.
2.  클라이언트 측에서 이러한 함수를 사용하여 `plugin.Serializer[Req, Resp]` 구조체를 만들고 `plugin.NewLoaderAdapter[Req, Resp](loader, customSerializer)`와 함께 사용합니다.
3.  플러그인 측에서 타입이 지정된 핸들러에 대해 `plugin.NewHandlerAdapter`를 만들 때 사용자 정의 언마샬/마샬 함수를 제공합니다.

## 라이선스

이 프로젝트는 MIT 라이선스에 따라 라이선스가 부여됩니다 - 자세한 내용은 [LICENSE](LICENSE) 파일을 참조하십시오.
