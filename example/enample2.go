package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/yuninks/errorx"
	"github.com/yuninks/langx"
)

// ---- 定义业务错误码 ---------------------------------------------------

// 预定义错误码，集中声明，类型安全。
var (
	ErrLoginFailed   = errorx.NewCode("login_failed", 401, "登录失败")
	ErrTokenExpired  = errorx.NewCode("token_expired", 401, "令牌已过期，请重新登录")
	ErrUserNotFound  = errorx.NewCode("user_not_found", 404, "用户 #name# 不存在")
	ErrParamInvalid  = errorx.NewCode("param_invalid", 422, "参数校验失败: #field#")
	ErrInternal      = errorx.NewCode("internal", 500, "服务器内部错误")
	ErrRateLimit     = errorx.NewCode("rate_limit", 429, "请求过于频繁，请 #seconds# 秒后重试")
)

// ---- 补充语言包（可通过 embed/文件 批量导入） --------------------------

func init() {
	langx.RegisterTrans("en", map[string]string{
		"login_failed":  "Login failed",
		"token_expired": "Token expired, please re-login",
		"user_not_found": "User #name# not found",
		"param_invalid": "Parameter validation failed: #field#",
		"internal":      "Internal server error",
		"rate_limit":    "Too many requests, retry in #seconds# seconds",
	})
}

// ======================================================================
// 示例 1：基础用法 — 创建错误并获取多语言消息
// ======================================================================
func exampleBasic() {
	fmt.Println("=== 示例1：基础用法 ===")

	ctx := context.Background()

	// 创建带上下文的运行时错误
	err := ErrLoginFailed.New(ctx)
	fmt.Println("中文:", err.Error())

	// 切换到英文
	errEn := err.WithLang("en")
	fmt.Println("英文:", errEn.Error())

	// 原实例不受影响
	fmt.Println("原实例仍是中文:", err.Error())

	// 直接从 ErrorCode 获取翻译消息（不创建 error）
	fmt.Println("直接翻译:", ErrLoginFailed.Msg(ctx))
	fmt.Println()
}

// ======================================================================
// 示例 2：占位符替换
// ======================================================================
func examplePlaceholder() {
	fmt.Println("=== 示例2：占位符替换 ===")

	ctx := context.Background()

	// 使用 Newf 创建带占位符的错误
	err := ErrUserNotFound.Newf(ctx, map[string]string{"name": "admin"})
	fmt.Println("中文:", err.Error())

	// 链式派生：追加更多占位符
	err2 := err.WithKV("extra", "value")
	fmt.Println("追加占位符:", err2.Error())
	// err 不受影响
	fmt.Println("原实例不变:", err.Error())

	// 不可变链式：中文 → 英文
	errEn := ErrRateLimit.
		Newf(ctx, map[string]string{"seconds": "30"}).
		WithLang("en")
	fmt.Println("英文:", errEn.Error())

	// 批量替换 WithMap
	err3 := ErrParamInvalid.New(ctx).WithMap(map[string]string{
		"field": "email",
	})
	fmt.Println("WithMap:", err3.Error())
	fmt.Println()
}

// ======================================================================
// 示例 3：辅助函数 — 从 error 中提取信息
// ======================================================================
func exampleHelpers() {
	fmt.Println("=== 示例3：辅助函数 ===")

	ctx := context.Background()
	err := ErrParamInvalid.Newf(ctx, map[string]string{"field": "age"})

	// 类型安全提取
	e, ok := errorx.As(err)
	if ok {
		fmt.Println("Key:", e.Key())
		fmt.Println("Code:", e.Code())
		fmt.Println("Format:", e.Format())
	}

	// 快捷函数
	fmt.Println("CodeFrom:", errorx.CodeFrom(err))
	fmt.Println("KeyFrom:", errorx.KeyFrom(err))

	// 对非 langError 的容错处理
	fmt.Println("CodeFrom(context.Canceled):", errorx.CodeFrom(context.Canceled)) // -1
	fmt.Println()
}

// ======================================================================
// 示例 4：HTTP API 响应 — 模拟真实场景
// ======================================================================
func exampleHTTP() {
	fmt.Println("=== 示例4：HTTP API 响应 ===")

	// 模拟从请求中获取语言
	zhCtx := langx.SetCtxLang(context.Background(), "zh")
	enCtx := langx.SetCtxLang(context.Background(), "en")

	// 处理请求
	handleLogin := func(ctx context.Context) (int, string) {
		// 模拟登录失败
		err := ErrLoginFailed.New(ctx)
		return err.Code(), err.Error()
	}

	codeZH, msgZH := handleLogin(zhCtx)
	codeEN, msgEN := handleLogin(enCtx)

	fmt.Printf("中文响应: code=%d, msg=%s\n", codeZH, msgZH)
	fmt.Printf("英文响应: code=%d, msg=%s\n", codeEN, msgEN)

	// 模拟参数校验失败
	handleParam := func(ctx context.Context, field string) (int, string) {
		err := ErrParamInvalid.Newf(ctx, map[string]string{"field": field})
		return errorx.CodeFrom(err), err.Error()
	}
	fmt.Println()
	code, msg := handleParam(zhCtx, "username")
	fmt.Printf("参数校验(中文): code=%d, msg=%s\n", code, msg)
	fmt.Println()
}

// ======================================================================
// 示例 5：中间件 — 统一错误处理
// ======================================================================
func exampleMiddleware() {
	fmt.Println("=== 示例5：中间件模式 ===")

	// writeJSON 模拟写入 HTTP JSON 响应
	writeJSON := func(err error) {
		code := errorx.CodeFrom(err)
		msg := err.Error()
		if code == -1 {
			code = 500
			msg = "未知错误"
		}
		fmt.Printf("HTTP 响应: {\"code\":%d, \"msg\":\"%s\"}\n", code, msg)
	}

	// 场景1：业务错误
	ctx := context.Background()
	writeJSON(ErrTokenExpired.New(ctx))

	// 场景2：带占位符的业务错误
	writeJSON(ErrUserNotFound.Newf(ctx, map[string]string{"name": "test_user"}))

	// 场景3：非 errorx 错误也能兜底
	writeJSON(http.ErrServerClosed)

	fmt.Println()
}

// ======================================================================
// 示例 6：error 链与 errors.Is / errors.As 兼容
// ======================================================================
func exampleErrorChain() {
	fmt.Println("=== 示例6：与标准 errors 包兼容 ===")

	ctx := context.Background()
	baseErr := ErrInternal.New(ctx)
	wrappedErr := fmt.Errorf("处理订单失败: %w", baseErr)

	// errors.Is / errors.As 仍然可用
	e, ok := errorx.As(wrappedErr)
	if ok {
		fmt.Println("从包装后的 error 中提取成功:")
		fmt.Println("  Key:", e.Key())
		fmt.Println("  Code:", e.Code())
		fmt.Println("  消息:", e.Error())
	}
	fmt.Println()
}

// ======================================================================
// 示例 7：动态创建错误码（未预定义的场景）
// ======================================================================
func exampleDynamic() {
	fmt.Println("=== 示例7：动态创建错误码 ===")

	// 运行时动态注册错误码
	dbError := errorx.NewCode("db_connection_failed", 503, "数据库连接失败: #detail#")

	ctx := context.Background()
	err := dbError.Newf(ctx, map[string]string{"detail": "timeout after 30s"})

	fmt.Println("Key:", errorx.KeyFrom(err))
	fmt.Println("Code:", errorx.CodeFrom(err))
	fmt.Println("消息:", err.Error())
	fmt.Println()
}

// ---- main ------------------------------------------------------------

func main() {
	// 初始化 langx
	langx.InitLangx(
		langx.SetDefaultCode(0),
		langx.SetDefaultLanguage("zh"),
	)

	exampleBasic()
	examplePlaceholder()
	exampleHelpers()
	exampleHTTP()
	exampleMiddleware()
	exampleErrorChain()
	exampleDynamic()
}
