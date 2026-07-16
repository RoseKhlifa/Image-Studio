package ui

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"image-studio/gio-client/internal/windowing"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	mdicons "golang.org/x/exp/shiny/materialdesign/icons"
)

var errDesktopWindowRequired = errors.New("ui: desktop window is required")

type desktopWindowButtonTone uint8

const (
	desktopButtonNeutral desktopWindowButtonTone = iota
	desktopButtonPrimary
	desktopButtonDanger
	desktopButtonSelected
	desktopButtonDisabled
)

type desktopButtonVisual struct {
	Fill   color.NRGBA
	Border color.NRGBA
	Text   color.NRGBA
}

type desktopWindowIcons struct {
	play      *widget.Icon
	cancel    *widget.Icon
	clear     *widget.Icon
	canvas    *widget.Icon
	console   *widget.Icon
	progress  *widget.Icon
	workspace *widget.Icon
	main      *widget.Icon
	activate  *widget.Icon
	save      *widget.Icon
	undo      *widget.Icon
	redo      *widget.Icon
}

type desktopWindowView struct {
	root    *App
	request windowing.Request

	ops          op.Ops
	theme        *material.Theme
	canvas       workflowCanvasViewState
	config       app.Config
	metric       unit.Metric
	reportedSize image.Point
	desktopStyle string
	fontScale    float64
	previewRev   int
	previewOp    paint.ImageOp
	previewValid bool

	runButton           widget.Clickable
	cancelButton        widget.Clickable
	clearButton         widget.Clickable
	activateButton      widget.Clickable
	openCanvasButton    widget.Clickable
	openConsoleButton   widget.Clickable
	openProgressButton  widget.Clickable
	openWorkspaceButton widget.Clickable
	raiseMainButton     widget.Clickable
	applyDraftButton    widget.Clickable
	undoButton          widget.Clickable
	redoButton          widget.Clickable

	promptEditor        widget.Editor
	negativeEditor      widget.Editor
	draftModeButtons    []widget.Clickable
	draftSizeButtons    []widget.Clickable
	draftQualityButtons []widget.Clickable
	draftFormatButtons  []widget.Clickable
	draftWorkspaceID    string
	draftRevision       uint64
	draftBaseline       desktopDraftUpdate
	draftDirty          bool
	draftMode           string
	draftSize           string
	draftQuality        string
	draftFormat         string

	workspaceButtons map[string]*widget.Clickable
	consoleList      widget.List
	workspaceList    widget.List
	draftList        widget.List
	consoleFilterID  string
	commandError     string
	icons            desktopWindowIcons
	keyboardTag      struct{}
}

func newDesktopWindowView(root *App, request windowing.Request) *desktopWindowView {
	view := &desktopWindowView{
		root:             root,
		request:          request,
		theme:            newDetachedWindowTheme(),
		workspaceButtons: make(map[string]*widget.Clickable),
		icons: desktopWindowIcons{
			play:      newDetachedWindowIcon(mdicons.AVPlayArrow),
			cancel:    newDetachedWindowIcon(mdicons.NavigationCancel),
			clear:     newDetachedWindowIcon(mdicons.ContentClear),
			canvas:    newDetachedWindowIcon(mdicons.ActionTimeline),
			console:   newDetachedWindowIcon(mdicons.ActionCode),
			progress:  newDetachedWindowIcon(mdicons.AVEqualizer),
			workspace: newDetachedWindowIcon(mdicons.ActionViewQuilt),
			main:      newDetachedWindowIcon(mdicons.ActionLaunch),
			activate:  newDetachedWindowIcon(mdicons.NavigationCheck),
			save:      newDetachedWindowIcon(mdicons.ContentSave),
			undo:      newDetachedWindowIcon(mdicons.ContentUndo),
			redo:      newDetachedWindowIcon(mdicons.ContentRedo),
		},
		draftModeButtons:    make([]widget.Clickable, len(modeChoices)),
		draftSizeButtons:    make([]widget.Clickable, len(sizeChoices)),
		draftQualityButtons: make([]widget.Clickable, len(qualityChoices)),
		draftFormatButtons:  make([]widget.Clickable, len(formatChoices)),
	}
	view.consoleList.List.Axis = layout.Vertical
	view.workspaceList.List.Axis = layout.Horizontal
	view.draftList.List.Axis = layout.Vertical
	view.canvas.ensure()
	view.promptEditor.SingleLine = false
	view.negativeEditor.SingleLine = false
	return view
}

func newDetachedWindowTheme() *material.Theme {
	theme := material.NewTheme()
	collection := bundledFontCollection()
	if len(collection) > 0 {
		theme.Shaper = text.NewShaper(text.WithCollection(collection))
	} else {
		theme.Shaper = text.NewShaper()
	}
	theme.Face = uiSansTypeface
	return theme
}

func newDetachedWindowIcon(data []byte) *widget.Icon {
	icon, err := widget.NewIcon(data)
	if err != nil {
		return nil
	}
	return icon
}

func (view *desktopWindowView) Run(window *app.Window, actions windowing.WindowActions) error {
	if view == nil || view.root == nil {
		return errDesktopWindowRootRequired
	}
	if window == nil {
		return errDesktopWindowRequired
	}

	for {
		event := window.Event()
		if _, destroying := event.(app.DestroyEvent); !destroying {
			// Fetching the first event initializes the native driver. Processing
			// actions afterwards avoids racing an early close with Window.init.
			view.performPendingWindowActions(window, actions)
		}
		switch event := event.(type) {
		case app.FrameEvent:
			view.metric = event.Metric
			view.reportWindowSize()
			gtx := app.NewContext(&view.ops, event)
			view.layout(gtx, view.root.desktopSnapshot())
			event.Frame(gtx.Ops)
		case app.ConfigEvent:
			view.config = event.Config
			view.reportWindowSize()
		case app.DestroyEvent:
			return event.Err
		}
	}
}

func (view *desktopWindowView) reportWindowSize() {
	if view == nil || view.root == nil || view.metric.PxPerDp <= 0 || view.config.Size.X <= 0 || view.config.Size.Y <= 0 {
		return
	}
	size := image.Pt(
		int(math.Round(float64(view.metric.PxToDp(view.config.Size.X)))),
		int(math.Round(float64(view.metric.PxToDp(view.config.Size.Y)))),
	)
	if size.X <= 0 || size.Y <= 0 || size == view.reportedSize {
		return
	}
	view.reportedSize = size
	view.root.recordDesktopWindowSize(view.request, size)
}

type desktopWindowActionPerformer interface {
	Perform(system.Action)
}

func (view *desktopWindowView) performPendingWindowActions(window desktopWindowActionPerformer, actions windowing.WindowActions) {
	if window == nil || actions == nil {
		return
	}
	if pending := actions.Take(); pending != 0 {
		window.Perform(pending)
	}
}

func (view *desktopWindowView) layout(gtx layout.Context, publication desktopPublication) layout.Dimensions {
	spec := desktopThemeSpec(publication.DesktopStyle, publication.ColorMode)
	view.applyTheme(spec, publication.FontScale)
	gtx.Constraints.Min = gtx.Constraints.Max
	paint.FillShape(gtx.Ops, spec.Colors.bg, clip.Rect{Max: gtx.Constraints.Max}.Op())

	for view.raiseMainButton.Clicked(gtx) {
		view.enqueue(desktopCommand{Kind: desktopCommandRaiseMain})
	}

	switch view.request.Role {
	case windowing.RoleCanvas:
		return view.layoutDetachedCanvas(gtx, spec, publication)
	case windowing.RoleConsole:
		return view.layoutDetachedConsole(gtx, spec, publication)
	case windowing.RoleProgress:
		return view.layoutDetachedProgress(gtx, spec, publication)
	case windowing.RoleWorkspace:
		return view.layoutDetachedWorkspace(gtx, spec, publication)
	default:
		return view.layoutUnavailable(gtx, spec, "不支持的窗口", "该桌面窗口角色无法显示。")
	}
}

func (view *desktopWindowView) applyTheme(spec desktopThemeTokens, fontScale ...float64) {
	scale := float64(0)
	if len(fontScale) > 0 {
		scale = fontScale[0]
	}
	view.fontScale = normalizeFontScale(scale)
	view.desktopStyle = spec.Style
	view.theme.Palette = material.Palette{
		Bg:         spec.Colors.bg,
		Fg:         spec.Colors.text,
		ContrastBg: spec.Colors.accent,
		ContrastFg: desktopReadableText(spec.Colors.accent),
	}
	view.theme.Face = desktopSansTypeface(spec.Style)
	if spec.Style == desktopStyleMacOS {
		view.theme.TextSize = view.scaledTextSize(unit.Sp(13))
	} else {
		view.theme.TextSize = view.scaledTextSize(unit.Sp(14))
	}
}

func (view *desktopWindowView) scaledTextSize(size unit.Sp) unit.Sp {
	scale := float64(0)
	if view != nil {
		scale = view.fontScale
	}
	return unit.Sp(float32(size) * float32(normalizeFontScale(scale)))
}

func (view *desktopWindowView) boundWorkspace(publication desktopPublication) (desktopWorkspacePublication, bool) {
	workspaceID := strings.TrimSpace(view.request.WorkspaceID)
	if workspaceID == "" {
		workspaceID = strings.TrimSpace(publication.ActiveID)
	}
	return publication.workspace(workspaceID)
}

func (view *desktopWindowView) enqueue(command desktopCommand) bool {
	if view == nil {
		return false
	}
	if view.root == nil || !view.root.enqueueDesktopCommand(command) {
		view.commandError = "主窗口命令队列繁忙，请稍后重试"
		return false
	}
	view.commandError = ""
	return true
}

func (view *desktopWindowView) enqueueOpen(role windowing.Role, workspaceID string) bool {
	return view.enqueue(desktopCommand{
		Kind:        desktopCommandOpenWindow,
		WorkspaceID: workspaceID,
		WindowRole:  role,
	})
}

func (view *desktopWindowView) canvasCallbacks(workspaceID string) workflowCanvasCallbacks {
	return workflowCanvasCallbacks{
		Select: func(nodeID string) {
			view.enqueue(desktopCommand{
				Kind:        desktopCommandSelectNode,
				WorkspaceID: workspaceID,
				NodeID:      nodeID,
			})
		},
		MoveStart: func(nodeID string) {
			view.enqueue(desktopCommand{
				Kind:        desktopCommandBeginNodeMove,
				WorkspaceID: workspaceID,
				NodeID:      nodeID,
			})
		},
		Move: func(nodeID string, position image.Point) {
			view.enqueue(desktopCommand{
				Kind:        desktopCommandMoveNode,
				WorkspaceID: workspaceID,
				NodeID:      nodeID,
				Position:    position,
			})
		},
		MoveEnd: func(nodeID string) {
			view.enqueue(desktopCommand{
				Kind:        desktopCommandEndNodeMove,
				WorkspaceID: workspaceID,
				NodeID:      nodeID,
			})
		},
		RewireConnection: func(previous *workflowEdgeModel, replacement *workflowEdgeModel) {
			command := desktopCommand{
				Kind:        desktopCommandRewireConnection,
				WorkspaceID: workspaceID,
			}
			if previous != nil {
				command.HasPreviousEdge = true
				command.PreviousEdge = *previous
			}
			if replacement != nil {
				command.HasEdge = true
				command.Edge = *replacement
			}
			view.enqueue(command)
		},
	}
}

func (view *desktopWindowView) handleWorkflowHistoryShortcuts(gtx layout.Context, workspaceID string, selectedNode string) {
	event.Op(gtx.Ops, &view.keyboardTag)
	if gtx.Focused(&view.promptEditor) || gtx.Focused(&view.negativeEditor) {
		return
	}
	for {
		eventValue, ok := gtx.Event(
			key.Filter{Name: "Z", Required: key.ModCtrl, Optional: key.ModShift},
			key.Filter{Name: "Z", Required: key.ModCommand, Optional: key.ModShift},
			key.Filter{Name: "Y", Required: key.ModCtrl},
			key.Filter{Name: "Y", Required: key.ModCommand},
			key.Filter{Name: "M", Required: key.ModCtrl},
			key.Filter{Name: "M", Required: key.ModCommand},
			key.Filter{Name: key.NameDeleteBackward},
			key.Filter{Name: key.NameDeleteForward},
		)
		if !ok {
			break
		}
		keyEvent, ok := eventValue.(key.Event)
		if !ok || keyEvent.State != key.Press {
			continue
		}
		kind := desktopCommandUndoWorkflow
		switch keyEvent.Name {
		case "Y":
			kind = desktopCommandRedoWorkflow
		case "Z":
			if keyEvent.Modifiers.Contain(key.ModShift) {
				kind = desktopCommandRedoWorkflow
			}
		case "M":
			kind = desktopCommandToggleWorkflowNode
		case key.NameDeleteBackward, key.NameDeleteForward:
			kind = desktopCommandDeleteWorkflowNode
		}
		view.enqueue(desktopCommand{Kind: kind, WorkspaceID: workspaceID, NodeID: selectedNode})
	}
}

func (view *desktopWindowView) workspaceButton(key string) *widget.Clickable {
	if button := view.workspaceButtons[key]; button != nil {
		return button
	}
	button := new(widget.Clickable)
	view.workspaceButtons[key] = button
	return button
}

func (view *desktopWindowView) layoutToolbar(
	gtx layout.Context,
	spec desktopThemeTokens,
	title string,
	context string,
	actions ...layout.Widget,
) layout.Dimensions {
	heightToken := spec.Metrics.CommandBarHeight
	if heightToken < unit.Dp(40) {
		heightToken = unit.Dp(40)
	}
	height := gtx.Dp(minimumTextControlHeight(gtx, heightToken, view.scaledTextSize(unit.Sp(13)), unit.Dp(8)))
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, height))
	paint.FillShape(gtx.Ops, spec.Colors.toolbar, clip.Rect{Max: gtx.Constraints.Max}.Op())
	paint.FillShape(gtx.Ops, spec.Colors.border, clip.Rect(image.Rect(0, height-1, gtx.Constraints.Max.X, height)).Op())
	return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return view.label(gtx, title, unit.Sp(13), spec.Colors.text, font.SemiBold, 1)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return view.label(gtx, context, unit.Sp(11), spec.Colors.textMuted, font.Normal, 1)
					}),
				)
			}),
		}
		for index, action := range actions {
			if index > 0 {
				children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout))
			}
			children = append(children, layout.Rigid(action))
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

func (view *desktopWindowView) button(
	gtx layout.Context,
	spec desktopThemeTokens,
	button *widget.Clickable,
	icon *widget.Icon,
	label string,
	tone desktopWindowButtonTone,
) layout.Dimensions {
	interaction := buttonInteractionState{
		Hovered: button.Hovered(),
		Focused: gtx.Focused(button),
		Pressed: button.Pressed(),
	}
	visual := resolveDesktopButtonVisual(spec, tone, interaction)
	return button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.EnabledOp(tone != desktopButtonDisabled).Add(gtx.Ops)
		if name := strings.TrimSpace(label); name != "" {
			semantic.LabelOp(name).Add(gtx.Ops)
		}
		heightToken := spec.Metrics.ControlHeight
		if heightToken < unit.Dp(28) {
			heightToken = unit.Dp(28)
		}
		height := gtx.Dp(minimumTextControlHeight(gtx, heightToken, view.scaledTextSize(unit.Sp(11)), unit.Dp(8)))
		gtx.Constraints.Min.Y = height
		macro := op.Record(gtx.Ops)
		dims := layout.Inset{Left: unit.Dp(9), Right: unit.Dp(9), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 3)
			if icon != nil {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					size := gtx.Dp(unit.Dp(15))
					gtx.Constraints = layout.Exact(image.Pt(size, size))
					return icon.Layout(gtx, visual.Text)
				}))
				if strings.TrimSpace(label) != "" {
					children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout))
				}
			}
			if strings.TrimSpace(label) != "" {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return view.label(gtx, label, unit.Sp(11), visual.Text, font.Medium, 1)
				}))
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
		})
		call := macro.Stop()
		if dims.Size.Y < height {
			dims.Size.Y = height
		}
		radius := gtx.Dp(spec.Metrics.ControlRadius)
		rect := image.Rectangle{Max: dims.Size}
		shape := clip.RRect{Rect: rect, NE: radius, NW: radius, SE: radius, SW: radius}
		paint.FillShape(gtx.Ops, visual.Fill, shape.Op(gtx.Ops))
		paint.FillShape(gtx.Ops, visual.Border, clip.Stroke{Path: shape.Path(gtx.Ops), Width: 1}.Op())
		call.Add(gtx.Ops)
		return dims
	})
}

func resolveDesktopButtonVisual(spec desktopThemeTokens, tone desktopWindowButtonTone, interaction buttonInteractionState) desktopButtonVisual {
	fill := spec.Colors.surface2
	hoverFill := fill
	border := spec.Colors.border
	textColor := spec.Colors.text
	switch tone {
	case desktopButtonPrimary:
		fill = spec.Colors.accent
		hoverFill = fill
		border = spec.Colors.accent
		textColor = desktopReadableText(fill)
	case desktopButtonDanger:
		fill = spec.Colors.danger
		hoverFill = fill
		border = spec.Colors.danger
		textColor = desktopReadableText(fill)
	case desktopButtonSelected:
		fill = spec.Colors.accentSoft
		hoverFill = fill
		border = spec.Colors.focusRing
		textColor = spec.Colors.accent
	case desktopButtonDisabled:
		fill = spec.Colors.surface
		hoverFill = fill
		border = spec.Colors.border
		textColor = spec.Colors.textDim
	}
	if tone == desktopButtonNeutral {
		hoverFill = spec.Colors.toolHoverBg
	}
	if tone == desktopButtonDisabled {
		interaction = buttonInteractionState{}
	}
	interactionColors := resolveButtonInteractionColors(fill, hoverFill, border, spec.Colors.focusRing, desktopReadableText(fill), interaction)
	if (interaction.Hovered || interaction.Focused) && tone == desktopButtonNeutral {
		textColor = spec.Colors.toolHoverText
	}
	if interaction.Pressed && (tone == desktopButtonPrimary || tone == desktopButtonDanger) {
		textColor = desktopReadableText(interactionColors.Fill)
	}
	return desktopButtonVisual{Fill: interactionColors.Fill, Border: interactionColors.Border, Text: textColor}
}

func (view *desktopWindowView) label(
	gtx layout.Context,
	value string,
	size unit.Sp,
	colorValue color.NRGBA,
	weight font.Weight,
	maxLines int,
) layout.Dimensions {
	style := material.Label(view.theme, view.scaledTextSize(size), value)
	style.Color = colorValue
	style.Font.Weight = weight
	style.MaxLines = maxLines
	return style.Layout(gtx)
}

func (view *desktopWindowView) monoLabel(
	gtx layout.Context,
	value string,
	size unit.Sp,
	colorValue color.NRGBA,
	maxLines int,
) layout.Dimensions {
	style := material.Label(view.theme, view.scaledTextSize(size), value)
	style.Color = colorValue
	style.Font.Typeface = desktopMonoTypeface(view.desktopStyle)
	style.MaxLines = maxLines
	return style.Layout(gtx)
}

func (view *desktopWindowView) statusChip(gtx layout.Context, spec desktopThemeTokens, label string, colorValue color.NRGBA) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return view.label(gtx, label, unit.Sp(10), colorValue, font.Medium, 1)
	})
	call := macro.Stop()
	radius := gtx.Dp(spec.Metrics.BadgeRadius)
	fillColor := detachedStatusChipFill(spec, colorValue)
	paint.FillShape(gtx.Ops, withAlpha(fillColor, 0x20), clip.RRect{Rect: image.Rectangle{Max: dims.Size}, NE: radius, NW: radius, SE: radius, SW: radius}.Op(gtx.Ops))
	call.Add(gtx.Ops)
	return dims
}

func detachedStatusChipFill(spec desktopThemeTokens, textColor color.NRGBA) color.NRGBA {
	switch textColor {
	case spec.Colors.accentText:
		return spec.Colors.accent
	case spec.Colors.successText:
		return spec.Colors.success
	case spec.Colors.warningText:
		return spec.Colors.warning
	case spec.Colors.dangerText:
		return spec.Colors.danger
	default:
		return textColor
	}
}

func (view *desktopWindowView) progressBar(gtx layout.Context, spec desktopThemeTokens, progress float32) layout.Dimensions {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	height := max(3, gtx.Dp(unit.Dp(4)))
	width := gtx.Constraints.Max.X
	paint.FillShape(gtx.Ops, spec.Colors.surface2, clip.RRect{Rect: image.Rect(0, 0, width, height), NE: height / 2, NW: height / 2, SE: height / 2, SW: height / 2}.Op(gtx.Ops))
	filled := int(float32(width) * progress)
	if filled > 0 {
		paint.FillShape(gtx.Ops, spec.Colors.accent, clip.RRect{Rect: image.Rect(0, 0, filled, height), NE: height / 2, NW: height / 2, SE: height / 2, SW: height / 2}.Op(gtx.Ops))
	}
	return layout.Dimensions{Size: image.Pt(width, height)}
}

func (view *desktopWindowView) imagePreview(gtx layout.Context, spec desktopThemeTokens, source image.Image, revision int) layout.Dimensions {
	gtx.Constraints.Min = gtx.Constraints.Max
	rect := image.Rectangle{Max: gtx.Constraints.Max}
	radius := gtx.Dp(spec.Metrics.CardRadius)
	paint.FillShape(gtx.Ops, spec.Colors.canvasBg, clip.RRect{Rect: rect, NE: radius, NW: radius, SE: radius, SW: radius}.Op(gtx.Ops))
	paintWorkflowRectOutline(gtx, rect, radius, 1, spec.Colors.border)
	if source == nil {
		view.previewValid = false
		view.previewOp = paint.ImageOp{}
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return view.label(gtx, "等待预览", unit.Sp(11), spec.Colors.textMuted, font.Medium, 1)
		})
	}
	if !view.previewValid || view.previewRev != revision {
		view.previewRev = revision
		view.previewOp = paint.NewImageOp(source)
		view.previewValid = true
	}
	inset := layout.UniformInset(unit.Dp(4))
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return widget.Image{
			Src:      view.previewOp,
			Fit:      widget.Contain,
			Position: layout.Center,
		}.Layout(gtx)
	})
}

func (view *desktopWindowView) layoutUnavailable(gtx layout.Context, spec desktopThemeTokens, title string, message string) layout.Dimensions {
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		maxWidth := gtx.Dp(unit.Dp(420))
		if gtx.Constraints.Max.X > maxWidth {
			gtx.Constraints.Max.X = maxWidth
		}
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return view.label(gtx, title, unit.Sp(18), spec.Colors.text, font.SemiBold, 1)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return view.label(gtx, message, unit.Sp(12), spec.Colors.textMuted, font.Normal, 3)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(18)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return view.button(gtx, spec, &view.raiseMainButton, view.icons.main, "返回主窗口", desktopButtonPrimary)
			}),
		)
	})
}

func (view *desktopWindowView) missingWorkspace(gtx layout.Context, spec desktopThemeTokens, publication desktopPublication) layout.Dimensions {
	if publication.Revision == 0 {
		return view.layoutUnavailable(gtx, spec, "等待主窗口", "正在同步桌面工作区状态。")
	}
	id := strings.TrimSpace(view.request.WorkspaceID)
	if id == "" {
		id = "当前工作区"
	}
	return view.layoutUnavailable(gtx, spec, "工作区不可用", fmt.Sprintf("工作区 %s 已关闭或尚未恢复。", id))
}

func detachedStatusColor(spec desktopThemeTokens, publication desktopPublication) color.NRGBA {
	if strings.TrimSpace(publication.LastError) != "" {
		return spec.Colors.dangerText
	}
	if publication.Running {
		return spec.Colors.accentText
	}
	if publication.Completed > 0 {
		return spec.Colors.successText
	}
	return spec.Colors.textMuted
}

func detachedStatusLabel(publication desktopPublication) string {
	if publication.Running {
		return "运行中"
	}
	if strings.TrimSpace(publication.LastError) != "" {
		return "需要处理"
	}
	if strings.TrimSpace(publication.Status) != "" {
		return publication.Status
	}
	return "就绪"
}

func desktopWorkspaceRunning(publication desktopPublication, workspaceID string) bool {
	workspace, ok := publication.workspace(workspaceID)
	return ok && workspace.Running
}

func desktopWorkspaceQueued(publication desktopPublication, workspaceID string) bool {
	workspace, ok := publication.workspace(workspaceID)
	return ok && workspace.Queued
}

func detachedWorkspaceStatusLabel(publication desktopPublication, workspaceID string) string {
	workspace, ok := publication.workspace(workspaceID)
	if !ok {
		return "工作区不可用"
	}
	if workspace.Running {
		return chooseNonEmpty(workspace.Status, "运行中")
	}
	if workspace.Queued {
		return "等待队列"
	}
	return chooseNonEmpty(workspace.Status, "就绪")
}

func detachedWorkspaceStatusColor(spec desktopThemeTokens, publication desktopPublication, workspaceID string) color.NRGBA {
	workspace, ok := publication.workspace(workspaceID)
	if !ok {
		return spec.Colors.textMuted
	}
	if strings.TrimSpace(workspace.LastError) != "" {
		return spec.Colors.dangerText
	}
	if workspace.Running {
		return spec.Colors.accentText
	}
	if workspace.Queued {
		return spec.Colors.warningText
	}
	if workspace.Completed > 0 {
		return spec.Colors.successText
	}
	return spec.Colors.textMuted
}

var _ windowing.View = (*desktopWindowView)(nil)
