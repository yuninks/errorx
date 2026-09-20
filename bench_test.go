package errorx_test

import (
	"context"
	"errors"
	"testing"

	"github.com/yuninks/errorx"
)

// 错误路径的性能基准。以下是在 Ryzen 7 5800H / go1.26 上的参考值：
//
//	BenchmarkErr        370 ns/op   160 B/op   2 allocs/op   （含一次调用栈采集）
//	BenchmarkErrKV      825 ns/op   944 B/op   7 allocs/op
//	BenchmarkWrap       543 ns/op   272 B/op   3 allocs/op
//	BenchmarkMsg        133 ns/op    16 B/op   1 allocs/op   （只渲染文案，不抓栈）
//
// 栈只在构造错误值时抓取一次，且只存程序计数器；函数名/文件/行号在调用
// StackTrace() 或 %+v 时才解析。
var benchErr = errorx.New("bench_err", 1, "基准错误: #a#")

func BenchmarkErr(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = benchErr.Err()
	}
}

func BenchmarkErrKV(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = benchErr.ErrKV("a", "1")
	}
}

func BenchmarkWrap(b *testing.B) {
	cause := errors.New("boom")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = benchErr.Wrap(cause)
	}
}

func BenchmarkMsg(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = benchErr.Msg(ctx)
	}
}
