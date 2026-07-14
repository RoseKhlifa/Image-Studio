# Image Studio Gio Client

`gio-client/` is an independent high-performance desktop client for Windows, macOS, and Linux. It uses Gio for the complete native GUI and reuses the existing Go request kernel from `go-cli/pkg/client`.

It does not embed the React frontend, Wails, WebView2, or WebKitGTK. The current Wails desktop app remains in `image-studio/` and continues to build through the existing WebView2/WebKit path.

## Architecture

```text
gio-client/
├── cmd/image-studio-gio/      # Gio app entrypoint
├── internal/compat/           # WebView2-compatible state bridge
├── internal/desktopstate/     # Gio-only workspace/window preferences
├── internal/windowing/        # Native multi-window lifecycle manager
├── internal/ui/               # Gio immediate-mode frontend
└── internal/kernel/           # adapter around go-cli/pkg/client
```

The compact toolbar switches between two experiences:

- **Simple** keeps the existing control-panel / image-canvas / history-rail workflow for new users.
- **Workflow** provides a node canvas, workspace library, inspector, queue, console, error view, and artifact view for advanced desktop use.

Workflow mode can open four independent native top-level windows: canvas, console, progress, and workspace. Each window owns its Gio widgets, text shaper, operations, focus, and viewport. Shared state crosses windows only through immutable publications and commands processed by the main UI thread.

The design language is selectable in Settings independently of light/dark appearance:

- **macOS** follows compact HIG desktop metrics, system-blue semantics, solid semantic surfaces, a contextual toolbar, hideable creation/history sidebars, and macOS typography fallbacks.
- **Windows** follows the existing Fluent visual contract, including its tighter radii and Windows desktop dimensions.

Request payload construction, retry behavior, SSE parsing, Images API support, proxy handling, and default model constants remain owned by `go-cli/pkg/client`.

The GUI entrypoint is built for Windows, macOS desktop, and Linux. Android, iOS, and WebAssembly are intentionally out of scope for this multi-window frontend.

The release workflow currently publishes Windows amd64/arm64 binaries, Linux
amd64/arm64 tarballs, and an unsigned macOS arm64 binary tarball. The macOS
artifact is not a signed or notarized `.app` bundle, so it does not claim Finder
integration or ownership of the `image-studio://` scheme.

## Desktop Session State

Gio-only preferences and workspace documents live at:

```text
<stable data root>/gio/desktop-state.json
```

This file stores the selected desktop style, simple/workflow mode, window preferences, workspace drafts, stable result references, and workflow graph positions. It does not store API keys, image bytes, base64 payloads, thumbnails, or process-local `memory://` references. See `internal/desktopstate/README.md` and `internal/ui/README.md` for ownership and extension rules.

Gio does not expose cross-platform window coordinates, parent/owner relationships, or reliable always-on-top behavior on Windows/Linux. Detached windows can be moved between monitors manually; their role and size can be restored, but exact display coordinates are not persisted.

## Custom Workflow Nodes

Workflow mode can install versioned declarative node types from the node-library import button. Installed manifests live at:

```text
<stable data root>/gio/workflow-nodes/<node-id>@<version>.json
```

A manifest creates a real reusable node type with its own globally namespaced ID, semantic version, display name, category, description, and defaults. It selects one of the trusted runtime operators (`prompt`, `source`, `generate`, `preview`, or `export`) instead of loading Go plugins, native libraries, or shell commands into the desktop process. Unknown fields, operators, properties, invalid defaults, oversized files, and attempts to replace an existing `ID@Version` are rejected.

```json
{
  "format": "image-studio-workflow-node",
  "schemaVersion": 1,
  "id": "com.example.cinematic-generate",
  "version": "1.0.0",
  "displayName": "Cinematic Generate",
  "description": "High-quality 3:2 cinematic generation preset",
  "category": "Custom/Generate",
  "operator": "generate",
  "defaults": {
    "mode": "generate",
    "quality": "high",
    "size": "1536x1024",
    "image_model": "gpt-image-2",
    "batch_count": "1"
  }
}
```

Workflow documents pin `typeId`, `typeVersion`, and the trusted operator contract. Existing workflows therefore remain executable when their installed catalog entry is temporarily unavailable; installing a newer manifest version only affects newly added nodes.

## Prompt Import Protocol

The prompt website can open the desktop client through:

```text
image-studio://import?token=XXXXXXXX
```

Protocol ownership is split intentionally:

- macOS: Wails remains the packaged default handler; a packaged Gio `.app` can receive `app.URLEvent` when its bundle declares the scheme
- Windows / Linux: handled by `gio-client/`

The Gio client exposes protocol helpers on Windows/Linux through its CLI:

```bash
go run ./cmd/image-studio-gio protocol register
go run ./cmd/image-studio-gio protocol unregister
go run ./cmd/image-studio-gio protocol status
```

For manual Linux registration you can also use:

```bash
bash ../scripts/register-gio-linux-scheme.sh /absolute/path/to/image-studio-gio
```

When the client is not yet the default `image-studio://` handler, the first launch shows an explicit registration prompt instead of silently taking over the scheme.

## WebView2 Compatibility

Gio and the Wails/WebView2 client share a compatibility state file:

```text
<stable data root>/compat/state.json
```

It stores non-secret settings, upstream profiles, the active profile id, prompt presets, prompt history, trusted output roots, and generation history. API keys are not written to JSON; both clients use the same OS keyring service, `Image Studio`, with `api-key:profile:<profile-id>`.

On Windows the stable data root is the same registry-backed root used by WebView2, `HKCU\Software\YuanHua\Image Studio\DataRoot`, defaulting to `Documents\Image Studio`. Linux uses the user config directory at `image-studio`.

## Local Build

```bash
cd gio-client
go test ./...
go build -o /tmp/image-studio-gio ./cmd/image-studio-gio
/tmp/image-studio-gio
```

Linux requires Gio's native build libraries:

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
```

Generated images default to the shared Image Studio output root unless the output directory field is changed.
