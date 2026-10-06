# 数据位置与故障排除

## 提 Issue 前先自查

很多报错看起来像“软件坏了”，实际是上游配置、权限、兼容性或网络环境问题。

如果你是在对照 GitHub issue 排查，先看 [issue-progress.md](./issue-progress.md)。
那份文档会标出哪些问题已经有代码覆盖、哪些还缺真机或真实上游验证。

先做这组最短排查:

1. 在当前 profile 里点一次「测试连接」，确认 `BASE_URL`、`API Key`、请求模型 ID 至少能通过基础校验。
2. 确认自己选对了 API 形态:
   - `Responses API` 需要所填请求模型可用，并且上游真正实现了 `/v1/responses` 与 SSE。
   - `Images API` 需要上游真正实现 `/v1/images/generations` 或 `/v1/images/edits`。
3. 打开历史详情或 raw 响应，先看真实 `HTTP status` 和上游错误 message，不要只看页面上的 toast。
4. 如果同样的 `BASE_URL + Key + 模型 ID` 在 curl、Postman 或上游自带调试页里也失败，优先联系上游服务商。
5. 如果你换一个已知正常的上游后问题立即消失，通常也不是本项目的软件 bug。

### 这些情况通常不是软件 bug

- `401 / 403 / model not found`:
  常见原因是 Key 没权限、请求模型 ID 填错、账号没开通对应模型或 IP 白名单未放行。
- `524 / 504 / 5xx`:
  常见原因是 Cloudflare / Nginx / relay 网关超时、上游排队过久、服务商限流、上游临时故障，不一定是本地程序崩溃。
- 多参考图、蒙版、`seed`、`negative_prompt` 不生效:
  常见原因是 relay 只“接受字段”但没有真正透传，或目标模型本身就不支持这些能力。
- `Responses API` 一直失败:
  常见原因是上游根本没实现 `/v1/responses`、会缓冲 SSE，或者 Key 没有所填请求模型的权限。
- `Responses WebSocket mode` 握手失败:
  如果 raw 日志或上游报错里出现 `WebSocket upgrade required (Upgrade: websocket)`，说明当前链路把 WebSocket 请求降成了普通 HTTP，请求没有真正完成 Upgrade。常见原因是中转站 / 反向代理 / 网关没有正确放行 `Upgrade: websocket`，这种情况应直接切回 `HTTP SSE` 或修上游代理配置。
- Android 看不到“目录”或保存位置和桌面端不一样:
  常见原因是 Android 走 `MediaStore` / 系统相册，不会像桌面文件管理器那样直接暴露同一个物理目录。
- 浏览器预览里看到 `memory://...`:
  这是运行时虚拟路径，不代表文件已经写入真实磁盘。

### 适合提 Issue 的情况

- 应用崩溃、白屏、按钮无响应、页面状态明显错乱。
- 在已知正常的上游上稳定复现，且 raw 响应表明请求是应用自己构造错了。
- 图片明明已经生成成功，但应用保存、展示、历史记录或导出行为明显异常。
- 同一版本在某个平台稳定回归，旧版本正常，新版本异常。

### 不适合提 Issue 的情况

- 账号余额不足、Key 失效、模型未开通、服务商限流、IP 白名单限制。
- 你的上游没有实现某个 OpenAI 接口，却希望客户端自动兼容。
- relay 静默忽略扩展字段，或服务商文档本来就写了“不支持蒙版 / 多图 / seed”。
- 只在单次请求里偶发超时，换上游、降尺寸或稍后重试后恢复正常。

### 提 Issue 时至少提供这些信息

- 平台: `Windows / macOS / Linux / Android`
- 应用版本: release 版本号或 Actions artifact 对应 commit / 构建日期
- API 形态: `Responses API` 或 `Images API`
- `BASE_URL` 类型: 官方 / OpenAI 兼容中转 / 自建网关
- 请求模型 ID
- 最短复现步骤
- raw 响应中的 `HTTP status`、错误 message 或截图

## 数据存储位置

| 类型 | 位置 |
|---|---|
| 桌面端 API Key | 系统安全存储(Keychain / Credential Manager / Secret Service)。 |
| Android API Key | 应用私有 SharedPreferences。 |
| 上游配置(API 形态、BASE_URL、模型 ID) | 前端本地存储。 |
| 历史记录元数据 | IndexedDB。 |
| 用户偏好 | 前端本地存储。 |
| 生成图片 | 桌面端输出目录下的 `images/`；Android 端优先保存到 MediaStore `Pictures/ImageStudio`。 |
| 原始响应日志 | 桌面端输出目录下的 `log/`；浏览器/Android 远程内核可能使用虚拟 raw 路径或壳层文件能力。 |
| 拖入 / 粘贴 / 变换中间图 | 桌面端系统 config 目录下的 `image-studio/imports/`；Android 端应用私有 `imports` 目录。 |

默认输出目录:

| 平台 | 默认输出目录 |
|---|---|
| Windows | `%APPDATA%\image-studio\` |
| macOS | `~/Pictures/Image Studio/` |
| Linux | `~/Pictures/Image Studio/` |
| Android | 应用外部图片目录；保存到系统相册时使用 `Pictures/ImageStudio`。 |

桌面端输出目录里会继续拆成:

```text
images/
log/
```

`images/` 存图，`log/` 存 Responses SSE dump 或 Images API JSON 响应，避免图片浏览目录被日志污染。

## 一直 524 / 504

这通常是上游网关超时，不一定是本地程序崩溃。

处理顺序:

1. 如果当前是 Images API，优先切到 Responses API。
2. 确认 key 有所填请求模型的权限。
3. 降低质量或尺寸，缩短单次推理时间。
4. 从历史项查看 raw 响应，确认是 Cloudflare 524/504、上游 JSON 5xx，还是模型权限错误。
5. 如果上游本身不支持 SSE 或会缓冲 SSE，换上游或走 Images API。

## Responses WebSocket 握手失败 / `Upgrade: websocket`

如果你在 raw 日志里看到类似：

```text
websocket handshake failed: HTTP 400: WebSocket upgrade required (Upgrade: websocket)
```

或上游直接返回：

```json
{
  "error": {
    "message": "WebSocket upgrade required (Upgrade: websocket)",
    "type": "invalid_request_error"
  }
}
```

这代表问题发生在 **握手阶段**，不是生图中途断流：

1. 客户端已经尝试走 `Responses WebSocket mode`
2. 但链路上的某一层把请求当成了普通 HTTP
3. 上游没有收到正确的 `Upgrade: websocket`

常见原因：

- Nginx / 网关把 `Connection` 头清空了
- `Upgrade` 头没有透传
- 某些 HTTP/2 / H2C 转发链路不支持或没有正确桥接 WebSocket Upgrade
- 上游只实现了普通 `/v1/responses`，没有真正实现 WS `/v1/responses`

处理建议：

1. 先把当前 profile 的传输切回 `HTTP SSE`
2. 如果同一个 `BASE_URL + Key` 在 Codex 自己的 WS 能正常，而本应用不行，优先检查：
   - 你在本应用里填的是不是站点根地址，而不是已经带 `/v1` 的地址
   - 真实链路上是否存在额外的代理层
3. 如果上游明说支持 WS，但仍返回这个错误，优先让服务商检查 `Upgrade: websocket` 是否真的到达了后端

## `model not found` / 401 / 403

Responses API:

- key 没有请求模型权限。
- 请求模型 ID 在该上游不可用。
- 上游虽然列出了该模型，但没有允许它通过 Responses API 调用。

Images API:

- 请求模型 ID 不存在。
- key 没有 image endpoint 权限。
- 上游只实现了 `/v1/chat/completions`，没有实现 `/v1/images/generations` 或 `/v1/images/edits`。

## 多参考图、蒙版、seed、negative prompt 没生效

先检查当前 profile 的请求策略:

- `OpenAI 标准` 会尽量只发官方字段。
- `兼容中转扩展` 才会额外发送 seed / negative_prompt 等扩展字段。

还需要注意:

- Images API 的多参考图支持取决于 relay；标准 OpenAI Images Edits 通常只接受单张 `image`。
- 蒙版是否生效取决于目标模型和上游是否正确透传 multipart 或 Responses input mask。
- 有些中转站会接受字段但静默忽略。

## Android 保存或打开目录行为

Android 与桌面端不同:

- 保存图片优先走 MediaStore，结果会出现在系统相册的 `ImageStudio` 目录。
- 打开输出目录在 Android 10+ 上会打开系统图片集合，不一定是具体文件夹浏览器。
- 如果壳层能力不可用，前端会回退 Web Share API 或下载链接。

## 浏览器预览里的 `memory://`

浏览器预览和部分远程内核路径会使用 `memory://image/...` 或 `memory://text/...` 虚拟路径。它们只存在于当前页面运行时，用于调试和回退，不等同于已经写入真实文件系统。

需要真实持久化时，应在 Wails 桌面端或 Android 壳层中运行。

## 查看 raw 响应

历史项右键可以查看 raw 响应:

- Responses API:通常是 `sse-response-*.txt`。
- Images API:通常是 `images-response-*.json`。

排查时优先看:

- HTTP status。
- 上游返回的错误 message。
- 是否出现 `retryable=true`、524、504、5xx。
- Responses API 是否有 `partial_image_b64` 或 final image 事件。
