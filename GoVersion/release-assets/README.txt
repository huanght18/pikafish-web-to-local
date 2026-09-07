pkf-web2local - 首次使用说明
==============================

1. 请先把整个 ZIP 解压到一个可写目录，不要直接在压缩包内运行。
2. 本程序不包含 Pikafish 引擎，请事先准备好 Pikafish 的 Windows EXE。
3. Chrome 打开 chrome://extensions/，Edge 打开 edge://extensions/。开启开发者
   模式，点击“加载已解压的扩展程序”，选择包内的 extension 文件夹。
4. 如需兼容旧方式，也可以安装包内的 pkf-web2local.user.js。不要同时启用浏览器
   扩展和油猴脚本，否则可能重复注入。
5. 双击 pkf-web2local.exe。
6. 首次运行会根据 config.example.json 创建 config.json，并询问 Pikafish EXE
   的实际路径。可以粘贴路径或把文件拖入窗口，/ 和 \ 两种分隔符均支持。
7. 路径验证成功后服务会继续运行。使用网页期间请保持程序窗口开启。

配置说明
--------

- 实际配置保存在同目录的 config.json；请勿把其中的本机路径分享给他人。
- config.example.json 是初始化模板，一般不需要修改。
- 手动编辑 JSON 时，反斜杠需要写成两个反斜杠，或者直接使用 /。
- 默认服务地址为 ws://localhost:8765。

故障排查
--------

- 首先查看程序窗口显示的错误信息。
- 如果启用了 config.json 中的 log.file，日志位于同目录的 logs 文件夹。
- 网页没有连接时，请确认浏览器扩展或油猴脚本已启用，并确认没有同时运行
  Python 版和 Go 版服务。
