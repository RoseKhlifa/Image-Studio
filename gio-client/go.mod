module image-studio/gio-client

go 1.25.5

toolchain go1.26.3

require (
	gioui.org v0.10.1
	github.com/gen2brain/avif v0.4.4
	github.com/yuanhua/image-gptcodex v0.0.0-00010101000000-000000000000
	github.com/zalando/go-keyring v0.2.6
	golang.org/x/exp/shiny v0.0.0-20250408133849-7e4ce0ab07d0
	golang.org/x/image v0.41.0
	golang.org/x/sys v0.39.0
	image-studio/shared/compat v0.0.0
)

require (
	al.essio.dev/pkg/shellescape v1.5.1 // indirect
	gioui.org/shader v1.0.8 // indirect
	github.com/danieljoos/wincred v1.2.2 // indirect
	github.com/ebitengine/purego v0.8.3 // indirect
	github.com/go-text/typesetting v0.3.4 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/tetratelabs/wazero v1.9.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/text v0.37.0 // indirect
)

replace github.com/yuanhua/image-gptcodex => ../go-cli

replace image-studio/shared/compat => ../shared/compat-go

// Remove when upstream Gio ships the macOS 14+ NSView display-link fix.
replace gioui.org => ./third_party/gioui
