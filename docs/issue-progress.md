# Issue 进展记录

更新时间：2026-10-06（Asia/Shanghai）。本次重新读取上游全部 open issue 及评论，共 12 项。旧记录里的“可关闭”结论已重新核对，GitHub 上的 open 状态不等于当前分支缺少实现。

| Issue | 本次核对与处理 | 验证与剩余条件 |
|---|---|---|
| [#58 历史图片不完整](https://github.com/RoseKhlifa/Image-Studio/issues/58) | 移除前端和 Gio 的 120 条截断；取消生成/导入时按可见列表删除数据库；按时间与 ID 分页；导出完整档案，合并旧客户端的部分快照；修复筛选加载及未加载记录的日期清理 | IndexedDB 650 条分页、同时间戳、旧全图保留、共享快照导入/导出测试；已被旧版本删除的提示词不能由图片文件自动恢复 |
| [#57 失败声音提示](https://github.com/RoseKhlifa/Image-Studio/issues/57) | 提交失败、生成失败、处理结果失败均触发可见错误提示；复用任务提示音开关；后台且授权时发失败系统通知；同次提交的并发错误只提醒一次，取消保持静默；Gio 同步支持 | 真实 store 事件链与声音/通知 fixture 测试，覆盖并发失败、提交拒绝、取消；实际扬声器与通知权限依赖宿主环境 |
| [#55 拖出复制导致其他应用无法拖动](https://github.com/RoseKhlifa/Image-Studio/issues/55) | 五个入口统一鼠标手势启动原生拖动，取消 HTML/native 双循环；处理 mouseup、Esc、失焦、卸载；Windows 拒绝已结束的手势，添加 OLE 定时退出与捕获释放；失败显示 toast | 手势事件、退出策略单测与 Windows amd64/arm64 编译；仍需 Windows 上确认 Explorer 和其他应用拖拽恢复 |
| [#53 可选 API 蒙版](https://github.com/RoseKhlifa/Image-Studio/issues/53) | 现有桌面、Android、Gio 导入/清除入口已覆盖；Responses 发送 `input_image_mask`，Images edits 发送 multipart `mask` 并保留真实 MIME | 共享请求模型、Go、远程内核及 Android 测试；真实上游的 PNG/尺寸/透明通道约束仍适用 |
| [#52 OneCommander 导出成 .url](https://github.com/RoseKhlifa/Image-Studio/issues/52) | 保留原生 OLE 与 Unicode `CF_HDROP`，统一五个入口；浏览器回退在写文件格式后保留内部历史 MIME，避免元数据被 `clearData` 清掉 | `CF_HDROP` 双 NUL/Unicode、拖拽格式测试与 Windows 双架构编译；OneCommander 实机验收仍待完成 |
| [#51 Nano Banana 2](https://github.com/RoseKhlifa/Image-Studio/issues/51) | 复核官方 Interactions 文档与现有路由；官方主机/Google provider 使用 Interactions，标准兼容 relay 保持 Images 协议 | Go、共享模型与远程内核请求/响应 fixture；未调用收费真实上游，实际模型权限需发布时验证 |
| [#50 URL 图片结果](https://github.com/RoseKhlifa/Image-Studio/issues/50) | 现有 URL 下载与 base64 转换可用，检查 Go、浏览器和 Android 原生二进制路径及 50MB/图片类型限制 | Go、前端、Android fixture 和本机 mock upstream |
| [#49 Prompt 历史区分不清](https://github.com/RoseKhlifa/Image-Studio/issues/49) | 现有序号、分隔线和跨平台编号语义已覆盖 | Prompt 模板/历史测试及 Gio 测试 |
| [#44 文件夹批处理](https://github.com/RoseKhlifa/Image-Studio/issues/44) | 保留输入目录/多文件、并发、Auto 比例、输出目录、原名/前缀/去重；修复队列两轮之间错误清零与提交失败计数 | 真实 store 多轮队列、650 次结算、输出路径及客户端测试 |
| [#30 Windows 标题栏颜色](https://github.com/RoseKhlifa/Image-Studio/issues/30) | 现有 CSS token 与原生 Wails 亮/暗色配置匹配，主题切换路径仍受测试保护 | 自动化颜色一致性；Windows 原生标题栏视觉验收仍待完成 |
| [#24 全部删除](https://github.com/RoseKhlifa/Image-Studio/issues/24) | 现有原子清空与全工作区引用清理有效；本次补单条/按日期删除的旧库副本清理，避免再次迁移恢复 | 当前/旧库、全图、未加载记录、待保存队列及工作区回归测试 |
| [#14 Web 端问答](https://github.com/RoseKhlifa/Image-Studio/issues/14) | 已核对问题与“未来会有”的回复；README 已说明没有独立在线 Web 版，浏览器仅用于前端预览 | 产品问答已记录；当前描述没有独立 Web 产品的验收要求，不据此新增线上服务 |

根因、维护约束和复现命令见 [issue-fixes-20261006.md](./issue-fixes-20261006.md)。本机验证结果见 [verification/issue-audit-20261006.json](./verification/issue-audit-20261006.json)。

同日后续的客户端性能优化见 [performance-20261006.md](./performance-20261006.md)：保留全部历史，优化定位读取、配置同步和界面订阅，并新增相应回归验证。

`scripts/issue-close-data.json` 和 [issue-close-comments.md](./issue-close-comments.md) 同步本次清单：本地验证覆盖的 8 项、待 Windows 实机的 3 项、产品问答 1 项。这些文件是维护记录与待使用的评论草稿；本次没有向上游发评论或关闭 issue。
