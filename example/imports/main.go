// Package main 演示 langx 的三种资源加载方式，以及它们和 errorx 的配合。
//
// 运行：go run ./example/imports
package main

import (
	"context"
	"embed"
	"fmt"
	"os"

	"github.com/yuninks/errorx"
	"github.com/yuninks/langx"
)

//go:embed lang
var assetsFs embed.FS

// errorx 定义的错误常量：code 与默认文案都跟着常量走
var (
	ErrSuccess = errorx.New("success", 200, "操作成功")
	ErrError   = errorx.New("error", 400, "操作失败")
)

func main() {
	regByAppend()
	regByEmbed()
	regByDir()
	withErrorx()
}

// 1) Append：零散追加，不会覆盖已有资源，推荐业务代码使用
func regByAppend() {
	fmt.Println("=== 1) Append：追加注册 ===")
	langx.AppendCode(map[string]int{"append_key": 1001})
	langx.AppendTrans("zh", map[string]string{"append_key": "追加的错误"})

	fmt.Println("zh:", trans("zh", "append_key", nil))
	fmt.Println("en:", trans("en", "append_key", nil), "(en 没有，回退到默认语言)")
}

// 2) Embed：//go:embed 把 lang 目录打进二进制
func regByEmbed() {
	fmt.Println()
	fmt.Println("=== 2) Embed：打包资源 ===")
	if err := langx.RegisterEmbed(assetsFs); err != nil {
		fmt.Println("RegisterEmbed:", err)
		return
	}
	fmt.Println("zh:", trans("zh", "success", nil))
	fmt.Println("en:", trans("en", "error", map[string]string{"msg": "disk full"}))
	fmt.Println("第 1 步的 append_key 仍在:", trans("zh", "append_key", nil), "(RegisterEmbed 是合并，不覆盖)")
}

// 3) Dir：运行时从目录读取，适合运维改文案不用重新编译
func regByDir() {
	fmt.Println()
	fmt.Println("=== 3) Dir：读取目录 ===")
	dir := langDir()
	if err := langx.RegisterDir(dir); err != nil {
		fmt.Println("RegisterDir:", err)
		return
	}
	fmt.Println("zh:", trans("zh", "success", nil), "(dir =", dir+")")
}

// 4) 与 errorx 组合。
// langx 有两种注册语义（v0.1.0 起）：
//   - RegisterCode / RegisterTrans：整体替换对应注册表
//   - RegisterEmbed / RegisterDir / RegisterFS：按键合并，不动已有资源
//
// 上面用的是后者，所以 errorx 在 init 阶段追加的内置资源还在。
// 即使用户改用 RegisterCode/RegisterTrans 覆盖掉，错误常量自带 code 与默认文案，
// 也不会退化成裸 key —— code 永远来自常量自身。
func withErrorx() {
	fmt.Println()
	fmt.Println("=== 4) 与 errorx 组合 ===")
	ctx := context.Background()
	enCtx := langx.SetCtxLang(ctx, "en")

	// code 来自错误常量自身，msg 优先用 langx 资源里的文案
	fmt.Println("code          :", ErrError.Err().GetCode())
	fmt.Println("zh(资源优先)  :", ErrError.ErrKV("msg", "磁盘满了").Msg(ctx))
	fmt.Println("en(资源优先)  :", ErrError.ErrKV("msg", "disk full").Msg(enCtx))
	fmt.Println("资源里的 key  :", errorx.NewError(ctx, "error"))
	fmt.Println("成功          :", ErrSuccess.Err().GetCode(), ErrSuccess.Msg(ctx))
	// error_with_msg 没有写进资源文件，回退到错误常量自带的默认文案
	fmt.Println("en(未注册回退):", errorx.ErrWithMsg.ErrKV("msg", "disk full").Msg(enCtx))
}

func trans(lang, key string, format map[string]string) string {
	code, msg := langx.GetTransFormat(lang, key, format)
	return fmt.Sprintf("[%d] %s", code, msg)
}

// langDir 兼容两种运行方式：仓库根目录 go run ./example/imports，或进入目录 go run .
func langDir() string {
	for _, p := range []string{"lang", "example/imports/lang"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "example/imports/lang"
}
