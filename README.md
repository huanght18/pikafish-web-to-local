# pikafish-hack

把 `xiangqiai.com` 网页版象棋界面内置 WASM 引擎，替换为本机 `Pikafish` 可执行文件象棋引擎，同时尽量不影响网页原有功能。

## 项目原理

本项目由两部分组成：

1. 浏览器端油猴脚本（`pkf-local.js`）
2. 本地 Python WebSocket 服务（`servepkf2.py`）

工作流程：

1. 网页初始化 `window.Pikafish` 时，油猴脚本拦截并包装其 `sendCommand`。
2. 网页发出的 UCI 命令优先走 `ws://localhost:8765` 发给本地服务。
3. Python 服务把命令转发给本地 `pikafish-*.exe`，再把标准输出逐行回传网页。
4. 网页继续使用原有 UI 解析逻辑，因此交互和显示基本保持不变。

## 文件说明

- `pkf-local.js`：油猴脚本，劫持网页引擎通信并转接到本地 WS。
- `servepkf2.py`：推荐使用的服务端。每个 WS 连接对应一个独立引擎进程。
- `ref/servepkf.py`：旧版服务端（单进程共享），保留作对照参考。
- `ref/pikafish-clip.js`：从网页中截取的部分引擎相关代码片段，pkf-local的开发参考。

## 环境要求

- Windows（当前脚本路径和示例以 Windows 为主）
- Python 3.10+
- `websockets` Python 库
- Tampermonkey（或兼容用户脚本扩展）
- 本地 Pikafish 可执行文件（如 `pikafish-bmi2.exe`）

## 快速开始

> 注意：请确定先启动本地服务（python servepkf2.py）再加载网页（激活油猴脚本），若网页启动后启动本地服务，则再刷新网页即可

### 1) 安装 Python 依赖

```powershell
pip install websockets
```

### 2) 修改引擎路径

编辑 [`servepkf2.py`](/c:/Users/80563/Desktop/pikafish-hack/servepkf2.py)，并确认以下中的可配置项：

- `ENGINE_PATH`：改成你本机 `pikafish-*.exe` 的绝对路径
- `HASH`：默认强制为 `512`，可自行修改（这是因为网页代码虽然支持最大为512MB，但实测大于384MB时会只写为384MB），而至于线程数可在网页端配置
- `HOST`：默认 `localhost`
- `PORT`：默认 `8765`

### 3) 启动本地服务

```powershell
python servepkf2.py
```

看到类似输出即表示服务正常监听：

```text
[Python] ws server listening on ws://localhost:8765
```

### 4) 安装油猴脚本

1. 打开 Tampermonkey，新建脚本。
2. 粘贴 [`pkf-local.js`](/c:/Users/80563/Desktop/pikafish-hack/pkf-local.js) 的全部内容并保存。
3. 确认 `@match` 包含 `https://xiangqiai.com/*`。

### 5) 打开网页版验证

访问 `https://xiangqiai.com/` 并打开开发者工具：

- 若看到 `[HACK] installed (document-start).` 和 `ws connected`，说明脚本生效。
- Python 终端出现 `[WEB -> ENGINE]` / `[ENGINE -> WEB]` 日志，说明转发正常。

## 设计要点

- 无侵入替换：不改网页源码，只在运行时劫持引擎函数。
- 保留回退：WS 未连接时，脚本会回落到网页原 WASM `sendCommand`。
- 多标签隔离：`servepkf2.py` 为每个 WS 连接创建独立引擎进程，减少相互干扰。
- Hash 覆写：检测到 `setoption name Hash value ...` 时，服务端会统一改写为 `HASH` 常量。

## 常见问题

### 网页无响应或仍在用 WASM 引擎

- 检查 Python 服务是否已启动并监听 `ws://localhost:8765`。
- 检查浏览器控制台是否有 `[HACK]` 日志。
- 检查油猴脚本是否启用、`@match` 是否正确。

### 无法启动引擎进程

- 确认 `ENGINE_PATH` 真实存在，且可在命令行直接运行。
- 避免路径包含权限受限目录，必要时用管理员权限启动终端。

### 多个网页标签互相影响

- 使用 `servepkf2.py`，不要用旧版 `servepkf.py`。

## 安全与风险说明

- 本项目依赖本地开放的 WS 端口（默认仅 `localhost`）。
- 请勿把服务绑定到公网地址，避免被外部访问。
- 用户脚本会接管网页引擎通信，升级目标网站后可能需要适配。

## 免责声明

本项目仅用于技术研究与个人学习，请遵守目标网站的服务条款与当地法律法规。
