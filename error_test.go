package errorx_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/yuninks/errorx"
	"github.com/yuninks/langx"
)

// 测试用的错误常量：key + 业务码 + 默认文案（占位符用 #name#）
var (
	ErrUserNotFound = errorx.New("test_user_not_found", 10001, "用户不存在")
	ErrParamInvalid = errorx.New("test_param_invalid", 10002, "参数错误: #field#")
	ErrDB           = errorx.New("test_db_error", 50001, "数据库繁忙")
)

func TestMain(m *testing.M) {
	langx.InitLangx(
		langx.SetDefaultLanguage("zh"),
		langx.SetDefaultCode(-1), // 故意设成 -1，用来验证未注册的 key 不会退化成默认码
	)
	langx.AppendTrans("en", map[string]string{
		"test_user_not_found": "user not found",
		"test_param_invalid":  "invalid parameter: #field#",
		"test_db_error":       "database is busy",
	})
	os.Exit(m.Run())
}

func TestErrBasics(t *testing.T) {
	e := ErrUserNotFound.Err()

	if got, want := e.GetCode(), 10001; got != want {
		t.Fatalf("GetCode() = %d, want %d", got, want)
	}
	if got, want := e.GetKey(), "test_user_not_found"; got != want {
		t.Fatalf("GetKey() = %q, want %q", got, want)
	}
	if got, want := e.GetDefault(), "用户不存在"; got != want {
		t.Fatalf("GetDefault() = %q, want %q", got, want)
	}
	if got, want := e.Error(), "用户不存在"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}

	// 错误常量本身也实现了 error 接口
	var err error = ErrUserNotFound
	if got, want := err.Error(), "用户不存在"; got != want {
		t.Fatalf("ErrorLanguage.Error() = %q, want %q", got, want)
	}

	if got, want := ErrUserNotFound.GetCode(), 10001; got != want {
		t.Fatalf("ErrorLanguage.GetCode() = %d, want %d", got, want)
	}
	if got, want := ErrUserNotFound.GetDefault(), "用户不存在"; got != want {
		t.Fatalf("ErrorLanguage.GetDefault() = %q, want %q", got, want)
	}
}

func TestPlaceholder(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"ErrKV", ErrParamInvalid.ErrKV("field", "age").Error(), "参数错误: age"},
		{"ErrWith", ErrParamInvalid.ErrWith(map[string]string{"field": "name"}).Error(), "参数错误: name"},
		{"Err().WithFormatKV", ErrParamInvalid.Err().WithFormatKV("field", "id").Error(), "参数错误: id"},
		{"MsgKV", ErrParamInvalid.MsgKV(context.Background(), "field", "phone"), "参数错误: phone"},
		{"MsgWith", ErrParamInvalid.MsgWith(context.Background(), map[string]string{"field": "mail"}), "参数错误: mail"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestMultiLanguage(t *testing.T) {
	ctx := context.Background()
	enCtx := langx.SetCtxLang(ctx, "en")

	if got, want := ErrUserNotFound.Msg(ctx), "用户不存在"; got != want {
		t.Fatalf("中文 Msg = %q, want %q", got, want)
	}
	if got, want := ErrUserNotFound.Msg(enCtx), "user not found"; got != want {
		t.Fatalf("英文 Msg = %q, want %q", got, want)
	}
	if got, want := ErrParamInvalid.MsgKV(enCtx, "field", "age"), "invalid parameter: age"; got != want {
		t.Fatalf("英文占位符 = %q, want %q", got, want)
	}
	// 固定语言：不再跟随 ctx
	err := ErrUserNotFound.Err().WithLang("en")
	if got, want := err.Msg(ctx), "user not found"; got != want {
		t.Fatalf("WithLang 后 Msg = %q, want %q", got, want)
	}
	if got, want := err.Error(), "user not found"; got != want {
		t.Fatalf("WithLang 后 Error = %q, want %q", got, want)
	}
	// 未注册的语言回退到默认语言，而不是吐裸 key
	if got, want := ErrUserNotFound.Msg(langx.SetCtxLang(ctx, "ja")), "用户不存在"; got != want {
		t.Fatalf("未注册语言 = %q, want %q", got, want)
	}
}

func TestImmutability(t *testing.T) {
	base := ErrParamInvalid.Err()
	derived := base.WithFormatKV("field", "age")

	if got, want := base.Error(), "参数错误: #field#"; got != want {
		t.Fatalf("派生后原错误被修改：%q, want %q", got, want)
	}
	if got, want := derived.Error(), "参数错误: age"; got != want {
		t.Fatalf("派生错误 = %q, want %q", got, want)
	}
	if got, want := ErrParamInvalid.Error(), "参数错误: #field#"; got != want {
		t.Fatalf("全局常量被污染：%q, want %q", got, want)
	}

	// GetFormat 返回副本，外部修改不影响错误本身
	f := derived.GetFormat()
	f["field"] = "hacked"
	if got, want := derived.Error(), "参数错误: age"; got != want {
		t.Fatalf("GetFormat 泄漏了内部 map：%q, want %q", got, want)
	}

	// WithLang / WithFormat 不互相影响
	a := base.WithLang("en")
	b := base.WithFormatKV("field", "x")
	if a.GetLang() != "en" || b.GetLang() != "" {
		t.Fatalf("派生值互相污染：a.lang=%q b.lang=%q", a.GetLang(), b.GetLang())
	}
}

func TestConcurrentUse(t *testing.T) {
	// 同一个错误值被多个 goroutine 共享：旧版本要么 fatal: concurrent map writes，
	// 要么不同请求之间互相改写文案。
	base := ErrParamInvalid.Err()
	const workers, rounds = 32, 50

	var wg sync.WaitGroup
	bad := make(chan string, workers)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				v := fmt.Sprintf("%d-%d", w, r)
				if got, want := base.WithFormatKV("field", v).Error(), "参数错误: "+v; got != want {
					bad <- fmt.Sprintf("派生错误被串改: got %q, want %q", got, want)
					return
				}
				if got, want := base.GetFormat()["field"], ""; got != want {
					bad <- fmt.Sprintf("原错误被派生值污染: got %q, want %q", got, want)
					return
				}
				if got, want := ErrUserNotFound.Error(), "用户不存在"; got != want {
					bad <- fmt.Sprintf("全局常量被改写: got %q, want %q", got, want)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(bad)
	for msg := range bad {
		t.Fatal(msg)
	}
}

func TestUnknownKeyIsNotSuccess(t *testing.T) {
	e := errorx.NewError(context.Background(), "totally_unknown_key")

	if got, want := e.GetCode(), errorx.ErrUnregistered.GetCode(); got != want {
		t.Fatalf("未注册 key 的 code = %d, want %d", got, want)
	}
	if e.GetCode() == 200 {
		t.Fatal("未注册的 key 被伪装成了成功(200)")
	}
	if !strings.Contains(e.Error(), "totally_unknown_key") {
		t.Fatalf("未注册 key 的文案应包含原始 key，实际 %q", e.Error())
	}
	// 空 key 同样按未注册处理
	if got, want := errorx.NewError(context.Background(), "").GetCode(), errorx.ErrUnregistered.GetCode(); got != want {
		t.Fatalf("空 key 的 code = %d, want %d", got, want)
	}
}

func TestRegistryOverwriteKeepsBuiltin(t *testing.T) {
	// 用户习惯用 Register*，它会整体替换 langx 的注册表
	langx.RegisterCode(map[string]int{"biz_code": 500})
	langx.RegisterTrans("zh", map[string]string{"biz_code": "业务错误"})
	defer func() {
		langx.AppendCode(map[string]int{"error": 400, "success": 200, "error_with_msg": 400})
		langx.AppendTrans("zh", map[string]string{
			"error": "操作失败", "success": "操作成功", "error_with_msg": "操作失败: #msg#",
		})
	}()

	// code 来自错误常量自身，不会被清空
	if got, want := errorx.Error.Err().GetCode(), 400; got != want {
		t.Fatalf("内置错误 code = %d, want %d（旧版本会变成 200）", got, want)
	}
	// 文案用错误常量自带的默认值兜底，不会退化成裸 key
	if got, want := errorx.Error.Err().Error(), "操作失败"; got != want {
		t.Fatalf("内置错误文案 = %q, want %q", got, want)
	}
	if got, want := errorx.ErrWithMsg.ErrKV("msg", "超时").Error(), "操作失败: 超时"; got != want {
		t.Fatalf("内置错误占位符 = %q, want %q", got, want)
	}
	// 用户仍然可以用资源里的 key
	if got, want := errorx.NewError(context.Background(), "biz_code").GetCode(), 500; got != want {
		t.Fatalf("资源 key 的 code = %d, want %d", got, want)
	}
}

func TestNilAndDirtyCtx(t *testing.T) {
	if got, want := ErrUserNotFound.Msg(nil), "用户不存在"; got != want {
		t.Fatalf("nil ctx = %q, want %q", got, want)
	}
	// langx 会对非 string 的 ctx 值做强制断言，errorx 必须兜住
	dirty := context.WithValue(context.Background(), "Accept-Language", 123)
	if got, want := ErrUserNotFound.Msg(dirty), "用户不存在"; got != want {
		t.Fatalf("脏 ctx = %q, want %q", got, want)
	}
	if got, want := errorx.NewError(nil, "test_user_not_found").Error(), "用户不存在"; got != want {
		t.Fatalf("nil ctx 动态构造 = %q, want %q", got, want)
	}
}

func TestErrorsIsAsUnwrap(t *testing.T) {
	cause := errors.New("dial tcp 127.0.0.1:3306: connection refused")
	err := ErrDB.Wrap(cause)

	if !errors.Is(err, ErrDB) {
		t.Fatal("errors.Is(err, ErrDB) = false, want true")
	}
	if !errors.Is(err, cause) {
		t.Fatal("errors.Is 应该能穿透到底层错误")
	}
	if errors.Is(err, ErrUserNotFound) {
		t.Fatal("不同 key 的错误不应相等")
	}
	if got := errors.Unwrap(err); got != cause {
		t.Fatalf("Unwrap() = %v, want %v", got, cause)
	}
	var ei errorx.ErrorInterface
	if !errors.As(err, &ei) {
		t.Fatal("errors.As(err, &ErrorInterface) = false, want true")
	}
	if ei.GetCode() != 50001 {
		t.Fatalf("errors.As 拿到的 code = %d", ei.GetCode())
	}
	// 文案里不应出现底层错误的细节
	if strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("底层错误不应出现在对外文案里：%q", err.Error())
	}
	// 常量在左边：错误常量直接当 error 返回时走 ErrorLanguage.Is
	if !errors.Is(ErrUserNotFound, ErrUserNotFound.Err()) {
		t.Fatal("errors.Is(常量, 错误值) = false, want true")
	}
	if errors.Is(ErrUserNotFound, ErrDB.Err()) {
		t.Fatal("不同 key 的常量不应匹配")
	}
	if errors.Is(ErrUserNotFound, errors.New("plain")) {
		t.Fatal("常量不应匹配非 errorx 的错误")
	}
	// 非 errorx 的普通错误不应匹配
	if errors.Is(err, errors.New("boom")) {
		t.Fatal("普通 error 不应匹配 errorx 错误")
	}
}

func TestStackTrace(t *testing.T) {
	st := ErrUserNotFound.Err().StackTrace()
	if len(st) == 0 {
		t.Fatal("StackTrace() 为空")
	}
	var found bool
	for _, f := range st {
		if strings.HasPrefix(f.Function, "github.com/yuninks/errorx.") {
			t.Fatalf("栈里不应包含 errorx 自身帧：%s", f.Function)
		}
		if strings.Contains(f.Function, "TestStackTrace") {
			found = true
		}
	}
	if !found {
		t.Fatal("栈里没有调用方帧")
	}
	if !strings.Contains(st.String(), "error_test.go") {
		t.Fatalf("StackTrace.String() 内容异常：%s", st.String())
	}
	// 空栈渲染为空串，不 panic
	var empty errorx.StackTrace
	if got := empty.String(); got != "" {
		t.Fatalf("空 StackTrace.String() = %q, want \"\"", got)
	}
	var f errorx.Frame
	if !strings.Contains(f.String(), ":0") {
		t.Fatalf("Frame.String() = %q", f.String())
	}
}

func TestFormatVerb(t *testing.T) {
	err := ErrDB.Wrap(errors.New("boom"))

	if got, want := fmt.Sprintf("%v", err), "数据库繁忙"; got != want {
		t.Fatalf("%%v = %q, want %q", got, want)
	}
	if got, want := fmt.Sprintf("%s", err), "数据库繁忙"; got != want {
		t.Fatalf("%%s = %q, want %q", got, want)
	}
	if got, want := fmt.Sprintf("%q", err), "\"数据库繁忙\""; got != want {
		t.Fatalf("%%q = %q, want %q", got, want)
	}
	plus := fmt.Sprintf("%+v", err)
	if !strings.Contains(plus, "cause: boom") || !strings.Contains(plus, "stack:") {
		t.Fatalf("%%+v 应包含 cause 与 stack，实际：%s", plus)
	}
}

func TestFromRegistry(t *testing.T) {
	langx.AppendCode(map[string]int{"test_registry_key": 777})
	langx.AppendTrans("zh", map[string]string{"test_registry_key": "注册表错误: #msg#"})

	e := errorx.NewErrorWith(context.Background(), "test_registry_key", map[string]string{"msg": "磁盘满了"})
	if got, want := e.GetCode(), 777; got != want {
		t.Fatalf("code = %d, want %d", got, want)
	}
	if got, want := e.Error(), "注册表错误: 磁盘满了"; got != want {
		t.Fatalf("msg = %q, want %q", got, want)
	}
	// 构造时带上 ctx 语言
	enCtx := langx.SetCtxLang(context.Background(), "en")
	langx.AppendTrans("en", map[string]string{"test_registry_key": "registry error: #msg#"})
	if got, want := errorx.NewErrorWith(enCtx, "test_registry_key", map[string]string{"msg": "full"}).Error(), "registry error: full"; got != want {
		t.Fatalf("en msg = %q, want %q", got, want)
	}
}

func TestResolveBranches(t *testing.T) {
	// 资源里只有文案、没有 code：保留文案，code 用 CodeUnknown 兜底
	langx.AppendTrans("zh", map[string]string{"test_msg_only": "只有文案"})
	e := errorx.NewError(context.Background(), "test_msg_only")
	if got, want := e.GetCode(), errorx.CodeUnknown; got != want {
		t.Fatalf("只有文案时 code = %d, want %d", got, want)
	}
	if got, want := e.Error(), "只有文案"; got != want {
		t.Fatalf("只有文案时 msg = %q, want %q", got, want)
	}

	// 资源里只有 code、没有文案：code 生效，文案退化为 key
	langx.AppendCode(map[string]int{"test_code_only": 1234})
	e2 := errorx.NewError(context.Background(), "test_code_only")
	if got, want := e2.GetCode(), 1234; got != want {
		t.Fatalf("只有 code 时 code = %d, want %d", got, want)
	}
	if got, want := e2.Error(), "test_code_only"; got != want {
		t.Fatalf("只有 code 时 msg = %q, want %q", got, want)
	}
}

func TestZeroValueAndEmptyKey(t *testing.T) {
	var l errorx.ErrorLanguage
	if l.GetCode() != 0 || l.GetKey() != "" {
		t.Fatalf("零值访问器异常：code=%d key=%q", l.GetCode(), l.GetKey())
	}
	var err error = l
	if err.Error() == "" {
		t.Fatal("零值错误也应该能安全渲染")
	}
	if l.Err().Error() == "" {
		t.Fatal("零值错误常量也应该能安全渲染")
	}
}

func TestNewPanicsOnEmptyKey(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("空 key 应该 panic，避免注册出无法定位的错误")
		}
	}()
	_ = errorx.New("", 1, "x")
}

func TestNewErrorWith(t *testing.T) {
	langx.AppendCode(map[string]int{"test_dynamic": 901})
	langx.AppendTrans("zh", map[string]string{"test_dynamic": "动态错误: #msg#"})

	e := errorx.NewErrorWith(context.Background(), "test_dynamic", map[string]string{"msg": "磁盘满了"})
	if got, want := e.GetCode(), 901; got != want {
		t.Fatalf("code = %d, want %d", got, want)
	}
	if got, want := e.Error(), "动态错误: 磁盘满了"; got != want {
		t.Fatalf("msg = %q, want %q", got, want)
	}
	// 带 ctx 语言
	enCtx := langx.SetCtxLang(context.Background(), "en")
	langx.AppendTrans("en", map[string]string{"test_dynamic": "dynamic error: #msg#"})
	if got, want := errorx.NewErrorWith(enCtx, "test_dynamic", map[string]string{"msg": "full"}).Error(), "dynamic error: full"; got != want {
		t.Fatalf("en msg = %q, want %q", got, want)
	}
}
