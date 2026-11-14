# 源码构建与验证

本文档聚焦源码构建、开发模式、验证脚本和 CI。预编译安装包与下载渠道见 [packages.md](./packages.md)。

## 环境要求

- Go 1.25.x。当前 `go.mod` 使用 `go 1.25.5` 与 `toolchain go1.26.3`。
- Node.js 24 或更新版本。前端测试直接加载 TypeScript，平台验证 CI 使用 Node 24；仓库 `.nvmrc` 与此一致。
- Wails CLI v2.12.0。非 macOS release workflow 使用 `go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0`。
- Android 构建需要 JDK 17、Android SDK 34、Build Tools 34.0.0、Gradle 8.7。

## 克隆源码

```bash
git clone https://github.com/RoseKhlifa/Image-Studio.git
cd Image-Studio
```

如果你正在跟随当前开发分支而不是上游正式 release，建议改用你实际工作的 fork 地址。

## 桌面开发模式

```bash
cd image-studio
wails dev
```

`image-studio/wails.json` 当前会执行：

- `frontend:install`: `npm ci`
- `frontend:build`: `npm run build`
- `frontend:dev:watcher`: `npm run dev`

前端脚本会按宿主平台自动选择 `macos` / `windows` / `linux` 对应主题，不需要手动改环境变量。

## 前端独立预览

```bash
cd image-studio/frontend
npm ci

npm run dev
npm run dev:macos
npm run dev:windows
npm run dev:linux
npm run dev:android
npm run dev:android-pad
```

打包静态资源：

```bash
npm run build
npm run build:macos
npm run build:windows
npm run build:linux
npm run build:android
npm run build:android-pad
```

这些命令只切换 `VITE_TARGET_PLATFORM` 对应的主题和壳层，不改变主业务逻辑。

## macOS 本地发布包

```bash
bash scripts/package-local-macos-app.sh
```

产物位于：

```text
image-studio/build/bin/Image Studio.app
```

脚本会分别构建 arm64 与 amd64，再用 `lipo` 合成 universal 二进制，并执行本地自签。

## Windows / Linux Wails 构建

Wails v2 桌面端需要在目标平台原生构建。

Windows：

```bash
cd image-studio
wails build -platform windows/amd64 -clean
```

如果你要生成“用户直接解压双击 `exe` 也能运行”的 Windows 便携包，不要复用普通 release 的裸 `exe`。仓库提供了独立 workflow：

- GitHub Actions 工作流：`Windows Portable Fixed WebView2`
- 触发方式：手动 `workflow_dispatch`
- 产物：`image-studio-<version>-windows-amd64-portable-fixed-webview.zip` 与 `image-studio-<version>-windows-arm64-portable-fixed-webview.zip`

这两个 zip 会把 `image-studio.exe` 和微软 Fixed Version WebView2 Runtime 一起打包，并在应用启动时优先使用同目录下的 `WebView2FixedRuntime/`。

Linux Ubuntu 24.04 / Debian 新版本：

```bash
sudo apt-get update
sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev

cd image-studio
wails build -platform linux/amd64 -clean -tags webkit2_41
```

Ubuntu 22.04 系通常使用 `libwebkit2gtk-4.0-dev`，构建时不加 `webkit2_41` tag。

## Windows / Linux Gio 测试客户端

Gio 客户端位于 `gio-client/`，与 Wails / WebView2 主实现独立。它复用 `go-cli/pkg/client` 请求内核，不读取 `image-studio/frontend/dist`。

Windows：

```bash
cd gio-client
go test ./...
go build -o ../dist/image-studio-gio.exe ./cmd/image-studio-gio
```

Linux：

```bash
sudo apt-get update
sudo apt-get install -y \
  pkg-config \
  libegl1-mesa-dev \
  libvulkan-dev \
  libwayland-dev \
  libx11-dev \
  libx11-xcb-dev \
  libxcursor-dev \
  libxfixes-dev \
  libxkbcommon-dev \
  libxkbcommon-x11-dev

cd gio-client
go test ./...
go build -o ../dist/image-studio-gio ./cmd/image-studio-gio
```

release workflow 会单独上传 `image-studio-gio-*` artifacts，不改变现有 `image-studio-*` Wails artifacts。

## Android APK

```bash
cd android-shell
./gradlew assembleRelease
```

Gradle 会先执行 `image-studio/frontend` 的 `npm run build:android`，再把 `dist/` 拷贝进 APK assets。APK 内部运行同一个 Android 前端目标，phone / pad 布局由运行时窗口尺寸和方向决定。

可选环境变量：

| 变量 | 用途 |
|---|---|
| `IMAGE_STUDIO_ANDROID_VERSION_NAME` | Android `versionName`。 |
| `IMAGE_STUDIO_ANDROID_VERSION_CODE` | Android `versionCode`。 |
| `IMAGE_STUDIO_KEYSTORE_PATH` | release 签名 keystore。未提供时使用自动生成的 debug keystore。 |
| `IMAGE_STUDIO_KEYSTORE_PASSWORD` | keystore 密码。 |
| `IMAGE_STUDIO_KEY_ALIAS` | key alias。 |
| `IMAGE_STUDIO_KEY_PASSWORD` | key 密码。 |
| `IMAGE_STUDIO_ANDROID_USE_PREBUILT_FRONTEND` | 设为 `1` / `true` 时复用已有 `frontend/dist`。 |

MuMu 模拟器调试流程见 [mumu-android-debug.md](./mumu-android-debug.md)。

## 版本元数据

release workflow 会先执行：

```bash
./scripts/compute-version.sh
```

它会从 tag 或 `image-studio/wails.json` 计算桌面版本、前端版本、Android `versionName` / `versionCode`。随后 `scripts/sync-version-metadata.mjs` 会同步：

- `image-studio/wails.json`
- `image-studio/frontend/package.json`
- `image-studio/frontend/package-lock.json`

本地不要手动维护多份版本号，除非你明确在调整下一次发布的基准版本。

## 验证入口

常用验证：

```bash
cd image-studio/frontend
npm run test
npm run build

cd ../..
cd image-studio
GOPATH="../.gopath" GOMODCACHE="../.gomodcache" GOCACHE="../.gocache" go test ./...

cd ../go-cli
GOPATH="../.gopath" GOMODCACHE="../.gomodcache" GOCACHE="../.gocache" go test ./...

cd ../gio-client
GOPATH="../.gopath" GOMODCACHE="../.gomodcache" GOCACHE="../.gocache" go test ./...
```

跨平台内核本地全量验证：

```bash
node scripts/verify-local-platform-kernel.mjs
```

该脚本会跑前端测试 / 构建、Worker 测试、本地 live verify smoke、本地 smoke、Android shell 本地校验、Go 测试和 macOS 发布包验证。Go 部分会显式覆盖：

- `image-studio` 模块
- `shared/compat-go`
- `go-cli/pkg/client`

是否能全绿取决于本机 Android SDK / JDK 与 macOS 构建工具是否齐全。

其他入口：

```bash
node scripts/verify-local-android-shell.mjs
node scripts/verify-local-macos-release.mjs
node scripts/verify-local-live-verify.mjs
node scripts/local-smoke-check.mjs
node scripts/live-verify.mjs
node scripts/verify-issue-close-tooling.mjs
```

`verify-issue-close-tooling.mjs` 会校验：

- `scripts/issue-close-data.json`
- `docs/issue-close-comments.md`
- `scripts/issue-close-helper.mjs`
- GitHub 当前 open issue 是否仍与本地“可关单 / 保持 open / defer”分组一致
- mock GitHub 下的 `comment-only` / `comment-and-close` 执行路径
- 导出评论包与 `manifest.json` / `plan.json` / `plan.md`

如果你希望把这条校验也并入本地平台总链，可以显式开启：

```bash
IMAGE_STUDIO_INCLUDE_ISSUE_CLOSE_VERIFY=1 node scripts/verify-local-platform-kernel.mjs
```

当前 CI 里的 `.github/workflows/verify-platform-kernel.yml` 已固定开启这条校验，但会设置：

```text
IMAGE_STUDIO_SKIP_ISSUE_CLOSE_GITHUB_SYNC=1
```

也就是：

- 在常规 CI 里校验数据源 / helper / 渲染文档是否一致
- 不把 GitHub 当前 open issue 状态变化变成每次 PR 的硬失败条件

本地独立运行时，如果环境里没有 `GITHUB_TOKEN` / `GH_TOKEN`，脚本也会自动切到同样的离线模式。

如果需要带 GitHub 当前 open issue 状态一起核对，可单独手动触发：

- `.github/workflows/verify-issue-close-tooling.yml`

该 workflow 会把 Actions 自带的 `github.token` 传给 `verify-issue-close-tooling.mjs`，
让只读 issue 列表查询优先走鉴权请求，降低公共限频带来的偶发失败概率。

它还会额外导出一份可直接使用的关单评论包 artifact，当前包含：

- `manifest.json`
- `README.md`
- `plan.json`
- `plan.md`
- `issue-24.md` ... `issue-42.md`

artifact 名称为：

- `verify-issue-close-tooling-results`
- `verify-issue-close-comment-bundle`

如果你想把多条验证结果统一写到同一个目录，优先使用：

```bash
IMAGE_STUDIO_VERIFY_RESULTS_DIR=.tmp/verify-results node scripts/verify-local-platform-kernel.mjs
```

各脚本仍兼容原来的单文件 `*_OUTPUT_PATH` 变量；但新用法更推荐只传一个
`IMAGE_STUDIO_VERIFY_RESULTS_DIR`。

对于当前仍需 Windows 真机、Android 真机或真实高并发上游才能确认的项，统一参考：

- [manual-verification.md](./manual-verification.md)

如果你想把当前最新总链证据、`#30/#36` 手工验证模板，以及 issue 关单评论包统一整理成一个交接目录，可直接执行：

```bash
node scripts/prepare-external-verification-bundle.mjs
```

默认输出目录类似：

```text
.tmp/external-verify-bundles/<YYYY-MM-DD>/issue-30-36-handoff/
```

其中会包含：

- `evidence/`
- `manual-verify/`
- `issue-close-bundle/`
- `README.md`

开始手工验证前，建议先用模板脚本初始化目录：

```bash
node scripts/init-manual-verification.mjs 36-android
```

常用预设还有：

```bash
node scripts/init-manual-verification.mjs 36-windows
node scripts/init-manual-verification.mjs 30-windows
```

如果要做自定义验证主题：

```bash
node scripts/init-manual-verification.mjs custom "my regression check"
```

脚本会生成 `report.md`、`meta.json` 以及 `screenshots/`、`raw/`、`logs/` 目录，方便把实机截图、原始响应和结论统一沉淀到 `.tmp/manual-verify/<date>/...` 下。

`verify-local-android-shell.mjs` 当前会校验：

- `:app:testDebugUnitTest` 可完成，覆盖 Android 壳层流式事件解析等纯逻辑单测。
- `:app:assembleDebug` 可完成。
- `output-metadata.json` 与 APK `badging` 中的 `applicationId` / `versionCode` / `versionName` 正确。
- APK 签名能通过 `apksigner verify -v`。
- APK 内已经带上前端 `assets/index.html` 和构建后的静态资源。

如果本机已经连了 Android 设备或模拟器，还可以额外开启 APK 安装 / 启动 smoke：

```bash
IMAGE_STUDIO_ANDROID_DEVICE_SMOKE=1 node scripts/verify-local-android-shell.mjs
```

可选地指定设备序列号：

```bash
IMAGE_STUDIO_ANDROID_DEVICE_SMOKE=1 IMAGE_STUDIO_ANDROID_SERIAL=<serial> node scripts/verify-local-android-shell.mjs
```

未开启时，脚本会明确返回 `deviceSmoke.attempted=false`，不会因为没有设备而失败。

`verify-local-live-verify.mjs` 会在本地启动 `runtime-smoke-server.mjs` 作为
mock upstream，再驱动 `live-verify.mjs` 完成一轮 direct vs worker parity。
它的意义不是替代真实上游验证，而是证明：

- `live-verify.mjs` 本身可以真实执行
- parity checks / JSON 输出 / summary 渲染链条在本地是通的

`verify-local-live-verify.mjs` 会启动仓库内 `runtime-smoke-server.mjs` 作为 mock upstream，
再驱动 `live-verify.mjs` 走一遍 direct vs worker parity。它主要用于证明：

- `live-verify.mjs` 本身可以真实执行，而不只是语法通过。
- `live verify` 的 JSON 输出与 parity checks 在本地 smoke 环境下是可用的。

`verify-local-macos-release.mjs` 当前会额外校验：

- `Info.plist` 的 `CFBundleVersion` / `CFBundleShortVersionString` 是否与 `VITE_APP_VERSION` 一致。
- 编译后的 macOS App 在探针模式下启动后，是否会把“同 semver core 的 CI 版本不应提示更新”判断为 `false`。

如果当前环境没有可用的图形桌面会话，Wails GUI 进程可能无法启动。这种情况下可以先跳过运行时探针：

```bash
IMAGE_STUDIO_SKIP_RUNTIME_UPDATE_PROBE=1 node scripts/verify-local-macos-release.mjs
```

正常的编译版更新判断联调，仍建议在交互式 macOS 桌面会话里完整运行，不跳过探针。

当前 `.github/workflows/verify-platform-kernel.yml` 会固定设置
`IMAGE_STUDIO_SKIP_RUNTIME_UPDATE_PROBE=1`，因为 GitHub Actions 的 macOS runner
不保证可用的交互式桌面会话。

该 workflow 还会把结构化验证结果上传为 artifact：

- `verify-platform-kernel-results/platform-kernel-summary.json`
- `verify-platform-kernel-results/live-verify.json`
- `verify-platform-kernel-results/local-smoke.json`
- `verify-platform-kernel-results/android-shell.json`
- `verify-platform-kernel-results/macos-release.json`

即使验证步骤失败，artifact 上传步骤也会继续执行，尽量保留已生成的
summary 和子结果；如果某个结果文件根本没产出，只会给出 warning，不会
覆盖原始失败原因。

另外，workflow 会把 `platform-kernel-summary.json` 渲染成 Markdown，直接写到
GitHub Actions job summary，方便不下载 artifact 时先看关键结果。summary
里会带上 Node / 平台 / Android SDK / `JAVA_HOME` 等环境指纹。

真实上游对比验证需要先按 `scripts/live-verify.env.example` 准备 `.env.live` 或 `.env.local`。

`live-verify-platform-kernel.yml` 会把 `scripts/live-verify.mjs` 的结果写成
JSON artifact，并渲染到 job summary。当前 artifact 名称为：

- `live-verify-platform-kernel-results/live-verify.json`

和本地平台验证 workflow 一样，artifact 上传与 summary 渲染都放在
`always()` 步骤里；如果 live verify 中途失败，仍会尽量保留已经写出的
JSON 结果和失败摘要。live verify 的 summary 也会带上 Node / 平台 / 模型 ID /
worker 端口等环境指纹，并列出每条 parity check 的通过/失败情况。

同样，本地几条子验证脚本在中途失败时，也会尽量把已知环境和错误原因写进
对应 JSON，方便在 artifact 之外单独排查：

- `local-smoke-check.mjs`
- `verify-local-live-verify.mjs`
- `verify-local-android-shell.mjs`
- `verify-local-macos-release.mjs`

## CI

当前发布链路在 `.github/workflows/release.yml`：

- 并行构建 Windows、macOS、Linux Wails 桌面产物。
- Windows 额外产出单个自适应架构的 NSIS installer `image-studio-<version>-windows-installer.exe`，内部同时包含 amd64 与 arm64 二进制，供正式安装分发或 Microsoft Store Win32 提交使用。
- Windows 额外产出 `image-studio-<version>-windows-x64.msix`、`image-studio-<version>-windows-arm64.msix` 与 `image-studio-<version>-windows.msixbundle`，供 Microsoft Store / 企业分发使用。
- 单独构建一个 Android release APK。
- tag 为 `v*` 时将所有产物附加到 GitHub Release。

### Windows 签名

Windows 11 的 Smart App Control / SmartScreen 会重点拦截无法验证发布者的 `exe`。仓库里的 release workflow 现在约定：

- 如果配置了 Windows 签名证书，workflow 会自动对 Windows `exe` 做 Authenticode 签名并校验。
- 如果没有配置签名证书，workflow 仍然继续产出 Windows artifact，但会在日志里明确警告这些 `exe` 可能被 Win11 拦截。

需要配置的 GitHub Actions secrets：

| 变量 | 用途 |
|---|---|
| `IMAGE_STUDIO_WINDOWS_CERT_BASE64` | Base64 编码后的 `.pfx` 证书内容。 |
| `IMAGE_STUDIO_WINDOWS_CERT_PASSWORD` | `.pfx` 密码。 |

可选 GitHub Actions variable：

| 变量 | 用途 |
|---|---|
| `IMAGE_STUDIO_WINDOWS_TIMESTAMP_URL` | RFC 3161 时间戳地址；默认 `http://timestamp.acs.microsoft.com`。 |

签名步骤由 `scripts/sign-windows-binary.ps1` 执行，内部会调用 `signtool sign`、`signtool verify /pa /all` 和 `Get-AuthenticodeSignature` 做校验。

### Microsoft Store MSIX 身份

MSIX / MSIXBundle 打包内置了当前 Microsoft Store 产品 `9P9DTWG1G93N` 的公开身份值，因此首次发布不需要额外配置 GitHub Actions variables：

| 字段 | 当前值 |
|---|---|
| Identity Name | `Image-Studio.Image-Studio` |
| Package Display Name | `Image-Studio` |
| Publisher | `CN=D9AC33A5-F2A0-4194-9BCA-D17B6B918495` |
| Publisher Display Name | `Image-Studio` |

如需切换到新的 Partner Center 产品，可以用 repository variables 覆盖内置默认值：

| 变量 | 用途 |
|---|---|
| `IMAGE_STUDIO_MSIX_IDENTITY_NAME` | Partner Center 中保留的包 Identity Name。 |
| `IMAGE_STUDIO_MSIX_PUBLISHER` | Partner Center 对应的 Publisher，例如 `CN=...`，必须与商店身份完全一致。 |
| `IMAGE_STUDIO_MSIX_PUBLISHER_DISPLAY_NAME` | 商店展示的发布者名称。 |

注意：

- `IMAGE_STUDIO_MSIX_PUBLISHER` 不能自行猜测，必须直接使用 Partner Center / Store association 给出的值。
- 当前内置身份值会让 release workflow 直接产出 `x64.msix`、`arm64.msix` 和 `windows.msixbundle`；只有在内置值被清空或覆盖为不完整配置时，MSIX 相关 job 才会跳过。
- 面向 Microsoft Store 提交时可以保持未签名，由商店在提交后重新签名；如果你要本地侧载测试，则还需要额外签一个测试证书。

平台内核验证 workflow：

- `.github/workflows/verify-platform-kernel.yml`
- `.github/workflows/live-verify-platform-kernel.yml`
