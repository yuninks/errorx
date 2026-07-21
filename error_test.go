package errorx_test

import (
	"context"
	"testing"

	"github.com/yuninks/errorx"
	"github.com/yuninks/langx"
)

func init() {
	langx.InitLangx(
		langx.SetDefaultCode(0),
		langx.SetDefaultLanguage("zh"),
	)
	langx.RegisterCode(map[string]int{
		"login_success": 200,
		"error":         400,
		"username":      201,
		"password":      202,
	})
	langx.RegisterTrans("zh", map[string]string{
		"login_success": "登录成功",
		"error":         "错误",
		"username":      "你好 #name#",
		"password":      "密码错误: #reason#",
	})
	langx.RegisterTrans("en", map[string]string{
		"login_success": "Login success",
		"error":         "Error",
		"username":      "Hello #name#",
		"password":      "Password error: #reason#",
	})
}

func TestNewError_Basic(t *testing.T) {
	ctx := context.Background()

	err := errorx.NewError(ctx, "error")
	t.Log(err.Error()) // 输出：错误

	if code := err.Code(); code != 400 {
		t.Fatalf("expected code 400, got %d", code)
	}
}

func TestNewErrorf_Placeholder(t *testing.T) {
	ctx := context.Background()

	err := errorx.NewErrorf(ctx, "username", map[string]string{
		"name": "yuninks",
	})
	t.Log(err.Error()) // 输出：你好 yuninks

	// 不可变：WithLang 返回新实例，原实例不变
	errEn := err.WithLang("en")
	t.Log(err.Error())   // 仍然是中文
	t.Log(errEn.Error()) // 输出：Hello yuninks
}

func TestErrorCode_Predefined(t *testing.T) {
	ctx := context.Background()

	// 使用预定义 ErrorCode 创建运行时错误
	err := errorx.Error.New(ctx)
	t.Log(err.Error()) // 输出：操作失败
	if code := err.Code(); code != 400 {
		t.Fatalf("expected code 400, got %d", code)
	}
}

func TestErrorCode_Newf(t *testing.T) {
	ctx := context.Background()

	err := errorx.ErrWithMsg.Newf(ctx, map[string]string{"msg": "参数校验失败"})
	t.Log(err.Error()) // 输出：操作失败: 参数校验失败
}

func TestErrorCode_Msg(t *testing.T) {
	ctx := context.Background()

	msg := errorx.Success.Msg(ctx)
	t.Log(msg) // 输出：操作成功
	if msg != "操作成功" {
		t.Fatalf("expected '操作成功', got '%s'", msg)
	}

	// 验证中文环境
	if code := errorx.Success.Code(); code != 200 {
		t.Fatalf("expected code 200, got %d", code)
	}
}

func TestHelper_As(t *testing.T) {
	ctx := context.Background()

	err := errorx.NewError(ctx, "username")
	e, ok := errorx.As(err)
	if !ok {
		t.Fatal("expected errorx.As to succeed")
	}
	t.Log(e.Key())  // username
	t.Log(e.Code()) // 201
}

func TestHelper_CodeFrom(t *testing.T) {
	ctx := context.Background()

	err := errorx.NewError(ctx, "error")
	if code := errorx.CodeFrom(err); code != 400 {
		t.Fatalf("expected 400, got %d", code)
	}

	// 非 langError 返回 -1
	if code := errorx.CodeFrom(context.Canceled); code != -1 {
		t.Fatalf("expected -1 for non-langError, got %d", code)
	}
}

func TestHelper_KeyFrom(t *testing.T) {
	err := errorx.NewError(context.Background(), "login_success")
	if key := errorx.KeyFrom(err); key != "login_success" {
		t.Fatalf("expected 'login_success', got '%s'", key)
	}
}

func TestImmutability_CreateChain(t *testing.T) {
	// 演示不可变链式调用
	ctx := context.Background()

	base := errorx.Error.New(ctx)
	zh := base.WithKV("msg", "中文错误")
	en := zh.WithLang("en").WithKV("msg", "English error")

	t.Log(base.Error()) // 操作失败
	t.Log(zh.Error())   // 操作失败: 中文错误
	t.Log(en.Error())   // 操作失败: English error

	// 各自独立，互不影响
	zh2 := zh.WithKV("extra", "additional")
	t.Log(zh.Error())  // 仍然只有 msg
	t.Log(zh2.Error()) // msg + extra 都有了
}

func TestConcurrency_Safety(t *testing.T) {
	// 多个 goroutine 共享同一个 ErrorCode，各自派生独立实例
	ctx := context.Background()
	done := make(chan bool, 10)

	for range 10 {
		go func() {
			_ = errorx.Error.New(ctx).WithKV("msg", "go routine")
			done <- true
		}()
	}
	for range 10 {
		<-done
	}
	// 无竞态即为通过
}
