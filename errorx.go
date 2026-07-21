package errorx

import (
	"context"

	"github.com/yuninks/langx"
)

// ErrorCode 表示一个预定义的错误码，是纯值类型，无状态、并发安全。
// 声明为 var 后不会被意外修改。
type ErrorCode struct {
	key  string
	code int
}

// NewCode 创建一个错误码，并自动注册到 langx 全局表中。
//   - key:  唯一标识符
//   - code: 业务错误码
//   - defaultMsg: 默认语言下的消息模板
func NewCode(key string, code int, defaultMsg string) ErrorCode {
	langx.AppendCode(map[string]int{key: code})
	langx.AppendTrans(langx.GetDefaultLang(), map[string]string{key: defaultMsg})
	return ErrorCode{key: key, code: code}
}

// Key 返回唯一标识符。
func (ec ErrorCode) Key() string { return ec.key }

// Code 返回业务错误码。
func (ec ErrorCode) Code() int { return ec.code }

// New 创建一个携带上下文的运行时错误。
func (ec ErrorCode) New(ctx context.Context) *langError {
	return NewError(ctx, ec.key)
}

// Newf 创建一个携带占位符键值对的运行时错误。
func (ec ErrorCode) Newf(ctx context.Context, kv map[string]string) *langError {
	return NewErrorf(ctx, ec.key, kv)
}

// Msg 直接获取当前上下文语言下的翻译消息。
func (ec ErrorCode) Msg(ctx context.Context) string {
	return langx.GetFormat(langx.GetCtxLang(ctx), ec.key, nil)
}

// Msgf 直接获取带占位符替换的翻译消息。
func (ec ErrorCode) Msgf(ctx context.Context, kv map[string]string) string {
	return langx.GetFormat(langx.GetCtxLang(ctx), ec.key, kv)
}

// ---- 预定义错误码 ----------------------------------------------------

var (
	Success    = NewCode("success", 200, "操作成功")
	Error      = NewCode("error", 400, "操作失败")
	ErrWithMsg = NewCode("error_with_msg", 400, "操作失败: #msg#")
)
