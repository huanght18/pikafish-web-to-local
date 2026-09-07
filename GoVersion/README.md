# pkf-local-go

`main.py` 的 Go 重写版（原 Python 文件名 `servepkf2.py`），行为与 Python 版**完全对齐**：

- 监听 `ws://localhost:8765`（可在 `config.json` 中修改）
- **每个浏览器标签（每个 WS 连接）独立启动一个 Pikafish 引擎子进程**
- 吞掉引擎启动 banner
- 强制覆写 `setoption name Hash value …` 为配置里的 `hash_mb`
- 引擎 EOF 时回发 `info string engine exited`
- WS 断开时 `Kill` 引擎进程并回收
- 浏览器 Origin 白名单和同时运行的引擎数量限制
- 服务退出时主动关闭 WS，并等待对应引擎完成回收

油猴脚本 `../tampermonkey/pkf-local.js` **无需任何改动**，因为协议沿用"一行一条 text message"。

## 目录结构

```
GoVersion/
├── go.mod              // 模块声明，依赖 gorilla/websocket
├── main.go             // 入口：加载配置 → 注册 handler → 启动 http server
├── config.go           // Config 结构体 + LoadConfig + 默认值 + 自动生成 config.json
├── engine.go           // Engine 封装：Start / DrainBanner / Send / ReadLine / Kill
├── server.go           // WS handler（每个连接双向转发 + 优雅清理）
├── logger.go           // 根据 LogConfig 接管全局 log 输出
├── server_test.go      // Origin、Hash 改写和配置校验测试
├── config.example.json // 示例配置（首次启动也会自动写到 config.json）
└── README.md           // 本文件
```

## 配置：`config.json`

exe 启动时会读**同目录**下的 `config.json`。文件不存在则自动生成一份默认配置（字段值与 `config.example.json` 一致），方便直接编辑。

字段含义：

| 字段 | 类型 | 说明 |
|---|---|---|
| `host` | string | WS 监听地址；仅允许 `localhost`、`127.0.0.1` 或 `::1` |
| `port` | int | WS 监听端口，需与 `../tampermonkey/pkf-local.js` 中 `WS_URL` 一致 |
| `engine_path` | string | Pikafish 引擎绝对路径（**必填**），例如 `C:\path\to\pikafish-bmi2.exe` |
| `hash_mb` | int | 强制覆写的 Hash 大小（MB）。网页对 >384MB 只会传 384，这里统一改成目标值 |
| `drain_banner` | bool | 是否吞掉引擎启动后的第一行 banner；通常 `true` |
| `allowed_origins` | string[] | 允许发起连接的网页 Origin；默认仅 `https://xiangqiai.com` |
| `max_connections` | int | 同时运行的引擎进程上限；范围 1–32，默认 4 |
| `log.console` | bool | 是否输出到终端 |
| `log.file` | bool | 是否额外写到日志文件 |
| `log.file_path` | string | 日志文件在 `logs/` 内的相对路径 |

命令行 `-host` / `-port` 可临时覆盖配置文件的对应字段。

开启 `log.file` 后会自动创建 EXE 所在目录下的 `logs/`。例如
`file_path: "pkf-local-go.log"` 最终写入 `<EXE目录>/logs/pkf-local-go.log`；
绝对路径和跳出该目录的 `..` 路径会被拒绝。

## 编译

```powershell
cd GoVersion
go mod tidy
go build -ldflags "-s -w" -o pkf-local-go.exe .
```

生成 `GoVersion\pkf-local-go.exe`，**双击即可运行**，无需再装 Go。
exe 首次启动会在同目录生成 `config.json` 后退出，编辑后再次双击即生效。

## 设置图标

Go 工具链本身不内置图标嵌入，需要借助 `akavel/rsrc` 把 `.ico` 转成 `.syso`，
`go build` 会自动 pick up 同目录的 `.syso` 并链接进 exe。

```powershell
# 一次性安装 rsrc（全局工具）
go install github.com/akavel/rsrc@latest

# 用项目根目录下的图标生成 syso（也可以用任何 .ico 文件）
D:\GoPath\bin\rsrc.exe -ico "..\icon\bitbug_favicon_x128.ico" -o pkf-local-go.syso

# 重新 build —— syso 自动嵌入 exe
go build -ldflags "-s -w" -o pkf-local-go.exe .
```

注意：

- `.syso` 必须放在 main 包同目录（`GoVersion/`）。
- 一个 main 包只能有一个 `.syso`。
- 图标是 build 时嵌入，复制 `.syso` 到已有 exe **不会生效**，必须重新 build。
- 文件管理器换图标后偶有缓存未刷新：右键 exe →刷新，或运行 `ie4uinit.exe -show`。
- 想验证是否嵌入成功：

  ```powershell
  $icon = [System.Drawing.Icon]::ExtractAssociatedIcon(".\pkf-local-go.exe")
  $icon.Size   # 不抛异常 = 嵌入成功
  ```

## 直接运行（无需编译）

```powershell
cd GoVersion
go mod tidy
go run .
```

## 与 Python 版对应关系

| main.py（原 servepkf2.py） | pkf-local-go |
|---|---|
| `subprocess.Popen(ENGINE_PATH, ...)` | `engine.StartEngine(cfg.EnginePath)` |
| `p.stdout.readline` 读 banner | `engine.DrainBanner` |
| `proc.stdin.write(msg + "\n")` | `engine.Send(line)` |
| `p.stdout.readline` + `ws.send` | `engine.ReadLine` + `ws.WriteMessage` |
| 精确匹配 Hash `setoption` | `rewriteCommand(line)` |
| `proc.terminate()` | `engine.Kill()`（内含 `Process.Kill` + `cmd.Wait`） |
| `asyncio` 后台任务 + WS handler | 两个 goroutine + `sync.WaitGroup` + `context` |
| `websockets.serve` | `gorilla/websocket` 的 `Upgrader` + `http.ListenAndServe` |

## 平台注意（Windows）

- `cmd.Stderr = cmd.Stdout`：把 stderr 合并到 stdout，与 Python 版一致。
- `cmd.StdinPipe / StdoutPipe`：使用 OS pipe，**不经文本模式**，因此写 `line+"\n"` 不会被自动转成 `\r\n`。UCI 引擎两种换行都接受，无需特别处理。
- `Process.Kill()` 在 Windows 上等价 `TerminateProcess`（与 Python `proc.terminate()` 同语义）。
- `cmd.Wait()` 必须在 `Process.Kill()` 之后调用，否则 Windows 上会留僵尸进程 / 句柄。
- `gorilla/websocket` 的 `Upgrader.CheckOrigin` 会校验 `allowed_origins`；无 Origin 的本机原生客户端仍可调试。

## 与 `../tampermonkey/pkf-local.js` 的协议契约

- WS 帧类型：仅 `TextMessage`。
- 每条 UCI 命令作为**一条 text message**，不含 `\n`（服务端 trim 后回填 `\n` 写入引擎）。
- 每条 UCI 输出作为**一条 text message**，不含尾部换行。
- 引擎意外退出：服务端补发 `info string engine exited`（与 Python 版一致）。

## 常见问题

- **油猴脚本回退到 WASM**：本服务未启动、连接超时、端口不一致或 Origin 未在白名单。检查 `[Go] ws server listening on ws://...` 是否出现。
- **`[ERR] start engine: ...`**：`engine_path` 不存在或没有运行权限。改成绝对路径、必要时用管理员权限启动终端。
- **多标签页面同时使用**：每个标签 = 一个独立引擎进程，与 Python 版一致，刷新页面引擎即重建。
- **想清空 Hash 表**：刷新页面即可，引擎子进程随 WS 断开被 Kill 掉。

## 开发检查

```powershell
go test ./...
go vet ./...
```
