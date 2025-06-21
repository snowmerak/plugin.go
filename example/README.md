# Plugin.go 예제

이 디렉토리는 `plugin.go` 라이브러리를 사용하는 실제 동작하는 예제들을 포함합니다.

## 구조

- `simple.go` - 간단한 Echo 플러그인 사용 예제
- `host/` - 복합 플러그인을 로드하고 호출하는 호스트 애플리케이션  
- `plugins/` - 다양한 플러그인 구현 예제들
  - `echo/` - 간단한 에코 플러그인
  - `calculator/` - JSON 기반 계산기 플러그인

## 빠른 시작

1. 플러그인 빌드:
   ```bash
   cd example
   go build -o plugins/echo/echo ./plugins/echo
   go build -o plugins/calculator/calculator ./plugins/calculator
   ```

2. 간단한 예제 실행:
   ```bash
   go run simple.go
   ```

3. 복합 예제 실행:
   ```bash
   go run ./host
   ```

## 각 예제 설명

### simple.go
- 가장 기본적인 플러그인 사용 예제
- Echo 플러그인을 로드하고 호출
- JSON 어댑터 사용 시연

### Echo Plugin  
- 입력받은 메시지를 그대로 반환
- JSON 기반 통신
- 플러그인 개발의 기본 구조 시연

### Calculator Plugin
- JSON 기반 계산기 플러그인
- 기본적인 수학 연산 지원 (덧셈, 뺄셈, 곱셈, 나눗셈)
- 에러 핸들링 예제 포함

### Host Application
- 여러 플러그인을 동시에 사용하는 예제
- 플러그인 간 독립성 시연
- 실제 애플리케이션 시나리오
