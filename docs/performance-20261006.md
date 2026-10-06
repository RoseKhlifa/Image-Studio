# 2026-10-06 性能优化记录

本轮针对客户端的配置同步、历史读取、界面重绘和初始资源加载进行优化。基线为 `613d491`，原始测量结果保存在 [verification/performance-20261006.json](./verification/performance-20261006.json)。

## 优化前后

数据测试使用同一组 1000 条历史，每页 24 条；每项运行 3 次，取中位数。环境为 macOS arm64、Node v25.8.1 和 fake-indexeddb。这些时间是客户端逻辑的微基准，不表示真实 WebView 启动时间或上游推理速度。

| 指标 | 优化前 | 优化后 | 变化 |
|---|---:|---:|---:|
| 1000 条相同时间戳历史完整分页 | 2945.45 ms | 191.43 ms | 耗时减少 93.5% |
| 同一分页场景的索引记录读取次数 | 21,705 | 1,123 | 减少 94.8% |
| 1000 条不同时间戳历史完整分页 | 41.29 ms | 26.72 ms | 耗时减少 35.3% |
| 200 次无关进度更新的配置变化检测 | 281.91 ms | 0.18 ms | 不再重复处理整份历史 |
| 静态区域在 200 次无关更新后的额外渲染 | 200 次 | 0 次 | 相关历史和选中图片变化仍更新 |
| macOS 入口及静态依赖 JavaScript | 1,114,655 B | 589,308 B | 减少 47.1% |
| 入口加桌面工作区全部静态依赖 JavaScript | 1,114,655 B | 1,051,277 B | 减少 5.7% |

渲染次数来自 React test renderer 的真实 store/hook 探针；资源体积来自 Vite manifest 的导入图。后两行分别描述入口加载和桌面工作区可用所需的资源，不能把入口体积减少 47.1%解释成整次启动快 47.1%。时间会随硬件、运行负载和浏览器实现变化；读取次数与功能回归检查不依赖时间阈值。

## 配置同步

以前每次 store 更新都重新为整份历史生成 JSON 指纹，进度、日志、缩放和预览帧也会走这条路径。现在先比较需要持久化的字段引用和少量 localStorage 偏好；有实际变化时才做完整指纹检查。输入字段使用完整的 TypeScript `Record` 列表，增加持久化字段时编译器会要求同步更新检测表。

去抖期间只保留最新的字段引用，真正写入前才复制历史和配置。尚未执行就被替换的事件不会复制整个档案。同一宿主的导出写入串行执行，慢写期间的新配置保留到下一次写入，避免旧写入覆盖新快照。

相关测试检查：200 次进度/预览更新没有读取全图字段；输出目录、历史参数等真实变化仍触发导出；50 次连续调度只保存最终快照；慢写期间不并发写入，也不丢失等待中的配置。

## 历史读取

保留 IndexedDB 版本 2 与 `(createdAt, id)` 分页语义。支持的运行时通过 `continuePrimaryKey` 直接定位到上一次主键，跨过同时间戳中已经读取的记录；未提供该 API 的旧 WebView 仍使用完整、正确的遍历回退。API 语义依据 [IndexedDB 标准](https://w3c.github.io/IndexedDB/#dom-idbcursor-continueprimarykey)与 [MDN](https://developer.mozilla.org/en-US/docs/Web/API/IDBCursor/continuePrimaryKey)。

旧库迁移在一次运行中共享同一个完成结果，成功后不再为每页重复 count/getAll/open；失败时保留重试机会，完成后关闭旧库连接。数据库打开失败也可重试，版本变化时释放旧连接。

没有截断档案、降低图片质量或删除完整图像。回归覆盖同毫秒记录、旧 API 回退、旧版本库及全图读取、迁移失败重试、按日期清理和全部删除。

## 界面与画布

12 个常驻区域改用 `useStudioFields` 订阅实际需要的字段。历史栏只订阅当前图片/对比图的 ID，预览像素变化不再带动整栏重绘；顶部标签计数和底部任务计数使用派生值订阅。

桌面与 Android 画布保留自身状态更新，使用 `memo` 避免父层状态栏刷新带来的重复绘制。批次排序和历史导航使用引用缓存；画布坐标、图像加载和蒙版计算仍使用当前数据，避免改变上一轮的图片一致性修复。

维护约束：

- 数组和对象使用新引用更新，不原地修改 store 内容。
- 渲染期间读取的值必须有对应字段或派生 selector；事件处理器可用 `getState()` 获取最新数据。
- 只缓存由不可变输入确定的纯计算，不缓存随画布坐标/图像加载变化的结果。
- 配置检测的新字段必须加入完整列表；不能靠减少历史保留条数降低成本。

## 资源拆分

移除把所有 Android/Desktop 模块强制合并为 `platform-ui` 的规则。原有 Shell 和弹窗的 lazy import 现在能保持独立；React、图标与画布依赖仍共享。新增的 React 渲染器仅用于测试，不进入生产资源。

## 复验

```bash
cd image-studio/frontend
npm ci
npm test
npm run benchmark:performance -- --revision=613d491 --output=../../.tmp/performance-baseline.json
npm run benchmark:performance -- --output=../../.tmp/performance-current.json
npm run benchmark:performance -- --timestamps=unique --output=../../.tmp/performance-unique.json
npm run build:windows
npm run build:macos
npm run build:linux
npm run build:android-pad
npm run build:android
```

`--records` 与 `--rounds` 可调整数据规模。对比不同提交时传入确切的 commit hash；脚本读取该提交的运行时代码，使用当前环境的依赖，所有测试数据均为临时 fixture。

本机验证：210 项前端测试、五种目标平台构建和依赖审计通过。Android JVM/APK 与 macOS universal 应用启动探针的最终结果记录在结构化文件的 `verification` 中。上一轮的 Windows 真机拖拽/标题栏待验证项仍以 [manual-verification.md](./manual-verification.md) 为准。
