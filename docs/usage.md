# 使用与上游配置

## 首次启动

首次启动会自动打开「上游配置」。也可以之后从设置里重新打开。

需要填写:

1. API 形态:Responses API 或 Images API。
2. BASE_URL:你自己的 OpenAI 兼容中转站地址。
3. API Key。
4. 请求模型 ID:Responses API 直接把它写入请求的 `model` 字段，支持上游允许的文本或图像模型；Images API 也把它作为请求的 `model` 字段。
5. 测试连接:保存前建议先点一次，确认当前 profile 能真正走通。

本应用不内置任何默认上游，也不会向除你配置的 BASE_URL 以外的生成服务发请求。

## API 形态怎么选

### Responses API

调用 `/v1/responses`，通过模型内置的 `image_generation` 工具触发图像生成，SSE 流式接收事件。

适合:

- 图像推理容易超过 100 秒。
- 上游在 Cloudflare / Nginx 后面，常见 524/504。
- 你的 key 有所填请求模型的权限，例如默认的 `gpt-5.5`，或上游支持的图像模型。

特点:

- SSE 事件会持续到达，网关更不容易把连接判定为空闲。
- 本地有 3 次自动重试和 15 秒 backoff。
- 如果 final image 未拿到但已经收到 `partial_image_b64`，会尽量保存最后一个部分结果。
- 生成请求默认要求上游按原始 prompt 传给图像工具，不再提供提示词改写开关。细节见 [no-prompt-revision](./no-prompt-revision/README.md)。

### Images API

调用标准图像接口:

- `/v1/images/generations`:文生图。
- `/v1/images/edits`:图生图，multipart 上传。

适合:

- 上游不支持 Responses API。
- key 只绑定 image 分组。
- 只需要最大兼容性。

限制:

- 一次性 JSON 响应，没有 SSE 保活。
- 长推理在 Cloudflare 后面仍有 524/504 风险。
- 多参考图、seed、negative prompt 是否生效取决于上游兼容实现。

## 参数策略

在上游 profile 中可以选择请求策略:

| 策略 | 行为 |
|---|---|
| OpenAI 标准 | 默认策略，只发送 OpenAI 官方公开字段，适合 OpenAI 直连或严格兼容实现。 |
| 兼容中转扩展 | 额外发送 relay 常见扩展字段，例如 seed / negative_prompt，适合明确知道上游支持这些字段的场景。 |

字段映射:

- Responses 模式:
  - `mask` 走 `input_image_mask`。
  - `seed` / `negative_prompt` 只在兼容中转扩展策略下附带发送。
- Images Edits 模式:
  - `mask` 作为 multipart file。
  - `seed` / `negative_prompt` 只在兼容中转扩展策略下附带发送。
  - 多参考图中第二张及之后使用兼容字段发送，标准 OpenAI 实现可能忽略。

## 基本流程

文生图:

1. 选择「文生图」。
2. 输入 prompt。
3. 选择比例、质量、输出格式、风格。
   - 内置比例不够时，可以打开「自定义比例」弹窗，新增并保存常用宽高比。
   - 如果想理解“为什么比例切到 Auto 后，分辨率也不能再单独锁定”，见 [size-auto.md](./size-auto.md)。
4. 根据需要设置 seed 或 negative prompt。
5. 点击「生成」，或使用 `Cmd+Enter` / `Ctrl+Enter`。

图生图:

1. 拖入本地图片、粘贴剪贴板图片，或点击添加图片。
2. 切换到「图生图」。
3. 输入修改要求。
4. 如需蒙版，在画板里切换到蒙版工具绘制。
5. 点击「生成」。

### 图生图批处理

桌面端图生图支持批处理模式，入口位于 `源图片 / 参考图` 区域顶部的 `普通图生图 / 批处理` 切换。

适合:

- 一个目录里有很多源图。
- 所有图片共用同一组 prompt 和图生图参数。

基本流程:

1. 切到 `图生图`。
2. 在 `源图片 / 参考图` 里切到 `批处理`。
3. 选择输入目录。
4. 确认扫描到的图片数量。
5. 选择默认回原目录，或单独指定输出目录。
6. 设置并发数。
7. 点击 `编辑` 开始处理。

详细说明见 [batch-img2img](./batch-img2img/README.md)。

生成成功后，toast 会提供查看详情入口；详情抽屉里可以查看图片预览、全部参数、原始 prompt、优化后 prompt、保存路径与 raw 响应路径。

## 输出与历史

- 生成图片默认落到输出目录的 `images/` 子目录。
- 原始响应和排错日志默认落到输出目录的 `log/` 子目录。
- 历史元数据保存在 IndexedDB。
- 自定义比例保存在本地，下次启动仍会回到参数按钮区。
- 历史可以导出为 JSON，也可以重新导入。

具体路径见 [troubleshooting.md](./troubleshooting.md)。
