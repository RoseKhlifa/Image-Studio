# Issue 进展记录

更新时间：2026-07-05

GitHub issue 状态已按 2026-07-05 当天的 GitHub API 重新核对。

## 2026-07-05 当前 open issue 补充

本轮重新核对到当前仍 open 的 issue 为 `#14` `#24` `#30` `#44` `#49` `#50` `#51` `#52` `#53`。

| 编号 | 标题 | 当前仓库状态 | 下一步 |
|---|---|---|---|
| `#53` | 支持发送蒙版的 API 参数 | 已补主前端导入蒙版图片入口；提交时复用现有 `maskB64`，Responses 与 Images API 都会发送蒙版 | 可在真实 Images edits / Responses edit 上游做一次手工验证后关闭 |
| `#52` | OneCommander 拖拽结果图片另存为异常 | 已补 Windows 原生 OLE 文件拖拽实现，前端拖拽数据也改成本地文件路径优先，避免落成 `.url` | 需要 Windows + OneCommander 实机拖拽验证 |
| `#51` | 兼容 Google nano banana2 | 已支持 Google 官方 OpenAI 兼容 base URL 形态；`gemini-*` / `imagen-*` 图像模型自动走非流式 Images API 兼容请求，模型列表也能识别为图像模型 | 用 Google 官方 key 验证 `https://generativelanguage.googleapis.com/v1beta/openai` + `gemini-3.1-flash-image` |
| `#50` | Images API 返回 URL 生图兼容 | 已支持 `data[0].url` 下载并转为 base64，桌面 Go client 与远程内核 JSON 路径都覆盖 50MB 上限和图片类型检查 | 可用返回 URL 的中转站做一次真实生成 |
| `#49` | 模板/历史里的历史条数混乱 | 同提示词分组卡片显示序号范围，详情弹窗继续保留单项序号 | 可关闭 |
| `#44` | 添加批处理功能 | 批处理已有基础实现；本轮补可编辑输出文件名前缀，保留原文件名并继续自动去重 | 可关闭；更复杂命名模板可作为新增强 issue |
| `#30` | 标题背景与标题栏颜色不一致 | 仍沿用 6 月结论：代码已修，缺 Windows 真机视觉确认 | 需要 Windows 真机确认 |
| `#24` | 支持全部删除 | 当前历史栏已有“全部删除”入口，仍可关闭 | 可关闭 |
| `#14` | 有 Web 端吗 | 仍沿用 6 月结论：无独立在线 Web 版，当前交付形态是桌面端和 Android 壳层 | 保持搁置或评论说明后关闭 |

## 全部 issue 总表

下表保留 2026-06-07 历史快照，只统计 GitHub 上真正的 issue，不包含 PR 编号。当前 open issue 以本页顶部 2026-07-05 补充为准。

| 编号 | 标题 | GitHub 状态 | 当前仓库状态 | 下一步 |
|---|---|---|---|---|
| `#1` | Releases v0.1.0 里没有包含 `image-studio-windows-amd64.exe` | closed | 已关闭 | 无 |
| `#2` | 比例增加 auto / 自定义分辨率 | closed | 已关闭 | 无 |
| `#3` | 无法识别透明背景参考图 | closed | 已关闭 | 无 |
| `#4` | 无法并发生图 | closed | 已关闭 | 无 |
| `#9` | 多张出图只会保存一张 | closed | 已关闭 | 无 |
| `#10` | 无法右击已生成的图片 | closed | 已关闭 | 无 |
| `#11` | 点击全屏按钮会黑屏 | closed | 已关闭 | 无 |
| `#12` | `gpt-image-2` 也会优化提示词且无法关闭 | closed | 已关闭 | 无 |
| `#13` | `/v1/images/edits` 的 mask MIME 类型问题 | closed | 已关闭 | 无 |
| `#14` | 有 web 端吗 | open | 已明确“无独立在线 Web 版”，当前目标也已搁置 | 保持搁置，不继续推进 |
| `#15` | 点击历史图后提示词保留可复制修改 | closed | 已关闭 | 无 |
| `#18` | 点了“比例/分辨率自动”后无法再更改 | closed | 已关闭 | 无 |
| `#20` | macOS 版本无法删除配置 | closed | 已关闭 | 无 |
| `#21` | PC 端 log 文件自动清除 | closed | 已关闭 | 无 |
| `#22` | relay 已支持 `b64_json` 但程序提示不支持 | closed | 已关闭 | 无 |
| `#23` | 不生图 | open | 按当前任务约定视为上游问题，不纳入本轮收口 | 保持不处理 |
| `#24` | 支持全部删除 | open | 已有代码覆盖 | 如允许维护 issue，可直接关闭或补评论 |
| `#25` | 上游配置升级后的同步问题 | open | 已有代码覆盖 | 如允许维护 issue，可直接关闭或补评论 |
| `#26` | 失败重试开关 | open | 已有代码覆盖 | 如允许维护 issue，可直接关闭或补评论 |
| `#27` | 上游失败重试路由功能 | open | 已有代码覆盖 | 如允许维护 issue，可直接关闭或补评论 |
| `#28` | 保存用户自定义提示词模板 | open | 已有代码覆盖 | 如允许维护 issue，可直接关闭或补评论 |
| `#29` | 生图完成时触发提示音 | closed | 已关闭 | 无 |
| `#30` | 标题背景与标题栏颜色不一致 | open | 代码已修，但仍缺 Windows 真机视觉确认 | 需要 Windows 真机手工验证 |
| `#32` | 拖结果到文件复制 | open | 已有代码覆盖 | 如允许维护 issue，可直接关闭或补评论 |
| `#33` | 点击参考图缩略图无法查看大图 | closed | 已关闭 | 无 |
| `#34` | 应用参数未包含参考图 | closed | 已关闭 | 无 |
| `#35` | 工作区滚动方向改为横向 | open | 已有代码覆盖 | 如允许维护 issue，可直接关闭或补评论 |
| `#36` | 多图并发时模糊 / 只出轮廓 | open | 核心修复已落地，但仍缺真实上游与真机长链路证明 | 需要 Android / Windows 真机 + 真实高并发上游复核 |
| `#37` | 结果多图希望能左右键浏览 | open | 已有代码覆盖 | 如允许维护 issue，可直接关闭或补评论 |
| `#40` | 多图张数按钮选择 | open | 已有代码覆盖 | 如允许维护 issue，可直接关闭或补评论 |
| `#42` | 一些图生图 / 模板 / 对比功能需求 | open | 已有代码覆盖 | 如允许维护 issue，可直接关闭或补评论 |

这份文档只记录当前仓库代码与本地验证状态，不代表 GitHub issue 已关闭。

## 按状态分组

### 已关闭的 issue

`#1` `#2` `#3` `#4` `#9` `#10` `#11` `#12` `#13` `#15` `#18` `#20` `#21` `#22` `#29` `#33` `#34`

这些 issue 当前 GitHub 状态已经是 `closed`，本地不需要再补额外动作。

### GitHub 仍 open，但当前代码已覆盖

`#24` `#25` `#26` `#27` `#28` `#32` `#35` `#37` `#40` `#42`

这批 issue 的共同特点是：

- 当前仓库已经有对应实现。
- 本地验证链已经覆盖到主要行为。
- 没有新的外部前置条件阻塞。

如果允许直接维护 GitHub issue，优先建议先清这一批，避免后续重复盘点。

可直接复用的评论模板见 [issue-close-comments.md](./issue-close-comments.md)。

当前这批 open issue 还额外有一层本地工具链保证：

- 数据源：`scripts/issue-close-data.json`
- helper：`scripts/issue-close-helper.mjs`
- 独立校验：`scripts/verify-issue-close-tooling.mjs`

截至 2026-06-07 的最新本地实证结果：

- `closable = 10`
- `holdOpen = 2`
- `deferred = 2`
- `unexpectedOpen = 0`
- 最新总链结果目录：
  - `/private/tmp/verify-results-issue-close-final4/platform-kernel-summary.json`
  - `/private/tmp/verify-results-issue-close-final4/issue-close-tooling.json`
  - `/private/tmp/verify-results-issue-close-final4/issue-close-export-bundle/manifest.json`

另外，本地 issue 收口工具链已经在 mock GitHub 环境里补了两类真实执行证明：

- `comment-only` 会发评论，但不会关闭 issue。
- `comment-and-close` 会发评论，并发送关闭 issue 的 patch。
- 无 `GITHUB_TOKEN` / `GH_TOKEN` 时，独立校验会默认切到离线模式，不再因为 GitHub rate limit 打断本地验证。

也就是说：

- 当前建议关闭的 open issue 仍然就是这 10 个。
- 需要继续保留 open 的仍然只有 `#30` 和 `#36`。
- 目前没有冒出新的、未被本地分组覆盖的 open issue。

## 已有代码覆盖

### `#42` 一些功能需求

当前代码已覆盖：

- 参考图存在时，图生图默认切到 `Auto` 比例。
- 会根据参考图比例推导最接近的 2K / 4K 分辨率。
- 支持自定义提示词模板与快捷填入。
- 结果图与参考图对比查看。

主要落点：

- `image-studio/frontend/src/components/panel/sizeCapabilities.ts`
- `image-studio/frontend/src/components/panel/PromptTemplateManagerModal.tsx`
- `image-studio/frontend/src/components/canvas/CompareOverlay.tsx`

### `#40` 多图张数按钮选择

Android 端多图张数已从滑杆改为带边界的按钮选择。

主要落点：

- `image-studio/frontend/src/platform/android/AndroidPhoneComposePanel.tsx`
- `image-studio/frontend/src/platform/android/AndroidPadComposePanel.tsx`

### `#37` 结果里生成的多张图片查看问题

当前代码已支持在画板里对同批结果做左右切换浏览。

主要落点：

- `image-studio/frontend/src/components/canvas/useCanvasShortcuts.ts`
- `image-studio/frontend/src/state/studioStore.media.ts`

### `#35` 工作区滚轮改横向

工作区滚轮方向已改为横向滚动。

主要落点：

- `image-studio/frontend/src/components/layout/WorkspaceBar.tsx`

### `#32` 拖结果到文件复制

桌面端结果图已支持拖出到文件管理器复制。

主要落点：

- `image-studio/backend/dialogs.go`
- `image-studio/frontend/src/lib/dragExport.ts`

### `#29` 生图完成时触发提示音

已支持：

- 完成提示音开关。
- 默认 / 自定义提示音切换。
- 预览提示音。
- 整批任务完成时只播放一次。

主要落点：

- `image-studio/frontend/src/lib/completionSound.ts`
- `image-studio/frontend/src/components/panel/SettingsPanel.tsx`
- `image-studio/frontend/src/platform/android/settings/AndroidSettingsPanel.tsx`

### `#28` 提示词模板

已支持添加、删除、修改、保存、点击填充提示词模板。

主要落点：

- `image-studio/frontend/src/lib/promptTemplates.ts`
- `image-studio/frontend/src/components/panel/PromptTemplateManagerModal.tsx`

### `#27` 上游失败重试路由

已支持把失败后的重试路由到用户指定的备用上游。

主要落点：

- `go-cli/pkg/client/retry.go`
- `image-studio/frontend/src/components/panel/UpstreamProfileEditor.tsx`

### `#26` 失败重试开关

已支持全局失败自动重试开关。

主要落点：

- `image-studio/frontend/src/components/panel/SettingsPanel.tsx`
- `image-studio/frontend/src/platform/android/settings/AndroidSettingsPanel.tsx`

### `#25` 上游配置升级后的同步问题

已支持上游配置导入 / 导出；兼容状态也会落到宿主侧共享路径。

主要落点：

- `image-studio/frontend/src/lib/upstreamConfigTransfer.ts`
- `image-studio/backend/compatibility.go`

### `#24` 支持全部删除

历史结果已支持全部删除。

主要落点：

- `image-studio/frontend/src/components/history/HistoryRail.tsx`

## 已部分解决，但还缺更强实机证明

### `#36` 多张并发时模糊 / 只出轮廓

当前已做：

- 不再把 `partial_image` 预览当成最终成功结果。
- “只拿到流式预览，没有最终图”会被判定为不完整并进入重试。
- Android native HTTP 桥接做了长超时、精简进度、原始响应落盘、宿主侧提取最终图。
- 新增“流式预览保护”开关：
  - Android：并发 `>=2` 自动关闭流式预览。
  - Android：2K / 4K 大尺寸自动关闭流式预览。
  - Desktop：并发 `>=8` 自动关闭流式预览。
- “生成后保存提示”与“完成提示音”都已有设置入口，并补了回归测试，确认关掉后仍能重新打开。

当前证据：

- 前端测试：`114/114` 通过。
- `go-cli/pkg/client`、`image-studio/backend`、`shared/compat-go` 测试通过。
- 2026-06-07 本地平台总链再次通过：
  - `IMAGE_STUDIO_INCLUDE_ISSUE_CLOSE_VERIFY=1 IMAGE_STUDIO_VERIFY_RESULTS_DIR=/private/tmp/verify-results-issue-close-final4 node scripts/verify-local-platform-kernel.mjs`
  - `platform-kernel-summary.json` 状态为 `passed`，共 `11` 个步骤。
- 总的平台验证链已显式覆盖：
  - 本地 `live verify` smoke
  - `image-studio`
  - `shared/compat-go`
  - `go-cli/pkg/client`
- `live-verify.mjs` 已有本地 smoke 证明：
  - `scripts/verify-local-live-verify.mjs`
  - 会启动仓库内 mock upstream，再跑 direct vs worker parity，确认 `live verify` 脚本本身可执行。
- Android 本地校验脚本可通过：
  - `scripts/verify-local-android-shell.mjs`
  - 会先跑 Android JVM 单测，再检查 debug APK 的 `versionName` / `versionCode`、APK 签名和前端资源注入。
  - 若本机接入设备或模拟器，还可用 `IMAGE_STUDIO_ANDROID_DEVICE_SMOKE=1` 继续做 APK 安装 / 启动 smoke。
- 2026-06-07 的结构化子结果也都已通过：
  - `/private/tmp/verify-results-issue-close-final4/live-verify.json`
  - `/private/tmp/verify-results-issue-close-final4/local-smoke.json`
  - `/private/tmp/verify-results-issue-close-final4/android-shell.json`
  - `/private/tmp/verify-results-issue-close-final4/macos-release.json`
  - `/private/tmp/verify-results-issue-close-final4/issue-close-tooling.json`
- `issue-close-tooling.json` 还会额外沉淀：
  - mock GitHub 下的 `comment-only` / `comment-and-close` 执行结果
  - 导出评论包摘要
  - 是否跳过 GitHub 实时同步
- 本地 smoke / local live verify / Android shell / macOS release 这几条子验证脚本在中途失败时，也会尽量把环境与错误原因写进各自 JSON 结果，避免只剩控制台日志。

这轮还额外确认了两个与 `#36` 相邻的关键点：

- 设置里新增的“流式预览保护”开关已经在桌面端和 Android 端落地，不再只是策略硬编码。
- 编译后的 macOS App 已完成实际探针联调，确认同 semver core 的 CI 版本不会再误报更新：
  - `runtimeUpdateProbe.hasUpdate = false`
  - `runtimeUpdateProbe.shouldShowUpdate = false`
  - `runtimeUpdateProbe.appUpdateModalOpen = false`

仍缺：

- 真实高并发上游环境下的长链路实机证明。
- Windows 真机 / Android 真机上更大批量、更大尺寸任务的稳定性复核。

## 已修代码，但缺目标平台外观实机确认

### `#30` 窗口标题背景与标题栏颜色不一致

Windows Fluent 标题区和工作区上沿的颜色对齐代码已调整。

主要落点：

- `image-studio/frontend/src/styles/fluent/_windows-theme.css`
- `image-studio/frontend/src/components/layout/WorkspaceBar.tsx`
- `image-studio/frontend/test/windowsThemeParity.test.mjs`

当前额外保证：

- 前端标题区与 workspace 条带现在共用 `--window-titlebar-bg` token。
- 自动化测试会校验这个 token 与 `main.go` 里 Wails 原生标题栏颜色一致，避免后续再漂移。

仍缺：

- Windows 真机视觉确认。

## 按当前范围暂不继续推进

### `#23` 不生图

按当前任务约定，视为上游问题，本轮不继续处理。

### `#14` 有 web 端吗

当前仓库已经明确说明：

- 没有独立在线 Web / SaaS 版。
- 浏览器形态主要用于调试和目标平台预览。

但“独立在线 Web 版”已经被明确后移，当前暂不继续推进。

## 建议的下一步

1. 如果有 Windows 真机，先补 `#30` 外观确认，再决定是否需要继续微调标题区样式。
2. 如果有稳定可复现的真实上游并发环境，重点补 `#36` 的 Windows / Android 实机长链路证明。
   手工验证步骤可直接按 [manual-verification.md](./manual-verification.md) 执行。
3. 如果允许维护 issue 状态，可以把上述“代码已覆盖”的 issue 补评论或直接关闭，避免后续重复盘点。
4. `#14` 等独立在线 Web 版需求，继续保持搁置，直到桌面 / Android 路径完全收口。

## 关单顺序建议

如果下一步允许直接维护 GitHub issue，建议按这个顺序处理：

1. 先关闭“代码已覆盖且不依赖外部条件”的 open issue：
   `#24` `#25` `#26` `#27` `#28` `#32` `#35` `#37` `#40` `#42`
2. 保留 `#30` 与 `#36` 为 open，直到真机 / 真实上游证据补齐。
3. 保持 `#23` 不处理、`#14` 搁置，避免范围重新发散。

## 剩余执行计划

### 阶段 1：清理已完成但未关单的 issue

目标：

- 先把“代码已经落地、当前本地验证已覆盖、且不依赖外部条件”的 open issue 清掉。

建议顺序：

1. `#24` 全部删除
2. `#25` 上游配置导入导出 / 共享
3. `#26` 自动重试开关
4. `#27` 失败重试路由
5. `#28` 提示词模板
6. `#32` 拖结果到文件复制
7. `#35` 工作区横向滚动
8. `#37` 画板内左右切换同批结果
9. `#40` Android 多图张数按钮
10. `#42` 图生图相关需求集合

执行方式：

- 每个 issue 至少补一条评论，附上对应功能点与主要落点。
- 若仓库维护方式允许，评论后可直接关闭。

### 阶段 2：完成 `#30` Windows 真机确认

目标：

- 确认原生标题栏、应用标题区、workspace 条带在 Windows 真机上亮/暗色都无明显断层。

执行方式：

1. 按 [manual-verification.md](./manual-verification.md) 初始化 `30-windows` 手工验证目录。
2. 使用当前构建版本在 Windows 真机截图取证。
3. 通过后给 `#30` 补图和结论，再关闭 issue。

### 阶段 3：完成 `#36` 真实环境长链路验证

目标：

- 验证高并发与大尺寸场景下，“流式预览保护”策略和“最终图完整性优先”策略在真实环境里成立。

执行方式：

1. Android 真机 / 模拟器执行 `36-android` 验证矩阵。
2. Windows 真机执行 `36-windows` 验证矩阵。
3. 尽量使用真实高并发上游，覆盖：
   - 并发 1
   - 并发 2 / 8
   - 2K / 4K
   - 保护开关开 / 关对照
4. 保存截图、raw 响应、日志和结论。
5. 证据充分后再决定是否关闭 `#36`。

### 阶段 4：保持范围稳定

当前明确不进入本轮收口的内容：

- `#23`：按上游问题处理。
- `#14`：独立在线 Web 版继续搁置。

如果后续重新启动这两项，应作为新阶段单独规划，不与当前桌面 / Android 收尾混在一起。
