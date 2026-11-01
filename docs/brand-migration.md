# Proxi 更名与升级

Windows 版统一称为 **Proxi**，与 macOS 版使用同一品牌和图形标识。

- Windows 源码与更新：[proxi-windows](https://github.com/whrss9527/proxi-windows)。
- macOS 源码与更新：[proxi](https://github.com/whrss9527/proxi)。
- 当前 Windows 文件：`Proxi.exe`、`Proxi-arm64.exe`、`Proxi-Setup.exe`。
- 新安装的数据在 `%APPDATA%\Proxi`，设置窗口的缓存使用 `%LOCALAPPDATA%\Proxi\WebView`。

## 原有数据

首次运行时先让旧程序正常退出，再完整复制配置、订阅、节点选择、状态、流量数据和日志。复制全部完成后才启用新目录；原数据作为迁移前备份保留，不自动删除。已有 Proxi 配置时不覆盖。便携模式继续使用程序旁的配置。

只替换程序生成的说明注释；用户自己起的配置名、订阅地址、规则与点击偏好保持原样。已有多机同步目录继续使用，避免因为改名把多台电脑拆成两份不同步的数据。

## 旧版兼容

当前界面、菜单、通知显示名、安装包和主程序均使用 Proxi。旧名称仅保留在迁移识别、历史记录和升级兼容中：

- 老网页链接继续转交给 Proxi，新的链接使用 `proxi://`。
- 隐藏的实例和通知身份保持稳定，旧安装程序能关闭当前实例，原通知偏好也能沿用。
- 旧客户端只认识旧附件名，因此发布提供与 Proxi 内容完全相同的兼容附件；新用户使用 Proxi 文件。

主程序的更新地址始终对应 Windows 仓库，和 macOS 发布分别管理。
