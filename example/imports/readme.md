# langx 资源加载示例

本目录演示 langx 的三种资源加载方式，以及它们与 errorx 的配合。

| 方式 | 函数 | 语义 |
| --- | --- | --- |
| 追加 | `langx.AppendCode` / `langx.AppendTrans` | 按键合并，**业务代码推荐** |
| 打包 | `langx.RegisterEmbed(fs)` | 按键合并，用 //go:embed 把 lang 目录打进二进制，随程序发布 |
| 目录 | `langx.RegisterDir(dir)` | 按键合并，运行时读目录，改文案不用重新编译 |

## 资源文件格式

- `code.json`：`{"key": 业务码}`
- `zh.json` / `en.json`：`{"key": "文案 #占位符#"}`
- 子目录的第一段是语言名：`zh/common.json`、`zh/errors.json` 都会并入 `zh`

## 注意两种注册语义（langx v0.1.0 起）

- `RegisterCode` / `RegisterTrans`：**整体替换**对应的注册表，会冲掉 `errorx.New` 在 init 阶段追加的内置文案；
- `RegisterEmbed` / `RegisterDir` / `RegisterFS`：**按键合并**，不影响已经注册的资源。

即使被整体替换，errorx 的错误常量自带 code 与默认文案，查不到时会自动回填，
所以 `errorx.Error.Err().GetCode()` 永远是 400、文案永远可读，不会退化成裸 key。
但如果直接用 `langx.GetTransFormat` 读这些 key，就会看到裸 key。

运行：

```bash
go run ./example/imports
```
