# 2026-10-06 修复与防回归说明

审核覆盖 React/TypeScript 前端、Wails Go 后端、共享请求模型、Go CLI、Gio、Cloudflare Worker、Android 壳层及构建/issue 工具。逐项结果见 [issue-progress.md](./issue-progress.md)。

## 历史保留与分页（#58、#24）

旧代码同时限制前端和 Gio 历史为 120 条。前端生成和导入后还调用 `persistTrimmedHistory`，将当前加载列表作为唯一保留清单删除 IndexedDB。即使没有达到 120 条，未加载的其他页也可能被删除。按天分页还会一次读取同一天的全部图片，再经过截断跳过该天剩余记录。

`lib/history.ts` 现在只合并、去重、排序。`storage.ts` 用 `(createdAt, id)` 作为游标，每页有固定上限，并读取下一条确认是否还有更多。生成、导入、Gio 和共享配置均保留完整档案。共享配置导出查询全部持久化记录，导入旧客户端部分快照时合并历史。分页返回后使用最新 store 状态合并，避免覆盖等待期间新生成的结果。

按日期清理通过数据库索引删除指定范围及其全图，覆盖未加载记录；单条删除和日期清理同时移除旧 keyval 副本。全部删除仍使用同一事务清空当前 metadata/full stores，并清理各工作区、详情、对比及待保存队列。

维护约束：

- 视图页、缩略图缓存和归档历史分别管理。可见条数不代表允许删除数据库记录。
- 时间戳不是唯一键；分页、去重和结算均需要记录/任务 ID。
- 生成和配置导入不能自动清理档案；删除只能来自用户明确的单条、日期或全部删除操作。
- 导入/导出必须覆盖未加载记录；共享配置中缺少一条记录不能解释为用户删除它。
- 不恢复 `MAX_HISTORY_ITEMS`、`trimHistory` 或按可见列表运行的 `pruneHistoryStorage`。

回归测试：`historyStorage.test.mjs`、`compatState.test.mjs`、`storageClear.test.mjs`、`historyCleanup.test.mjs` 和 Gio `TestMergeHistoryRetainsLargeBatchAndDeduplicates`。旧版本已删除的提示词没有可恢复来源；修复防止继续丢失，不声称能从图片反推出原记录。

## 失败反馈与队列（#57、#44）

以前只有成功事件触发完成声音，错误事件仅更新工作区错误字段。现在所有生成失败路径经过每次提交独立的 `failureAlert`，首次失败显示 toast、按设置播放任务提示音，并在后台且通知已授权时显示失败通知。取消不发失败提醒。Gio 同步支持，首次错误用原子比较交换去重。音频被宿主拒绝时返回 `false`，可见错误提示仍保留。

`runningJobs` 暂时为空可能只是下一轮任务尚未启动。旧代码在这个间隙将 `jobsTotal` 归零，队列会提前结束；提交拒绝也没有正确递增完成数。`completeWorkspaceJob` 现在按任务 ID 结算、拒绝重复计数，在所有预定任务结束后才清零，并在队列间隙保持忙碌。失败停止循环时仅保留已经启动的任务计数，避免卡住忙碌状态。

维护约束：

- 提交失败、生成错误、结果处理错误都需要反馈，不能只检查成功分支。
- 一个提交共享同一失败提醒 gate；新提交创建新 gate。
- 取消不冒充失败；后台系统通知遵循用户设置和宿主权限。
- 空运行列表不能单独证明队列完成；成功、错误和提交拒绝都结算一次。

回归测试：`failureAlert.test.mjs`、`completionSound.test.mjs`、`workspaceRuntime.test.mjs`、`studioStoreWorkflow.test.mjs` 及 Gio 失败声音/通知测试。store 集成测试调用真实提交逻辑和 runtime 事件，只替换宿主服务，不请求收费上游。

## 原生拖出与浏览器回退（#55、#52）

发现的风险是：五处 UI 从 HTML `dragstart` 取消浏览器拖动，再异步调用 Go `DoDragDrop`，WebView 原生拖动/鼠标捕获退出可能与新 OLE 循环重叠。这与 issue 描述相符，但本机是 macOS，未将推断写成 Windows 实机已证实的根因。

五处入口统一使用 `useNativeFileDrag`：本机文件不再启用 HTML draggable；左键移动超过 5px 后，在当前输入事件结束后启动原生拖动。mouseup、Esc、失焦、卸载都取消尚未启动的手势；错误显示 toast。虚拟路径和 URL 不交给本机文件拖动。

Windows 保留 Unicode `CF_HDROP`、完整路径和双 NUL 文件列表。进入 OLE 前检查真实左键状态，拒绝晚到的手势；退出释放当前线程鼠标捕获。定时器在 30 秒期限到达后向当前 OLE 捕获窗口投递 Esc，退出检查不会只依赖用户再次移动鼠标；没有发送全局键盘输入。此保护不能强制中断任意卡死的第三方 COM 调用。

浏览器回退统一调用 `writeHistoryItemFileDragData`：先写文件格式/清除浏览器默认内容，再写内部历史 MIME。旧顺序会在 `clearData` 时丢掉历史元数据。

维护约束：

- 不从 HTML `dragstart` 调用原生 OLE/NSDragging；五个入口共享同一手势处理。
- 不把 `memory://`、URL 或相对路径当作本机文件。
- 不移除 `CF_HDROP`、双 NUL、退出检查和定时退出；取消只针对当前拖动线程。
- 文件格式写入后保留内部 MIME，避免应用内拖放退化。
- 交叉编译/单测不能替代 OneCommander 和 Windows 全局拖拽实机验收。

微软依据：[DoDragDrop](https://learn.microsoft.com/en-us/windows/win32/api/ole2/nf-ole2-dodragdrop)、[QueryContinueDrag](https://learn.microsoft.com/en-us/windows/win32/api/oleidl/nf-oleidl-idropsource-querycontinuedrag)、[ReleaseCapture](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-releasecapture)、[SetTimer](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-settimer)。

回归测试：`nativeFileDragGesture.test.mjs`、`dragExport.test.mjs`、`native_drag_policy_test.go`、`native_drag_hdrop_test.go`，并编译 Windows amd64/arm64 后端。实机步骤见 [manual-verification.md](./manual-verification.md)。

## 既有功能的重新核对

蒙版、URL 图片结果、Prompt 历史编号、批处理输出命名和 Windows 标题栏 token 已有实现。本次保留请求格式与现有测试，重新测试/构建，不依据旧进展记录跳过验证。

Nano Banana 2 的路由按[官方图像生成说明](https://ai.google.dev/gemini-api/docs/image-generation)和 [Interactions API](https://ai.google.dev/api/interactions-api)核对。Google 原生请求与标准 Images relay 分支分开验证，错误不静默回退。没有用 fixture 成功代替真实 key/模型权限证明。

前端锁文件的 7 项依赖审计问题已更新到现有主版本范围内的修复版。新增测试依赖 `fake-indexeddb`，重新运行依赖审计和五种目标平台构建。

## 本机复验

在仓库根目录执行以下命令。Go 缓存目录可复用现有配置；HTTP mock 需要允许监听 `127.0.0.1`。

```bash
(cd image-studio/frontend && npm ci && npm test)
(cd cloudflare-worker && npm test)
(cd image-studio && go test ./...)
(cd go-cli && go test ./...)
(cd gio-client && go test ./...)
(cd shared/compat-go && go test ./...)
(cd image-studio/frontend && npm run build:windows && npm run build:macos && npm run build:linux && npm run build:android-pad && npm run build:android)
node scripts/verify-local-live-verify.mjs
node scripts/local-smoke-check.mjs
node scripts/verify-issue-close-tooling.mjs
git diff --check
```

Android JVM/APK 和 macOS universal 包按 [build.md](./build.md) 的工具链执行。本轮命令、结果和未执行项见 [verification/issue-audit-20261006.json](./verification/issue-audit-20261006.json)，避免把编译、fixture、真机和真实上游混为同一种验证。
