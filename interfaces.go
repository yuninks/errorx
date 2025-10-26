package errorx

import (
	"context"

	"github.com/yuninks/langx"
)

// Key生成错误信息
type ErrorInterface interface {
	Copy() ErrorInterface               // 复制一个新的错误信息
	Error() string                      // 实现error接口&获取翻译后的错误信息
	GetCode() int                       // 获取翻译后的Code
	GetKey() string                     // 获取原Key值
	GetFormat() map[string]string       // 获取附加数据
	SetFormat(format map[string]string) // 设置附加数据
	SetCtx(ctxHttp context.Context)     // 设置上下文
	SetLang(lang string)                // 设置语言
	SetFormatKV(key, value string)      // 设置附加数据键值对
}

type defaultError struct {
	ctx    context.Context
	key    string
	format map[string]string
}

func (l *defaultError) Copy() ErrorInterface {
	return &defaultError{
		ctx:    l.ctx,
		key:    l.key,
		format: l.format,
	}
}

func (e *defaultError) SetFormat(format map[string]string) {
	e.format = format
}

func (l *defaultError) SetFormatKV(key, value string) {
	if l.format == nil {
		l.format = make(map[string]string)
	}
	l.format[key] = value
}

func (e *defaultError) SetCtx(ctxHttp context.Context) {
	e.ctx = ctxHttp
}

func (l *defaultError) Error() string {
	errLang := langx.GetCtxLang(l.ctx)
	return langx.GetFormat(errLang, l.key, l.format)
}

func (e *defaultError) GetCode() int {
	return langx.GetCode(e.key)
}

func (e *defaultError) GetKey() string {
	return e.key
}

func (e *defaultError) GetFormat() map[string]string {
	if e.format == nil {
		e.format = make(map[string]string)
	}
	return e.format
}

func (e *defaultError) SetLang(lang string) {
	e.ctx = langx.SetCtxLang(e.ctx, lang)
}

func NewErrorf(ctx context.Context, key string, format map[string]string) error {
	return &defaultError{
		ctx:    ctx,
		key:    key,
		format: format,
	}
}

func NewError(ctx context.Context, key string) error {
	return &defaultError{
		ctx: ctx,
		key: key,
	}
}

func NewStruct(ctx context.Context, key string, format map[string]string) ErrorInterface {
	return &defaultError{
		ctx:    ctx,
		key:    key,
		format: format,
	}
}
