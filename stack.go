package errorx

import (
	"fmt"
	"runtime"
	"strings"
)

// selfPackage 用于过滤 errorx 自身的栈帧。
const selfPackage = "github.com/yuninks/errorx."

const stackMaxDepth = 32

// Frame 是一个调用栈帧。
type Frame struct {
	Function string
	File     string
	Line     int
}

// String 返回 "file:line" 形式，方便日志里直接打印。
func (f Frame) String() string {
	return fmt.Sprintf("%s:%d", f.File, f.Line)
}

// StackTrace 是错误创建时的调用栈（已过滤 errorx 自身帧）。
type StackTrace []Frame

// String 按 "函数名\n\t文件:行号" 的格式输出整条调用栈。
func (st StackTrace) String() string {
	if len(st) == 0 {
		return ""
	}
	var b strings.Builder
	for _, f := range st {
		b.WriteString(f.Function)
		b.WriteString("\n\t")
		b.WriteString(f.String())
		b.WriteString("\n")
	}
	return b.String()
}

// captureStack 只抓取程序计数器，真正解析成函数名/文件/行号推迟到 StackTrace()，
// 保证创建错误的开销足够小。
func captureStack() []uintptr {
	var buf [stackMaxDepth]uintptr
	n := runtime.Callers(2, buf[:])
	if n <= 0 {
		return nil
	}
	pcs := make([]uintptr, n)
	copy(pcs, buf[:n])
	return pcs
}

// resolveStack 把程序计数器解析成可读的栈帧。
func resolveStack(pcs []uintptr) StackTrace {
	if len(pcs) == 0 {
		return nil
	}
	frames := runtime.CallersFrames(pcs)
	out := make(StackTrace, 0, len(pcs))
	for {
		f, more := frames.Next()
		if f.Function != "" && !strings.HasPrefix(f.Function, selfPackage) {
			out = append(out, Frame{Function: f.Function, File: f.File, Line: f.Line})
		}
		if !more {
			break
		}
	}
	return out
}
