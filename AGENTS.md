# AGENTS.md

## Language policy

- 默认使用简体中文回答。
- 除非我明确要求英文，否则不要切换英文叙述。
- 代码、命令、报错、API 名称保持原文，不要强行翻译。
- 提问澄清时也使用中文。

## Windows 沙箱下的 Go 工具链

Go 把构建缓存写在 `%LOCALAPPDATA%\go-build`、遥测写在 `%APPDATA%\go`，都在工作区之外。
在 workspace-write 沙箱下执行 `go build` / `go vet` / `go test` 会报：

```
open C:\Users\<user>\AppData\Local\go-build\...: Access is denied.
```

这是沙箱策略拒绝，不是命令本身的问题：直接在 `run_code` 里对同一条 go 命令申请
`sandbox_permissions: danger-full-access` 提权执行即可，不必绕路改写 `GOCACHE`。

`gofmt -l` 会因工作区已检出的 CRLF 行尾把未改动的文件也列出来，判断格式时以自己改动的文件为准。

判断语法是否通过要看 gofmt 的退出码与 stderr：语法错误只写到 stderr，`-l` 的 stdout 会空着，
只看 stdout 会把一个写坏的文件误判成"格式没问题"。
