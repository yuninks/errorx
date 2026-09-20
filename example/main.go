// Package main 演示 errorx 的基础用法。
//
// 运行：go run ./example
package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/yuninks/errorx"
	"github.com/yuninks/langx"
)

// 1) 定义错误常量：key + 业务码 + 默认文案。
// 占位符写成 #name#，渲染的时候再替换。
var (
	ErrUserNotFound = errorx.New("user_not_found", 10001, "用户不存在")
	ErrParamInvalid = errorx.New("param_invalid", 10002, "参数错误: #field#")
	ErrDB           = errorx.New("db_error", 50001, "数据库繁忙")
)

func init() {
	// 2) 补充其它语言的文案。
	// errorx.New 已经把默认文案注册进默认语言（langx 默认是 zh），这里只补 en。
	langx.AppendTrans("en", map[string]string{
		"user_not_found": "user not found",
		"param_invalid":  "invalid parameter: #field#",
		"db_error":       "database is busy",
	})
}

func main() {
	demoBasic()
	demoFormat()
	demoI18n()
	demoWrap()
	demoStack()
}

func demoBasic() {
	fmt.Println()
	fmt.Println("=== 1. 定义与取值 ===")

	e := ErrUserNotFound.Err() // Err() 返回 ErrorInterface，它本身就是 error
	fmt.Println("error     :", e)
	fmt.Println("GetCode() :", e.GetCode())
	fmt.Println("GetKey()  :", e.GetKey())

	// 错误常量本身也实现了 error，可以直接 return / 打印
	var raw error = ErrUserNotFound
	fmt.Println("常量当error:", raw)
}

func demoFormat() {
	fmt.Println()
	fmt.Println("=== 2. 占位符 ===")

	fmt.Println("ErrKV     :", ErrParamInvalid.ErrKV("field", "age"))
	fmt.Println("ErrWith   :", ErrParamInvalid.ErrWith(map[string]string{"field": "name"}))
	fmt.Println("MsgKV     :", ErrParamInvalid.MsgKV(context.Background(), "field", "phone"))

	// 错误值是不可变的：派生不会影响错误常量，也不会影响别的请求
	derived := ErrParamInvalid.ErrKV("field", "age")
	fmt.Println("常量未被改:", ErrParamInvalid)
	fmt.Println("派生值    :", derived)
}

func demoI18n() {
	fmt.Println()
	fmt.Println("=== 3. 多语言 ===")

	ctx := context.Background()
	enCtx := langx.SetCtxLang(ctx, "en") // 也可以由 HTTP 头 Accept-Language 决定

	fmt.Println("zh            :", ErrUserNotFound.Msg(ctx))
	fmt.Println("en            :", ErrUserNotFound.Msg(enCtx))
	fmt.Println("固定 en       :", ErrUserNotFound.Err().WithLang("en"))
	fmt.Println("未注册语言回退:", ErrUserNotFound.Msg(langx.SetCtxLang(ctx, "ja")))
}

func demoWrap() {
	fmt.Println()
	fmt.Println("=== 4. 包装底层错误 ===")

	sqlErr := errors.New("dial tcp 127.0.0.1:3306: connect: connection refused")
	err := ErrDB.Wrap(sqlErr)

	fmt.Println("对外文案  :", err) // 只暴露业务文案，不泄露底层细节
	fmt.Println("errors.Is :", errors.Is(err, ErrDB))
	fmt.Println("errors.Is :", errors.Is(err, sqlErr))
	fmt.Println("底层错误  :", errors.Unwrap(err))

	var ei errorx.ErrorInterface
	if errors.As(err, &ei) {
		fmt.Println("errors.As :", ei.GetCode())
	}
}

func demoStack() {
	fmt.Println()
	fmt.Println("=== 5. 调用栈（错误日志输出）===")
	err := loadConfig()
	fmt.Printf("%+v", err)
	fmt.Println()
}

func loadConfig() error {
	return errorx.ErrInternal.Wrap(errors.New("config.yaml: no such file or directory"))
}
