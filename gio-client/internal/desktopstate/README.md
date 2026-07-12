# Desktop State

`desktopstate` owns Gio-only desktop presentation state. It is independent of
`internal/ui`, Gio widgets, the request kernel, and the WebView compatibility
state.

The package stores:

- desktop preferences, including the selected Apple or Windows visual language;
- the simple or workflow experience mode;
- detached-window roles, requested sizes, and workspace bindings;
- typed workspace drafts and stable result references;
- lightweight workspace workflow graphs with nodes and edges.

It does not store API keys, upstream profiles, generation history, image data,
decoded images, active jobs, or Gio widget state. Those remain owned by the
existing compatibility, runtime, and per-window UI layers. Workflow nodes should
refer to durable artifacts by identifier instead of embedding image bytes.
Process-local `memory://` image and text references are filtered by the UI
bridge on save and load because their backing stores disappear at process exit.

The schema reserves window `mode`/`visible` and graph `viewport` fields for a
future platform adapter. The current Gio bridge writes detached windows as
visible/windowed, restores role and size only, and keeps pan/zoom window-local.

## File

The owner chooses and injects the full path into `NewStore`. Production callers
should use a path ending in `desktop-state.json` under the Gio desktop data
directory. Tests can use any temporary path.

The current format is schema version 1:

```json
{
  "schemaVersion": 1,
  "revision": 4,
  "updatedAt": 1783737600000,
  "preferences": {
    "interfaceStyle": "windows",
    "experienceMode": "workflow",
    "defaultWindowLayout": "dual",
    "autoShowProgress": true,
    "reopenDetachedWindows": true,
    "restoreSession": true
  },
  "windows": [
    {
      "id": "canvas-1",
      "role": "canvas",
      "workspaceId": "workspace-1",
      "widthDp": 1100,
      "heightDp": 760,
      "mode": "windowed",
      "visible": true
    }
  ],
  "workspaces": [
    {
      "id": "workspace-1",
      "name": "Campaign",
      "draft": {
        "prompt": "Editorial cover with clean studio lighting",
        "mode": "generate",
        "size": "1024x1024",
        "quality": "high",
        "outputFormat": "png",
        "sourcePaths": [],
        "batchCount": 1
      },
      "result": {
        "historyId": "history-item-id",
        "savedPath": "/path/to/result.png",
        "rawPath": "/path/to/raw-response.json",
        "revisedPrompt": "Revised prompt returned by the upstream"
      },
      "workflow": {
        "nodes": [],
        "edges": [],
        "viewport": {"offsetX": 0, "offsetY": 0, "zoom": 1}
      }
    }
  ]
}
```

## Workspace ownership

`WorkspaceDraft` is the typed, window-independent authoring state used to
restore a workspace. It includes prompts, generation options, source file paths,
preset selection, loop settings, and batch settings. It intentionally contains
no API key, bearer token, upstream credential, keyring material, or other secret.
Credentials remain in the shared compatibility state and OS keyring.

`WorkspaceResult` stores only stable references and small text metadata:
`historyId`, `savedPath`, `rawPath`, and `revisedPrompt`. It never stores decoded
images, base64 payloads, thumbnails, previews, or any other image bytes. Durable
image metadata and binary ownership remain with generation history and the file
system. A workflow node should point at those durable artifacts instead of
copying them into `desktop-state.json`.

`Store.Save` normalizes the model, advances `revision`, updates `updatedAt`, and
atomically replaces the file through a same-directory temporary file. A missing
file loads as `Default()` without error. Invalid JSON or an invalid schema loads
as `Default()` together with `*CorruptStateError`, so the caller can continue in
a known state while still reporting or backing up the damaged file. A newer
schema returns `*UnsupportedVersionError` and is never overwritten implicitly.

## API

- `Default()` returns a new normalized version-1 state.
- `Normalize(State)` repairs enum values, identifiers, dimensions, graph
  collections, references, and viewport values.
- `NewStore(path)` creates a path-injected, concurrency-safe store.
- `(*Store).Load()` loads and normalizes state.
- `(*Store).Save(state)` atomically persists state and returns the saved revision.
- `Load(path)` and `Save(path, state)` are convenience wrappers.
