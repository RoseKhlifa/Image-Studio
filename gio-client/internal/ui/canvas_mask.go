package ui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"github.com/yuanhua/image-gptcodex/pkg/client"
)

type canvasBrushMode string

const (
	canvasBrushPaint canvasBrushMode = "paint"
	canvasBrushErase canvasBrushMode = "erase"
)

type canvasMaskStroke struct {
	Points   []f32.Point
	SizeNorm float32
	Erase    bool
}

func cloneCanvasMaskStrokes(items []canvasMaskStroke) []canvasMaskStroke {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]canvasMaskStroke, len(items))
	for idx, item := range items {
		points := make([]f32.Point, len(item.Points))
		copy(points, item.Points)
		cloned[idx] = canvasMaskStroke{
			Points:   points,
			SizeNorm: item.SizeNorm,
			Erase:    item.Erase,
		}
	}
	return cloned
}

func normalizeCanvasMaskPoint(point image.Point, bounds image.Rectangle) f32.Point {
	width := max(1, bounds.Dx())
	height := max(1, bounds.Dy())
	x := float32(point.X-bounds.Min.X) / float32(width)
	y := float32(point.Y-bounds.Min.Y) / float32(height)
	if x < 0 {
		x = 0
	}
	if x > 1 {
		x = 1
	}
	if y < 0 {
		y = 0
	}
	if y > 1 {
		y = 1
	}
	return f32.Pt(x, y)
}

func denormalizeCanvasMaskPoint(point f32.Point, dims image.Point) f32.Point {
	width := float32(max(1, dims.X))
	height := float32(max(1, dims.Y))
	return f32.Pt(point.X*width, point.Y*height)
}

func normalizeCanvasBrushSize(size int, bounds image.Rectangle) float32 {
	base := float32(max(1, max(bounds.Dx(), bounds.Dy())))
	return float32(max(1, size)) / base
}

func denormalizeCanvasBrushSize(sizeNorm float32, dims image.Point) float32 {
	base := float32(max(1, max(dims.X, dims.Y)))
	size := sizeNorm * base
	if size < 1 {
		size = 1
	}
	return size
}

func (a *App) currentCanvasBrushMode() canvasBrushMode {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.canvasBrushMode == "" {
		return canvasBrushPaint
	}
	return a.canvasBrushMode
}

func (a *App) setCanvasBrushMode(mode canvasBrushMode) {
	a.mu.Lock()
	a.canvasBrushMode = mode
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) currentCanvasBrushSize() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.canvasBrushSize <= 0 {
		return 30
	}
	return a.canvasBrushSize
}

func (a *App) adjustCanvasBrushSize(delta int) {
	a.mu.Lock()
	size := a.canvasBrushSize
	if size <= 0 {
		size = 30
	}
	size += delta
	if size < 5 {
		size = 5
	}
	if size > 120 {
		size = 120
	}
	a.canvasBrushSize = size
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) canvasMaskState() ([]canvasMaskStroke, *canvasMaskStroke, bool, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var draft *canvasMaskStroke
	if a.canvasMaskDraft != nil {
		cp := *a.canvasMaskDraft
		cp.Points = append([]f32.Point(nil), a.canvasMaskDraft.Points...)
		draft = &cp
	}
	return cloneCanvasMaskStrokes(a.canvasMaskStrokes), draft, len(a.canvasMaskUndo) > 0, len(a.canvasMaskRedo) > 0
}

func (a *App) resetCanvasMaskLocked() {
	a.canvasImportedMaskB64 = ""
	a.canvasMaskStrokes = nil
	a.canvasMaskDraft = nil
	a.canvasMaskUndo = nil
	a.canvasMaskRedo = nil
	a.canvasMaskUndoAt = nil
	a.canvasMaskRedoAt = nil
}

func (a *App) pushCanvasMaskUndoLocked() {
	a.canvasMaskUndo = append(a.canvasMaskUndo, cloneCanvasMaskStrokes(a.canvasMaskStrokes))
	a.canvasMaskUndoAt = append(a.canvasMaskUndoAt, time.Now())
	if len(a.canvasMaskUndo) > 32 {
		a.canvasMaskUndo = a.canvasMaskUndo[len(a.canvasMaskUndo)-32:]
		a.canvasMaskUndoAt = a.canvasMaskUndoAt[len(a.canvasMaskUndoAt)-32:]
	}
	a.canvasMaskRedo = nil
	a.canvasMaskRedoAt = nil
	a.canvasAnnotationRedo = nil
	a.canvasAnnotationRedoAt = nil
}

func (a *App) startCanvasMaskStroke(point image.Point, bounds image.Rectangle) {
	mode := a.currentCanvasBrushMode()
	size := a.currentCanvasBrushSize()
	stroke := &canvasMaskStroke{
		Points:   []f32.Point{normalizeCanvasMaskPoint(point, bounds)},
		SizeNorm: normalizeCanvasBrushSize(size, bounds),
		Erase:    mode == canvasBrushErase,
	}
	a.mu.Lock()
	a.canvasMaskDraft = stroke
	a.mu.Unlock()
	a.invalidateSoon(16 * time.Millisecond)
}

func (a *App) appendCanvasMaskStrokePoint(point image.Point, bounds image.Rectangle) {
	a.mu.Lock()
	if a.canvasMaskDraft == nil {
		a.mu.Unlock()
		return
	}
	a.canvasMaskDraft.Points = append(a.canvasMaskDraft.Points, normalizeCanvasMaskPoint(point, bounds))
	a.mu.Unlock()
	a.invalidateSoon(16 * time.Millisecond)
}

func (a *App) commitCanvasMaskStroke() {
	a.mu.Lock()
	draft := a.canvasMaskDraft
	if draft == nil || len(draft.Points) == 0 {
		a.canvasMaskDraft = nil
		a.mu.Unlock()
		return
	}
	a.pushCanvasMaskUndoLocked()
	a.canvasMaskStrokes = append(a.canvasMaskStrokes, *draft)
	a.canvasMaskDraft = nil
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) clearCanvasMask() {
	a.mu.Lock()
	if len(a.canvasMaskStrokes) == 0 && strings.TrimSpace(a.canvasImportedMaskB64) == "" {
		a.canvasMaskDraft = nil
		a.mu.Unlock()
		return
	}
	if len(a.canvasMaskStrokes) > 0 {
		a.pushCanvasMaskUndoLocked()
	}
	a.canvasImportedMaskB64 = ""
	a.canvasMaskStrokes = nil
	a.canvasMaskDraft = nil
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) undoCanvasMask() {
	a.mu.Lock()
	if len(a.canvasMaskUndo) == 0 {
		a.mu.Unlock()
		return
	}
	last := a.canvasMaskUndo[len(a.canvasMaskUndo)-1]
	a.canvasMaskUndo = a.canvasMaskUndo[:len(a.canvasMaskUndo)-1]
	a.canvasMaskUndoAt = a.canvasMaskUndoAt[:len(a.canvasMaskUndoAt)-1]
	a.canvasMaskRedo = append(a.canvasMaskRedo, cloneCanvasMaskStrokes(a.canvasMaskStrokes))
	a.canvasMaskRedoAt = append(a.canvasMaskRedoAt, time.Now())
	a.canvasMaskStrokes = cloneCanvasMaskStrokes(last)
	a.canvasMaskDraft = nil
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) redoCanvasMask() {
	a.mu.Lock()
	if len(a.canvasMaskRedo) == 0 {
		a.mu.Unlock()
		return
	}
	last := a.canvasMaskRedo[len(a.canvasMaskRedo)-1]
	a.canvasMaskRedo = a.canvasMaskRedo[:len(a.canvasMaskRedo)-1]
	a.canvasMaskRedoAt = a.canvasMaskRedoAt[:len(a.canvasMaskRedoAt)-1]
	a.canvasMaskUndo = append(a.canvasMaskUndo, cloneCanvasMaskStrokes(a.canvasMaskStrokes))
	a.canvasMaskUndoAt = append(a.canvasMaskUndoAt, time.Now())
	a.canvasMaskStrokes = cloneCanvasMaskStrokes(last)
	a.canvasMaskDraft = nil
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) canUndoCanvasAction() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.canvasMaskUndo) > 0 || len(a.canvasAnnotationUndo) > 0
}

func (a *App) canRedoCanvasAction() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.canvasMaskRedo) > 0 || len(a.canvasAnnotationRedo) > 0
}

func latestTimeOrZero(times []time.Time) time.Time {
	if len(times) == 0 {
		return time.Time{}
	}
	return times[len(times)-1]
}

func (a *App) undoLatestCanvasAction() {
	a.mu.Lock()
	maskAt := latestTimeOrZero(a.canvasMaskUndoAt)
	annotationAt := latestTimeOrZero(a.canvasAnnotationUndoAt)
	a.mu.Unlock()
	switch {
	case maskAt.IsZero() && annotationAt.IsZero():
		return
	case annotationAt.After(maskAt):
		a.undoCanvasAnnotation()
	default:
		a.undoCanvasMask()
	}
}

func (a *App) redoLatestCanvasAction() {
	a.mu.Lock()
	maskAt := latestTimeOrZero(a.canvasMaskRedoAt)
	annotationAt := latestTimeOrZero(a.canvasAnnotationRedoAt)
	a.mu.Unlock()
	switch {
	case maskAt.IsZero() && annotationAt.IsZero():
		return
	case annotationAt.After(maskAt):
		a.redoCanvasAnnotation()
	default:
		a.redoCanvasMask()
	}
}

func drawMaskDot(img *image.Gray, center f32.Point, radius float32, value uint8) {
	if img == nil || radius <= 0 {
		return
	}
	minX := max(img.Bounds().Min.X, int(math.Floor(float64(center.X-radius))))
	maxX := min(img.Bounds().Max.X-1, int(math.Ceil(float64(center.X+radius))))
	minY := max(img.Bounds().Min.Y, int(math.Floor(float64(center.Y-radius))))
	maxY := min(img.Bounds().Max.Y-1, int(math.Ceil(float64(center.Y+radius))))
	r2 := radius * radius
	for y := minY; y <= maxY; y++ {
		dy := float32(y) - center.Y
		for x := minX; x <= maxX; x++ {
			dx := float32(x) - center.X
			if dx*dx+dy*dy > r2 {
				continue
			}
			img.SetGray(x, y, color.Gray{Y: value})
		}
	}
}

func rasterizeMaskStroke(img *image.Gray, stroke canvasMaskStroke, dims image.Point) {
	if img == nil || len(stroke.Points) == 0 {
		return
	}
	value := uint8(255)
	if stroke.Erase {
		value = 0
	}
	radius := denormalizeCanvasBrushSize(stroke.SizeNorm, dims) / 2
	if radius < 0.5 {
		radius = 0.5
	}
	last := denormalizeCanvasMaskPoint(stroke.Points[0], dims)
	drawMaskDot(img, last, radius, value)
	for _, point := range stroke.Points[1:] {
		current := denormalizeCanvasMaskPoint(point, dims)
		dx := current.X - last.X
		dy := current.Y - last.Y
		steps := int(math.Ceil(math.Max(math.Abs(float64(dx)), math.Abs(float64(dy)))))
		if steps < 1 {
			steps = 1
		}
		for step := 1; step <= steps; step++ {
			t := float32(step) / float32(steps)
			drawMaskDot(img, f32.Pt(last.X+dx*t, last.Y+dy*t), radius, value)
		}
		last = current
	}
}

func buildCanvasMaskB64(strokes []canvasMaskStroke, dims image.Point) string {
	if len(strokes) == 0 || dims.X <= 0 || dims.Y <= 0 {
		return ""
	}
	img := image.NewGray(image.Rect(0, 0, dims.X, dims.Y))
	hasWhite := false
	for _, stroke := range strokes {
		if len(stroke.Points) == 0 {
			continue
		}
		if !stroke.Erase {
			hasWhite = true
		}
		rasterizeMaskStroke(img, stroke, dims)
	}
	if !hasWhite {
		return ""
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func paintCanvasMaskStroke(gtx layout.Context, stroke canvasMaskStroke, origin image.Point, displaySize image.Point) {
	if len(stroke.Points) == 0 || displaySize.X <= 0 || displaySize.Y <= 0 {
		return
	}
	var path clip.Path
	path.Begin(gtx.Ops)
	first := denormalizeCanvasMaskPoint(stroke.Points[0], displaySize)
	path.MoveTo(f32.Pt(float32(origin.X)+first.X, float32(origin.Y)+first.Y))
	for _, point := range stroke.Points[1:] {
		next := denormalizeCanvasMaskPoint(point, displaySize)
		path.LineTo(f32.Pt(float32(origin.X)+next.X, float32(origin.Y)+next.Y))
	}
	width := denormalizeCanvasBrushSize(stroke.SizeNorm, displaySize)
	if width < 1 {
		width = 1
	}
	color := accentAlpha(0xc4)
	if stroke.Erase {
		color = dangerAlpha(0x9a)
	}
	paint.FillShape(gtx.Ops, color, clip.Stroke{
		Path:  path.End(),
		Width: width,
	}.Op())
}

func (a *App) paintCanvasMaskStrokes(gtx layout.Context, strokes []canvasMaskStroke, draft *canvasMaskStroke, origin image.Point, displaySize image.Point) {
	for _, stroke := range strokes {
		paintCanvasMaskStroke(gtx, stroke, origin, displaySize)
	}
	if draft != nil {
		paintCanvasMaskStroke(gtx, *draft, origin, displaySize)
	}
}

func imageDimensionsForPath(path string) image.Point {
	path = strings.TrimSpace(path)
	if path == "" {
		return image.Point{}
	}
	if imageB64, ok := readVirtualImageB64(path); ok {
		if img, err := decodeImageB64(imageB64); err == nil && img != nil {
			return img.Bounds().Size()
		}
	}
	if img, err := decodeImageFile(path); err == nil && img != nil {
		return img.Bounds().Size()
	}
	return image.Point{}
}

func (a *App) currentMaskTargetDimensions() image.Point {
	for _, path := range a.sourcePaths() {
		if dims := imageDimensionsForPath(path); dims.X > 0 && dims.Y > 0 {
			return dims
		}
	}
	snap := a.readSnapshot()
	if path := strings.TrimSpace(snap.Result.SavedPath); path != "" {
		if dims := imageDimensionsForPath(path); dims.X > 0 && dims.Y > 0 {
			return dims
		}
	}
	if imageB64 := strings.TrimSpace(snap.Result.Item.ImageB64); imageB64 != "" {
		if img, err := decodeImageB64(imageB64); err == nil && img != nil {
			return img.Bounds().Size()
		}
	}
	if snap.Result.Image != nil {
		return snap.Result.Image.Bounds().Size()
	}
	return image.Point{}
}

func (a *App) currentCanvasMaskB64() string {
	a.mu.Lock()
	strokes := cloneCanvasMaskStrokes(a.canvasMaskStrokes)
	imported := strings.TrimSpace(a.canvasImportedMaskB64)
	a.mu.Unlock()
	if len(strokes) == 0 {
		return imported
	}
	dims := a.currentMaskTargetDimensions()
	return buildCanvasMaskB64(strokes, dims)
}

func (a *App) importCanvasMask(path string) error {
	dataURL, err := client.ImageFileToDataURL(path)
	if err != nil {
		return err
	}
	comma := strings.Index(dataURL, ",")
	if comma < 0 || comma == len(dataURL)-1 {
		return fmt.Errorf("蒙版图片 data URL 无效")
	}
	a.mu.Lock()
	a.canvasImportedMaskB64 = dataURL[comma+1:]
	a.canvasMaskStrokes = nil
	a.canvasMaskDraft = nil
	a.canvasMaskUndo = nil
	a.canvasMaskRedo = nil
	a.canvasMaskUndoAt = nil
	a.canvasMaskRedoAt = nil
	a.mu.Unlock()
	a.invalidateNow()
	return nil
}

func (a *App) hasImportedCanvasMask() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return strings.TrimSpace(a.canvasImportedMaskB64) != "" && len(a.canvasMaskStrokes) == 0
}
