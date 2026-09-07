# pkf-web2local 浏览器扩展

这是面向 Chrome 和 Edge 的 Manifest V3 扩展，用来代替 Tampermonkey 自动加载
网页桥接脚本。扩展只在 `https://xiangqiai.com/*` 上运行，不需要额外权限。

## 安装

1. 保留完整的 `extension` 文件夹，不要单独移动其中的文件。
2. Chrome 打开 `chrome://extensions/`；Edge 打开 `edge://extensions/`。
3. 开启“开发者模式”。
4. 点击“加载已解压的扩展程序”，选择本 `extension` 文件夹。
5. 确认 `pkf-web2local` 已启用，然后刷新目标网页。

扩展通过 `document_start` 在网页主执行环境中加载，所以可以在网页创建
`window.Pikafish` 时接管通信。页面底部出现“桥接已加载”状态标签即表示扩展
脚本已注入；“本地已连接”表示已经连到本机服务。

## 使用

1. 启动 Python 版服务，或双击发布包中的 `pkf-web2local.exe`。
2. 保持服务窗口开启。
3. 打开或刷新 `https://xiangqiai.com/`。

如果本地服务不可用，网页会回退到自带的 WASM 引擎。

## 更新与卸载

替换扩展文件后，在扩展管理页面点击该扩展的“重新加载”，再刷新目标网页。
不再使用时，可在同一页面停用或移除扩展。

不要同时启用本扩展和 `pkf-web2local.user.js` 油猴脚本，否则同一页面可能被
重复注入。
