package ui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"
	"time"
)

type canvasToolMode string

const (
	canvasToolPan      canvasToolMode = "pan"
	canvasToolMask     canvasToolMode = "mask"
	canvasToolAnnotate canvasToolMode = "annotate"
)

type canvasAnnotationKind string

const (
	canvasAnnotationKindRect     canvasAnnotationKind = "rect"
	canvasAnnotationKindArrow    canvasAnnotationKind = "arrow"
	canvasAnnotationKindFreehand canvasAnnotationKind = "freehand"
	canvasAnnotationKindText     canvasAnnotationKind = "text"
)

var canvasAnnotationColors = []color.NRGBA{
	rgb(0xff4d4d),
	rgb(0xff9c00),
	rgb(0xffd400),
	rgb(0x7bd400),
	rgb(0x00c8ff),
	rgb(0x4d7cff),
	rgb(0xa060ff),
	rgb(0xff60c8),
}

type canvasAnnotation struct {
	ID     string
	Kind   canvasAnnotationKind
	Color  color.NRGBA
	Rect   image.Rectangle
	Points []image.Point
	Text   string
}

type canvasAnnotationDraft struct {
	Kind    canvasAnnotationKind
	Color   color.NRGBA
	Start   image.Point
	Current image.Point
	Points  []image.Point
}

func normalizeCanvasAnnotationKind(kind canvasAnnotationKind) canvasAnnotationKind {
	if kind == "" {
		return canvasAnnotationKindRect
	}
	return kind
}

func cloneCanvasAnnotations(items []canvasAnnotation) []canvasAnnotation {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]canvasAnnotation, len(items))
	for idx, item := range items {
		points := append([]image.Point(nil), item.Points...)
		cloned[idx] = canvasAnnotation{
			ID:     item.ID,
			Kind:   item.Kind,
			Color:  item.Color,
			Rect:   item.Rect,
			Points: points,
			Text:   item.Text,
		}
	}
	return cloned
}

func normalizeCanvasAnnotationRect(start image.Point, current image.Point) image.Rectangle {
	minX := min(start.X, current.X)
	minY := min(start.Y, current.Y)
	maxX := max(start.X, current.X)
	maxY := max(start.Y, current.Y)
	return image.Rect(minX, minY, maxX, maxY)
}

func validCanvasAnnotationRect(rect image.Rectangle) bool {
	return rect.Dx() >= 4 && rect.Dy() >= 4
}

func pointToSegmentDistance(point image.Point, start image.Point, end image.Point) float64 {
	px := float64(point.X)
	py := float64(point.Y)
	x1 := float64(start.X)
	y1 := float64(start.Y)
	x2 := float64(end.X)
	y2 := float64(end.Y)
	dx := x2 - x1
	dy := y2 - y1
	if dx == 0 && dy == 0 {
		return math.Hypot(px-x1, py-y1)
	}
	t := ((px-x1)*dx + (py-y1)*dy) / (dx*dx + dy*dy)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	projX := x1 + t*dx
	projY := y1 + t*dy
	return math.Hypot(px-projX, py-projY)
}

func canvasAnnotationBounds(annotation canvasAnnotation) image.Rectangle {
	switch normalizeCanvasAnnotationKind(annotation.Kind) {
	case canvasAnnotationKindRect, canvasAnnotationKindArrow:
		return annotation.Rect
	case canvasAnnotationKindFreehand:
		if len(annotation.Points) == 0 {
			return image.Rectangle{}
		}
		minX, minY := annotation.Points[0].X, annotation.Points[0].Y
		maxX, maxY := minX, minY
		for _, point := range annotation.Points[1:] {
			minX = min(minX, point.X)
			minY = min(minY, point.Y)
			maxX = max(maxX, point.X)
			maxY = max(maxY, point.Y)
		}
		return image.Rect(minX, minY, maxX, maxY)
	case canvasAnnotationKindText:
		return annotation.Rect
	default:
		return annotation.Rect
	}
}

func hitCanvasAnnotation(items []canvasAnnotation, point image.Point) (string, bool) {
	for idx := len(items) - 1; idx >= 0; idx-- {
		item := items[idx]
		switch normalizeCanvasAnnotationKind(item.Kind) {
		case canvasAnnotationKindRect:
			if point.In(item.Rect) {
				return item.ID, true
			}
		case canvasAnnotationKindArrow:
			if pointToSegmentDistance(point, item.Rect.Min, item.Rect.Max) <= 8 {
				return item.ID, true
			}
		case canvasAnnotationKindFreehand:
			if len(item.Points) == 1 {
				if pointToSegmentDistance(point, item.Points[0], item.Points[0]) <= 8 {
					return item.ID, true
				}
				continue
			}
			for p := 1; p < len(item.Points); p++ {
				if pointToSegmentDistance(point, item.Points[p-1], item.Points[p]) <= 8 {
					return item.ID, true
				}
			}
		case canvasAnnotationKindText:
			if point.In(item.Rect) {
				return item.ID, true
			}
		}
	}
	return "", false
}

func (a *App) currentCanvasTool() canvasToolMode {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.canvasTool == "" {
		return canvasToolPan
	}
	return a.canvasTool
}

func (a *App) currentCanvasInteractionTool() canvasToolMode {
	a.mu.Lock()
	defer a.mu.Unlock()
	tool := a.canvasTool
	if tool == "" {
		tool = canvasToolPan
	}
	if a.canvasSpacePan {
		return canvasToolPan
	}
	return tool
}

func (a *App) setCanvasTool(tool canvasToolMode) {
	a.mu.Lock()
	a.canvasTool = tool
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) setCanvasSpacePan(active bool) {
	a.mu.Lock()
	if a.canvasSpacePan == active {
		a.mu.Unlock()
		return
	}
	a.canvasSpacePan = active
	a.mu.Unlock()
	a.invalidateSoon(16 * time.Millisecond)
}

func (a *App) currentCanvasAnnotationKind() canvasAnnotationKind {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.canvasAnnotationKind == "" {
		return canvasAnnotationKindRect
	}
	return a.canvasAnnotationKind
}

func (a *App) setCanvasAnnotationKind(kind canvasAnnotationKind) {
	a.mu.Lock()
	a.canvasAnnotationKind = kind
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) currentCanvasAnnotationColor() color.NRGBA {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.canvasAnnotationColor.A == 0 {
		return canvasAnnotationColors[0]
	}
	return a.canvasAnnotationColor
}

func (a *App) setCanvasAnnotationColor(color color.NRGBA) {
	a.mu.Lock()
	a.canvasAnnotationColor = color
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) canvasAnnotationState() ([]canvasAnnotation, string, *canvasAnnotationDraft, bool, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var draft *canvasAnnotationDraft
	if a.canvasAnnotationDraft != nil {
		cp := *a.canvasAnnotationDraft
		cp.Points = append([]image.Point(nil), a.canvasAnnotationDraft.Points...)
		draft = &cp
	}
	return cloneCanvasAnnotations(a.canvasAnnotations), a.canvasSelectedAnnotationID, draft, len(a.canvasAnnotationUndo) > 0, len(a.canvasAnnotationRedo) > 0
}

func (a *App) resetCanvasAnnotationsLocked() {
	a.canvasAnnotations = nil
	a.canvasAnnotationUndo = nil
	a.canvasAnnotationRedo = nil
	a.canvasAnnotationUndoAt = nil
	a.canvasAnnotationRedoAt = nil
	a.canvasSelectedAnnotationID = ""
	a.canvasAnnotationDraft = nil
}

func (a *App) resetCanvasAnnotations() {
	a.mu.Lock()
	a.resetCanvasAnnotationsLocked()
	a.mu.Unlock()
}

func (a *App) pushCanvasAnnotationUndoLocked() {
	a.canvasAnnotationUndo = append(a.canvasAnnotationUndo, cloneCanvasAnnotations(a.canvasAnnotations))
	a.canvasAnnotationUndoAt = append(a.canvasAnnotationUndoAt, time.Now())
	if len(a.canvasAnnotationUndo) > 32 {
		a.canvasAnnotationUndo = a.canvasAnnotationUndo[len(a.canvasAnnotationUndo)-32:]
		a.canvasAnnotationUndoAt = a.canvasAnnotationUndoAt[len(a.canvasAnnotationUndoAt)-32:]
	}
	a.canvasAnnotationRedo = nil
	a.canvasAnnotationRedoAt = nil
	a.canvasMaskRedo = nil
	a.canvasMaskRedoAt = nil
}

func (a *App) addCanvasAnnotationItem(item canvasAnnotation) {
	a.mu.Lock()
	a.pushCanvasAnnotationUndoLocked()
	a.canvasAnnotations = append(a.canvasAnnotations, item)
	a.canvasSelectedAnnotationID = item.ID
	a.canvasAnnotationDraft = nil
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) addCanvasAnnotation(rect image.Rectangle) {
	if !validCanvasAnnotationRect(rect) {
		return
	}
	a.addCanvasAnnotationItem(canvasAnnotation{
		ID:    "ann-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		Kind:  canvasAnnotationKindRect,
		Color: a.currentCanvasAnnotationColor(),
		Rect:  rect,
	})
}

func validCanvasFreehandPoints(points []image.Point) bool {
	if len(points) < 2 {
		return false
	}
	for idx := 1; idx < len(points); idx++ {
		if points[idx] != points[idx-1] {
			return true
		}
	}
	return false
}

func (a *App) updateCanvasAnnotationDraft(kind canvasAnnotationKind, color color.NRGBA, start image.Point, current image.Point) {
	a.mu.Lock()
	a.canvasAnnotationDraft = &canvasAnnotationDraft{
		Kind:    kind,
		Color:   color,
		Start:   start,
		Current: current,
		Points:  nil,
	}
	a.mu.Unlock()
	a.invalidateSoon(16 * time.Millisecond)
}

func (a *App) startCanvasFreehandDraft(color color.NRGBA, start image.Point) {
	a.mu.Lock()
	a.canvasAnnotationDraft = &canvasAnnotationDraft{
		Kind:   canvasAnnotationKindFreehand,
		Color:  color,
		Start:  start,
		Points: []image.Point{start},
	}
	a.mu.Unlock()
	a.invalidateSoon(16 * time.Millisecond)
}

func (a *App) appendCanvasFreehandDraftPoint(point image.Point) {
	a.mu.Lock()
	if a.canvasAnnotationDraft == nil || a.canvasAnnotationDraft.Kind != canvasAnnotationKindFreehand {
		a.mu.Unlock()
		return
	}
	a.canvasAnnotationDraft.Points = append(a.canvasAnnotationDraft.Points, point)
	a.canvasAnnotationDraft.Current = point
	a.mu.Unlock()
	a.invalidateSoon(16 * time.Millisecond)
}

func (a *App) clearCanvasAnnotationDraft() {
	a.mu.Lock()
	a.canvasAnnotationDraft = nil
	a.mu.Unlock()
	a.invalidateSoon(16 * time.Millisecond)
}

func (a *App) selectCanvasAnnotation(id string) {
	a.mu.Lock()
	a.canvasSelectedAnnotationID = id
	a.mu.Unlock()
	a.invalidateSoon(16 * time.Millisecond)
}

func (a *App) clearCanvasAnnotationSelection() {
	a.mu.Lock()
	a.canvasSelectedAnnotationID = ""
	a.mu.Unlock()
	a.invalidateSoon(16 * time.Millisecond)
}

func (a *App) clearCanvasAnnotations() {
	a.mu.Lock()
	if len(a.canvasAnnotations) == 0 {
		a.canvasAnnotationDraft = nil
		a.canvasSelectedAnnotationID = ""
		a.mu.Unlock()
		return
	}
	a.pushCanvasAnnotationUndoLocked()
	a.canvasAnnotations = nil
	a.canvasAnnotationDraft = nil
	a.canvasSelectedAnnotationID = ""
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) deleteSelectedCanvasAnnotation() {
	a.mu.Lock()
	selectedID := a.canvasSelectedAnnotationID
	if selectedID == "" || len(a.canvasAnnotations) == 0 {
		a.mu.Unlock()
		return
	}
	next := make([]canvasAnnotation, 0, len(a.canvasAnnotations))
	removed := false
	for _, item := range a.canvasAnnotations {
		if item.ID == selectedID {
			removed = true
			continue
		}
		next = append(next, item)
	}
	if !removed {
		a.mu.Unlock()
		return
	}
	a.pushCanvasAnnotationUndoLocked()
	a.canvasAnnotations = next
	a.canvasSelectedAnnotationID = ""
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) undoCanvasAnnotation() {
	a.mu.Lock()
	if len(a.canvasAnnotationUndo) == 0 {
		a.mu.Unlock()
		return
	}
	last := a.canvasAnnotationUndo[len(a.canvasAnnotationUndo)-1]
	a.canvasAnnotationUndo = a.canvasAnnotationUndo[:len(a.canvasAnnotationUndo)-1]
	a.canvasAnnotationUndoAt = a.canvasAnnotationUndoAt[:len(a.canvasAnnotationUndoAt)-1]
	a.canvasAnnotationRedo = append(a.canvasAnnotationRedo, cloneCanvasAnnotations(a.canvasAnnotations))
	a.canvasAnnotationRedoAt = append(a.canvasAnnotationRedoAt, time.Now())
	a.canvasAnnotations = cloneCanvasAnnotations(last)
	if a.canvasSelectedAnnotationID != "" {
		if _, ok := hitCanvasAnnotation(a.canvasAnnotations, a.selectedCanvasAnnotationCenterLocked()); !ok {
			a.canvasSelectedAnnotationID = ""
		}
	}
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) redoCanvasAnnotation() {
	a.mu.Lock()
	if len(a.canvasAnnotationRedo) == 0 {
		a.mu.Unlock()
		return
	}
	last := a.canvasAnnotationRedo[len(a.canvasAnnotationRedo)-1]
	a.canvasAnnotationRedo = a.canvasAnnotationRedo[:len(a.canvasAnnotationRedo)-1]
	a.canvasAnnotationRedoAt = a.canvasAnnotationRedoAt[:len(a.canvasAnnotationRedoAt)-1]
	a.canvasAnnotationUndo = append(a.canvasAnnotationUndo, cloneCanvasAnnotations(a.canvasAnnotations))
	a.canvasAnnotationUndoAt = append(a.canvasAnnotationUndoAt, time.Now())
	a.canvasAnnotations = cloneCanvasAnnotations(last)
	if a.canvasSelectedAnnotationID != "" {
		if _, ok := hitCanvasAnnotation(a.canvasAnnotations, a.selectedCanvasAnnotationCenterLocked()); !ok {
			a.canvasSelectedAnnotationID = ""
		}
	}
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) selectedCanvasAnnotationCenterLocked() image.Point {
	for _, item := range a.canvasAnnotations {
		if item.ID == a.canvasSelectedAnnotationID {
			rect := canvasAnnotationBounds(item)
			return image.Pt((rect.Min.X+rect.Max.X)/2, (rect.Min.Y+rect.Max.Y)/2)
		}
	}
	return image.Point{}
}

func canvasAnnotationDimensionsFromState(state resultState) image.Point {
	if state.Image != nil {
		return state.Image.Bounds().Size()
	}
	if path := strings.TrimSpace(state.SavedPath); path != "" {
		return imageDimensionsForPath(path)
	}
	if imageB64 := strings.TrimSpace(state.Item.ImageB64); imageB64 != "" {
		if img, err := decodeImageB64(imageB64); err == nil && img != nil {
			return img.Bounds().Size()
		}
	}
	return image.Point{}
}

func augmentPromptWithCanvasAnnotations(prompt string, annotations []canvasAnnotation, dims image.Point) string {
	prompt = strings.TrimSpace(prompt)
	if len(annotations) == 0 {
		return prompt
	}
	rects := make([]canvasAnnotation, 0, len(annotations))
	for _, annotation := range annotations {
		if normalizeCanvasAnnotationKind(annotation.Kind) == canvasAnnotationKindRect && validCanvasAnnotationRect(annotation.Rect) {
			rects = append(rects, annotation)
		}
	}
	if len(rects) == 0 {
		return prompt
	}
	describe := func(index int, annotation canvasAnnotation) string {
		if dims.X <= 0 || dims.Y <= 0 {
			return fmt.Sprintf("区域 %d", index+1)
		}
		cx := float64(annotation.Rect.Min.X+annotation.Rect.Dx()/2) / float64(dims.X)
		cy := float64(annotation.Rect.Min.Y+annotation.Rect.Dy()/2) / float64(dims.Y)
		hPart := "中"
		switch {
		case cx < 0.34:
			hPart = "左"
		case cx > 0.66:
			hPart = "右"
		}
		vPart := "中"
		switch {
		case cy < 0.34:
			vPart = "上"
		case cy > 0.66:
			vPart = "下"
		}
		return vPart + hPart + "部"
	}
	positions := make([]string, 0, len(rects))
	for idx, annotation := range rects {
		positions = append(positions, describe(idx, annotation))
	}
	if prompt == "" {
		return fmt.Sprintf("(请重点关注%s标注区域)", strings.Join(positions, "、"))
	}
	return prompt + "\n(请重点关注" + strings.Join(positions, "、") + "标注区域)"
}

func (a *App) currentCanvasAugmentedPrompt(prompt string) string {
	a.mu.Lock()
	annotations := cloneCanvasAnnotations(a.canvasAnnotations)
	state := a.result
	a.mu.Unlock()
	if len(annotations) == 0 {
		return strings.TrimSpace(prompt)
	}
	dims := canvasAnnotationDimensionsFromState(state)
	return augmentPromptWithCanvasAnnotations(prompt, annotations, dims)
}

func (a *App) currentSelectedCanvasCropRect() (image.Rectangle, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	selectedID := a.canvasSelectedAnnotationID
	if selectedID == "" {
		return image.Rectangle{}, false
	}
	for _, annotation := range a.canvasAnnotations {
		if annotation.ID != selectedID {
			continue
		}
		if normalizeCanvasAnnotationKind(annotation.Kind) != canvasAnnotationKindRect {
			return image.Rectangle{}, false
		}
		rect := annotation.Rect
		if !validCanvasAnnotationRect(rect) {
			return image.Rectangle{}, false
		}
		return rect, true
	}
	return image.Rectangle{}, false
}
