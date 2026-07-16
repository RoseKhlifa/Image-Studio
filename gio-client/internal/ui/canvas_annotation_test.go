package ui

import (
	"image"
	"strings"
	"testing"
)

func TestNormalizeCanvasAnnotationRect(t *testing.T) {
	rect := normalizeCanvasAnnotationRect(image.Pt(20, 30), image.Pt(5, 10))
	if rect != image.Rect(5, 10, 20, 30) {
		t.Fatalf("rect=%v want %v", rect, image.Rect(5, 10, 20, 30))
	}
}

func TestHitCanvasAnnotationPrefersTopmost(t *testing.T) {
	items := []canvasAnnotation{
		{ID: "a", Rect: image.Rect(0, 0, 20, 20)},
		{ID: "b", Rect: image.Rect(5, 5, 25, 25)},
	}
	id, ok := hitCanvasAnnotation(items, image.Pt(10, 10))
	if !ok || id != "b" {
		t.Fatalf("hit=(%q,%v) want (b,true)", id, ok)
	}
}

func TestCanvasAnnotationUndoRedo(t *testing.T) {
	app := &App{}
	app.addCanvasAnnotation(image.Rect(0, 0, 20, 20))
	if len(app.canvasAnnotations) != 1 {
		t.Fatalf("annotations=%d want 1", len(app.canvasAnnotations))
	}
	app.undoCanvasAnnotation()
	if len(app.canvasAnnotations) != 0 {
		t.Fatalf("after undo annotations=%d want 0", len(app.canvasAnnotations))
	}
	app.redoCanvasAnnotation()
	if len(app.canvasAnnotations) != 1 {
		t.Fatalf("after redo annotations=%d want 1", len(app.canvasAnnotations))
	}
}

func TestAugmentPromptWithCanvasAnnotationsUsesRelativeRegions(t *testing.T) {
	prompt := augmentPromptWithCanvasAnnotations("base prompt", []canvasAnnotation{
		{ID: "a", Rect: image.Rect(0, 0, 20, 20)},
		{ID: "b", Rect: image.Rect(80, 80, 100, 100)},
	}, image.Pt(100, 100))
	if prompt == "base prompt" {
		t.Fatal("prompt should be augmented")
	}
	if !strings.Contains(prompt, "上左部") || !strings.Contains(prompt, "下右部") {
		t.Fatalf("augmented prompt=%q want relative region labels", prompt)
	}
}

func TestUndoLatestCanvasActionAcrossAnnotationAndMask(t *testing.T) {
	app := &App{}
	app.addCanvasAnnotation(image.Rect(0, 0, 20, 20))
	app.startCanvasMaskStroke(image.Pt(1, 1), image.Rect(0, 0, 100, 100))
	app.appendCanvasMaskStrokePoint(image.Pt(20, 20), image.Rect(0, 0, 100, 100))
	app.commitCanvasMaskStroke()

	if len(app.canvasAnnotations) != 1 || len(app.canvasMaskStrokes) != 1 {
		t.Fatalf("expected one annotation and one mask stroke, got ann=%d mask=%d", len(app.canvasAnnotations), len(app.canvasMaskStrokes))
	}

	app.undoLatestCanvasAction()
	if len(app.canvasMaskStrokes) != 0 || len(app.canvasAnnotations) != 1 {
		t.Fatalf("first undo should remove latest mask action, got ann=%d mask=%d", len(app.canvasAnnotations), len(app.canvasMaskStrokes))
	}

	app.undoLatestCanvasAction()
	if len(app.canvasAnnotations) != 0 {
		t.Fatalf("second undo should remove annotation, got ann=%d", len(app.canvasAnnotations))
	}

	app.redoLatestCanvasAction()
	if len(app.canvasAnnotations) != 1 || len(app.canvasMaskStrokes) != 0 {
		t.Fatalf("first redo should restore annotation only, got ann=%d mask=%d", len(app.canvasAnnotations), len(app.canvasMaskStrokes))
	}
}
