import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { Stage, Layer, Image as KonvaImage, Line, Rect, Arrow } from "react-konva";
import Konva from "konva";
import { useStudioStore } from "../../state/studioStore";
import { HistoryItem } from "../../types/domain";
import { usePlatform } from "../../platform/context";
import { ContextMenu, MenuItem } from "../common/ContextMenu";
import { BatchResultGrid, type BatchGridSlot } from "./BatchResultGrid";
import { CompareOverlay } from "./CompareOverlay";
import type { Stroke } from "../../state/studioStore.types";
import { EmptyState } from "./EmptyState";
import { copyImageB64ToClipboard, copyImageURLToClipboard, useImageFromSource } from "./canvasImage";
import { AnnotationShape } from "./AnnotationShape";
import { useCanvasShortcuts } from "./useCanvasShortcuts";
import { StreamPreviewBadge } from "./StreamPreviewBadge";
import { streamPreviewItemsFromPreviews } from "../../state/studioStore.streamPreview";
import { historyFullSrc, orderedNavigationItemsForCurrent, sortHistoryItemsByCreatedAtAsc } from "../../lib/images";
import { DragExportHandle } from "./DragExportHandle";

export function CanvasStage() {
  const {
    currentImage, tool, brushSize, brushMode,
    annotationKind, annotationColor,
    selectedAnnotationId,
    annotations, addAnnotation, removeAnnotation, clearAnnotations,
    setMaskDataURL,
    maskDataURL,
    strokes, pushStroke,
    undoStack, redoStack, undo, redo,
    compareB, compareSplit, setCompareSplit, setCompareB,
    isRunning, cancel, errorMessage, setField,
    streamPreview,
    streamPreviews,
    runningJobs,
    jobsTotal,
    jobsCompleted,
    toggleFullscreen,
    history,
    batchResults, resultGridOpen, selectBatchResult, closeResultGrid,
    canvasViewResetTick,
    stepBatchResult,
  } = useStudioStore();
  const { isMac } = usePlatform();
  const streamPreviewItems = streamPreviewItemsFromPreviews(streamPreviews, {
    workspaceId: useStudioStore.getState().activeWorkspaceId,
    mode: useStudioStore.getState().mode,
    prompt: useStudioStore.getState().prompt,
    size: useStudioStore.getState().size,
    quality: useStudioStore.getState().quality,
    outputFormat: useStudioStore.getState().outputFormat,
    currentImage,
  });
  const orderedBatchResults = sortHistoryItemsByCreatedAtAsc(batchResults);
  const navigationItems = orderedNavigationItemsForCurrent(currentImage?.id, history, batchResults);
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

  // Hold-space-for-pan: while space is held, override tool to "pan".
  const [spacePan, setSpacePan] = useState(false);
  const effectiveTool = spacePan ? "pan" : tool;

  const stageRef = useRef<Konva.Stage | null>(null);
  const imageLayerRef = useRef<Konva.Layer | null>(null);
  const maskLayerRef = useRef<Konva.Layer | null>(null);
  const roRef = useRef<ResizeObserver | null>(null);

  const currentImageURL = historyFullSrc(currentImage, null);
  const image = useImageFromSource(currentImage?.imageBlob ?? null, currentImage?.imageB64, currentImageURL);
  const importedMaskURL = maskDataURL && maskDataURL !== "__PENDING_MASK__" ? maskDataURL : null;
  const importedMaskImage = useImageFromSource(null, undefined, importedMaskURL);

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

  // ★ Measure the OUTER wrapper (.stage-host) — which is a normal grid item
  // bounded by its parent shell — instead of the inner absolute container.
  // This breaks the feedback loop where the Konva canvas width (= hostSize.w)
  // would otherwise expand its parent in normal flow and push hostSize → ∞.
  const [hostSize, setHostSize] = useState({ w: 0, h: 0 });
  const hostRef = useCallback((node: HTMLDivElement | null) => {
    if (roRef.current) { roRef.current.disconnect(); roRef.current = null; }
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

  // Plain function — not useMemo — so it is always computed with the very
  // latest hostSize / image references on every render. Avoids the closure
  // race we saw with useMemo deps.
  function computeFit(img: HTMLImageElement | null, hw: number, hh: number) {
    if (!img || hw === 0 || hh === 0) return { scale: 1, x: 0, y: 0, w: 0, h: 0 };
    const pad = 40;
    const sw = (hw - pad * 2) / img.width;
    const sh = (hh - pad * 2) / img.height;
    const scale = Math.min(sw, sh, 1);
    const w = img.width * scale;
    const h = img.height * scale;
    return { scale, x: (hw - w) / 2, y: (hh - h) / 2, w, h };
  }
  const fit = computeFit(image, hostSize.w, hostSize.h);

  // `userView` only holds explicit user manipulation (pan / wheel zoom).
  // The effective view is `userView ?? fit`, so the displayed image is always
  // centered by default. userView is reset whenever currentImage.id changes.
  const [userView, setUserView] = useState<{ scale: number; x: number; y: number } | null>(null);
  const view = userView ?? { scale: fit.scale, x: fit.x, y: fit.y };

  // Imperatively push the latest fit onto the Konva Stage *after* React commits
  // and *before* paint. This is the belt-and-suspenders fix: even if React
  // props somehow lag a frame behind, this guarantees the visible stage is at
  // the right position whenever image / hostSize / userView changes.
  useLayoutEffect(() => {
    const stage = stageRef.current;
    if (!stage) return;
    stage.x(view.x);
    stage.y(view.y);
    stage.scaleX(view.scale);
    stage.scaleY(view.scale);
    stage.batchDraw();
    setField("viewZoom", view.scale);
  }, [view.x, view.y, view.scale, image, hostSize.w, hostSize.h]);

  // Double-click on the stage: cycle between fit and 100%.
  function onStageDblClick() {
    if (!image || hostSize.w === 0) return;
    if (!userView || Math.abs(userView.scale - 1) > 0.001) {
      // Currently fit (or not at 100%) → snap to 100% centred on image.
      const cx = (hostSize.w - image.width) / 2;
      const cy = (hostSize.h - image.height) / 2;
      setUserView({ scale: 1, x: cx, y: cy });
    } else {
      setUserView(null); // back to fit
    }
  }

  // Local "in-flight" stroke buffer — only the completed strokes live in the
  // store (so we don't spam zustand on every mousemove). Forces re-render via
  // a tick counter when the in-progress stroke needs to redraw.
  const drawingRef = useRef<{ active: boolean; current: Stroke | null }>({ active: false, current: null });
  const [, setDrawingTick] = useState(0);

  // Annotation drag state.
  const [drag, setDrag] = useState<null | { kind: "rect" | "arrow" | "freehand" | "text"; sx: number; sy: number; x: number; y: number }>(null);
  const [canvasMenu, setCanvasMenu] = useState<null | { x: number; y: number }>(null);

  // When the displayed image identity changes, clear the user's manual view
  // and per-image canvas state. This guarantees the new image starts at fit.
  // canvasViewResetTick 触发同样的重置 —— 用于 旋转 / 翻转 / 裁剪 这些「就地编辑」
  // 操作:currentImage.id 没变(就是原来那张),但底图尺寸 / 坐标已变,残留的 pan/zoom
  // 与蒙版坐标系都失效了。
  useEffect(() => {
    setUserView(null);
    setMaskDataURL(null);
    drawingRef.current = { active: false, current: null };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentImage?.id, canvasViewResetTick]);

  // setView is the only writer of userView. Treat any explicit pan/zoom as a
  // user override; auto-recenter happens by resetting to null elsewhere.
  function setView(v: { scale: number; x: number; y: number }) {
    setUserView(v);
  }

  // Mouse wheel zoom around cursor.
  function onWheel(e: Konva.KonvaEventObject<WheelEvent>) {
    e.evt.preventDefault();
    const stage = stageRef.current;
    if (!stage) return;
    const oldScale = view.scale;
    const pointer = stage.getPointerPosition();
    if (!pointer) return;
    const mousePointTo = {
      x: (pointer.x - view.x) / oldScale,
      y: (pointer.y - view.y) / oldScale,
    };
    const direction = e.evt.deltaY > 0 ? -1 : 1;
    const factor = 1.15;
    const newScale = Math.max(0.05, Math.min(8, direction > 0 ? oldScale * factor : oldScale / factor));
    setView({
      scale: newScale,
      x: pointer.x - mousePointTo.x * newScale,
      y: pointer.y - mousePointTo.y * newScale,
    });
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

  // In-progress freehand annotation buffer (kept in ref to keep mousemove cheap).
  const freehandRef = useRef<number[] | null>(null);

  function onMouseDown(e: Konva.KonvaEventObject<MouseEvent>) {
    if (!image) return;
    const local = stagePointerToImageCoord();
    if (!local) return;
    if (effectiveTool === "mask") {
      drawingRef.current = { active: true, current: { points: [local.x, local.y], size: brushSize, erase: brushMode === "erase" } };
    } else if (effectiveTool === "annotate") {
      // Click on empty area while in annotate mode clears any selection.
      // Click on an annotation shape is handled by the shape's own onClick.
      const target = e.target;
      if (target === stageRef.current || target.getClassName?.() === "Image") {
        setField("selectedAnnotationId", null);
      }
      if (annotationKind === "freehand") {
        freehandRef.current = [local.x, local.y];
        setDrawingTick((n) => n + 1);
      } else if (annotationKind === "text") {
        // Text annotations are created via a prompt on mouse down (no drag).
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

  function openCanvasMenu(e: Konva.KonvaEventObject<PointerEvent>) {
    if (!currentImage) return;
    e.evt.preventDefault();
    setCanvasMenu({ x: e.evt.clientX, y: e.evt.clientY });
  }

  const canvasMenuItems: MenuItem[] = currentImage ? [
    { label: "查看详情", icon: "ℹ", onClick: () => void useStudioStore.getState().openResultDetail(currentImage) },
    { label: "另存为", icon: "💾", onClick: () => void useStudioStore.getState().saveCurrentImageAs() },
    {
      label: modeLabelForMenu(currentImage),
      icon: "→",
      onClick: () => void useStudioStore.getState().reuseAsSource(currentImage),
    },
    { separatorBefore: true, label: "清空画板", icon: "✕", onClick: () => useStudioStore.getState().setField("currentImage", null) },
  ] : [];

  function onMouseMove() {
    if (!image) return;
    const local = stagePointerToImageCoord();
    if (!local) return;
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

  function onMouseUp() {
    if (effectiveTool === "mask" && drawingRef.current.active && drawingRef.current.current) {
      const finished = drawingRef.current.current;
      drawingRef.current = { active: false, current: null };
      pushStroke(finished);
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

  // Keep the store flag in sync so submit can cheaply know whether any mask
  // exists, but defer the expensive PNG export until the user actually submits.
  useEffect(() => {
    const hasImportedMask = !!maskDataURL && maskDataURL !== "__PENDING_MASK__";
    if (!image) {
      setMaskDataURL(null);
      return;
    }
    if (strokes.length === 0) {
      if (!hasImportedMask) setMaskDataURL(null);
      return;
    }
    const hasWhite = strokes.some((s) => !s.erase);
    setMaskDataURL(hasWhite ? "__PENDING_MASK__" : null);
  }, [strokes, image, maskDataURL, setMaskDataURL]);

  function resetView() {
    setUserView(null);
  }

  // Expose helpers via window for the toolbar reset buttons.
  useEffect(() => {
    (window as any).__canvasResetView = resetView;
    return () => {
      delete (window as any).__canvasResetView;
    };
  }, [fit.scale, fit.x, fit.y]);

  useCanvasShortcuts({
    brushSize,
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
    isMac,
    isRunning,
    navigateBatchResult: (delta) => {
      void stepBatchResult(delta);
    },
    redo,
    removeAnnotation,
    resetView,
    selectedAnnotationId,
    setBrushSize: (value) => setField("brushSize", value),
    setCompareB,
    setErrorMessage: (value) => setField("errorMessage", value),
    toggleFullscreen,
    setSelectedAnnotationId: (value) => setField("selectedAnnotationId", value),
    setTool: (value) => setField("tool", value),
    undo,
  });

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement | null;
      const isTyping = !!t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable);
      if (isTyping) return;
      if (e.code === "Space" && !e.repeat) {
        e.preventDefault();
        setSpacePan(true);
      }
    };
    const onKeyUp = (e: KeyboardEvent) => {
      if (e.code === "Space") setSpacePan(false);
    };
    window.addEventListener("keydown", onKeyDown);
    window.addEventListener("keyup", onKeyUp);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
      window.removeEventListener("keyup", onKeyUp);
    };
  }, []);

  // Host div is rendered unconditionally so its ref (and the ResizeObserver
  // attached to it) survives the empty-state → has-image transition. Previously
  // the empty branch had its own <div ref={hostRef}>, the host unmounted on
  // first generate, and the observer kept reporting the stale initial size.
  return (
    <>
      <div
        ref={hostRef}
        className="stage-host"
        style={{ cursor: !currentImage ? "default" : (effectiveTool === "pan" ? (spacePan ? "grabbing" : "grab") : "crosshair") }}
      >
        {!currentImage && !showingResultGrid && <EmptyState />}
        {streamPreview && currentImage && !showingLiveBatchGrid ? (
          <div className="stream-preview-overlay">
            <StreamPreviewBadge />
          </div>
        ) : null}
        {!showingResultGrid && currentImage ? (
          <DragExportHandle
            item={currentImage}
            sourceURL={currentImageURL}
            className="stage-drag-export"
          />
        ) : null}
        {showingResultGrid && (
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
        )}
        {!showingResultGrid && currentImage && compareB && (
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
        )}
        {!showingResultGrid && currentImage && !compareB && hostSize.w > 0 && hostSize.h > 0 && (
        // The Stage canvas is wrapped in an absolutely positioned container so
        // its (potentially very large) layout footprint cannot push back on the
        // stage-host's grid-derived width. stage-host stays bounded by the grid
        // track; this wrapper takes whatever size stage-host gives it via inset:0.
        <div className="stage-canvas-wrap" style={{ position: "absolute", inset: 0, overflow: "hidden" }}>
        <Stage
          ref={stageRef}
          width={hostSize.w}
          height={hostSize.h}
          x={view.x}
          y={view.y}
          scaleX={view.scale}
          scaleY={view.scale}
          draggable={effectiveTool === "pan"}
          onDragEnd={(e) => setView({ ...view, x: e.target.x(), y: e.target.y() })}
          onWheel={onWheel}
          onMouseDown={onMouseDown}
          onMouseMove={onMouseMove}
          onMouseUp={onMouseUp}
          onMouseLeave={onMouseUp}
          onDblClick={onStageDblClick}
          onContextMenu={openCanvasMenu}
        >
          <Layer ref={imageLayerRef}>
            {image && <KonvaImage image={image} listening={false} />}
          </Layer>

          <Layer ref={maskLayerRef}>
            {image && importedMaskImage ? (
              <KonvaImage
                image={importedMaskImage}
                width={image.width}
                height={image.height}
                opacity={0.35}
                listening={false}
              />
            ) : null}
            {strokes.map((s, i) => (
              <Line
                key={i}
                points={s.points}
                stroke={s.erase ? "rgba(226,85,85,0.55)" : "rgba(77,124,255,0.55)"}
                strokeWidth={s.size}
                lineCap="round"
                lineJoin="round"
                tension={0.4}
                dash={s.erase ? [s.size * 0.4, s.size * 0.4] : undefined}
                listening={false}
                globalCompositeOperation={s.erase ? "destination-out" : "source-over"}
              />
            ))}
            {drawingRef.current.current && (
              <Line
                // ★ 必须 .slice() 出新数组引用 —— onMouseMove 原地 push 不会改变
                // points 数组引用,react-konva 走 prop 浅比较会跳过更新,导致
                // 拖拽期间只画起点 / 终点,松手才一次性补全所有中间点。
                points={drawingRef.current.current.points.slice()}
                stroke={drawingRef.current.current.erase ? "rgba(226,85,85,0.55)" : "rgba(77,124,255,0.55)"}
                strokeWidth={drawingRef.current.current.size}
                lineCap="round"
                lineJoin="round"
                tension={0.4}
                dash={drawingRef.current.current.erase ? [drawingRef.current.current.size * 0.4, drawingRef.current.current.size * 0.4] : undefined}
                listening={false}
                globalCompositeOperation={drawingRef.current.current.erase ? "destination-out" : "source-over"}
              />
            )}
          </Layer>

          <Layer>
            {annotations.map((a) => (
              <AnnotationShape
                key={a.id}
                annotation={a}
                selected={selectedAnnotationId === a.id}
                onSelect={() => setField("selectedAnnotationId", a.id)}
              />
            ))}
            {drag && drag.kind === "rect" && (
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
            )}
            {drag && drag.kind === "arrow" && (
              <Arrow
                points={[drag.sx, drag.sy, drag.x, drag.y]}
                stroke={annotationColor}
                strokeWidth={2 / view.scale}
                fill={annotationColor}
                pointerLength={12 / view.scale}
                pointerWidth={12 / view.scale}
                listening={false}
              />
            )}
            {freehandRef.current && freehandRef.current.length >= 4 && (
              <Line
                // 同上:.slice() 强制每帧新引用,绕过 react-konva 的浅比较跳更新。
                points={freehandRef.current.slice()}
                stroke={annotationColor}
                strokeWidth={3 / view.scale}
                lineCap="round"
                lineJoin="round"
                tension={0.4}
                listening={false}
              />
            )}
          </Layer>
        </Stage>
        </div>
        )}
      </div>
      {canvasMenu && currentImage ? (
        <ContextMenu
          x={canvasMenu.x}
          y={canvasMenu.y}
          items={canvasMenuItems}
          onClose={() => setCanvasMenu(null)}
        />
      ) : null}
    </>
  );
}

function modeLabelForMenu(item: HistoryItem) {
  return item.mode === "edit" ? "设为继续编辑源图" : "设为图生图源图";
}
