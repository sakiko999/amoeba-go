# 本机开发与载具环境排障

> 2026-11-20。记录本机 Windows 开发环境、VS Code 调试、Codex 工具权限与
> WebView2 载具的几个已验证结论;这些是环境问题,不是业务代码契约,
> 换机器或升级工具后可能变化。

## VS Code 调试(Delve)

约定:

- 安装 Delve 到 `GOPATH/bin`:`go install github.com/go-delve/delve/cmd/dlv@latest`,
  当前为 `1.27.2`,`C:\Users\Yalla\go\bin\dlv.exe`。
- 工作区显式指定路径,避免 VS Code Go 扩展继承到不同的 `PATH`:
  `.vscode/settings.json:2` 的 `go.dlvPath`。
- `launch.json` 的 `preLaunchTask` 是 `build: 前端`,只构建 chrome 与页面组件;
  Go 侧由 dlv 自己编译启动,不要在前置任务里再跑 `go build`。见
  `.vscode/launch.json:14` 与 `.vscode/tasks.json:9`。
- 调试二进制固定写入 `tmp/debug/mygo-host.exe`(`launch.json` 的 `output`),
  不再使用 dlv 默认的仓库根 `__debug_bin.exe`;`tmp/` 已被忽略,调试产物
  不会污染工作区根目录。
- `program`/`cwd` 都必须是仓库根 `X:\electron-space`;`-pages`/`-vendor` 的默认
  相对路径依赖这个工作目录。

如果报找不到 dlv:先执行 `dlv version` 确认二进制可用,再在 VS Code 里
`Go: Restart Language Server` 或重载窗口;扩展会缓存启动时的环境。

## Codex 工具权限

### `node_repl` 沙箱与线程权限快照

`node_repl` 的沙箱不是 MCP 配置项,也不读取 `[mcp_servers.node_repl]` 的
`sandbox` 字段。app-server 在每次工具调用时把**当前线程的 permission profile**
放进 request metadata,`node_repl` 再据此启动自己的 sandbox。因此:

- 修改 `config.toml` 的 `sandbox_mode`/`default_permissions` 不会热更新已经运行的
  MCP host;线程快照仍是旧的 `managed + restricted`。
- 现象是 `node_repl` 里 `spawnSync('git', ...)` 失败,而普通 `exec` 已经可以运行。
- 处理:完全重启 Codex(不是只新开一个会话),让 MCP host 按当前 Full Access
  重新拉起。重启后 `permissionProfile.type` 应为 `disabled`,`git --version` 可正常
  执行。

如果只是想读取/检索文件,直接用 `exec` 更稳定;不要把 `node_repl` 当 shell 用。

### 为什么一度走 `cmd.exe`

本机默认的 WindowsApps 版 `pwsh.exe` 曾无法启动子进程,报
`CreateProcessAsUserW failed: 5 (拒绝访问)`;显式指定 `powershell.exe` 也仍被解析到
同一路径。当时只有 `cmd.exe` 能稳定启动,属于环境兜底,不是项目要求。
PowerShell 恢复正常后应优先用 `pwsh`;`COMSPEC` 指向 `cmd.exe` 只是 Windows
环境变量,不代表命令必须经过 cmd。

### context-mode 的实际价值

本机 shell 启动受限时,`context-mode` 的沙箱子进程是唯一能读文件的通道,并且
`ctx_resume` 能在服务重启后恢复断点;大输出留档也有用。但它的输出折叠曾把多文件
合并输出裁成只剩最后一个,反而增加重读轮次;token 节省也不应全部算在它头上。
结论:它是 shell 不可用时的执行/存档兜底,不是常规探索的加速器。shell 正常时
直接 `exec` 更简单。

## WebView2 载具

### `msedgewebview2.exe` 弹窗/残留

载具是独立子进程,profile 在 `profile/EBWebView`。`msedgewebview2.exe` 残留会锁住
profile,让下一次冷启动一直等不到 CDP 端口。当前有两层兜底:

- 载具子进程创建后加入 `KILL_ON_JOB_CLOSE` 的 JobObject,关闭 job 句柄时内核回收
  整棵 WebView2 子树,见 `components/service-browser/job_windows.go:3`。
- 停止载具时先关 stdin 让子进程正常退出,2 秒未退出再 `Process.Kill`,最后关闭
  job,见 `components/service-browser/carrier.go:296`。

启动前还会删除旧的 `EBWebView/DevToolsActivePort`,避免把上次进程的端口当成新端口,
见 `components/service-browser/carrier.go:227`。

需要注意:`TOOLWINDOW|NOACTIVATE` 只把 target 顶层窗口从任务栏隐藏、禁止抢焦点,
并不抑制 WebView2 进程自身的崩溃/错误对话框。用户看到的 `msedgewebview2.exe`
弹窗已关闭,本会话没有保存弹窗文本或事件日志,**根因未确认**;不能因为载具默认
隐藏窗口就断定它与载具无关。下次复现时先记录弹窗标题/正文、进程 PID 与父进程,
再对照 `Service.Status()` 的 `ProfileDir`/载具 PID(`components/service-browser/carrier.go:127`),
才能区分是载具 target、宿主页面还是其他
WebView2 应用。测试导致的弹窗可作为优先排查方向,但不要省略证据直接归因。

### 载具资源拦截

拦截在 target attach 后默认启用(`Network.enable` + `Network.setBlockedURLs`),
覆盖登录、搜索和捕获;捕获结束不再 `Network.disable`,见
`components/service-browser/blocked.go:52` 与 `components/service-browser/capture_test.go:70`。
规则细节与各平台取码实测见 `qr-login-research.md`。

## 其他已处理项

- 凭据文件写权限改为 `0o600`(`platform/credentials.go:56`);Windows 不落 ACL,
  依赖 userData 目录本身的用户级权限。
- 根目录构建产物 `/*.exe` 已加入 `.gitignore`。
- Biome 排除上游混淆脚本 `abogus.js` 与抓包样本 `testdata/`,见 `biome.json:12`;
  这不是放宽业务代码的 lint。
- `web/page-vendor/scripts/build.ts:36` 对空模块列表显式报错,避免生成不可用的
  vendor entry。
