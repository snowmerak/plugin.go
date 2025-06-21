# Multiplexer Package

고성능 메시지 멀티플렉싱 라이브러리

## 🚀 Quick Start

```go
package main

import (
    "context"
    "bytes"
    "github.com/snowmerak/plugin.go/lib/multiplexer"
)

func main() {
    // 자동 최적화 모드 (권장)
    reader := &bytes.Buffer{}
    writer := &bytes.Buffer{}
    
    mux := multiplexer.New(reader, writer)
    defer mux.Close()
    
    // 메시지 전송
    ctx := context.Background()
    data := []byte("Hello, World!")
    err := mux.WriteMessage(ctx, 1, data)
    if err != nil {
        panic(err)
    }
    
    // 메시지 수신
    message, err := mux.ReadMessage(ctx)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Received: %s\n", message.Data)
}
```

## 📊 성능 특성

| 메시지 크기 | 성능 향상 | 메모리 절약 | 권장 용도 |
|------------|----------|-------------|-----------|
| < 8KB | 기본 성능 | - | API 응답, IoT 데이터 |
| 8KB - 64KB | +9.3% | 59% | 파일 청크, 이미지 |
| > 64KB | +39% | 90% | 대용량 파일, 스트리밍 |

## 🎯 사용 시나리오

### IoT/센서 데이터
```go
mux := multiplexer.NewOptimized(reader, writer, multiplexer.OptimizeFor.SmallMessages)
```

### 파일 전송
```go
mux := multiplexer.NewOptimized(reader, writer, multiplexer.OptimizeFor.LargeMessages)
```

### 범용 (자동 최적화)
```go
mux := multiplexer.New(reader, writer) // 권장
```

## 🔧 고급 설정

```go
// 커스텀 임계값 설정
mux := multiplexer.NewWithConfig(reader, writer, multiplexer.Config{
    Threshold: 16 * 1024, // 16KB
    BufferSize: 64 * 1024, // 64KB
    MaxMessageSize: 10 * 1024 * 1024, // 10MB
})
```

## ⚡ 성능 최적화 팁

1. **작은 메시지 (< 8KB)**: 기본 모드가 최적
2. **큰 메시지 (> 8KB)**: 자동으로 최적화됨
3. **메모리 제약 환경**: 하이브리드 모드 권장
4. **처리량 우선**: 대용량 전용 모드 고려

## 📈 벤치마크 결과

```bash
# 벤치마크 실행
go test -bench=BenchmarkComparison -benchmem

# 결과 예시:
# BenchmarkComparison/Original_1KB-12     16589593    73.41 ns/op    48 B/op    3 allocs/op
# BenchmarkComparison/Hybrid_64KB-12        667698   1803 ns/op    432 B/op   18 allocs/op  
# BenchmarkComparison/Hybrid_1024KB-12       47683  26139 ns/op   1639 B/op   66 allocs/op
```

## 🛡️ 안전성

- ✅ 동시성 안전 (Goroutine-safe)
- ✅ 메모리 오버플로우 방지 (10MB 제한)
- ✅ 컨텍스트 취소 지원
- ✅ 자동 리소스 정리
- ✅ 종합 테스트 커버리지
