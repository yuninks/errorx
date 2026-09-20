// Package errorx 把"业务错误码 + 多语言文案 + 占位符"封装成不可变的错误常量。
//
// 基本用法：
//
//	var ErrUserNotFound = errorx.New("user_not_found", 10001, "用户不存在")
//
//	func handle(ctx context.Context) error {
//		if err := load(); err != nil {
//			return ErrUserNotFound.Wrap(err) // 保留底层错误，errors.Is / errors.Unwrap 可用
//		}
//		return nil
//	}
//
//	msg := ErrUserNotFound.Msg(ctx) // 按 ctx 里的语言渲染
package errorx

import (
	"context"
	"sync"

	"github.com/yuninks/langx"
)

// ErrorLanguage 是一个"错误常量"：把错误标识（key）、业务错误码（code）
// 和默认文案（defaultValue）绑定在一起。
//
// 它是不可变的值类型，可以直接作为包级变量并发使用：
//
//	var ErrUserNotFound = errorx.New("user_not_found", 10001, "用户不存在")
//
// 它只负责"定义"和"构造错误值"，读取与派生都在构造出来的 ErrorInterface 上做。
// 同时它实现了 error 接口，所以可以：
//
//	var err error = ErrUserNotFound              // 直接当错误用
//	errors.Is(someErr, ErrUserNotFound)          // 按 key 比较
type ErrorLanguage struct {
	key          string
	code         int
	defaultValue string
}

// New 定义一个新的错误常量，并把 code 与默认文案注册进 langx 与 errorx 的常量注册表。
//
// key 为空会 panic —— 这是定义期的编程错误，应该在启动时就暴露。
// 默认文案里的占位符使用 #name# 形式（可用 langx.SetReplaceKey 修改）。
//
// 建议只在包级变量或 init 中调用（它会持续写入注册表）；动态场景请用 NewError / NewErrorWith。
func New(key string, code int, defaultValue string) ErrorLanguage {
	if key == "" {
		panic("errorx: New 的 key 不能为空")
	}
	langx.AppendCode(map[string]int{key: code})
	langx.AppendTrans(langx.GetDefaultLang(), map[string]string{key: defaultValue})
	l := ErrorLanguage{key: key, code: code, defaultValue: defaultValue}
	register(l)
	return l
}

func (l ErrorLanguage) toError() *defaultError {
	return &defaultError{
		key:          l.key,
		code:         l.code,
		defaultValue: l.defaultValue,
	}
}

// ---------- 构造错误值 ----------

// Err 生成 errorx 错误值（自身即实现了 error）。
func (l ErrorLanguage) Err() ErrorInterface {
	e := l.toError()
	e.pcs = captureStack()
	return e
}

// ErrWith 生成带占位符数据的错误值。
func (l ErrorLanguage) ErrWith(format map[string]string) ErrorInterface {
	return l.Err().WithFormat(format)
}

// ErrKV 生成带占位符数据的错误值：ErrKV("name", "tom")。
func (l ErrorLanguage) ErrKV(pairs ...string) ErrorInterface {
	return l.Err().WithFormat(pairsToMap(pairs))
}

// Wrap 生成包装了底层错误的错误值，底层错误可通过 errors.Unwrap / errors.Is 获取，
// 且不会出现在 Msg 文案里。
func (l ErrorLanguage) Wrap(cause error) ErrorInterface {
	return l.Err().Wrap(cause)
}

// ---------- 渲染 ----------

// Error 实现 error 接口：按默认语言渲染。
func (l ErrorLanguage) Error() string { return l.toError().Error() }

// Msg 按 ctx 中携带的语言渲染文案。
func (l ErrorLanguage) Msg(ctx context.Context) string { return l.toError().Msg(ctx) }

// MsgWith 按 ctx 中携带的语言渲染文案，并替换占位符。
func (l ErrorLanguage) MsgWith(ctx context.Context, format map[string]string) string {
	return l.toError().MsgWith(ctx, format)
}

// MsgKV 是 MsgWith 的键值对写法。
func (l ErrorLanguage) MsgKV(ctx context.Context, pairs ...string) string {
	return l.toError().MsgKV(ctx, pairs...)
}

// ---------- 读取 ----------

// GetCode 返回业务错误码。
func (l ErrorLanguage) GetCode() int { return l.code }

// GetKey 返回错误标识。
func (l ErrorLanguage) GetKey() string { return l.key }

// GetDefault 返回默认文案。
func (l ErrorLanguage) GetDefault() string { return l.defaultValue }

// Is 让 errors.Is 按错误标识（key）匹配。
func (l ErrorLanguage) Is(target error) bool {
	if l.key == "" || target == nil {
		return false
	}
	if t, ok := target.(interface{ GetKey() string }); ok {
		return t.GetKey() == l.key
	}
	return false
}

// ---------- 从 langx 资源构造错误 ----------

// NewError 用 langx 中已注册的 key 构造错误（code 与文案都来自资源文件）。
//
// key 未注册时返回 ErrUnregistered（code 500）并带上原始 key，
// 不会把打错的 key 当成正常错误。
func NewError(ctx context.Context, key string) ErrorInterface {
	return newFromRegistry(ctx, key, nil)
}

// NewErrorWith 是 NewError 的带占位符版本。
func NewErrorWith(ctx context.Context, key string, format map[string]string) ErrorInterface {
	return newFromRegistry(ctx, key, format)
}

func newFromRegistry(ctx context.Context, key string, format map[string]string) ErrorInterface {
	lang := langx.GetCtxLang(ctx)
	code, defaultValue, ok := resolve(key)
	if !ok {
		return ErrUnregistered.ErrKV("key", key).WithLang(lang)
	}
	return &defaultError{
		key:          key,
		code:         code,
		defaultValue: defaultValue,
		lang:         lang,
		format:       cloneFormat(format),
		pcs:          captureStack(),
	}
}

// CodeUnknown 是"有文案但没有注册错误码"时使用的兜底码。
// 取 500 而不是 langx 的默认码（默认 -1，也可能被改成 200），避免配置缺失被当成成功。
const CodeUnknown = 500

// resolve 按 key 取出业务码与默认文案：
//
//  1. 优先用 errorx.New 定义过的错误常量（不依赖 langx 的注册表）；
//  2. 其次用 langx 资源文件（code.json / xx.json）里注册的内容；
//  3. 两者都没有时返回 ok=false，由调用方转成 ErrUnregistered。
func resolve(key string) (code int, defaultValue string, ok bool) {
	if key == "" {
		return 0, "", false
	}
	if l, hit := lookupOwn(key); hit {
		return l.code, l.defaultValue, true
	}

	lang := langx.GetDefaultLang()
	hasMsg := langx.Has(lang, key)
	hasCode := langx.HasCode(key)

	switch {
	case hasMsg && hasCode:
		return langx.GetCode(key), langx.GetMsg(lang, key), true
	case hasMsg:
		// 资源里只有文案没有 code：保留文案，但用 CodeUnknown 兜底
		return CodeUnknown, langx.GetMsg(lang, key), true
	case hasCode:
		// 资源里只有 code 没有文案：文案退化为 key 本身
		return langx.GetCode(key), key, true
	default:
		return 0, "", false
	}
}

// errorx 自己的错误常量注册表：即使 langx 的资源被 RegisterCode / RegisterTrans 覆盖，
// 用 errorx.New 定义过的常量依然能拿到正确的 code 与默认文案。
var (
	registryMu sync.RWMutex
	registry   = map[string]ErrorLanguage{}
)

func register(l ErrorLanguage) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[l.key] = l
}

func lookupOwn(key string) (ErrorLanguage, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	l, ok := registry[key]
	return l, ok
}

// 内置错误常量。
var (
	// Success 表示成功。
	Success = New("success", 200, "操作成功")
	// Error 表示通用失败。
	Error = New("error", 400, "操作失败")
	// ErrWithMsg 表示带失败原因的通用失败，文案里的 #msg# 由调用方填充。
	ErrWithMsg = New("error_with_msg", 400, "操作失败: #msg#")
	// ErrInternal 用于兜底的非业务错误。
	ErrInternal = New("errorx_internal", 500, "服务器内部错误")
	// ErrUnregistered 表示使用了未注册的 key，由 NewError 自动返回。
	ErrUnregistered = New("errorx_unregistered", 500, "未注册的错误: #key#")
)
