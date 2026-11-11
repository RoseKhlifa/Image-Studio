# GPT Image 2.6 输出格式

Image Studio 默认使用 `gpt-image-2.6-sunburst`。在 Images API profile 中，输出格式可以选择：

- `png`、`jpeg`、`webp`：普通静态图片。
- `gif`：动画图片。请求使用 `frame_count`（默认 9，范围 2–16）和 `frame_duration_ms`（默认 200，范围 20–2000），并始终关闭 `stream` / `partial_images`。
- `svg`：静态矢量 XML。只支持文生图，并始终关闭 `stream` / `partial_images`。

在「高级参数 → 输出控制」中选择 GIF 后，会显示动画帧数和每帧时长设置，并计算单次循环时长、播放帧率。每帧时长以 10ms 为步长；「恢复默认」设为 9 帧、每帧 200ms。切换到其他输出格式会隐藏动画控件并保留数值。GIF 参数按工作区保存，同时支持参数预设、历史参数复用和兼容状态导入导出。

GIF 和 SVG 只会发送给 GPT Image 2.6 Sunburst 或 Flare。图生图使用 SVG 作为源图会在本地被拒绝；GIF 可以作为栅格参考图上传。生成结果按真实扩展名保存，SVG 在桌面端直接使用原始矢量文件作为预览，GIF 使用首帧生成缩略图。

Responses API profile 在选择 GIF/SVG 时会自动切换到同一 profile 的 Images API 请求路径，因为这两种格式不支持 Responses 流式图像预览。
