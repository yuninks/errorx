package errorx

import (
	"context"
	"fmt"
	"io"

	"github.com/yuninks/langx"
)

// ErrorInterface 是 errorx 中所有错误值的统一接口。
//
// 三条设计约束：
//
//  1. 不可变：所有 WithXxx 方法都返回新的错误值，绝不修改接收者。
//     因此包级错误常量可以放心地被任意 goroutine 并发使用。
//
//  2. 不持有 context：错误值只保存语言标识（WithLang），渲染时再把 ctx 传进来
//     （Msg / MsgWith / MsgKV）。避免错误值被日志、队列、缓存长期持有的时候
//     把整个请求上下文一起拖住。
//
//  3. 渲染永不 panic：ctx 为 nil、ctx 里的语言类型不对、langx 资源被清空，
//     都会安全降级到默认文案，而不是崩溃或返回裸 key。
type ErrorInterface interface {
	error

	// GetCode 返回业务错误码。
	GetCode() int
	// GetKey 返回错误常量的唯一标识。
	GetKey() string
	// GetDefault 返回定义错误时写入的默认文案。
	GetDefault() string
	// GetFormat 返回占位符数据的副本，修改返回值不会影响错误本身。
	GetFormat() map[string]string
	// GetLang 返回错误值上固定的语言，空字符串表示跟随渲染时的 ctx。
	GetLang() string

	// Unwrap 返回被 Wrap 包装的底层错误，用于 errors.Is / errors.As / errors.Unwrap。
	Unwrap() error
	// StackTrace 返回创建错误时的调用栈，已过滤 errorx 自身的栈帧。
	StackTrace() StackTrace

	// WithFormat 返回携带占位符数据的新错误值，传 nil 表示清空占位符。
	WithFormat(format map[string]string) ErrorInterface
	// WithFormatKV 返回追加一组占位符数据的新错误值。
	WithFormatKV(key, value string) ErrorInterface
	// WithLang 返回固定了语言的新错误值。
	WithLang(lang string) ErrorInterface
	// Wrap 返回包装了底层错误的新错误值。
	Wrap(cause error) ErrorInterface

	// Msg 使用 ctx 中携带的语言渲染文案。
	Msg(ctx context.Context) string
	// MsgWith 使用 ctx 中携带的语言渲染文案，并替换占位符。
	MsgWith(ctx context.Context, format map[string]string) string
	// MsgKV 是 MsgWith 的键值对写法：MsgKV(ctx, "name", "tom")。
	MsgKV(ctx context.Context, pairs ...string) string
}

// defaultError 是 ErrorInterface 的默认实现。
// 所有字段在构造完成后不再变化，WithXxx 一律返回新实例。
type defaultError struct {
	key          string
	code         int
	defaultValue string
	format       map[string]string // 只读
	lang         string            // 固定语言，空串表示跟随渲染时的 ctx
	cause        error
	pcs          []uintptr // 惰性解析的调用栈
}

// ---------- 只读访问器 ----------

func (e *defaultError) GetCode() int { return e.code }

func (e *defaultError) GetKey() string { return e.key }

func (e *defaultError) GetDefault() string { return e.defaultValue }

func (e *defaultError) GetFormat() map[string]string { return cloneFormat(e.format) }

func (e *defaultError) GetLang() string { return e.lang }

func (e *defaultError) Unwrap() error { return e.cause }

func (e *defaultError) StackTrace() StackTrace { return resolveStack(e.pcs) }

// ---------- 不可变派生 ----------

func (e *defaultError) clone() *defaultError {
	cp := *e
	return &cp
}

func (e *defaultError) WithFormat(format map[string]string) ErrorInterface {
	cp := e.clone()
	cp.format = cloneFormat(format)
	return cp
}

func (e *defaultError) WithFormatKV(key, value string) ErrorInterface {
	cp := e.clone()
	cp.format = cloneFormat(e.format)
	cp.format[key] = value
	return cp
}

func (e *defaultError) WithLang(lang string) ErrorInterface {
	cp := e.clone()
	cp.lang = lang
	return cp
}

func (e *defaultError) Wrap(cause error) ErrorInterface {
	cp := e.clone()
	cp.cause = cause
	return cp
}

// ---------- 渲染 ----------

// Error 实现 error 接口：使用错误值上固定的语言，没有则用默认语言。
func (e *defaultError) Error() string { return e.render(e.lang) }

// Msg 使用 ctx 中携带的语言渲染；若错误值用 WithLang 固定了语言，则以固定语言为准。
func (e *defaultError) Msg(ctx context.Context) string {
	if e.lang != "" {
		return e.render(e.lang)
	}
	return e.render(Langx().GetCtxLang(ctx))
}

func (e *defaultError) MsgWith(ctx context.Context, format map[string]string) string {
	return e.WithFormat(format).Msg(ctx)
}

func (e *defaultError) MsgKV(ctx context.Context, pairs ...string) string {
	return e.WithFormat(pairsToMap(pairs)).Msg(ctx)
}

// render 是唯一真正读取 langx 的地方：langx 里没有对应文案时，
// 用错误常量自带的默认文案兜底，绝不返回裸 key。
func (e *defaultError) render(lang string) string {
	if e.key == "" {
		return emptyKeyMsg
	}
	inst := Langx()
	if lang == "" {
		lang = inst.GetDefaultLang()
	}
	if !inst.HasMsg(lang, e.key) {
		e.registerDefault(inst)
	}
	return inst.GetMsgf(lang, e.key, e.format)
}

// registerDefault 把默认文案回填到 langx 实例的默认语言。
// 用于用户调用 RegisterMsg / RegisterCode 覆盖资源、
// 切换了 langx 实例、或者默认语言在 main 里被改掉之后，仍然能拿到可读的文案。
func (e *defaultError) registerDefault(inst *langx.Instance) {
	if e.defaultValue == "" || e.defaultValue == e.key {
		return
	}
	// key 与语言名在构造期已经校验过，这里注册不会失败；忽略错误即可。
	_ = inst.AppendMsg(inst.GetDefaultLang(), map[string]string{e.key: e.defaultValue})
}

// Is 让 errors.Is 按错误标识（key）匹配：errors.Is(err, errorx.Error)。
func (e *defaultError) Is(target error) bool {
	return sameKey(e, target)
}

// Format 支持 %v / %s / %q，以及带堆栈和原因的 %+v。
func (e *defaultError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if !s.Flag('+') {
			io.WriteString(s, e.Error())
			return
		}
		io.WriteString(s, e.Error())
		if cause := e.Unwrap(); cause != nil {
			fmt.Fprintf(s, "\n  cause: %+v", cause)
		}
		if st := e.StackTrace(); len(st) > 0 {
			fmt.Fprintf(s, "\n  stack:\n%s", st)
		}
	case 's':
		io.WriteString(s, e.Error())
	case 'q':
		fmt.Fprintf(s, "%q", e.Error())
	}
}

// ---------- 内部工具 ----------

const emptyKeyMsg = "errorx: 未定义的错误"

// cloneFormat 复制占位符数据，保证错误值之间不共享 map。
func cloneFormat(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// pairsToMap 把 ("k1","v1","k2","v2") 转成 map；落单的 key 会被忽略。
func pairsToMap(pairs []string) map[string]string {
	m := make(map[string]string, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i]] = pairs[i+1]
	}
	return m
}

// sameKey 判断两个错误是否描述同一个业务错误。
func sameKey(e *defaultError, target error) bool {
	if e.key == "" || target == nil {
		return false
	}
	if t, ok := target.(interface{ GetKey() string }); ok {
		return t.GetKey() == e.key
	}
	return false
}
