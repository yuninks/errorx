# 语言包导入

errorx 支持三种方式导入多语言资源。

## 方式1：逐条追加（Append）

适合错误码较少、运行时动态注册的场景。

```go
langx.AppendCode(map[string]int{"success": 200})
langx.AppendTrans("zh", map[string]string{"success": "成功"})
```

## 方式2：Embed 导入

适合中型项目，JSON 文件编译进可执行文件。

目录结构：

```
lang/
  code.json    → {"success": 200, "error": 400}
  zh.json      → {"success": "成功", "error": "失败 #msg#"}
  en.json      → {"success": "Success", "error": "Error #msg#"}
```

Go 代码：

```go
//go:embed lang
var assetsFs embed.FS

langx.RegisterEmbed(assetsFs)
```

## 方式3：目录导入

适合部署时外部管理语言包，支持不重启更新。

```go
langx.RegisterDir("./lang")
```

## JSON 格式

- `code.json`：错误码映射 `key → int`
- `{lang}.json`：翻译映射 `key → 翻译文本`，支持 `#placeholder#` 占位符
