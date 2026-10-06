# 功能说明

本文档记录当前软件能力。应用界面展示见 [showcase.md](./showcase.md)，安装包选择见 [packages.md](./packages.md)，首次配置见 [usage.md](./usage.md)。

## 生成能力

- 文生图与图生图，支持多张参考图。
- 桌面端图生图批处理:可在图生图里切换到批处理模式，按目录逐张处理图片，默认保存回原图目录，也可单独指定输出目录。详见 [batch-img2img](./batch-img2img/README.md)。
- 输入图源:文件对话框、拖拽窗口、剪贴板粘贴、历史复用、双击历史项设为源图。
- 参数:Auto 与固定尺寸、Auto / high / medium / low 质量、PNG / JPEG / WebP 输出格式、seed、negative prompt、风格 chip。
- 尺寸语义:比例与分辨率是同一个 `size` 的拆分视图；当比例切到 `Auto` 时，底层尺寸会整体交给上游决定。细节见 [size-auto.md](./size-auto.md)。
- 参数预设:可在提示词上方的独立预设分区里展开查看、直接切换，并通过保存/新建/管理弹窗维护常用尺寸、质量、输出格式、风格与批量张数组合。
- 比例管理:内置常用宽高比，也支持通过「自定义比例」弹窗添加多个用户自定义宽高比并持久化保存；新增比例会立即出现在参数按钮区，并按当前 1K / 2K / 4K 档位自动换算尺寸。
- 双 API 形态:
  - Responses API:POST `/v1/responses`，使用 `image_generation` 工具，SSE 流式接收事件。
  - Images API:POST `/v1/images/generations` 与 `/v1/images/edits`。
- 参数策略:
  - `OpenAI 标准`:只发送官方公开字段。
  - `兼容中转扩展`:额外发送部分 relay 常见扩展字段，例如 seed / negative_prompt。
- prompt 辅助:prompt 历史、内置模板、一键 AI 优化 prompt；Responses API 请求默认要求上游按原始 prompt 生成。

## 画板

- Konva 画布，支持缩放、拖动、双击 fit 与 100% 切换。
- 蒙版:画笔、橡皮、大小滑块、实时半透明叠加。
- 标注:矩形、箭头、自由画笔、文字、颜色选择、选中删除。
- 图像变换:
  - macOS 桌面端优先走 Core Image / Metal。
  - Android、Windows、Linux 与浏览器预览优先走 WebGL 或 canvas 路径，再把结果持久化回宿主可读路径。
  - 不可用时回退 CPU / canvas。
  - 旋转、翻转、裁剪是就地编辑当前画板图，不创建新的生成历史条目。
- 历史对比:Shift + 点击历史项进入左右分屏对比，可拖动分割条。
- 全屏:`Ctrl+Cmd+F`(macOS) / `F11`(Windows/Linux)。

## 历史

- IndexedDB 本地持久化。
- 搜索 prompt、按 mode 筛选、按日期筛选。
- 历史项右键菜单:复制 prompt、复制本地路径、查看 raw 响应、设为源图、用作对比、以此参数重新生成、应用参数但不生成。
- JSON 导入 / 导出，便于跨设备迁移。

## Workspace

- 多 workspace 标签页。
- 每个 workspace 独立保存 prompt、参数、源图、当前图与运行状态。
- macOS 下 `Cmd+N` / `Cmd+W` 新建或关闭；Windows/Linux 下 `Ctrl+N` / `Ctrl+W`。
- 撤销 / 重做覆盖蒙版笔触、标注、清空等画板操作。

## 设置

- 上游配置:API 形态、BASE_URL、API Key、请求模型 ID、连接测试。Responses API 和 Images API 都只填写一个请求模型 ID。
- API Key:
  - 桌面端使用系统安全存储(Keychain / Credential Manager / Secret Service)。
  - Android 壳层使用应用私有 SharedPreferences。
- 主题:深色 / 浅色。
- 字号:小 / 中 / 大。
- 参数编辑:桌面端与 Android 端都共用同一套比例 / 分辨率 / 质量语义，差异只体现在展示形态与触控交互。
- 输出目录选择、打开输出目录、历史导入 / 导出、清除 API Key、清空历史。
- 关于窗口:版本号、AGPLv3 协议、GitHub、Issues。

## 平台能力

| 平台 | UI | 内核与宿主能力 |
|---|---|---|
| macOS | Apple 风格主题 | Wails + Go 本地内核；图像变换优先 Core Image / Metal；本地自签 universal app。 |
| Windows | Fluent 风格主题 | Wails + Go 本地内核；WebView2；图像变换走 WebGL/canvas 或本地持久化回退。 |
| Linux | 通用桌面主题 | Wails + Go 本地内核；依赖 GTK/WebKitGTK；图像变换走 WebGL/canvas 或本地持久化回退。 |
| Android | Material 3 phone/pad 自适应 | WebView 壳层 + 前端远程内核；壳层提供 native HTTP、图片选择、MediaStore 保存、历史导入导出、震动与全屏。 |
| 浏览器预览 | 按目标平台预览 | 主要用于前端调试；文件、保存和 raw 响应通过浏览器能力或内存虚拟路径回退。 |

Android APK 统一构建 `android` 前端目标，运行时根据窗口尺寸和方向切换 phone / pad 壳层，不再分别维护 phone/pad 两套 APK。

## 快捷键

| 快捷键 | 功能 |
|---|---|
| `Cmd+Enter`(macOS) / `Ctrl+Enter`(Windows/Linux) | 提交生成 |
| `Cmd+N` / `Cmd+W`(macOS) | 新建 / 关闭 workspace |
| `Ctrl+N` / `Ctrl+W`(Windows/Linux) | 新建 / 关闭 workspace |
| `Cmd+Z` / `Shift+Cmd+Z`(macOS) | 撤销 / 重做 |
| `Ctrl+Z` / `Ctrl+Shift+Z` / `Ctrl+Y`(Windows/Linux) | 撤销 / 重做 |
| `Cmd+C` / `Ctrl+C` | 复制当前画板图 |
| `Cmd+V` / `Ctrl+V` | 粘贴剪贴板图到画板 |
| `1` / `2` / `3` | 切换拖动 / 蒙版 / 标注工具 |
| `Space` | 按住临时切到拖动 |
| `F` | 重置视图 |
| 双击画板 | fit 与 100% 切换 |
| `Ctrl+Cmd+F`(macOS) / `F11`(Windows/Linux) | 全屏 |
| `[` / `]` | 笔刷大小减 / 加 5 |
| `Esc` | 取消生成、退出对比、清除选中或关闭错误 |
| `Delete` | 删除选中的标注 |
| `Shift` + 点击历史 | 设为对比图 B |
| 双击历史 | 作为源图 |
| 右键历史 | 打开上下文菜单 |
