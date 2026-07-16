# Issue 关单评论模板

更新时间：2026-07-10

本文档只覆盖当前 **GitHub 仍 open，但当前仓库代码与本地验证已经覆盖** 的 issue。

适用范围：

- `#24`
- `#44`
- `#49`
- `#50`
- `#51`
- `#53`

统一验证基线：

- 2026-07-10 已重新跑通本地平台总链：
  - `npm test; go test ./... (image-studio, go-cli, gio-client, shared/compat-go); npm run build:windows; npm run build:android; npm run build:macos; ./gradlew :app:testDebugUnitTest; git diff --check`
  - `platform-kernel-summary.json`：`status = passed`
  - 结构化结果：
- 前端测试当前为 `162/162` 通过。

建议用法：

1. 先确认对应功能没有被后续改坏。
2. 复制下面对应 issue 的评论模板。
3. 如仓库维护方式允许，评论后可直接关闭 issue。

也可以直接使用本地 helper：

```bash
node scripts/issue-close-helper.mjs list
node scripts/issue-close-helper.mjs comment 24
node scripts/issue-close-helper.mjs plan
node scripts/issue-close-helper.mjs verify-open
node scripts/issue-close-helper.mjs export .tmp/issue-close-export/manual-check
node scripts/render-issue-close-comments.mjs --write
```

`export` 产物当前会包含：

- `manifest.json`
- `README.md`
- `plan.json`
- `plan.md`
- `issue-24.md` ... `issue-42.md`

如果只是想先确认“当前哪些 issue 会被处理、处理方式是什么”，优先用：

```bash
node scripts/issue-close-helper.mjs plan
node scripts/issue-close-helper.mjs plan 24 25 --comment-only
```

`apply` 默认不会执行。只有显式提供 `--execute`，并且环境里存在 `GITHUB_TOKEN` 或 `GH_TOKEN` 时，才会真正对 GitHub 发评论或关闭 issue。例如：

```bash
node scripts/issue-close-helper.mjs apply 24 --comment-only --execute
node scripts/issue-close-helper.mjs apply all --comment-and-close --execute
```

建议顺序：

1. 先跑 `verify-open` 确认当前 open issue 状态没漂移。
2. 再跑 `plan` 看本次准备处理的 issue 列表。
3. 最后才决定是否真的执行 `apply`。

## `#24` 支持全部删除

Issue: [#24](https://github.com/RoseKhlifa/Image-Studio/issues/24)

```md
这个需求当前仓库已经完整覆盖。

- 历史栏与 Windows 主历史栏都有“全部删除”入口和二次确认。
- 删除会在同一个 IndexedDB 事务中清空 `history` 与 `historyFull`，并清理旧 keyval 历史，未加载分页也不会残留。
- 画布、对比、结果详情、待保存队列和全部 workspace 引用会同步清理；正在加载分页时会先等待既有请求，避免旧记录回写。

主要落点：

- `image-studio/frontend/src/lib/storage.ts`
- `image-studio/frontend/src/state/historyCleanup.ts`
- `image-studio/frontend/src/components/history/HistoryRail.tsx`
- `image-studio/frontend/src/components/history/WindowsHistoryRail.tsx`

最终前端全量测试为 162/162，这个 issue 可以关闭。
```

## `#44` [Feature]: 添加批处理功能

Issue: [#44](https://github.com/RoseKhlifa/Image-Studio/issues/44)

```md
这个需求当前仓库已经覆盖。

桌面端图生图批处理现已支持：

- 选择目录或多张图片作为输入。
- 按指定并发逐张处理。
- 默认回写源目录，或选择独立输出目录。
- 按源图比例自动适配统一分辨率档位。
- 保留原文件名，可编辑输出前缀，并对同名文件自动追加序号。
- 可选失败重试。

主要落点：

- `image-studio/frontend/src/components/panel/BatchProcessSection.tsx`
- `image-studio/frontend/src/state/studioStore.ts`
- `image-studio/backend/service.go`
- `docs/batch-img2img/README.md`

批处理路径已有前后端测试覆盖，这个 issue 可以关闭；更复杂的命名模板可以单独作为增强需求。
```

## `#49` [Feature]: 模板/历史里面的历史条数混乱，不够清晰的看到每一条的内容

Issue: [#49](https://github.com/RoseKhlifa/Image-Studio/issues/49)

```md
这个需求已经完成。

- Prompt 历史改为带 `01 / 02 / ...` 序号和明确分隔线的有序列表。
- Android 使用同一编号语义。
- Gio 客户端同步显示“历史 1 / 历史 2 / ...”，不再把连续 prompt 混成一段。

主要落点：

- `image-studio/frontend/src/components/panel/PromptPopover.tsx`
- `image-studio/frontend/src/lib/promptTemplates.ts`
- `image-studio/frontend/src/platform/android/AndroidPromptTemplateModal.tsx`
- `gio-client/internal/ui/layout_controls.go`

前端与 Gio 聚焦测试均通过，这个 issue 可以关闭。
```

## `#50` [Feature]: 生图可以支持url生图吗

Issue: [#50](https://github.com/RoseKhlifa/Image-Studio/issues/50)

```md
Images API 返回 URL 的兼容已经完成。

- `data[0].url` 会下载并转换为应用内部使用的 base64 图片。
- 桌面 Go client、远程内核和 Android 原生 HTTP 路径均已覆盖。
- Android 下载不再依赖 WebView `fetch`，避免 CDN CORS 阻断。
- 下载统一限制为 50MB，并校验 HTTP 状态、URL 协议和真实 PNG/JPEG/WebP 内容。

主要落点：

- `go-cli/pkg/client/images_api.go`
- `image-studio/frontend/src/platform/runtime/remote-kernel/images.ts`
- `android-shell/app/src/main/java/top/gptcodex/imagestudio/android/AndroidImageStudioBridge.kt`

fixture、Go、前端和 Android 单测均通过，这个 issue 可以关闭。
```

## `#51` [Feature]: 兼容google 的nano banana2

Issue: [#51](https://github.com/RoseKhlifa/Image-Studio/issues/51)

```md
Nano Banana 2 的客户端兼容已经完成。

- Google 官方主机 + `gemini-3.1-flash-image` 会窄路由到官方 `POST /v1beta/interactions`。
- 请求使用 `x-goog-api-key`、`input`、`response_format`、`store=false`，并解析 `steps[].content[]` 的 `image.data` / `image.uri`。
- 第三方 relay 仍走标准 Images API，不会被误送到 Google 原生协议。
- Interactions 报错不会静默回退，便于定位模型权限或协议问题。

官方依据：

- https://ai.google.dev/gemini-api/docs/image-generation
- https://ai.google.dev/api/interactions-api
- https://ai.google.dev/gemini-api/docs/openai

主要落点：

- `go-cli/pkg/client/google_interactions.go`
- `shared/kernel/requestModel.js`
- `image-studio/frontend/src/platform/runtime/remote-kernel/requestPayloads.ts`

请求/响应 fixture 与全量测试均通过，这个 issue 可以关闭。真实模型可用性仍取决于 Google key 权限或第三方 relay 的模型支持。
```

## `#53` [Feature]: 支持发送蒙版的API参数

Issue: [#53](https://github.com/RoseKhlifa/Image-Studio/issues/53)

```md
可选蒙版图片功能已经完成。

- Wails、Android 与 Gio 都可导入和清除蒙版图片。
- 画布绘制蒙版与导入蒙版复用同一 `maskB64` 请求字段。
- Responses API 按 `input_image_mask` data URL 发送。
- Images edits 按 multipart `mask` 文件发送，并保留图片真实 MIME。
- 无效或空蒙版会明确报错，不再静默忽略。

主要落点：

- `image-studio/frontend/src/state/studioStore.ts`
- `image-studio/frontend/src/platform/android/canvas/AndroidCanvasWorkspace.tsx`
- `gio-client/internal/ui/canvas_mask.go`
- `go-cli/pkg/client/images_api.go`
- `shared/kernel/requestModel.js`

前端、Go、Gio 与 Android 单测均通过，这个 issue 可以关闭。官方上游若要求 PNG、同尺寸或 alpha 通道，仍需提供符合模型约束的蒙版。
```

## 暂不建议关闭的 open issue

- `#30`：原生 Wails light/dark 标题栏颜色与 CSS token 已有自动化一致性验证，但仍缺 Windows 真机视觉确认。
- `#52`：五个拖出入口已统一走 Windows 原生 OLE，IDataObject 也显式提供 Unicode CF_HDROP；仍缺 Windows + OneCommander 实机确认。
- `#14`：这是已回答的产品问答；当前没有独立在线 Web 版，也没有可据此实现的功能验收标准。
