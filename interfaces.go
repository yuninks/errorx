package errorx

import (
	"context"
	"errors"

	"github.com/yuninks/langx"
)

// ErrorInfo 是对外暴露的错误信息接口，语义精简、不可变。
type ErrorInfo interface {
	error
	Code() int
	Key() string
	Format() map[string]string
}

// langError 是不可变的运行时错误实例，并发安全。
// 通过 With* 方法派生新实例，永远不会修改自身。
type langError struct {
	ctx    context.Context
	key    string
	format map[string]string
}

// ---- 实现 error 接口 -----------------------------------------------

func (e *langError) Error() string {
	return langx.GetFormat(langx.GetCtxLang(e.ctx), e.key, e.format)
}

// ---- 实现 ErrorInfo 接口 --------------------------------------------

func (e *langError) Code() int { return langx.GetCode(e.key) }

func (e *langError) Key() string { return e.key }

func (e *langError) Format() map[string]string {
	if len(e.format) == 0 {
		return nil
	}
	return e.copyFormat()
}

// ---- 不可变派生方法（返回新实例，不修改原值）------------------------

// WithCtx 派生一个携带新上下文的错误实例。
func (e *langError) WithCtx(ctx context.Context) *langError {
	return &langError{ctx: ctx, key: e.key, format: e.copyFormat()}
}

// WithLang 派生一个指定语言的错误实例。
func (e *langError) WithLang(lang string) *langError {
	return &langError{ctx: langx.SetCtxLang(e.ctx, lang), key: e.key, format: e.copyFormat()}
}

// WithKV 派生一个添加单个占位符键值对的错误实例。
func (e *langError) WithKV(k, v string) *langError {
	newFmt := e.copyFormat()
	if newFmt == nil {
		newFmt = make(map[string]string)
	}
	newFmt[k] = v
	return &langError{ctx: e.ctx, key: e.key, format: newFmt}
}

// WithMap 派生一个批量添加占位符键值对的错误实例（新 map 完全替换）。
func (e *langError) WithMap(m map[string]string) *langError {
	newFmt := make(map[string]string, len(m))
	for k, v := range m {
		newFmt[k] = v
	}
	return &langError{ctx: e.ctx, key: e.key, format: newFmt}
}

// ---- 内部工具 --------------------------------------------------------

func (e *langError) copyFormat() map[string]string {
	if len(e.format) == 0 {
		return nil
	}
	dst := make(map[string]string, len(e.format))
	for k, v := range e.format {
		dst[k] = v
	}
	return dst
}

func copyMapString(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// ---- 构造函数 --------------------------------------------------------

// NewError 创建一个携带 ctx 的运行时错误。
func NewError(ctx context.Context, key string) *langError {
	return &langError{ctx: ctx, key: key}
}

// NewErrorf 创建一个携带占位符键值对的运行时错误。
func NewErrorf(ctx context.Context, key string, kv map[string]string) *langError {
	return &langError{ctx: ctx, key: key, format: copyMapString(kv)}
}

// ---- 辅助函数，简化 error → ErrorInfo 提取 ---------------------------

// As 从 error 链中提取 *langError，失败返回 false。
func As(err error) (*langError, bool) {
	var e *langError
	ok := errors.As(err, &e)
	return e, ok
}

// CodeFrom 从 error 中获取错误码；若不是 langError 则返回 -1。
func CodeFrom(err error) int {
	if e, ok := As(err); ok {
		return e.Code()
	}
	return -1
}

// KeyFrom 从 error 中获取 Key；若不是 langError 则返回空串。
func KeyFrom(err error) string {
	if e, ok := As(err); ok {
		return e.Key()
	}
	return ""
}

// FormatFrom 从 error 中获取占位符键值对（深拷贝）；若不是 langError 则返回 nil。
func FormatFrom(err error) map[string]string {
	if e, ok := As(err); ok {
		return e.Format()
	}
	return nil
}
