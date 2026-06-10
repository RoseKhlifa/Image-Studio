# Image-Prompts 配套项目与导入说明

[Image-Prompts](https://prompts.sorry.ink/) 是 Image Studio 的配套提示词站，支持从网页端一键导入提示词到桌面端。

## 使用方式

1. 在 Image-Prompts 的提示词详情页点击 `Send to Image-Studio`。
2. 站点会通过 `image-studio://import?...` 拉起桌面端。
3. 客户端拉取提示词内容后，会先弹出确认界面，再写入当前 prompt 编辑器。

## 平台分流

- macOS: 由 `image-studio/` 下的 Wails 桌面端接收导入请求。
- Windows / Linux: 由 `gio-client/` 接收导入请求。

## 导入约束

- 当前导入能力仅面向桌面端，不包含 Android。
- 站点签发的是一次性 token，默认 24 小时内有效，消费后失效。
- Windows / Linux 下的协议注册、CLI 和实现细节见 [gio-client.md](./gio-client.md)。

## 相关链接

- 站点: [prompts.sorry.ink](https://prompts.sorry.ink/)
- 仓库: [RoseKhlifa/Image-Prompts](https://github.com/RoseKhlifa/Image-Prompts)
