# mira-mcp

Mira 在 Windows 上的本机 MCP 端点。用一个中文登录窗完成 Mira Cloud 账号登录，登录后在本机启动 MCP 服务，供同一账号的移动端调用。

## 状态

- 登录：接入云端既有 `POST /login` 契约（`{username,password}` → `{token,username}`），失败原因原样显示，不做静默降级。
- 凭据：DPAPI 加密后写入 `%APPDATA%\MiraMCP\credentials.bin`，不保存明文 token。
- 本机端点：`http://127.0.0.1:7788/mcp`，仅监听回环地址，要求 `Authorization: Bearer <本机口令>`。
- 窗口关闭后收进托盘，进程继续提供端点；托盘菜单可重新显示窗口或退出。

## 未实现（明确缺口）

1. **云端中继注册**：移动端经 Relay 找到本机所需的 `relayId` / token 由 Desktop 侧配对材料生成，本服务尚未接入该注册流程。当前移动端只能在同机或可达 `127.0.0.1:7788` 时直连。
2. **云端地址来源**：`CloudBaseURL` 取 `https://relay.tomz.io`（依据移动端 relay 规范与 `uichat-mira-relay` README 声明的 hosted endpoint）。`cloud-auth` 的公开登录域名尚未在任一仓库中确定；若该端点不提供 `/login`，界面会直接显示 HTTP 状态码，不会被吞掉。
3. **工具面**：只暴露 `mira_ping` 与 `mira_whoami`，用于验证链路。

## 构建

CI 在 `windows-latest` 上执行 `go vet`、`go test`、`go build -trimpath -ldflags "-s -w"`，把 exe 上传为构建产物并发布到 `mira-latest` Release，同时在 job summary 中记录实际体积。

本地构建：

```powershell
$env:CGO_ENABLED = '0'
go build -trimpath -ldflags "-s -w" -o mira-mcp.exe .
```

## 下载

```text
https://github.com/uichat-mira/mira-mcp/releases/download/mira-latest/mira-mcp.exe
```

未做代码签名，首次运行 SmartScreen 会提示；这是已知限制，不通过绕过方式规避。
