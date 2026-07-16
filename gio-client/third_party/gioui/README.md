# Gio - https://gioui.org

Immediate mode GUI programs in Go for Android, iOS, macOS, Linux,
FreeBSD, OpenBSD, Windows, and WebAssembly (experimental).

# Installation, examples, documentation

Go to [gioui.org](https://gioui.org).

[![builds.sr.ht status](https://builds.sr.ht/~eliasnaur/gio.svg)](https://builds.sr.ht/~eliasnaur/gio)

## Issues

File bugs and TODOs through the [issue tracker](https://todo.sr.ht/~eliasnaur/gio) or send an email
to [~eliasnaur/gio@todo.sr.ht](mailto:~eliasnaur/gio@todo.sr.ht). For general discussion, use the
mailing list: [~eliasnaur/gio@lists.sr.ht](mailto:~eliasnaur/gio@lists.sr.ht).

## Contributing

Post discussion to the [mailing list](https://lists.sr.ht/~eliasnaur/gio) and patches to
[gio-patches](https://lists.sr.ht/~eliasnaur/gio-patches). No Sourcehut
account is required and you can post without being subscribed.

See the [contribution guide](https://gioui.org/doc/contribute) for more details.

An [official GitHub mirror](https://github.com/gioui/gio) is available.

## Tags

Pre-1.0 tags are provided for reference only, and do not designate releases with ongoing support. Bugfixes will not be backported to older tags.

Tags follow semantic versioning. In particular, as the major version is zero:

- breaking API or behavior changes will increment the *minor* version component.
- non-breaking changes will increment the *patch* version component.

## Image Studio macOS compatibility patches

This directory is an otherwise unmodified mirror of `gioui.org` v0.10.1
(upstream commit `1a5fa17a39e38089c24e312dd3cd248cbebba3a5`). The upstream license is
preserved in `LICENSE`.

Image Studio carries a Darwin display-link patch for macOS 14 and newer.
On macOS 27, `CVDisplayLinkCreateWithActiveCGDisplays` returns
`kCVReturnInvalidArgument` (`-6661`) and a nil display link. Gio v0.10.1 ignores
that return code, then releases a `GioView` before its cgo handle is installed,
which panics with `runtime/cgo: misuse of an invalid Handle`.

The patch is limited to:

- `app/os_macos.m`: use the supported `NSView` `CADisplayLink` API on macOS 14+
  and validate every legacy CoreVideo return value;
- `app/os_macos.go`: pass the native view to display-link creation and ignore a
  zero handle during failed initialization cleanup;
- `app/os_darwin.go`, `app/os_ios.go`, and `app/os_ios.m`: carry the native view
  through the shared Darwin display-link constructor without changing iOS
  behavior.

The desktop multi-window integration also carries two narrowly scoped window
action fixes:

- `app/window.go`: snapshot the driver under `invMu` before `Perform` so a
  concurrent native close cannot race a driver read or turn the second lookup
  into a nil dereference;
- `app/os_macos.go`: post `performClose:` to the next main-loop turn so the Gio
  client event goroutine can return to `Event` and receive `DestroyEvent`.

It also carries an opt-in `app.FullscreenAuxiliary` option. The macOS backend
preserves unrelated `NSWindow.collectionBehavior` bits and controls only
`NSWindowCollectionBehaviorMoveToActiveSpace` and
`NSWindowCollectionBehaviorFullScreenAuxiliary`. Image Studio applies this
option through `internal/windowing.Manager` only; its primary window keeps Gio's
default collection behavior. The option is available through the common
`app.Config` API on every platform but is otherwise a no-op outside macOS.

Remove the local `replace gioui.org => ./third_party/gioui` and this directory
after an upstream Gio release contains equivalent macOS display-link handling.
Before removal, verify a real macOS launch with multiple Gio windows, normal
window closure without a cgo panic, Windows compilation, and native Linux CGO
compilation.
