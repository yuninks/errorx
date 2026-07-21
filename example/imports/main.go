package main

import (
	"context"
	"embed"
	"fmt"

	"github.com/yuninks/errorx"
	"github.com/yuninks/langx"
)

//go:embed lang
var assetsFs embed.FS

func main() {
	langx.InitLangx(
		langx.SetDefaultCode(0),
		langx.SetDefaultLanguage("zh"),
	)

	fmt.Println("=== 方式1：逐条追加（适合少量错误码） ===")
	regByAppend()

	fmt.Println()

	fmt.Println("=== 方式2：embed 导入 JSON 文件（适合中型项目） ===")
	regByEmbed()

	fmt.Println()

	fmt.Println("=== 方式3：目录文件导入（适合部署时外部管理） ===")
	regByDir()
}

// 方式1：逐条追加 — 适合少量错误码或动态注册场景。
func regByAppend() {
	// 先用 langx 注册语言包
	langx.AppendCode(map[string]int{"success": 200})
	langx.AppendTrans("zh-CN", map[string]string{"success": "成功！"})

	// 然后用 errorx.NewCode 创建错误码（会自动调用 langx.Append*）
	loginErr := errorx.NewCode("login_err", 401, "用户名或密码错误")
	langx.AppendTrans("en", map[string]string{"login_err": "Invalid username or password"})

	ctx := context.Background()
	err := loginErr.New(ctx)

	fmt.Printf("  code=%d, msg=%s\n", err.Code(), err.Error())
}

// 方式2：embed 导入 — JSON 文件编译进二进制，适合中型项目。
func regByEmbed() {
	if err := langx.RegisterEmbed(assetsFs); err != nil {
		fmt.Println("embed 导入失败:", err)
		return
	}

	// JSON 中已声明的 error/success，直接用 key 创建 errorx 实例
	ctx := context.Background()

	err1 := errorx.NewError(ctx, "error")
	fmt.Printf("  error: code=%d, msg(zh)=%s\n", err1.Code(), err1.Error())

	err2 := errorx.NewError(ctx, "success")
	fmt.Printf("  success: code=%d, msg(zh)=%s\n", err2.Code(), err2.Error())

	// 带占位符
	err3 := errorx.NewErrorf(ctx, "error", map[string]string{
		"msg": "数据库连接超时",
	})
	fmt.Printf("  error+format(zh): %s\n", err3.Error())

	// 切换到英文
	err4 := errorx.NewErrorf(ctx, "error", map[string]string{
		"msg": "database connection timeout",
	}).WithLang("en")
	fmt.Printf("  error+format(en): %s\n", err4.Error())
}

// 方式3：目录文件导入 — 语言文件放在外部目录，支持运行时热加载。
func regByDir() {
	langx.RegisterDir("./lang")

	ctx := context.Background()

	err := errorx.NewErrorf(ctx, "error", map[string]string{
		"msg": "磁盘空间不足",
	})
	fmt.Printf("  code=%d, msg=%s\n", errorx.CodeFrom(err), err.Error())
}
