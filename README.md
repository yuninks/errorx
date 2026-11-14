# errorx

把「业务错误码 + 多语言文案 + 占位符」封装成**不可变**的错误常量，并补齐 Go 错误处理的常用能力。

```go
var ErrUserNotFound = errorx.New("user_not_found", 10001, "用户不存在")

err := ErrUserNotFound.Wrap(sql.ErrNoRows) // 保留底层错误，不泄露给用户
fmt.Println(err.GetCode(), err.Msg(ctx))   // 10001 用户不存在
fmt.Println(errors.Is(err, ErrUserNotFound), errors.Is(err, sql.ErrNoRows)) // true true
```

## 特性

| 能力 | 说明 |
| --- | --- |
| 一行定义 | `errorx.New(key, code, defaultValue)` |
| 多语言 | 文案交给 langx，支持 ctx 传语言、固定语言、未注册语言回退 |
| 并发安全 | 错误值不可变，包级常量可被任意 goroutine 并发使用 |
| 不丢上下文 | `Wrap` 保留底层错误，`errors.Is / As / Unwrap` 全部可用 |
| 可排查 | 创建错误时抓调用栈，日志用 `%+v` 直接打印原因与栈 |
| 不伪装成功 | 未注册的 key 返回 500 + 原始 key，绝不把缺失的文案伪装成成功 |

## 安装

```bash
go get github.com/yuninks/errorx
```

## 快速开始

```go
package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/yuninks/errorx"
)

// 1) 定义错误常量：key + 业务码 + 默认文案（占位符写成 #name#）
var (
	ErrUserNotFound = errorx.New("user_not_found", 10001, "用户不存在")
	ErrParamInvalid = errorx.New("param_invalid", 10002, "参数错误: #field#")
)

func init() {
	// 2) 补充其它语言的文案；默认语言（zh）的文案 errorx.New 已经注册好了。
	// langx v0.1.1 起注册/取值都是 *langx.Instance 的方法，
	// errorx 通过 Langx() 暴露内部共享实例。
	if err := errorx.Langx().AppendMsg("en", map[string]string{
		"user_not_found": "user not found",
		"param_invalid":  "invalid parameter: #field#",
	}); err != nil {
		panic(err)
	}
}

// 业务层：只管 return error
func loadUser(ctx context.Context, id string) error {
	if id == "" {
		return ErrParamInvalid.ErrKV("field", "id")
	}
	if id == "404" {
		return ErrUserNotFound.Wrap(errors.New("sql: no rows in result set"))
	}
	return nil
}

func main() {
	// ctx 语言要写在 errorx 共享的 langx 实例上，errorx 渲染时才读得到
	ctx := errorx.Langx().SetCtxLang(context.Background(), "en")
	err := loadUser(ctx, "")

	var ei errorx.ErrorInterface
	if errors.As(err, &ei) {
		fmt.Println(ei.GetCode(), ei.Msg(ctx)) // 10002 invalid parameter: id
	}
	fmt.Println(errors.Is(err, ErrParamInvalid)) // true
}
```

更多可运行示例：

```bash
go run ./example          # 基础用法：定义、占位符、多语言、包装、堆栈
go run ./example/http     # HTTP 服务：业务层 return error，出口统一转 {code, msg}
go run ./example/imports  # langx 资源加载的三种方式与 errorx 的配合
```

## 核心概念

- **ErrorLanguage**：错误常量，`errorx.New` 的返回值。不可变的值类型，可直接作为包级变量。
  它本身实现了 `error`，所以 `var err error = ErrUserNotFound` 成立，`errors.Is(err, ErrUserNotFound)` 按 key 比较。
- **ErrorInterface**：错误值接口，`Err() / ErrWith() / Wrap()` 返回它；它本身就是 `error`，取 code 不需要类型断言。

### 与 langx 实例的关系

langx v0.1.1 起取消了包级全局状态，注册、取值、ctx 语言都是 `*langx.Instance` 的方法。
errorx 维护了一个进程内共享实例，并把它暴露给调用方：

| API | 说明 |
| --- | --- |
| `errorx.Langx() *langx.Instance` | errorx 使用的共享实例（懒初始化），资源加载、`SetCtxLang` 都写在这个实例上 |
| `errorx.SetLangx(inst)` | 切换到调用方自己的 langx 实例；已定义的错误常量无需重建 |

约定：

- **ctx 语言要写在同一个实例上**：`errorx.Langx().SetCtxLang(ctx, "en")`。
  langx 的显式 ctx 语言以实例指针作为 context 命名空间，写到别的实例上 errorx 读不到；
  ctx 里直接放 `Accept-Language`（langx 约定的全局键）不受影响，见 `example/http`。
- **多实例场景**：应用已经有一个 `*langx.Instance` 时，在 `main` 里 `errorx.SetLangx(inst)` 即可共用资源，
  不必把错误文案再注册一遍。
- **未知 key 的兜底码**：共享实例创建时把 langx 的默认码设成 `errorx.CodeUnknown`（500），
  `NewError` 再额外用 `HasMsg / HasCode` 判断 key 是否真的注册过。

### 错误值是不可变的

所有 `WithXxx` 都返回新值，`GetFormat()` 返回副本，派生不会影响原值，也不需要加锁：

```go
base := ErrParamInvalid.Err()
a := base.WithFormatKV("field", "age")
b := base.WithFormatKV("field", "name")

fmt.Println(a, b, base)
// 参数错误: age / 参数错误: name / 参数错误: #field#
```

> 旧版的 `SetFormat / SetFormatKV / SetLang / SetCtx` 已删除：它们会直接修改共享的错误常量，
> 并发场景下会 `fatal error: concurrent map writes`，并且会让不同请求之间互相改写文案。

### 渲染规则

`Msg(ctx)` 依次尝试：固定语言（`WithLang`）→ ctx 里的语言 → 默认语言 → 错误常量自带的默认文案（回填进 langx）。
任何一步失败都不会 panic，也不会返回裸 key；ctx 为 nil 或 `Accept-Language` 不是字符串同样安全。

## API 速查

**定义错误常量**

| 函数 | 说明 |
| --- | --- |
| `errorx.New(key, code, defaultValue)` | 定义错误常量，key 为空会 panic |

**配置 langx 实例**

| 函数 | 说明 |
| --- | --- |
| `errorx.Langx()` | errorx 使用的共享 `*langx.Instance`（懒初始化） |
| `errorx.SetLangx(inst)` | 切换到调用方自己的 langx 实例；传 nil 会 panic |

**构造错误值**

| 方法 | 说明 |
| --- | --- |
| `Err()` | 生成错误值 |
| `ErrWith(map)` / `ErrKV(k, v, ...)` | 带占位符数据 |
| `Wrap(cause)` | 包装底层错误，文案里不会出现它 |
| `errorx.NewError(ctx, key)` | 从 langx 资源构造（code 与文案都来自资源） |
| `errorx.NewErrorWith(ctx, key, format)` | 同上，带占位符 |

**读取与渲染**

| 方法 | 说明 |
| --- | --- |
| `GetCode() / GetKey() / GetDefault() / GetFormat() / GetLang()` | 只读访问器 |
| `Msg(ctx) / MsgWith(ctx, map) / MsgKV(ctx, k, v)` | 渲染文案 |
| `Unwrap() / StackTrace()` | 底层错误 / 调用栈 |

**派生（返回新值）**：`WithFormat` / `WithFormatKV` / `WithLang` / `Wrap`

> 命名只有两种后缀：`With(map)` 与 `KV(k, v, ...)`。常量上的 `Err*` 只负责"造错误"，造出来的值用 `With*` 派生。

**内置常量**：`Success`(200) / `Error`(400) / `ErrWithMsg`(400) / `ErrInternal`(500) / `ErrUnregistered`(500)

## 多语言

- **语言来源**：`errorx.Langx().SetCtxLang(ctx, "en")`，或 ctx 里的 `Accept-Language`（langx 约定），见 `example/http`。
- **资源加载**：`errorx.Langx().AppendCode / AppendMsg`（追加）、`RegisterEmbed`（打包）、`RegisterDir`（目录），见 `example/imports`。

> **注意两种注册语义（langx v0.1.1 起，方法都在 `errorx.Langx()` 上）：**
> - `RegisterCode` / `RegisterMsg`：**整体替换**对应的注册表，会把 `errorx.New` 在 init 阶段追加的内置文案冲掉；
> - `RegisterEmbed` / `RegisterDir` / `RegisterFS`：**按键合并**，不影响已注册的资源。
>
> 即使被整体替换，错误常量自带 code 与默认文案，查不到时会自动回填，不会退化成裸 key；
> 但直接用 `errorx.Langx().GetCodeMsgf` 读这些 key 仍可能看到裸 key —— 业务代码建议统一用 `Append*`。

## 用资源文件定义错误

code 与文案都放在 json 资源里时，用 `NewError / NewErrorWith` 动态构造：

```go
errorx.Langx().AppendCode(map[string]int{"biz_fail": 1001})
errorx.Langx().AppendMsg("zh", map[string]string{"biz_fail": "业务失败: #msg#"})

err := errorx.NewErrorWith(ctx, "biz_fail", map[string]string{"msg": "磁盘满了"})
fmt.Println(err.GetCode(), err) // 1001 业务失败: 磁盘满了
```

key 打错时不会静默变成成功：

```go
err := errorx.NewError(ctx, "typo_key")
fmt.Println(err.GetCode(), err) // 500 未注册的错误: typo_key
```

（langx 的 `GetCode` 对未知 key 返回默认码；errorx 用 `errorx.Langx().HasCode` / `HasMsg` 精确判断，
key 没注册就返回 500，资源里只有文案没有 code 时同样用 500 兜底，不会出现"打错 key 反而成功"。）

## 在 HTTP 服务里使用

```go
func toResponse(ctx context.Context, err error) Response {
	var ei errorx.ErrorInterface
	if errors.As(err, &ei) {
		return Response{Code: ei.GetCode(), Msg: ei.Msg(ctx)}
	}
	return Response{Code: errorx.ErrInternal.GetCode(), Msg: errorx.ErrInternal.Msg(ctx)}
}
```

业务层只 `return error`，出口层统一翻译，见 `example/http`。

## 日志

```go
log.Printf("%+v", err)
// 数据库繁忙
//   cause: dial tcp 127.0.0.1:3306: connect: connection refused
//   stack:
//   main.loadUser
//       /app/main.go:47
```

## 并发

错误值不可变、包级常量没有共享可变状态，可以放心在任意 goroutine 使用：

```go
go func() { _ = ErrUserNotFound.Wrap(err) }()
go func() { _ = ErrParamInvalid.ErrKV("field", "age").Msg(ctx) }()
```

> `errorx.New` 会写入 errorx 的常量注册表，建议只在包级变量或 `init` 中调用。

## 从旧版本迁移

| 旧写法 | 新写法 |
| --- | --- |
| `val.SetLang("en")` | `val = val.WithLang("en")` |
| `val.SetFormat(m)` | `val = val.WithFormat(m)` |
| `val.SetFormatKV(k, v)` | `val = val.WithFormatKV(k, v)` |
| `val.SetCtx(ctx)` | `val = val.WithLang(errorx.Langx().GetCtxLang(ctx))` |
| `ErrorWithMsg.Errf(m)` | `ErrWith(m)` |
| `ErrorWithMsg.ErrfKV(k, v)` | `ErrKV(k, v)` |
| `ErrorWithMsg.Msgf(ctx, m)` | `MsgWith(ctx, m)` |
| `errorx.NewLanguage(k, c, d)` | `errorx.New(k, c, d)` |
| `errorx.NewStruct(ctx, k, m)` | `errorx.NewErrorWith(ctx, k, m)` |
| `errorx.NewErrorf(ctx, k, m)` | `errorx.NewErrorWith(ctx, k, m)` |

行为变化：

1. 所有 `Set*` 改为 `With*` 并返回新值；错误值不可变，不再有全局污染与并发崩溃。
2. 错误值不再持有 context，只保存语言标识；渲染时把 ctx 传给 `Msg`。
3. `Err() / ErrWith() / ErrKV()` 返回 `ErrorInterface`（本身就是 `error`），取 code 不再需要类型断言。
4. `ErrorLanguage` 不再实现 `ErrorInterface`：常量的职责只有「定义 + 构造」，读取与派生都用 `Err()` 得到的值。
5. 兼容别名 `Errf / ErrfKV / Msgf / NewLanguage / NewStruct` 与空操作 `Copy()` 已删除，不再保留。
6. 未注册的 key 从「默认码 + 裸 key」变成「500 + 未注册提示」。

### 从 langx v0.1.0（全局单例）升级

langx v0.1.1 移除了包级函数，改为 `*langx.Instance` 的方法（v0.1.0 的 `AppendTrans` 对应 v0.1.1 的 `AppendMsg`）。
errorx 已同步适配：所有对 langx 的读写都通过 `errorx.Langx()` 返回的共享实例进行。

| 旧写法（langx v0.1.0） | 新写法（langx v0.1.1 + errorx） |
| --- | --- |
| `langx.AppendCode(m)` | `errorx.Langx().AppendCode(m)` |
| `langx.AppendTrans(lang, m)` | `errorx.Langx().AppendMsg(lang, m)` |
| `langx.RegisterCode / RegisterTrans` | `errorx.Langx().RegisterCode / RegisterMsg` |
| `langx.RegisterEmbed / RegisterDir / RegisterFS` | `errorx.Langx().RegisterEmbed / RegisterDir / RegisterFS` |
| `langx.SetCtxLang(ctx, lang)` | `errorx.Langx().SetCtxLang(ctx, lang)` |
| `langx.GetCtxLang(ctx)` | `errorx.Langx().GetCtxLang(ctx)` |
| `langx.GetTransFormat(lang, key, m)` | `errorx.Langx().GetCodeMsgf(lang, key, m)` |

如果不想让 errorx 使用自己的共享实例，用 `errorx.SetLangx(appInst)` 把它换成应用已有的实例即可
（显式 ctx 语言必须用同一个实例的 `SetCtxLang` 写入）。

## 目录结构

```
errorx.go      错误常量（ErrorLanguage）、New、从资源构造错误、内置常量
interfaces.go  ErrorInterface 接口与默认实现（不可变错误值、渲染、包装）
stack.go       调用栈采集与解析
example/       可运行示例
```

## 测试

```bash
go test ./...
go test -race ./...   # Windows 需要先装 gcc
```
