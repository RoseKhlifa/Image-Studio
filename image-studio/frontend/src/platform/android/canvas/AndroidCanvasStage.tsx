import { memo, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { Arrow, Image as KonvaImage, Layer, Line, Rect, Stage } from "react-konva";
import Konva from "konva";
import { useStudioStore } from "../../../state/studioStore";
import { useStudioFields } from "../../../state/useStudioFields";
import type { HistoryItem } from "../../../types/domain";
import type { Stroke } from "../../../state/studioStore.types";
import { AnnotationShape } from "../../../components/canvas/AnnotationShape";
import { BatchResultGrid, type BatchGridSlot } from "../../../components/canvas/BatchResultGrid";
import { CompareOverlay } from "../../../components/canvas/CompareOverlay";
import { copyImageB64ToClipboard, copyImageURLToClipboard, useImageFromSource } from "../../../components/canvas/canvasImage";
import { StreamPreviewBadge } from "../../../components/canvas/StreamPreviewBadge";
import { useCanvasShortcuts } from "../../../components/canvas/useCanvasShortcuts";
import { historyFullSrc, orderedNavigationItemsForCurrent, sortHistoryItemsByCreatedAtAsc } from "../../../lib/images";
import { streamPreviewItemsFromPreviews } from "../../../state/studioStore.streamPreview";
import { vibrateForPlatform } from "../bridge";
import { MaskOverlayLayer } from "../../../components/canvas/MaskOverlayLayer";

type ViewState = { scale: number; x: number; y: number };
type PinchState = {
  distance: number;
  center: { x: number; y: number };
  view: ViewState;
};

const MIN_VIEW_SCALE = 0.05;
const MAX_VIEW_SCALE = 8;
const ZOOM_STEP = 1.2;

function AndroidCanvasStageContent() {
  const {
    currentImage, tool, brushSize, brushMode,
    annotationKind, annotationColor,
    selectedAnnotationId,
    annotations, addAnnotation, removeAnnotation,
    maskDataURL,
    maskVisible, maskOpacity, activateMaskTool,
    strokes, pushStroke,
    undo, redo,
    compareB, compareSplit, setCompareSplit, setCompareB,
    isRunning, cancel, errorMessage, setField,
    streamPreview,
    streamPreviews,
    runningJobs,
    jobsTotal,
    jobsCompleted,
    activeWorkspaceId,
    fullscreen,
    toggleFullscreen,
    history,
    batchResults, resultGridOpen, selectBatchResult, closeResultGrid,
    canvasViewResetTick,
    stepBatchResult,
    mode, prompt, size, quality, outputFormat,
  } = useStudioFields([
    "currentImage", "tool", "brushSize", "brushMode",
    "annotationKind", "annotationColor", "selectedAnnotationId", "annotations",
    "addAnnotation", "removeAnnotation", "maskDataURL", "maskVisible",
    "maskOpacity", "activateMaskTool", "strokes", "pushStroke",
    "undo", "redo", "compareB", "compareSplit",
    "setCompareSplit", "setCompareB", "isRunning", "cancel",
    "errorMessage", "setField", "streamPreview", "streamPreviews",
    "runningJobs", "jobsTotal", "jobsCompleted", "activeWorkspaceId",
    "fullscreen", "toggleFullscreen", "history", "batchResults",
    "resultGridOpen", "selectBatchResult", "closeResultGrid", "canvasViewResetTick",
    "stepBatchResult",
    "mode", "prompt", "size", "quality", "outputFormat",
  ]);
  const streamPreviewItems = streamPreviewItemsFromPreviews(streamPreviews, {
    workspaceId: activeWorkspaceId,
    mode, prompt, size, quality, outputFormat,
    currentImage,
  });
  const orderedBatchResults = useMemo(() => sortHistoryItemsByCreatedAtAsc(batchResults), [batchResults]);
  const navigationItems = useMemo(
    () => orderedNavigationItemsForCurrent(currentImage?.id, history, batchResults),
    [currentImage?.id, history, batchResults],
  );
  const liveBatchSlotCount = runningJobs.length;
  const liveBatchSlots: BatchGridSlot[] = Array.from({ length: liveBatchSlotCount }, (_, index) => ({ type: "pending", id: `pending-${index}` }));
  for (const item of [...orderedBatchResults].reverse()) {
    const index = typeof item.previewSlotIndex === "number"
      ? item.previewSlotIndex
      : liveBatchSlots.findIndex((slot) => slot.type === "pending");
    if (index >= 0 && index < liveBatchSlots.length && liveBatchSlots[index].type === "pending") {
      liveBatchSlots[index] = { type: "result", item };
    }
  }
  for (const item of streamPreviewItems) {
    const index = typeof item.previewSlotIndex === "number"
      ? item.previewSlotIndex
      : liveBatchSlots.findIndex((slot) => slot.type === "pending");
    if (index >= 0 && index < liveBatchSlots.length) {
      liveBatchSlots[index] = { type: "preview", item };
    }
  }
  const showingLiveBatchGrid = isRunning && liveBatchSlotCount > 0;
  const showingResultGrid = showingLiveBatchGrid || (resultGridOpen && batchResults.length > 1);
  const currentBatchIndex = currentImage ? navigationItems.findIndex((item) => item.id === currentImage.id) : -1;
  const canNavigateBatchResults = currentBatchIndex >= 0 && navigationItems.length > 1;

  const stageRef = useRef<Konva.Stage | null>(null);
  const roRef = useRef<ResizeObserver | null>(null);
  const pinchRef = useRef<PinchState | null>(null);
  const [pinching, setPinching] = useState(false);
  const effectiveTool = pinching ? "pan" : tool;

  const currentImageURL = historyFullSrc(currentImage, null);
  const image = useImageFromSource(currentImage?.imageBlob ?? null, currentImage?.imageB64, currentImageURL);
  const importedMaskImage = useImageFromSource(null, undefined, maskDataURL);

  useEffect(() => {
    if (!currentImage?.previewOnly) return;
    if (currentImage.fullUrl || currentImage.imageId || currentImage.imageB64 || currentImage.imageBlob) return;
    if (currentImage.id.startsWith("preview-")) return;

    let cancelled = false;
    const selectedId = currentImage.id;
    void useStudioStore.getState().materializeCurrentImage(currentImage).then((full) => {
      if (cancelled || !full || full.previewOnly) return;
      if (useStudioStore.getState().currentImage?.id !== selectedId) return;
      setField("currentImage", full);
    }).catch(() => undefined);

    return () => {
      cancelled = true;
    };
  }, [
    currentImage?.id,
    currentImage?.previewOnly,
    currentImage?.fullUrl,
    currentImage?.imageId,
    currentImage?.imageB64,
    currentImage?.imageBlob,
    setField,
  ]);
  const [hostSize, setHostSize] = useState({ w: 0, h: 0 });
  const hostRef = useCallback((node: HTMLDivElement | null) => {
    if (roRef.current) {
      roRef.current.disconnect();
      roRef.current = null;
    }
    if (!node) return;
    const update = () => {
      const w = node.clientWidth;
      const h = node.clientHeight;
      if (w > 0 && h > 0) setHostSize({ w, h });
    };
    update();
    if (typeof ResizeObserver === "function") {
      const ro = new ResizeObserver(update);
      ro.observe(node);
      roRef.current = ro;
      return;
    }
    window.addEventListener("resize", update);
    roRef.current = {
      disconnect: () => window.removeEventListener("resize", update),
    } as ResizeObserver;
  }, []);

  function computeFit(img: HTMLImageElement | null, hw: number, hh: number) {
    if (!img || hw === 0 || hh === 0) return { scale: 1, x: 0, y: 0, w: 0, h: 0 };
    const pad = Math.max(18, Math.min(36, Math.floor(Math.min(hw, hh) * 0.07)));
    const sw = (hw - pad * 2) / img.width;
    const sh = (hh - pad * 2) / img.height;
    const scale = Math.min(sw, sh, 1);
    const w = img.width * scale;
    const h = img.height * scale;
    return { scale, x: (hw - w) / 2, y: (hh - h) / 2, w, h };
  }

  const fit = computeFit(image, hostSize.w, hostSize.h);
  const [userView, setUserView] = useState<ViewState | null>(null);
  const view = userView ?? { scale: fit.scale, x: fit.x, y: fit.y };

  useLayoutEffect(() => {
    const stage = stageRef.current;
    if (!stage) return;
    stage.x(view.x);
    stage.y(view.y);
    stage.scaleX(view.scale);
    stage.scaleY(view.scale);
    stage.batchDraw();
    setField("viewZoom", view.scale);
  }, [view.x, view.y, view.scale, image, hostSize.w, hostSize.h, setField]);

  function resetView() {
    setUserView(null);
  }

  function setView(next: ViewState) {
    setUserView(next);
  }

  function cycleZoom() {
    if (!image || hostSize.w === 0) return;
    vibrateForPlatform(10);
    if (!userView || Math.abs(userView.scale - 1) > 0.001) {
      setUserView({
        scale: 1,
        x: (hostSize.w - image.width) / 2,
        y: (hostSize.h - image.height) / 2,
      });
    } else {
      setUserView(null);
    }
  }

  function clampScale(value: number) {
    return Math.max(MIN_VIEW_SCALE, Math.min(MAX_VIEW_SCALE, value));
  }

  function zoomAround(point: { x: number; y: number }, nextScaleValue: number) {
    if (!image || hostSize.w === 0 || hostSize.h === 0) return;
    const oldScale = Math.max(MIN_VIEW_SCALE, view.scale);
    const nextScale = clampScale(nextScaleValue);
    const imagePoint = {
      x: (point.x - view.x) / oldScale,
      y: (point.y - view.y) / oldScale,
    };
    setView({
      scale: nextScale,
      x: point.x - imagePoint.x * nextScale,
      y: point.y - imagePoint.y * nextScale,
    });
  }

  function zoomBy(factor: number) {
    zoomAround({ x: hostSize.w / 2, y: hostSize.h / 2 }, view.scale * factor);
  }

  function stagePointerToImageCoord(): { x: number; y: number } | null {
    const stage = stageRef.current;
    if (!stage || !image) return null;
    const p = stage.getPointerPosition();
    if (!p) return null;
    return {
      x: (p.x - view.x) / view.scale,
      y: (p.y - view.y) / view.scale,
    };
  }

  const drawingRef = useRef<{ active: boolean; current: Stroke | null }>({ active: false, current: null });
  const previousImageIDRef = useRef(currentImage?.id);
  const previousCanvasResetTickRef = useRef(canvasViewResetTick);
  const freehandRef = useRef<number[] | null>(null);
  const [, setDrawingTick] = useState(0);
  const [maskCursor, setMaskCursor] = useState<{ x: number; y: number } | null>(null);
  const [drag, setDrag] = useState<null | { kind: "rect" | "arrow" | "freehand" | "text"; sx: number; sy: number; x: number; y: number }>(null);

  useEffect(() => {
    const imageChanged = previousImageIDRef.current !== currentImage?.id;
    previousImageIDRef.current = currentImage?.id;
    setUserView(null);
    if (imageChanged) {
      useStudioStore.setState({
        maskDataURL: null,
        maskTargetPath: null,
        strokes: [],
        annotations: [],
        selectedAnnotationId: null,
        undoStack: [],
        redoStack: [],
      });
    }
    drawingRef.current = { active: false, current: null };
    freehandRef.current = null;
    setMaskCursor(null);
    setDrag(null);
    pinchRef.current = null;
    setPinching(false);
  }, [currentImage?.id]);

  useEffect(() => {
    const canvasChanged = previousCanvasResetTickRef.current !== canvasViewResetTick;
    previousCanvasResetTickRef.current = canvasViewResetTick;
    if (!canvasChanged) return;
    setUserView(null);
    useStudioStore.setState({
      maskDataURL: null,
      maskTargetPath: null,
      strokes: [],
      annotations: [],
      selectedAnnotationId: null,
      undoStack: [],
      redoStack: [],
    });
    drawingRef.current = { active: false, current: null };
    freehandRef.current = null;
    setMaskCursor(null);
    setDrag(null);
    pinchRef.current = null;
    setPinching(false);
  }, [canvasViewResetTick]);

  useEffect(() => {
    setUserView(null);
    drawingRef.current = { active: false, current: null };
    freehandRef.current = null;
    setMaskCursor(null);
    setDrag(null);
    pinchRef.current = null;
    setPinching(false);
  }, [fullscreen]);

  function isInsideImage(point: { x: number; y: number }): boolean {
    return !!image && point.x >= 0 && point.y >= 0 && point.x <= image.width && point.y <= image.height;
  }

  function beginPointer(e: Konva.KonvaEventObject<PointerEvent>) {
    if (!image || pinching) return;
    const local = stagePointerToImageCoord();
    if (!local || !isInsideImage(local)) return;
    if (effectiveTool === "mask") {
      setMaskCursor(local);
      drawingRef.current = { active: true, current: { points: [local.x, local.y], size: brushSize, erase: brushMode === "erase" } };
      vibrateForPlatform(4);
    } else if (effectiveTool === "annotate") {
      const target = e.target;
      if (target === stageRef.current || target.getClassName?.() === "Image") {
        setField("selectedAnnotationId", null);
      }
      if (annotationKind === "freehand") {
        freehandRef.current = [local.x, local.y];
        setDrawingTick((n) => n + 1);
      } else if (annotationKind === "text") {
        const text = window.prompt("文字标注内容:");
        if (text && text.trim()) {
          addAnnotation({
            id: crypto.randomUUID(),
            kind: "text",
            x: local.x,
            y: local.y,
            text: text.trim(),
            color: annotationColor,
          });
        }
      } else {
        setDrag({ kind: annotationKind, sx: local.x, sy: local.y, x: local.x, y: local.y });
      }
    }
  }

  function movePointer() {
    if (!image || pinching) return;
    const local = stagePointerToImageCoord();
    if (!local) return;
    const insideImage = isInsideImage(local);
    setMaskCursor(effectiveTool === "mask" && insideImage ? local : null);
    if (!insideImage) {
      if (effectiveTool === "mask" && drawingRef.current.active) finishMaskStroke();
      return;
    }
    if (effectiveTool === "mask" && drawingRef.current.active && drawingRef.current.current) {
      drawingRef.current.current.points.push(local.x, local.y);
      setDrawingTick((n) => n + 1);
    } else if (effectiveTool === "annotate" && annotationKind === "freehand" && freehandRef.current) {
      freehandRef.current.push(local.x, local.y);
      setDrawingTick((n) => n + 1);
    } else if (effectiveTool === "annotate" && drag) {
      setDrag({ ...drag, x: local.x, y: local.y });
    }
  }

  function endPointer() {
    if (pinching) return;
    setMaskCursor(null);
    if (effectiveTool === "mask" && drawingRef.current.active) {
      finishMaskStroke();
    } else if (effectiveTool === "annotate" && annotationKind === "freehand" && freehandRef.current) {
      const pts = freehandRef.current;
      freehandRef.current = null;
      if (pts.length >= 4) {
        addAnnotation({
          id: crypto.randomUUID(),
          kind: "freehand",
          x: 0,
          y: 0,
          color: annotationColor,
          points: pts,
        });
      }
    } else if (effectiveTool === "annotate" && drag) {
      const w = drag.x - drag.sx;
      const h = drag.y - drag.sy;
      if (Math.abs(w) > 3 && Math.abs(h) > 3) {
        if (drag.kind === "rect") {
          addAnnotation({
            id: crypto.randomUUID(),
            kind: "rect",
            x: Math.min(drag.sx, drag.x),
            y: Math.min(drag.sy, drag.y),
            width: Math.abs(w),
            height: Math.abs(h),
            color: annotationColor,
          });
        } else if (drag.kind === "arrow") {
          addAnnotation({
            id: crypto.randomUUID(),
            kind: "arrow",
            x: drag.sx,
            y: drag.sy,
            width: drag.x - drag.sx,
            height: drag.y - drag.sy,
            color: annotationColor,
          });
        }
      }
      setDrag(null);
    }
  }

  function finishMaskStroke() {
    const finished = drawingRef.current.current;
    drawingRef.current = { active: false, current: null };
    if (finished) pushStroke(finished);
  }

  function leavePointer() {
    setMaskCursor(null);
    endPointer();
  }

  function touchPoint(touch: Touch) {
    const rect = stageRef.current?.container().getBoundingClientRect();
    return {
      x: touch.clientX - (rect?.left ?? 0),
      y: touch.clientY - (rect?.top ?? 0),
    };
  }

  function distance(a: { x: number; y: number }, b: { x: number; y: number }) {
    return Math.hypot(a.x - b.x, a.y - b.y);
  }

  function center(a: { x: number; y: number }, b: { x: number; y: number }) {
    return { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 };
  }

  function beginPinch(e: Konva.KonvaEventObject<TouchEvent>) {
    if (e.evt.touches.length < 2) return;
    e.evt.preventDefault();
    const a = touchPoint(e.evt.touches[0]);
    const b = touchPoint(e.evt.touches[1]);
    drawingRef.current = { active: false, current: null };
    freehandRef.current = null;
    setMaskCursor(null);
    setDrag(null);
    pinchRef.current = {
      distance: distance(a, b),
      center: center(a, b),
      view,
    };
    setPinching(true);
  }

  function movePinch(e: Konva.KonvaEventObject<TouchEvent>) {
    const pinch = pinchRef.current;
    if (!pinch || e.evt.touches.length < 2) return;
    e.evt.preventDefault();
    const a = touchPoint(e.evt.touches[0]);
    const b = touchPoint(e.evt.touches[1]);
    const nextCenter = center(a, b);
    const nextScale = clampScale(pinch.view.scale * (distance(a, b) / Math.max(1, pinch.distance)));
    const imagePoint = {
      x: (pinch.center.x - pinch.view.x) / pinch.view.scale,
      y: (pinch.center.y - pinch.view.y) / pinch.view.scale,
    };
    setView({
      scale: nextScale,
      x: nextCenter.x - imagePoint.x * nextScale,
      y: nextCenter.y - imagePoint.y * nextScale,
    });
  }

  function endPinch(e: Konva.KonvaEventObject<TouchEvent>) {
    if (e.evt.touches.length >= 2) return;
    pinchRef.current = null;
    setPinching(false);
  }

  function onWheel(e: Konva.KonvaEventObject<WheelEvent>) {
    e.evt.preventDefault();
    const stage = stageRef.current;
    if (!stage) return;
    const oldScale = view.scale;
    const pointer = stage.getPointerPosition();
    if (!pointer) return;
    const direction = e.evt.deltaY > 0 ? -1 : 1;
    const factor = 1.15;
    zoomAround(pointer, direction > 0 ? oldScale * factor : oldScale / factor);
  }

  useEffect(() => {
    (window as any).__canvasResetView = resetView;
    (window as any).__androidCanvasResetView = resetView;
    (window as any).__androidCanvasZoomIn = () => zoomBy(ZOOM_STEP);
    (window as any).__androidCanvasZoomOut = () => zoomBy(1 / ZOOM_STEP);
    return () => {
      delete (window as any).__canvasResetView;
      delete (window as any).__androidCanvasResetView;
      delete (window as any).__androidCanvasZoomIn;
      delete (window as any).__androidCanvasZoomOut;
    };
  }, [fit.scale, fit.x, fit.y, hostSize.h, hostSize.w, image, view.scale, view.x, view.y]);

  useCanvasShortcuts({
    brushSize,
    brushMode,
    canNavigateBatchResults,
    cancel,
    compareB,
    copyCurrentImage: () => {
      if (!currentImage) return;
      const copyPromise = useStudioStore.getState().materializeCurrentImage(currentImage).then((full) => (
        full.fullUrl
          ? copyImageURLToClipboard(full.fullUrl)
          : copyImageB64ToClipboard(full.imageB64 ?? "")
      ));
      copyPromise.then((ok) => {
        const pushToast = useStudioStore.getState().pushToast;
        if (ok) pushToast("已复制图片到剪贴板", "success");
        else pushToast("复制失败,当前运行环境拒绝写剪贴板", "error");
      });
    },
    currentImage,
    errorMessage,
    isMac: false,
    isRunning,
    navigateBatchResult: (delta) => {
      void stepBatchResult(delta);
    },
    redo,
    removeAnnotation,
    resetView,
    selectedAnnotationId,
    setBrushSize: (value) => setField("brushSize", value),
    setBrushMode: (value) => setField("brushMode", value),
    setCompareB,
    setErrorMessage: (value) => setField("errorMessage", value),
    toggleFullscreen,
    setSelectedAnnotationId: (value) => setField("selectedAnnotationId", value),
    setTool: (value) => {
      if (value === "mask") void activateMaskTool();
      else setField("tool", value);
    },
    tool,
    undo,
  });

  return (
    <div
      ref={hostRef}
      className="stage-host android-stage-host"
      style={{ cursor: !currentImage
        ? "default"
        : effectiveTool === "pan"
          ? "grab"
          : effectiveTool === "mask" && maskCursor
            ? "none"
            : "crosshair" }}
    >
      {showingResultGrid ? (
        <BatchResultGrid
          items={orderedBatchResults}
          slots={showingLiveBatchGrid ? liveBatchSlots : undefined}
          currentId={currentImage?.id ?? null}
          onSelect={selectBatchResult}
          onClose={closeResultGrid}
          showClose={!showingLiveBatchGrid}
          title={showingLiveBatchGrid ? `当前并发预览 · ${runningJobs.length} 路 · ${jobsCompleted}/${jobsTotal}` : undefined}
          livePreview={showingLiveBatchGrid}
        />
      ) : null}
      {!showingResultGrid && currentImage && compareB ? (
        <CompareOverlay
          aBlob={currentImage.imageBlob ?? null}
          aB64={currentImage.imageB64}
          aUrl={currentImage.fullUrl}
          bBlob={compareB.imageBlob ?? null}
          bB64={compareB.imageB64}
          bUrl={compareB.fullUrl}
          split={compareSplit}
          onSplit={setCompareSplit}
          aLabel="当前图"
          bLabel={compareB.id.startsWith("source-preview:") ? "参考图" : "对比图"}
        />
      ) : null}
      {!showingResultGrid && currentImage && !compareB && hostSize.w > 0 && hostSize.h > 0 ? (
        <div className="stage-canvas-wrap">
          <Stage
            ref={stageRef}
            width={hostSize.w}
            height={hostSize.h}
            x={view.x}
            y={view.y}
            scaleX={view.scale}
            scaleY={view.scale}
            draggable={effectiveTool === "pan" && !pinching}
            onDragEnd={(e) => setView({ ...view, x: e.target.x(), y: e.target.y() })}
            onWheel={onWheel}
            onPointerDown={beginPointer}
            onPointerMove={movePointer}
            onPointerUp={endPointer}
            onPointerLeave={leavePointer}
            onDblClick={cycleZoom}
            onDblTap={cycleZoom}
            onTouchStart={beginPinch}
            onTouchMove={movePinch}
            onTouchEnd={endPinch}
            onTouchCancel={endPinch}
          >
            <Layer>
              {image ? <KonvaImage image={image} listening={false} /> : null}
            </Layer>

            {image ? (
              <MaskOverlayLayer
                image={image}
                baseMaskImage={importedMaskImage}
                strokes={strokes}
                draft={drawingRef.current.current ? {
                  ...drawingRef.current.current,
                  points: drawingRef.current.current.points.slice(),
                } : null}
                visible={maskVisible}
                opacity={maskOpacity}
                cursor={maskCursor}
                showCursor={effectiveTool === "mask" && !pinching}
                brushSize={brushSize}
                brushMode={brushMode}
                viewScale={view.scale}
              />
            ) : null}

            <Layer>
              {annotations.map((a) => (
                <AnnotationShape
                  key={a.id}
                  annotation={a}
                  selected={selectedAnnotationId === a.id}
                  onSelect={() => setField("selectedAnnotationId", a.id)}
                />
              ))}
              {drag && drag.kind === "rect" ? (
                <Rect
                  x={Math.min(drag.sx, drag.x)}
                  y={Math.min(drag.sy, drag.y)}
                  width={Math.abs(drag.x - drag.sx)}
                  height={Math.abs(drag.y - drag.sy)}
                  stroke={annotationColor}
                  strokeWidth={2 / view.scale}
                  dash={[6 / view.scale, 4 / view.scale]}
                  listening={false}
                />
              ) : null}
              {drag && drag.kind === "arrow" ? (
                <Arrow
                  points={[drag.sx, drag.sy, drag.x, drag.y]}
                  stroke={annotationColor}
                  strokeWidth={2 / view.scale}
                  fill={annotationColor}
                  pointerLength={12 / view.scale}
                  pointerWidth={12 / view.scale}
                  listening={false}
                />
              ) : null}
              {freehandRef.current && freehandRef.current.length >= 4 ? (
                <Line
                  points={freehandRef.current.slice()}
                  stroke={annotationColor}
                  strokeWidth={3 / view.scale}
                  lineCap="round"
                  lineJoin="round"
                  tension={0.4}
                  listening={false}
                />
              ) : null}
            </Layer>
          </Stage>
        </div>
      ) : null}
      {streamPreview && currentImage && !showingLiveBatchGrid ? (
        <div className="stream-preview-overlay">
          <StreamPreviewBadge compact />
        </div>
      ) : null}
    </div>
  );
}

export const AndroidCanvasStage = memo(AndroidCanvasStageContent);

export function androidCanvasModeLabel(item: HistoryItem | null) {
  if (!item) return "空画布";
  return item.mode === "edit" ? "编辑结果" : "生成结果";
}
