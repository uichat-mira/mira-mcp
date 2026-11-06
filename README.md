# mira-mcp

Mira 在 Windows 上的本机 MCP 端点。用一个中文登录窗完成 Mira Cloud 账号登录，登录后在本机启动 MCP 服务，供同一账号的移动端调用。

## 状态

- 登录：接入 `cloud-auth` 的真实契约 `POST /v1/auth/login`（`{username,password}` → `{sessionToken,account{id,username},expiresAt}`），错误码与状态码原样显示，不做静默降级。
- 云端地址：`https://cloud-auth.tomzlab.cn`（依据 `cloud-auth` 的 `docs/deployment.md` 拓扑与线上 `/health` 实测 `{"status":"ok","service":"mira-cloud-auth"}`）。
- 凭据：DPAPI 加密后写入 `%APPDATA%\MiraMCP\credentials.bin`，不保存明文 token。
- 本机端点：`http://127.0.0.1:7788/mcp`，仅监听回环地址，要求 `Authorization: Bearer <本机口令>`；口令在登录成功后显示在窗口状态区。
- 窗口关闭后收进托盘，进程继续提供端点；托盘菜单可重新显示窗口或退出。

## 未实现（明确缺口）

1. **云端中继注册**：移动端经 Relay 找到本机所需的 `relayId` / token 由 Desktop 侧配对材料生成，本服务尚未接入该注册流程。当前移动端只能直连可达的 `127.0.0.1:7788`。
2. **设备注册**：`cloud-auth` 提供 `POST /v1/devices` 供客户端登记 `installationId` 与 `platform`，本版未接入，登录时不携带 `device` 上下文。
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
