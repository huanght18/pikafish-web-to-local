# pikafish-web-to-local

把 `xiangqiai.com`（皮卡鱼象棋网页版）内置的 Pikafish WASM 引擎替换为本机
`Pikafish` 可执行文件，同时复用网页原有的棋盘、分析结果和交互界面。

项目提供两套本地服务端：

- **Python 版**：配置和调试方便，是本文档的主要说明对象。
- **Go 版**：可编译为独立 EXE，根目录只介绍使用入口；实现和构建细节请查看
  [GoVersion/README.md](GoVersion/README.md)。

## 工作原理

1. `tampermonkey/pkf-web2local.js` 在网页初始化 `window.Pikafish` 时包装
   `sendCommand`。
2. 网页的 UCI 命令通过 `ws://localhost:8765` 发给本地服务。
3. 本地服务为每个 WebSocket 连接启动独立的 Pikafish 进程。
4. 服务把命令写入引擎 stdin，并将 stdout 逐行送回网页原有解析器。

Python 版和 Go 版使用相同的 WebSocket 协议及油猴脚本，同一时间只需启动其中一个。

## 目录结构

| 路径 | 用途 |
|---|---|
| `main.py` | Python 服务端入口 |
| `config.example.json` | Python 配置模板，使用假的引擎路径 |
| `requirements.txt` | Python 依赖 |
| `start/start.bat` | Python 版 Windows 快捷启动脚本 |
| `tampermonkey/pkf-web2local.js` | 浏览器端油猴脚本 |
| `test/` | Python 与油猴脚本测试 |
| `icon/` | Go EXE 图标资源 |
| `ref/` | 旧服务端及目标网页引擎代码参考 |
| `GoVersion/` | Go 服务端、配置示例、测试和详细文档 |

## 安装油猴脚本

1. 在浏览器安装 Tampermonkey 或兼容扩展。
2. 新建用户脚本，粘贴 `tampermonkey/pkf-web2local.js` 的全部内容并保存。
3. 确认脚本已启用，且 `@match` 包含 `https://xiangqiai.com/*`。

应先启动 Python 或 Go 本地服务，再打开或刷新目标网页。

## Python 版

### 环境要求

- Windows
- Python 3.10+
- 本地 Pikafish 可执行文件，例如 `pikafish-bmi2.exe`

### 安装依赖

```powershell
python -m pip install -r requirements.txt
```

如果项目中已有 `.venv`，可以使用：

```powershell
.venv\Scripts\python.exe -m pip install -r requirements.txt
```

### 首次配置

首次运行：

```powershell
python main.py
```

程序会根据根目录的 `config.example.json` 生成 `config.json`。模板中的
`engine_path` 是假路径，因此首次运行会提示输入真实的 Pikafish 可执行文件路径；
验证文件存在后会自动写入 `config.json` 并继续启动。

路径可以使用 `/` 或 `\`，也可以直接把 EXE 拖入终端。带引号的路径会自动去除
首尾引号。以后如果配置中的引擎路径失效，程序也会重新询问并写回。
如果手动编辑 JSON，单个反斜杠需要按 JSON 语法写成 `\\`；直接使用 `/` 最省事。

| 字段 | 默认值/作用 |
|---|---|
| `engine_path` | Pikafish 可执行文件绝对路径；支持 `/` 和 `\` |
| `host` | `localhost`；仅允许回环地址 |
| `port` | `8765`，需与油猴脚本中的 `WS_URL` 一致 |
| `hash_mb` | `512`；覆盖网页发送的 Hash 大小 |
| `drain_banner` | 是否过滤引擎启动 banner |
| `allowed_origins` | 默认仅允许 `https://xiangqiai.com` |
| `max_connections` | 同时运行的引擎进程上限，默认 `4` |
| `log.console` | 是否输出终端日志 |
| `log.file` | 是否写文件日志 |
| `log.file_path` | `logs/` 内的相对文件路径 |

当 `log.file` 为 `true` 时，程序会自动创建项目根目录下的 `logs/`。
例如 `file_path: "pkf-local-python.log"` 最终写入
`logs/pkf-local-python.log`。绝对路径或通过 `..` 跳出日志目录的配置会被拒绝。

### 启动

```powershell
python main.py
```

也可以双击：

```text
start\start.bat
```

正常启动后会看到：

```text
[Python] ws server listening on ws://localhost:8765
```

浏览器控制台出现 `[HACK] ws connected`，服务端出现
`[WEB -> ENGINE]` 和 `[ENGINE -> WEB]`，即表示本地引擎已接管。

## Go 版

Go 版与 Python 版协议一致，主要适合生成无需 Python 环境的独立 EXE：

```powershell
cd GoVersion
go build -ldflags "-s -w" -o pkf-local-go.exe .
```

使用要点：

- `config.example.json` 是配置模板，`config.json` 位于 EXE 所在目录。
- 首次运行会根据模板生成配置；假路径无效时会在终端询问真实路径并自动保存。
- 交互输入同样支持 `/`、`\` 和带引号的拖入路径；手动编辑 JSON 时
  反斜杠需要写成 `\\`。
- 文件日志写入 EXE 所在目录下的 `logs/`。
- 使用相同的 `tampermonkey/pkf-web2local.js`，不要同时启动 Python 和 Go 服务。

完整的配置字段、编译、图标嵌入、平台注意事项和实现对应关系见
[GoVersion/README.md](GoVersion/README.md)。

## 发布 Go 版

正式发布包由 `.github/workflows/release.yml` 在推送 `v*` 标签时自动生成，
无需向仓库提交 EXE 或 ZIP。例如：

```powershell
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

Release 资产命名为 `pkf-web2local-v0.1.0-windows-amd64.zip`，解压后只有一个
同名目录，其中包含 `pkf-web2local.exe`、`config.example.json`、带版本号的
`pkf-web2local.user.js` 和用户初始化说明。工作流会检查油猴脚本的 `@version` 与
发布标签一致。以后需要随包分发的静态文件可以放入 `GoVersion/release-assets/`；
本机 `config.json`、日志和 `dist/` 构建目录不会提交或发布。

## 关键设计

- **会话隔离**：每个网页连接独占一个 Pikafish 进程，刷新页面即可重建引擎并清空 Hash。
- **一致回退**：初次连接期间先缓存命令；本地连接失败时整批交给网页 WASM。
- **断线恢复**：本地连接中断后，向 WASM 重放必要的 UCI 配置、局面和搜索状态。
- **Hash 覆写**：精确识别 `setoption name Hash value ...`，统一使用配置中的
  `hash_mb`。
- **本地防护**：限制监听地址、浏览器 Origin、消息大小和同时运行的引擎数量。
- **进程回收**：连接断开或服务退出时终止并等待对应的引擎进程。

> 当前注入点是主页面的 `window.Pikafish`，对应网站的多线程引擎路径。
> 如果网站改用在 Web Worker 内初始化的单线程引擎，该会话可能继续使用 WASM。

## 常见问题

### 浏览器仍在使用 WASM

- 确认 Python 或 Go 服务已经启动，且端口与 `WS_URL` 一致。
- 查看浏览器控制台是否出现 `[HACK] installed` 和 `ws connected`。
- 确认油猴脚本已启用且目标网站使用多线程引擎模式。

### 无法启动 Pikafish

- 检查对应 `config.json` 中的 `engine_path`。
- 确认路径指向真实文件，并且当前用户具有运行权限。
- Python 与 Go 的配置文件位置不同，不要混用。

### 连接数量达到上限

每个网页标签都会占用一个引擎进程。关闭不再使用的标签，或在确认内存充足后调整
`max_connections`。

## 开发检查

Python 与油猴脚本：

```powershell
python -m unittest discover -s test -v
node --check tampermonkey/pkf-web2local.js
node test/test_pkf_web2local.js
```

Go：

```powershell
cd GoVersion
go test ./...
go vet ./...
```

## 安全与免责声明

服务端只允许绑定回环地址，默认只接受 `https://xiangqiai.com` 发起的浏览器连接。
不要修改代码把服务暴露到局域网或公网。

本项目仅用于技术研究与个人学习，请遵守目标网站的服务条款与当地法律法规。
