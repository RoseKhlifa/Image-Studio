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
		name        string
		style       string
		mode        string
		wantAccent  color.NRGBA
		wantBG      color.NRGBA
		wantText    color.NRGBA
		wantSurface string
	}{
		{name: "macos light", style: desktopStyleMacOS, mode: desktopColorModeLight, wantAccent: rgb(0x007aff), wantBG: rgb(0xeef1f6), wantText: rgb(0x111111), wantSurface: desktopSurfaceLiquidGlass},
		{name: "macos dark", style: desktopStyleMacOS, mode: desktopColorModeDark, wantAccent: rgb(0x0a84ff), wantBG: rgb(0x0d0f14), wantText: rgb(0xf5f5f7), wantSurface: desktopSurfaceLiquidGlass},
		{name: "windows light", style: desktopStyleWindows, mode: desktopColorModeLight, wantAccent: rgb(0x005fb8), wantBG: rgb(0xf3f3f3), wantText: rgb(0x1f1f1f), wantSurface: desktopSurfaceSolid},
		{name: "windows dark", style: desktopStyleWindows, mode: desktopColorModeDark, wantAccent: rgb(0x60cdff), wantBG: rgb(0x202020), wantText: rgb(0xf3f3f3), wantSurface: desktopSurfaceSolid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := desktopThemeSpec(tt.style, tt.mode)
			if spec.Colors.accent != tt.wantAccent || spec.Colors.bg != tt.wantBG || spec.Colors.text != tt.wantText {
				t.Fatalf("theme colors=%+v want accent=%v bg=%v text=%v", spec.Colors, tt.wantAccent, tt.wantBG, tt.wantText)
			}
			if spec.SurfaceTreatment != tt.wantSurface {
				t.Fatalf("surface treatment=%q want %q", spec.SurfaceTreatment, tt.wantSurface)
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
	if fresh.Metrics.ControlRadius != unit.Dp(17) {
		t.Fatalf("canonical control radius mutated to %v", fresh.Metrics.ControlRadius)
	}
}

func TestMacOSThemePaletteMatchesReactAppleFrontend(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want map[string]color.NRGBA
	}{
		{
			name: "light",
			mode: desktopColorModeLight,
			want: map[string]color.NRGBA{
				"accent":          rgb(0x007aff),
				"accent2":         rgb(0x409cff),
				"accentSoft":      rgba(0x007aff, 0x1a),
				"bg":              rgb(0xeef1f6),
				"bg2":             rgb(0xe5e9f1),
				"panel":           rgba(0xffffff, 0x94),
				"panel2":          rgba(0xffffff, 0x75),
				"surface":         rgba(0xffffff, 0x80),
				"surface2":        rgba(0xffffff, 0xa8),
				"surfaceElevated": rgba(0xffffff, 0xd1),
				"sidebar":         rgba(0xffffff, 0x6b),
				"inspector":       rgba(0xffffff, 0x61),
				"toolbar":         rgba(0xffffff, 0x5c),
				"border":          rgba(0x3c3c43, 0x21),
				"border2":         rgba(0x3c3c43, 0x33),
				"text":            rgb(0x111111),
				"textMuted":       rgba(0x3c3c43, 0xb8),
				"textDim":         rgba(0x3c3c43, 0x7a),
				"cardShadow":      rgba(0x161c2d, 0x1f),
				"cardGlow":        rgba(0xffffff, 0x5c),
				"bgGlow":          rgba(0x5ac8fa, 0x38),
				"canvasBg":        rgb(0xeef1f6),
				"canvasTile":      rgb(0xdce4ef),
				"windowOutline":   rgba(0xffffff, 0x5c),
			},
		},
		{
			name: "dark",
			mode: desktopColorModeDark,
			want: map[string]color.NRGBA{
				"accent":          rgb(0x0a84ff),
				"accent2":         rgb(0x5eb0ff),
				"accentSoft":      rgba(0x0a84ff, 0x2e),
				"bg":              rgb(0x0d0f14),
				"bg2":             rgb(0x11141b),
				"panel":           rgba(0x1e2026, 0x94),
				"panel2":          rgba(0xffffff, 0x1a),
				"surface":         rgba(0xffffff, 0x1c),
				"surface2":        rgba(0xffffff, 0x29),
				"surfaceElevated": rgba(0x1e212a, 0xe6),
				"sidebar":         rgba(0x14171f, 0xb8),
				"inspector":       rgba(0x14171f, 0xad),
				"toolbar":         rgba(0x151821, 0xa3),
				"border":          rgba(0xffffff, 0x24),
				"border2":         rgba(0xffffff, 0x38),
				"text":            rgb(0xf5f5f7),
				"textMuted":       rgba(0xebebf5, 0xc7),
				"textDim":         rgba(0xebebf5, 0x8a),
				"cardShadow":      rgba(0x000000, 0x52),
				"cardGlow":        rgba(0xffffff, 0x1a),
				"bgGlow":          rgba(0x0a84ff, 0x33),
				"canvasBg":        rgb(0x141821),
				"canvasTile":      rgb(0x252b37),
				"windowOutline":   rgba(0xffffff, 0x1a),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			colors := macOSDesktopColors(tt.mode)
			got := map[string]color.NRGBA{
				"accent":          colors.accent,
				"accent2":         colors.accent2,
				"accentSoft":      colors.accentSoft,
				"bg":              colors.bg,
				"bg2":             colors.bg2,
				"panel":           colors.panel,
				"panel2":          colors.panel2,
				"surface":         colors.surface,
				"surface2":        colors.surface2,
				"surfaceElevated": colors.surfaceElevated,
				"sidebar":         colors.sidebar,
				"inspector":       colors.inspector,
				"toolbar":         colors.toolbar,
				"border":          colors.border,
				"border2":         colors.border2,
				"text":            colors.text,
				"textMuted":       colors.textMuted,
				"textDim":         colors.textDim,
				"cardShadow":      colors.cardShadow,
				"cardGlow":        colors.cardGlow,
				"bgGlow":          colors.bgGlow,
				"canvasBg":        colors.canvasBg,
				"canvasTile":      colors.canvasTile,
				"windowOutline":   colors.windowOutline,
			}
			for name, want := range tt.want {
				if got[name] != want {
					t.Errorf("%s=%v want %v", name, got[name], want)
				}
			}
		})
	}
}

func TestDesktopThemeContrast(t *testing.T) {
	for _, style := range []string{desktopStyleMacOS, desktopStyleWindows} {
		for _, mode := range []string{desktopColorModeLight, desktopColorModeDark} {
			spec := desktopThemeSpec(style, mode)
			name := style + "/" + mode
			t.Run(name, func(t *testing.T) {
				surface := themeCompositeOver(spec.Colors.surface, spec.Colors.bg)
				if got := themeContrastRatio(spec.Colors.text, surface); got < 4.5 {
					t.Fatalf("body text contrast=%.2f want >=4.5", got)
				}
				if got := themeContrastRatio(spec.Colors.accent, surface); got < 3 {
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
					{name: "body", color: spec.Colors.text},
					{name: "muted", color: spec.Colors.textMuted},
					{name: "accent", color: spec.Colors.accentText},
					{name: "success", color: spec.Colors.successText},
					{name: "warning", color: spec.Colors.warningText},
					{name: "danger", color: spec.Colors.dangerText},
				}
				for _, foreground := range textColors {
					for _, background := range surfaces {
						if foreground.name == "muted" && (background.name == "background" || background.name == "secondary background" || background.name == "canvas") {
							continue
						}
						resolvedBackground := themeCompositeOver(background.color, spec.Colors.bg)
						if got := themeContrastRatio(foreground.color, resolvedBackground); got < 4.5 {
							t.Errorf("%s text on %s contrast=%.2f want >=4.5", foreground.name, background.name, got)
						}
					}
				}
			})
		}
	}
}

func TestDesktopThemeSupplementalTextContrastFloor(t *testing.T) {
	for _, style := range []string{desktopStyleMacOS, desktopStyleWindows} {
		for _, mode := range []string{desktopColorModeLight, desktopColorModeDark} {
			spec := desktopThemeSpec(style, mode)
			name := style + "/" + mode
			t.Run(name, func(t *testing.T) {
				surfaces := []color.NRGBA{
					spec.Colors.bg,
					spec.Colors.bg2,
					spec.Colors.panel,
					spec.Colors.surface,
					spec.Colors.toolbar,
					spec.Colors.canvasBg,
				}
				for index, surface := range surfaces {
					resolved := themeCompositeOver(surface, spec.Colors.bg)
					if got := themeContrastRatio(spec.Colors.textMuted, resolved); got < 4 {
						t.Errorf("muted supplemental text on surface[%d] contrast=%.2f want >=4.0", index, got)
					}
					if got := themeContrastRatio(spec.Colors.textDim, resolved); got < 2.4 {
						t.Errorf("dim decorative text on surface[%d] contrast=%.2f want >=2.4", index, got)
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
				panel := themeCompositeOver(spec.Colors.panel, spec.Colors.bg)
				for _, status := range statusColors {
					chipBackground := themeCompositeOver(withAlpha(status.color, 0x20), panel)
					minimum := 4.5
					if status.name == "idle" {
						minimum = 4
					}
					if got := themeContrastRatio(status.color, chipBackground); got < minimum {
						t.Errorf("%s status chip contrast=%.2f want >=%.1f", status.name, got, minimum)
					}
				}

				errorSurface := themeCompositeOver(spec.Colors.bg2, spec.Colors.bg)
				errorBackground := themeCompositeOver(spec.Colors.dangerSoft, errorSurface)
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
					toolbar := themeCompositeOver(spec.Colors.toolbar, spec.Colors.bg)
					hoverFill := themeCompositeOver(withAlpha(fill.color, 0xe6), toolbar)
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

func TestMacOSThemeUsesLiquidGlassSurfaces(t *testing.T) {
	for _, mode := range []string{desktopColorModeLight, desktopColorModeDark} {
		spec := desktopThemeSpec(desktopStyleMacOS, mode)
		if spec.SurfaceTreatment != desktopSurfaceLiquidGlass {
			t.Fatalf("mode=%s surface treatment=%q want %q", mode, spec.SurfaceTreatment, desktopSurfaceLiquidGlass)
		}
		opaque := []color.NRGBA{
			spec.Colors.bg,
			spec.Colors.bg2,
			spec.Colors.canvasBg,
			spec.Colors.canvasTile,
		}
		for index, value := range opaque {
			if value.A != 0xff {
				t.Fatalf("mode=%s opaque[%d] alpha=%d want 255", mode, index, value.A)
			}
		}
		glass := []color.NRGBA{
			spec.Colors.panel,
			spec.Colors.panel2,
			spec.Colors.surface,
			spec.Colors.surface2,
			spec.Colors.surfaceElevated,
			spec.Colors.sidebar,
			spec.Colors.inspector,
			spec.Colors.toolbar,
		}
		for index, value := range glass {
			if value.A == 0 || value.A >= 0xff {
				t.Fatalf("mode=%s glass[%d] alpha=%d want 1..254", mode, index, value.A)
			}
		}
		if spec.Colors.cardShadow.A == 0 || spec.Colors.cardGlow.A == 0 || spec.Colors.windowOutline.A == 0 {
			t.Fatalf("mode=%s static glass effects must remain visible: shadow=%v glow=%v outline=%v", mode, spec.Colors.cardShadow, spec.Colors.cardGlow, spec.Colors.windowOutline)
		}
	}
}

func TestWindowsThemeKeepsSolidSurfaces(t *testing.T) {
	for _, mode := range []string{desktopColorModeLight, desktopColorModeDark} {
		spec := desktopThemeSpec(desktopStyleWindows, mode)
		if spec.SurfaceTreatment != desktopSurfaceSolid {
			t.Fatalf("mode=%s surface treatment=%q want %q", mode, spec.SurfaceTreatment, desktopSurfaceSolid)
		}
		surfaces := []color.NRGBA{
			spec.Colors.bg,
			spec.Colors.bg2,
			spec.Colors.panel,
			spec.Colors.panel2,
			spec.Colors.surface,
			spec.Colors.surface2,
			spec.Colors.surfaceElevated,
			spec.Colors.sidebar,
			spec.Colors.inspector,
			spec.Colors.toolbar,
			spec.Colors.canvasBg,
			spec.Colors.canvasTile,
		}
		for index, value := range surfaces {
			if value.A != 0xff {
				t.Fatalf("mode=%s surface[%d] alpha=%d want 255", mode, index, value.A)
			}
		}
	}
}

func TestDesktopThemeMetrics(t *testing.T) {
	mac := desktopThemeSpec(desktopStyleMacOS, desktopColorModeLight).Metrics
	wantMac := desktopThemeMetrics{
		HeaderHeight:       68,
		WorkspaceBarHeight: 40,
		CommandBarHeight:   50,
		StatusBarHeight:    28,
		ControlHeight:      34,
		InputHeight:        34,
		IconTargetSize:     32,
		RowHeight:          34,
		LeftPaneWidth:      408,
		RightPaneWidth:     352,
		ConsoleHeight:      220,
		NodeWidth:          248,
		ControlRadius:      17,
		CardRadius:         22,
		BadgeRadius:        17,
		ModalRadius:        22,
		InputRadius:        14,
		NodeRadius:         14,
	}
	if mac != wantMac {
		t.Fatalf("unexpected macOS metrics: %+v", mac)
	}
	win := desktopThemeSpec(desktopStyleWindows, desktopColorModeLight).Metrics
	wantWin := desktopThemeMetrics{
		HeaderHeight:       48,
		WorkspaceBarHeight: 38,
		CommandBarHeight:   48,
		StatusBarHeight:    36,
		ControlHeight:      32,
		InputHeight:        32,
		IconTargetSize:     32,
		RowHeight:          32,
		LeftPaneWidth:      360,
		RightPaneWidth:     320,
		ConsoleHeight:      220,
		NodeWidth:          248,
		ControlRadius:      4,
		CardRadius:         8,
		BadgeRadius:        4,
		ModalRadius:        8,
		InputRadius:        4,
		NodeRadius:         8,
	}
	if win != wantWin {
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
	if currentDesktopThemeMetrics() != spec.Metrics {
		t.Fatalf("current metrics=%+v want %+v", currentDesktopThemeMetrics(), spec.Metrics)
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
