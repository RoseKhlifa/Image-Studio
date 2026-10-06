# 手工验证手册

本文档只覆盖当前仍然需要外部条件的验证项:

- `#36` 多张并发时模糊 / 只出轮廓
- `#30` Windows 标题栏与标题区颜色一致性真机确认
- `#52` OneCommander 拖出真实图片文件
- `#55` Windows 原生拖动结束后，其他应用仍可正常拖拽

2026-10-06 的本机自动验证与编译结果见 [issue-fixes-20261006.md](./issue-fixes-20261006.md) 和 [验证记录](./verification/issue-audit-20261006.json)。真机验收应引用实际测试版本与证据，不能只引用旧关单评论中的测试数量。

在开始手工验证前，建议先跑一遍当前仓库内可自动完成的验证链：

```bash
IMAGE_STUDIO_SKIP_RUNTIME_UPDATE_PROBE=1 node scripts/verify-local-platform-kernel.mjs
```

如果这一条不过，先不要进入真机/真实上游验证。

如果你需要把当前可自动证明的结果、`#30/#36` 模板和 issue 关单评论包一起整理给另一台机器或另一位验证者，可以先执行：

```bash
node scripts/prepare-external-verification-bundle.mjs
```

默认会生成：

```text
.tmp/external-verify-bundles/<YYYY-MM-DD>/issue-30-36-handoff/
```

开始记录之前，建议先初始化一份本次验证目录：

```bash
node scripts/init-manual-verification.mjs 36-android
node scripts/init-manual-verification.mjs 36-windows
node scripts/init-manual-verification.mjs 30-windows
```

如果不是预设场景，也可以自己给一个标题：

```bash
node scripts/init-manual-verification.mjs custom "my regression check"
```

脚本会在 `.tmp/manual-verify/<YYYY-MM-DD>/<slug>/` 下生成：

- `report.md`
- `meta.json`
- `screenshots/`
- `raw/`
- `logs/`

## 统一取证目录

建议把所有截图、raw 响应和说明统一放到：

```text
.tmp/manual-verify/<YYYY-MM-DD>/
```

建议至少保存：

- 平台与版本信息
- 验证步骤截图
- raw 响应或 log 文件
- 失败时的最短复现说明

## `#36` 高并发与大尺寸最终图完整性

### 目标

确认以下策略在真实环境里成立：

- 不会把流式预览误当成最终图
- 高并发或大尺寸时，自动关闭流式预览后，最终图仍然完整
- Android 壳层在真实设备 / 模拟器上不会因为 partial preview 路径而只留下模糊轮廓

### 前置条件

- 至少一条真实可用上游，且能稳定返回图像
- 已知正常的 `BASE_URL` / `API Key` / 请求模型 ID
- 如果是 Android:
  - 已接入真机或模拟器
  - 可先执行：

```bash
IMAGE_STUDIO_ANDROID_DEVICE_SMOKE=1 node scripts/verify-local-android-shell.mjs
```

若用 MuMu，可参考 [mumu-android-debug.md](./mumu-android-debug.md)。

### 建议矩阵

#### Windows / 桌面端

1. `protectStreamPreview = 开`
2. `partialImages > 0`
3. 做以下组合：

| 组合 | 预期 |
|---|---|
| 并发 1，1024 级尺寸 | 可看到流式预览，最终图完整 |
| 并发 8，1024 级尺寸 | 自动关闭流式预览，最终图完整 |
| 并发 8，2048/4K 级尺寸 | 自动关闭流式预览，最终图完整 |
| 并发 8，关闭保护开关 | 按当前预览帧数请求，用于对照问题是否复现 |

建议每组至少记录：

- 提交前参数截图
- 结果图截图
- raw 响应文件
- 是否出现只剩轮廓 / 明显模糊 / 少帧终止

#### Android 真机 / 模拟器

1. `protectStreamPreview = 开`
2. `partialImages > 0`
3. 做以下组合：

| 组合 | 预期 |
|---|---|
| 并发 1，1024 级尺寸 | 最终图完整 |
| 并发 2，1024 级尺寸 | 自动关闭流式预览，最终图完整 |
| 并发 1，2K/4K 级尺寸 | 自动关闭流式预览，最终图完整 |
| 并发 2，关闭保护开关 | 用于对照老问题是否仍可能复现 |

建议额外记录：

- APK 版本 / `versionName`
- 设备型号或模拟器类型
- 是否发生闪退
- 系统相册 / 输出目录里保存的最终图是否完整

### 通过标准

- 开启保护后，测试矩阵中的“应关闭预览”组合不再出现“只有轮廓/明显模糊但直接被当成功”的情况
- raw 响应里如果没有 final image，不应被应用当作最终成功结果
- Android 上保存到相册或输出目录的最终图与画面显示一致，不是只保存到某一帧 partial preview

## `#30` Windows 标题栏颜色真机确认

### 目标

确认 Windows 真机上的：

- 原生标题栏
- 应用内部标题区
- workspace 条带

在亮色和暗色下没有明显色差或断层。

### 前置条件

- Windows 真机
- 当前构建版本已通过：
  - `image-studio/frontend/test/windowsThemeParity.test.mjs`

### 步骤

1. 启动桌面版应用
2. 切到亮色主题截图一张
3. 切到暗色主题截图一张
4. 如果 workspace 数量 > 1，再补一张带 workspace 条带的截图

### 通过标准

- 标题栏颜色与应用内部标题区不出现肉眼明显断层
- workspace 条带和标题区背景保持同一主题层级
- 亮色 / 暗色切换后都没有出现“外层原生标题栏更浅/更深一截”的问题

## 结果回填建议

### `#52/#55` Windows 拖出与捕获释放

1. 使用本分支构建的 Windows 应用，记录 Windows、WebView2 和 OneCommander 版本。
2. 在画布“拖出复制”、历史缩略图、同提示词历史组、另存提示、结果详情五个入口分别拖一张已保存的图片到 Explorer 和 OneCommander。
3. 确认得到 PNG/JPEG/WebP 图片文件，能直接打开，文件内容与原图一致；不能得到 `.url`。
4. 每次正常松开、Esc 取消、快速按下/松开后，立即在桌面、Explorer 和另一个程序内拖动文件/内容，确认没有遗留鼠标捕获。
5. 保持鼠标不移动超过 30 秒，确认当前拖动取消；之后其他应用仍可拖动。此超时保护针对正常 OLE 消息循环，不代替对卡死第三方 COM 目标的实际检查。
6. 重复 20 次。至少保存成功拖出、取消后其他程序正常拖动的录屏，以及遇到错误时的 toast/日志。

通过标准：五处入口均输出真实图片，正常结束和取消后均恢复其他程序的拖拽；连续使用无挂起。检查完成前，`#52/#55` 保持待实机验证。

### 其他回填

如果允许继续维护 issue 状态，建议在 GitHub issue 回填：

- 平台
- 版本
- 验证矩阵
- 是否通过
- 关键截图 / raw 响应路径

若不直接关闭 issue，至少补一条“已在某平台实机确认/仍待某平台确认”的评论，避免后续重复盘点。
