## 下载与首次使用

请在下方 **Assets** 中下载名称形如
`pkf-web2local-vX.Y.Z-windows-amd64.zip` 的文件，不要下载 GitHub 自动生成的
`Source code` 压缩包。

1. 将 ZIP 完整解压到一个可写目录，不要直接在压缩包内运行程序。
2. 事先准备好 Pikafish 引擎的 Windows 可执行文件；发布包不包含引擎本体。
3. Chrome 打开 `chrome://extensions/`，Edge 打开 `edge://extensions/`；开启
   开发者模式，选择“加载已解压的扩展程序”，然后选择发布包内的 `extension`
   文件夹。
4. 如需兼容旧方式，也可以安装包内的 `pkf-web2local.user.js`；扩展和油猴脚本
   只能启用一个，避免重复注入。
5. 双击 `pkf-web2local.exe`。
6. 首次运行会根据 `config.example.json` 创建 `config.json`，并提示输入
   Pikafish EXE 的路径。可以粘贴路径或把文件拖入窗口，`/` 和 `\` 均支持。
7. 路径验证成功后服务会继续运行。使用期间请保持该窗口开启，然后访问目标网页。

如需手动修改配置，请编辑自动生成的 `config.json`，不要修改
`config.example.json`。JSON 中的反斜杠需要写成 `\\`，或者直接使用 `/`。

如果启动或连接失败，请先查看程序窗口中的错误。启用 `config.json` 里的
`log.file` 后，文件日志会写入程序所在目录的 `logs/` 文件夹。
