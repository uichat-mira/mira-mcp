# mira-mcp

Mira 在 Windows 上的本机 MCP 端点。用一个中文登录窗完成 Mira Cloud 账号登录，登录后在本机启动 MCP 服务。

## 使用

双击 `mira-mcp.exe` 即可，不会出现黑色控制台窗口。

- 登录：`POST https://cloud-auth.tomzlab.cn/v1/auth/login`，契约见 `cloud-auth` 的 `docs/auth-v0.md`。错误码与状态码原样显示，不做静默降级。
- 凭据：DPAPI 加密后写入 `%APPDATA%\MiraMCP\credentials.bin`，不保存明文 token；下次启动自动复用。
- 本机端点：`http://127.0.0.1:7788/mcp`，仅监听回环地址，要求 `Authorization: Bearer <本机口令>`；口令在登录成功后显示在窗口状态区。
- 关闭窗口收进托盘，进程继续提供端点；托盘菜单可重新显示窗口或退出。

## 未实现（明确缺口）

1. **跨设备发现**：本机端点目前只能由同机或可达 `127.0.0.1:7788` 的客户端访问。云端设备发现/中继协议尚未接入，移动端还无法远程找到这台机器。
2. **设备注册**：`cloud-auth` 提供 `POST /v1/devices` 供客户端登记 `installationId` 与 `platform`，本版登录不携带 `device` 上下文。
3. **工具面**：只暴露 `mira_ping` 与 `mira_whoami`，用于验证链路。

## 构建

CI 在 `windows-latest` 上执行 `go vet`、`go test`、`go build -trimpath -ldflags "-H windowsgui -s -w"`，上传构建产物并发布到 `mira-latest` Release，同时在 job summary 中记录实际体积。

本地构建：

```powershell
$env:CGO_ENABLED = '0'
go build -trimpath -ldflags "-H windowsgui -s -w" -o mira-mcp.exe .
```

`-H windowsgui` 让程序以 GUI 子系统启动，因此没有控制台窗口；启动期错误改用系统消息框呈现。

## 下载

```text
https://github.com/uichat-mira/mira-mcp/releases/download/mira-latest/mira-mcp.exe
```

未做代码签名，首次运行 SmartScreen 会提示；这是已知限制，不通过绕过方式规避。
