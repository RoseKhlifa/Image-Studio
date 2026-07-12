package ui

import (
	"image/color"
	"math"
	"testing"

	"gioui.org/unit"
)

func TestNormalizeDesktopStyleForGOOS(t *testing.T) {
	tests := []struct {
		name  string
		style string
		goos  string
		want  string
	}{
		{name: "explicit macos", style: "macOS", goos: "windows", want: desktopStyleMacOS},
		{name: "mac alias", style: "apple", goos: "linux", want: desktopStyleMacOS},
		{name: "explicit windows", style: "Windows", goos: "darwin", want: desktopStyleWindows},
		{name: "fluent alias", style: "fluent", goos: "darwin", want: desktopStyleWindows},
		{name: "darwin default", style: "", goos: "darwin", want: desktopStyleMacOS},
		{name: "windows default", style: "unknown", goos: "windows", want: desktopStyleWindows},
		{name: "linux default", style: "unknown", goos: "linux", want: desktopStyleWindows},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeDesktopStyleForGOOS(tt.style, tt.goos); got != tt.want {
				t.Fatalf("normalizeDesktopStyleForGOOS(%q, %q)=%q want %q", tt.style, tt.goos, got, tt.want)
			}
		})
	}
}

func TestDesktopThemeSpecCanonicalTokens(t *testing.T) {
	tests := []struct {
		name       string
		style      string
		mode       string
		wantAccent color.NRGBA
		wantBG     color.NRGBA
		wantText   color.NRGBA
	}{
		{name: "macos light", style: desktopStyleMacOS, mode: desktopColorModeLight, wantAccent: rgb(0x007aff), wantBG: rgb(0xf5f5f7), wantText: rgb(0x1d1d1f)},
		{name: "macos dark", style: desktopStyleMacOS, mode: desktopColorModeDark, wantAccent: rgb(0x0a84ff), wantBG: rgb(0x1c1c1e), wantText: rgb(0xf5f5f7)},
		{name: "windows light", style: desktopStyleWindows, mode: desktopColorModeLight, wantAccent: rgb(0x005fb8), wantBG: rgb(0xf3f3f3), wantText: rgb(0x1f1f1f)},
		{name: "windows dark", style: desktopStyleWindows, mode: desktopColorModeDark, wantAccent: rgb(0x60cdff), wantBG: rgb(0x202020), wantText: rgb(0xf3f3f3)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := desktopThemeSpec(tt.style, tt.mode)
			if spec.Colors.accent != tt.wantAccent || spec.Colors.bg != tt.wantBG || spec.Colors.text != tt.wantText {
				t.Fatalf("theme colors=%+v want accent=%v bg=%v text=%v", spec.Colors, tt.wantAccent, tt.wantBG, tt.wantText)
			}
			if spec.SurfaceTreatment != desktopSurfaceSolid {
				t.Fatalf("surface treatment=%q want %q", spec.SurfaceTreatment, desktopSurfaceSolid)
			}
		})
	}
}

func TestDesktopThemeSpecReturnsIndependentValues(t *testing.T) {
	spec := desktopThemeSpec(desktopStyleMacOS, desktopColorModeLight)
	spec.Colors.accent = rgb(0xff00ff)
	spec.Metrics.ControlRadius = 99

	fresh := desktopThemeSpec(desktopStyleMacOS, desktopColorModeLight)
	if fresh.Colors.accent != rgb(0x007aff) {
		t.Fatalf("canonical accent mutated to %v", fresh.Colors.accent)
	}
	if fresh.Metrics.ControlRadius != unit.Dp(6) {
		t.Fatalf("canonical control radius mutated to %v", fresh.Metrics.ControlRadius)
	}
}

func TestDesktopThemeContrast(t *testing.T) {
	for _, style := range []string{desktopStyleMacOS, desktopStyleWindows} {
		for _, mode := range []string{desktopColorModeLight, desktopColorModeDark} {
			spec := desktopThemeSpec(style, mode)
			name := style + "/" + mode
			t.Run(name, func(t *testing.T) {
				if got := themeContrastRatio(spec.Colors.text, spec.Colors.surface); got < 4.5 {
					t.Fatalf("body text contrast=%.2f want >=4.5", got)
				}
				if got := themeContrastRatio(spec.Colors.accent, spec.Colors.surface); got < 3 {
					t.Fatalf("accent control contrast=%.2f want >=3", got)
				}
			})
		}
	}
}

func TestDesktopThemeSmallTextContrast(t *testing.T) {
	for _, style := range []string{desktopStyleMacOS, desktopStyleWindows} {
		for _, mode := range []string{desktopColorModeLight, desktopColorModeDark} {
			spec := desktopThemeSpec(style, mode)
			name := style + "/" + mode
			t.Run(name, func(t *testing.T) {
				surfaces := []struct {
					name  string
					color color.NRGBA
				}{
					{name: "background", color: spec.Colors.bg},
					{name: "secondary background", color: spec.Colors.bg2},
					{name: "panel", color: spec.Colors.panel},
					{name: "secondary panel", color: spec.Colors.panel2},
					{name: "surface", color: spec.Colors.surface},
					{name: "secondary surface", color: spec.Colors.surface2},
					{name: "elevated surface", color: spec.Colors.surfaceElevated},
					{name: "sidebar", color: spec.Colors.sidebar},
					{name: "inspector", color: spec.Colors.inspector},
					{name: "toolbar", color: spec.Colors.toolbar},
					{name: "canvas", color: spec.Colors.canvasBg},
				}
				textColors := []struct {
					name  string
					color color.NRGBA
				}{
					{name: "muted", color: spec.Colors.textMuted},
					{name: "dim", color: spec.Colors.textDim},
					{name: "accent", color: spec.Colors.accentText},
					{name: "success", color: spec.Colors.successText},
					{name: "warning", color: spec.Colors.warningText},
					{name: "danger", color: spec.Colors.dangerText},
				}
				for _, foreground := range textColors {
					for _, background := range surfaces {
						if got := themeContrastRatio(foreground.color, background.color); got < 4.5 {
							t.Errorf("%s text on %s contrast=%.2f want >=4.5", foreground.name, background.name, got)
						}
					}
				}
			})
		}
	}
}

func TestDesktopThemeCompositeStatusTextContrast(t *testing.T) {
	for _, style := range []string{desktopStyleMacOS, desktopStyleWindows} {
		for _, mode := range []string{desktopColorModeLight, desktopColorModeDark} {
			spec := desktopThemeSpec(style, mode)
			name := style + "/" + mode
			t.Run(name, func(t *testing.T) {
				statusColors := []struct {
					name  string
					color color.NRGBA
				}{
					{name: "idle", color: spec.Colors.textMuted},
					{name: "running", color: spec.Colors.accentText},
					{name: "success", color: spec.Colors.successText},
					{name: "warning", color: spec.Colors.warningText},
					{name: "danger", color: spec.Colors.dangerText},
				}
				for _, status := range statusColors {
					chipBackground := themeCompositeOver(withAlpha(status.color, 0x20), spec.Colors.panel)
					if got := themeContrastRatio(status.color, chipBackground); got < 4.5 {
						t.Errorf("%s status chip contrast=%.2f want >=4.5", status.name, got)
					}
				}

				errorBackground := themeCompositeOver(spec.Colors.dangerSoft, spec.Colors.bg2)
				if got := themeContrastRatio(spec.Colors.dangerText, errorBackground); got < 4.5 {
					t.Errorf("danger text on danger-soft background contrast=%.2f want >=4.5", got)
				}
			})
		}
	}
}

func TestDesktopReadableTextContrast(t *testing.T) {
	black := rgb(0x000000)
	white := rgb(0xffffff)
	for _, style := range []string{desktopStyleMacOS, desktopStyleWindows} {
		for _, mode := range []string{desktopColorModeLight, desktopColorModeDark} {
			spec := desktopThemeSpec(style, mode)
			name := style + "/" + mode
			t.Run(name, func(t *testing.T) {
				fills := []struct {
					name  string
					color color.NRGBA
				}{
					{name: "primary", color: spec.Colors.accent},
					{name: "danger", color: spec.Colors.danger},
				}
				for _, fill := range fills {
					foreground := desktopReadableText(fill.color)
					if foreground != black && foreground != white {
						t.Errorf("%s foreground=%v want black or white", fill.name, foreground)
					}
					if got := themeContrastRatio(foreground, fill.color); got < 4.5 {
						t.Errorf("%s button contrast=%.2f want >=4.5", fill.name, got)
					}
					hoverFill := themeCompositeOver(withAlpha(fill.color, 0xe6), spec.Colors.toolbar)
					if got := themeContrastRatio(foreground, hoverFill); got < 4.5 {
						t.Errorf("%s hover button contrast=%.2f want >=4.5", fill.name, got)
					}
				}
			})
		}
	}
}

func TestDesktopReadableTextUsesRelativeLuminance(t *testing.T) {
	if got := desktopReadableText(rgb(0x007aff)); got != rgb(0x000000) {
		t.Fatalf("Apple blue foreground=%v want black", got)
	}
	if got := desktopReadableText(rgb(0x005fb8)); got != rgb(0xffffff) {
		t.Fatalf("Fluent blue foreground=%v want white", got)
	}
}

func TestMacOSThemeUsesOpaqueSurfaces(t *testing.T) {
	for _, mode := range []string{desktopColorModeLight, desktopColorModeDark} {
		spec := desktopThemeSpec(desktopStyleMacOS, mode)
		colors := []color.NRGBA{
			spec.Colors.bg,
			spec.Colors.panel,
			spec.Colors.panel2,
			spec.Colors.surface,
			spec.Colors.surface2,
			spec.Colors.surfaceElevated,
			spec.Colors.sidebar,
			spec.Colors.inspector,
			spec.Colors.toolbar,
		}
		for index, value := range colors {
			if value.A != 0xff {
				t.Fatalf("mode=%s surface[%d] alpha=%d want 255", mode, index, value.A)
			}
		}
	}
}

func TestDesktopThemeMetrics(t *testing.T) {
	mac := desktopThemeSpec(desktopStyleMacOS, desktopColorModeLight).Metrics
	if mac.HeaderHeight != 44 || mac.CommandBarHeight != 40 || mac.ControlHeight != 28 || mac.InputHeight != 28 || mac.ControlRadius != 6 || mac.CardRadius != 8 || mac.InputRadius != 6 || mac.NodeRadius != 8 {
		t.Fatalf("unexpected macOS metrics: %+v", mac)
	}
	win := desktopThemeSpec(desktopStyleWindows, desktopColorModeLight).Metrics
	if win.HeaderHeight != 48 || win.WorkspaceBarHeight != 38 || win.ControlHeight != 32 || win.InputHeight != 32 || win.ControlRadius != 4 || win.CardRadius != 8 || win.InputRadius != 4 || win.NodeRadius != 8 {
		t.Fatalf("unexpected Windows metrics: %+v", win)
	}
}

func TestInstallDesktopThemeSpecUpdatesCompatibilityTokens(t *testing.T) {
	previous := installedDesktopTheme
	defer installDesktopThemeSpec(previous.Style, previous.ColorMode)

	spec := installDesktopThemeSpec(desktopStyleMacOS, desktopColorModeDark)
	if fluent != spec.Colors {
		t.Fatal("fluent compatibility colors were not installed")
	}
	if fluentControlRadius != spec.Metrics.ControlRadius || fluentCardRadius != spec.Metrics.CardRadius || fluentBadgeRadius != spec.Metrics.BadgeRadius || fluentModalRadius != spec.Metrics.ModalRadius || fluentInputRadius != spec.Metrics.InputRadius {
		t.Fatalf("compatibility radii do not match installed metrics: %+v", spec.Metrics)
	}
}

func themeContrastRatio(foreground color.NRGBA, background color.NRGBA) float64 {
	foreground = themeCompositeOver(foreground, background)
	l1 := themeRelativeLuminance(foreground)
	l2 := themeRelativeLuminance(background)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func themeCompositeOver(foreground color.NRGBA, background color.NRGBA) color.NRGBA {
	alpha := float64(foreground.A) / 255
	blend := func(foregroundComponent uint8, backgroundComponent uint8) uint8 {
		return uint8(math.Round(float64(foregroundComponent)*alpha + float64(backgroundComponent)*(1-alpha)))
	}
	return color.NRGBA{
		R: blend(foreground.R, background.R),
		G: blend(foreground.G, background.G),
		B: blend(foreground.B, background.B),
		A: 0xff,
	}
}

func themeRelativeLuminance(value color.NRGBA) float64 {
	linear := func(component uint8) float64 {
		channel := float64(component) / 255
		if channel <= 0.04045 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(value.R) + 0.7152*linear(value.G) + 0.0722*linear(value.B)
}
