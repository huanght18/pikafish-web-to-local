# pikafish-hack

把 `xiangqiai.com` 网页版象棋界面内置 WASM 引擎，替换为本机 `Pikafish` 可执行文件象棋引擎，同时尽量不影响网页原有功能。

## 项目原理

本项目由两部分组成：

1. 浏览器端油猴脚本（`tampermonkey/pkf-local.js`）
2. 本地 WebSocket 服务（推荐使用 `main.py`，另提供 Go 版）

工作流程：

1. 网页初始化 `window.Pikafish` 时，油猴脚本拦截并包装其 `sendCommand`。
2. 网页发出的 UCI 命令优先走 `ws://localhost:8765` 发给本地服务。
3. 本地服务把命令转发给 `pikafish-*.exe`，再把标准输出逐行回传网页。
4. 网页继续使用原有 UI 解析逻辑，因此交互和显示基本保持不变。

## 文件说明

- `tampermonkey/pkf-local.js`：油猴脚本，劫持网页引擎通信并转接到本地 WS。
- `main.py`：Python 版服务端（文件名沿用历史，旧叫 `servepkf2.py`）。每个 WS 连接对应一个独立引擎进程。
- `GoVersion/`：行为对齐的 Go 版，可编译为不依赖 Python 环境的单文件 EXE。
- `start/start.bat`：Windows 快捷启动脚本，会优先使用项目内 `.venv`。
- `icon/`：Go EXE 使用的默认图标资源。
- `test/test_main.py`：Python 配置与命令改写的单元测试。
- `test/test_pkf_local.js`：油猴脚本排队、断线恢复和 WASM 回退测试。
- `ref/servepkf.py`：旧版服务端（单进程共享），保留作对照参考。
- `ref/pikafish-clip.js`：从网页中截取的部分引擎相关代码片段，pkf-local的开发参考。

## 环境要求

- Windows（当前脚本路径和示例以 Windows 为主）
- Python 3.10+
- `websockets` Python 库
- Tampermonkey（或兼容用户脚本扩展）
- 本地 Pikafish 可执行文件（如 `pikafish-bmi2.exe`）

## 快速开始

> 注意：请确定先启动本地服务（python main.py）再加载网页（激活油猴脚本），若网页启动后启动本地服务，则再刷新网页即可

### 1) 安装 Python 依赖

```powershell
python -m pip install -r requirements.txt
```

### 2) 修改引擎路径

首次启动 `main.py` 会**自动在脚本同目录生成 `config.json`**（默认配置），编辑它即可。字段含义与 `GoVersion/config.example.json` 一致：

- `engine_path`：本机 `pikafish-*.exe` 的绝对路径
- `hash_mb`：默认强制为 `512`（这是因为网页代码虽然支持最大为 512MB，但实测大于 384MB 时会只写为 384MB），可在配置里改
- `host`：仅允许回环地址 `localhost`、`127.0.0.1` 或 `::1`
- `port`：`8765`
- `drain_banner`：是否吞掉引擎首行 banner，默认 `true`
- `allowed_origins`：允许连接本地服务的网页来源，默认仅 `https://xiangqiai.com`
- `max_connections`：同时运行的引擎进程上限，默认 `4`
- `log`：日志配置（`console` 输出到终端，`file` 写到 `log.file_path`）

如果你不想编辑 json，**也可以直接改 `main.py` 顶部 `DEFAULTS` 字典里的默认值**。

### 3) 启动本地服务

```powershell
python main.py
```

Windows 也可以直接双击 `start\start.bat`。

看到类似输出即表示服务正常监听：

```text
[Python] ws server listening on ws://localhost:8765
```

### 4) 安装油猴脚本

1. 打开 Tampermonkey，新建脚本。
2. 粘贴 `tampermonkey/pkf-local.js` 的全部内容并保存。
3. 确认 `@match` 包含 `https://xiangqiai.com/*`。

### 5) 打开网页版验证

访问 `https://xiangqiai.com/` 并打开开发者工具：

- 若看到 `[HACK] installed (document-start).` 和 `ws connected`，说明脚本生效。
- Python 终端出现 `[WEB -> ENGINE]` / `[ENGINE -> WEB]` 日志，说明转发正常。

## 设计要点

- 无侵入替换：不改网页源码，只在运行时劫持引擎函数。
- 一致回退：初次连接期间命令先排队；连接失败时整批交给 WASM。中途断线会向 WASM 重放必要状态，避免两个引擎状态分叉。
- 多标签隔离：`main.py` 为每个 WS 连接创建独立引擎进程，减少相互干扰。
- Hash 覆写：检测到 `setoption name Hash value ...` 时，服务端会统一改写为配置中的 `hash_mb`。
- 本地防护：浏览器 Origin 白名单、连接数上限和断线子进程回收。

> 当前注入点是主页面的 `window.Pikafish`，对应网站的多线程引擎路径。若网站回退到在 Web Worker 内初始化的单线程引擎，该会话可能继续使用 WASM。

## 常见问题

### 网页无响应或仍在用 WASM 引擎

- 检查 Python 服务是否已启动并监听 `ws://localhost:8765`。
- 检查浏览器控制台是否有 `[HACK]` 日志。
- 检查油猴脚本是否启用、`@match` 是否正确。
- 确认网站正在使用可由 `window.Pikafish` 注入的多线程引擎模式。

### 无法启动引擎进程

- 确认 `config.json` 的 `engine_path` 真实存在，且可在命令行直接运行。
- 避免路径包含权限受限目录，必要时用管理员权限启动终端。

### 多个网页标签互相影响

- 使用 `main.py`，不要用旧版 `servepkf.py`。

## 安全与风险说明

- 本项目依赖本地开放的 WS 端口（默认仅 `localhost`）。
- 默认只接受 `https://xiangqiai.com` 发起的浏览器连接；本机无 Origin 客户端仍可用于调试。
- 服务端会拒绝绑定非回环地址，避免无意暴露到局域网或公网。
- 用户脚本会接管网页引擎通信，升级目标网站后可能需要适配。

## 免责声明

本项目仅用于技术研究与个人学习，请遵守目标网站的服务条款与当地法律法规。

## 开发检查

```powershell
python -m unittest discover -s test -v
node --check tampermonkey/pkf-local.js
node test/test_pkf_local.js
cd GoVersion
go test ./...
go vet ./...
```
