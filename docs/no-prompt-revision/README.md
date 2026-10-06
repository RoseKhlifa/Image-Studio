# 原始提示词传递说明

Image Studio 现在默认要求 Responses API 请求模型不要改写用户输入的 prompt。

它用于 Responses API 模式。Image Studio 会在 `/v1/responses` 请求顶层加入一段 `instructions`，要求请求模型把用户 prompt 原样传给 `image_generation`，不要重写、扩写、润色或调整措辞。界面不再提供关闭或开启该行为的按钮；所有 Responses 生成请求都按这个策略发送。

## 适合什么场景

- 你已经精修过 prompt，希望图像模型尽量逐字执行。
- prompt 里有固定格式、专有名词、镜头参数、构图要求或中英文混排内容。
- 你想减少 Responses API 请求模型二次发挥导致的风格漂移。

## 功能边界

- 这是一个模型指令约束，不是上游 API 提供的强制参数。
- 它能明显降低 prompt 被改写的概率，但不能保证所有上游、所有模型 100% 遵守。
- 只对 Responses API 模式有意义；Images API 模式本来就是直接把 prompt 发给图像接口。

## 实现路径

请求 payload 会包含类似下面的顶层指令：

```text
Pass the user prompt to image_generation VERBATIM.
DO NOT rewrite, expand, polish, or revise it in any way.
Use the exact text the user gave.
```

桌面 Wails 后端、前端 remote kernel、Android/Web 路径和 Cloudflare Worker 共享同一套语义：Responses API payload 始终会带上这条 `VERBATIM` 指令。

## 如何判断它是否生效

生成完成后打开输出目录的 `log/sse-response-*.txt`，查看上游响应中的 `instructions` 和 `revised_prompt`：

- 请求侧会带 `VERBATIM` 指令。
- 如果上游遵守，`revised_prompt` 应尽量贴近原始 prompt。
