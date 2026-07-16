import {
  ArrowRight,
  Brush,
  CheckCircle2,
  Crop,
  Eraser,
  Expand,
  FlipHorizontal,
  FlipVertical,
  Hand,
  ImagePlus,
  Info,
  Loader2,
  Maximize,
  Minimize,
  ZoomIn,
  ZoomOut,
  Pencil,
  RotateCcw,
  RotateCw,
  Save,
  Scissors,
  Square,
  Trash2,
  Type as TypeIcon,
  Undo2,
  Redo2,
  Upload,
  X,
} from "lucide-react";
import type { ReactNode } from "react";
import { useEffect, useState } from "react";
import { ANNOTATION_COLORS, type AnnotationKind, type HistoryItem, type SourceImage } from "../../../types/domain";
import { useStudioStore } from "../../../state/studioStore";
import { base64ToBlob, useBlobURL } from "../../../lib/images";
import { qualityLabel, sizeLabel } from "../../../components/history/historyLabels";
import { StreamPreviewBadge } from "../../../components/canvas/StreamPreviewBadge";
import { OpenImageDialog } from "../../runtime/host";
import { genId } from "../../../state/studioStore.shared";
import { usePlatform } from "../../context";
import { vibrateForPlatform } from "../bridge";
import { AndroidCanvasStage, androidCanvasModeLabel } from "./AndroidCanvasStage";

type CanvasTool = "pan" | "mask" | "annotate";
type BrushMode = "paint" | "erase";

export function AndroidCanvasWorkspace() {
  const {
    currentImage,
    mode,
    sources,
    isRunning,
    progress,
    streamPreview,
    runningJobs,
    jobsCompleted,
    jobsTotal,
    currentImage: item,
    tool,
    brushMode,
    brushSize,
    annotationKind,
    annotationColor,
    annotations,
    selectedAnnotationId,
    fullscreen,
    viewZoom,
    batchResults,
    resultGridOpen,
    undoStack,
    redoStack,
    setField,
    undo,
    redo,
    importMaskImage,
    resetMask,
    clearAnnotations,
    saveCurrentImageAs,
    rotateCurrent,
    flipCurrent,
    cropToRect,
    openResultGrid,
    closeResultGrid,
    openResultDetail,
    selectSourceImage,
    viewSourceOnCanvas,
    compareSourceOnCanvas,
    removeSource,
    reorderSources,
    clearSources,
    reuseAsSource,
    pushToast,
    toggleFullscreen,
  } = useStudioStore();
  const { isAndroidPad, androidOrientation } = usePlatform();
  const [sourceOpen, setSourceOpen] = useState(true);
  const hasImage = !!currentImage;
  const hasSources = mode === "edit" && sources.length > 0;
  const selRect = annotations.find((a) => a.id === selectedAnnotationId && a.kind === "rect");
  const cropAction = selRect && selRect.width && selRect.height
    ? () => cropToRect(selRect.x, selRect.y, selRect.width!, selRect.height!)
    : null;
  const showBatchToggle = batchResults.length > 1;
  const statusLabel = isRunning ? (progress?.stage ?? "处理中") : androidCanvasModeLabel(item);
  const shouldShowSourceStrip = hasSources && sourceOpen;
  const dockMode = tool === "mask" ? "mask" : tool === "annotate" ? "annotate" : "image";

  useEffect(() => {
    if (hasSources) setSourceOpen(true);
  }, [hasSources]);

  const runAction = (action: () => void | Promise<void>, vibration = 8) => {
    vibrateForPlatform(vibration);
    void action();
  };

  const importToCanvas = async () => {
    try {
      vibrateForPlatform(10);
      const res = await OpenImageDialog();
      if (!res?.path || (!res.previewUrl && !res.imageB64)) return;
      const name = res.path.split(/[\\/]/).pop() ?? `import-${Date.now()}.png`;
      const imageBlob = res.previewUrl ? null : base64ToBlob(res.imageB64 ?? "");
      const imported: HistoryItem = {
        id: genId(),
        imageId: res.imageId || undefined,
        previewUrl: res.previewUrl || undefined,
        previewWidth: res.previewWidth,
        previewHeight: res.previewHeight,
        imageB64: res.previewUrl ? undefined : res.imageB64,
        imageBlob: null,
        previewBlob: imageBlob,
        previewOnly: true,
        prompt: `(导入)${name}`,
        mode: "edit",
        size: "auto",
        quality: "medium",
        createdAt: Date.now(),
        savedPath: res.path,
      };
      const alreadyIn = useStudioStore.getState().sources.some((source) => source.path === res.path);
      setField("currentImage", imported);
      setField("mode", "edit");
      setField("resultGridOpen", false);
      if (!alreadyIn) {
        setField("sources", [
          ...useStudioStore.getState().sources,
          {
            path: res.path,
            name,
            size: res.size ?? 0,
            previewUrl: res.previewUrl,
            imageB64: res.previewUrl ? undefined : res.imageB64,
            imageBlob,
          },
        ]);
      }
      pushToast("已导入到画布", "success");
    } catch (error: any) {
      pushToast(`导入失败:${error?.message ?? error}`, "error");
    }
  };

  const resetView = () => {
    vibrateForPlatform(6);
    (window as any).__androidCanvasResetView?.();
  };

  const zoomCanvas = (direction: "in" | "out") => {
    vibrateForPlatform(5);
    if (direction === "in") (window as any).__androidCanvasZoomIn?.();
    else (window as any).__androidCanvasZoomOut?.();
  };

  return (
    <div
      className="android-canvas-workspace"
      data-dock-mode={dockMode}
      data-has-source-strip={shouldShowSourceStrip ? "true" : "false"}
      data-orientation={androidOrientation}
      data-device={isAndroidPad ? "pad" : "phone"}
    >
      <AndroidCanvasHeader
        currentImage={currentImage}
        isRunning={isRunning}
        progressLabel={statusLabel}
        streamPreviewActive={!!streamPreview}
        zoomLabel={hasImage ? `${Math.round(viewZoom * 100)}%` : "Fit"}
        batchCount={batchResults.length}
        sourceCount={sources.length}
        hasSources={hasSources}
        sourceOpen={sourceOpen}
        onToggleSources={() => runAction(() => setSourceOpen((value) => !value), 5)}
        onOpenGrid={showBatchToggle ? () => runAction(() => (resultGridOpen ? closeResultGrid() : openResultGrid()), 8) : undefined}
        gridOpen={resultGridOpen}
        progressMeta={isRunning ? {
          elapsed: progress?.elapsed,
          bytes: progress?.bytes,
          runningJobs: runningJobs.length,
          jobsCompleted,
          jobsTotal,
        } : null}
      />

      <div className="android-canvas-main">
        <div className="android-canvas-stage-panel">
          <AndroidCanvasStage />
          {!hasImage && !isRunning && !resultGridOpen ? (
            <AndroidCanvasEmptyState
              onImport={importToCanvas}
              onGoCompose={() => {
                vibrateForPlatform(8);
                document.querySelector<HTMLButtonElement>(".android-bottom-nav .android-nav-button:first-child")?.click();
              }}
            />
          ) : null}
          {hasImage ? (
            <div className="android-canvas-floating-meta">
              <span>{sizeLabel(currentImage.size)}</span>
              <span>{qualityLabel(currentImage.quality)}</span>
              <span>{currentImage.mode === "edit" ? "图生图" : "文生图"}</span>
            </div>
          ) : null}
        </div>
      </div>

      {shouldShowSourceStrip ? (
        <AndroidSourceStrip
          currentImage={currentImage}
          sources={sources}
          onAdd={() => runAction(selectSourceImage, 8)}
          onCompare={() => runAction(() => compareSourceOnCanvas(0), 8)}
          onPreview={(index) => runAction(() => viewSourceOnCanvas(index), 8)}
          onRemove={(index) => runAction(() => removeSource(index), 5)}
          onMove={(from, to) => runAction(() => reorderSources(from, to), 5)}
          onClear={() => runAction(clearSources, 5)}
        />
      ) : null}

      <AndroidCanvasDock>
        <div className="android-canvas-dock-scroll">
          <AndroidToolSegment
            value={tool}
            disabled={!hasImage}
            onChange={(next) => runAction(() => setField("tool", next), 8)}
          />
          <div className="android-canvas-dock-group compact">
            <DockIconButton title="撤销" disabled={undoStack.length === 0} onClick={() => runAction(undo, 6)}>
              <Undo2 />
            </DockIconButton>
            <DockIconButton title="重做" disabled={redoStack.length === 0} onClick={() => runAction(redo, 6)}>
              <Redo2 />
            </DockIconButton>
          </div>

          {tool === "mask" ? (
            <AndroidMaskControls
              brushMode={brushMode}
              brushSize={brushSize}
              onSetBrushMode={(next) => runAction(() => setField("brushMode", next), 5)}
              onSetBrushSize={(next) => setField("brushSize", next)}
              onImportMask={() => runAction(importMaskImage, 8)}
              onResetMask={() => runAction(resetMask, 6)}
            />
          ) : null}

          {tool === "annotate" ? (
            <AndroidAnnotationControls
              annotationKind={annotationKind}
              annotationColor={annotationColor}
              onSetKind={(next) => runAction(() => setField("annotationKind", next), 5)}
              onSetColor={(next) => runAction(() => setField("annotationColor", next), 3)}
              onClear={() => runAction(clearAnnotations, 6)}
            />
          ) : null}

          <div className="android-canvas-dock-group">
            <DockIconButton title="缩小画布" disabled={!hasImage} onClick={() => zoomCanvas("out")}>
              <ZoomOut />
            </DockIconButton>
            <DockIconButton title="放大画布" disabled={!hasImage} onClick={() => zoomCanvas("in")}>
              <ZoomIn />
            </DockIconButton>
            <DockIconButton title="重置视图" disabled={!hasImage} onClick={resetView}>
              <Expand />
            </DockIconButton>
            <DockIconButton title="左转 90°" disabled={!currentImage?.savedPath} onClick={() => runAction(() => rotateCurrent(-90), 8)}>
              <RotateCcw />
            </DockIconButton>
            <DockIconButton title="右转 90°" disabled={!currentImage?.savedPath} onClick={() => runAction(() => rotateCurrent(90), 8)}>
              <RotateCw />
            </DockIconButton>
            <DockIconButton title="水平翻转" disabled={!currentImage?.savedPath} onClick={() => runAction(() => flipCurrent(true), 8)}>
              <FlipHorizontal />
            </DockIconButton>
            <DockIconButton title="竖直翻转" disabled={!currentImage?.savedPath} onClick={() => runAction(() => flipCurrent(false), 8)}>
              <FlipVertical />
            </DockIconButton>
            <DockIconButton title="裁出选中矩形" disabled={!cropAction} onClick={() => cropAction && runAction(cropAction, 8)}>
              <Crop />
            </DockIconButton>
          </div>

          <div className="android-canvas-dock-group">
            <DockIconButton title={fullscreen ? "退出全屏" : "全屏"} onClick={() => runAction(toggleFullscreen, 8)}>
              {fullscreen ? <Minimize /> : <Maximize />}
            </DockIconButton>
            <DockIconButton title="详情" disabled={!hasImage} onClick={() => currentImage && runAction(() => openResultDetail(currentImage), 8)}>
              <Info />
            </DockIconButton>
            <DockIconButton title="另存为" disabled={!hasImage} onClick={() => runAction(saveCurrentImageAs, 8)}>
              <Save />
            </DockIconButton>
            <DockIconButton title="设为图生图源图" disabled={!hasImage} onClick={() => currentImage && runAction(() => reuseAsSource(currentImage), 8)}>
              <Scissors />
            </DockIconButton>
            <DockIconButton title="清空画板" disabled={!hasImage} danger onClick={() => runAction(() => setField("currentImage", null), 8)}>
              <Trash2 />
            </DockIconButton>
          </div>
        </div>
      </AndroidCanvasDock>
    </div>
  );
}

function AndroidCanvasEmptyState({
  onImport,
  onGoCompose,
}: {
  onImport: () => void;
  onGoCompose: () => void;
}) {
  return (
    <div className="android-canvas-empty">
      <div className="android-canvas-empty-icon">
        <ImagePlus />
      </div>
      <div className="android-canvas-empty-copy">
        <strong>还没有图片</strong>
        <span>先导入一张图，或回到参数页开始生成。</span>
      </div>
      <div className="android-canvas-empty-actions">
        <button type="button" className="primary" onClick={onImport}>
          <ImagePlus /> 从相册导入
        </button>
        <button type="button" onClick={onGoCompose}>
          去参数页
        </button>
      </div>
    </div>
  );
}

function AndroidCanvasHeader({
  currentImage,
  isRunning,
  progressLabel,
  streamPreviewActive,
  zoomLabel,
  batchCount,
  sourceCount,
  hasSources,
  sourceOpen,
  onToggleSources,
  onOpenGrid,
  gridOpen,
  progressMeta,
}: {
  currentImage: HistoryItem | null;
  isRunning: boolean;
  progressLabel: string;
  streamPreviewActive: boolean;
  zoomLabel: string;
  batchCount: number;
  sourceCount: number;
  hasSources: boolean;
  sourceOpen: boolean;
  onToggleSources: () => void;
  onOpenGrid?: () => void;
  gridOpen: boolean;
  progressMeta: {
    elapsed?: number;
    bytes?: number;
    runningJobs: number;
    jobsCompleted: number;
    jobsTotal: number;
  } | null;
}) {
  const prompt = currentImage?.revisedPrompt || currentImage?.prompt || "";
  return (
    <header className="android-canvas-header">
      <div className="android-canvas-status-dot" data-running={isRunning ? "true" : "false"}>
        {isRunning ? <Loader2 /> : currentImage ? <CheckCircle2 /> : <Square />}
      </div>
      <div className="android-canvas-header-copy">
        <div className="android-canvas-kicker">{progressLabel}</div>
        <div className="android-canvas-title">
          {currentImage ? (prompt || "当前图片") : "画布工作区"}
        </div>
      </div>
      <div className="android-canvas-header-actions">
        {streamPreviewActive ? (
          <button type="button" className="android-canvas-status-chip stream-preview-chip" title="流式预览">
            <StreamPreviewBadge compact />
          </button>
        ) : null}
        <button type="button" className="android-canvas-status-chip primary">
          {zoomLabel}
        </button>
        {onOpenGrid ? (
          <button
            type="button"
            className={`android-canvas-status-chip ${gridOpen ? "active" : ""}`}
            onClick={onOpenGrid}
          >
            {gridOpen ? "单图" : `${batchCount} 图`}
          </button>
        ) : null}
        {hasSources ? (
          <button
            type="button"
            className={`android-canvas-status-chip ${sourceOpen ? "active" : ""}`}
            onClick={onToggleSources}
          >
            参考 {sourceCount}
          </button>
        ) : null}
        {progressMeta && typeof progressMeta.elapsed === "number" ? (
          <span className="android-canvas-status-chip progress-meta-chip">{progressMeta.elapsed.toFixed(1)}s</span>
        ) : null}
        {progressMeta && typeof progressMeta.bytes === "number" && progressMeta.bytes > 0 ? (
          <span className="android-canvas-status-chip progress-meta-chip">{formatBytes(progressMeta.bytes)}</span>
        ) : null}
        {progressMeta && progressMeta.jobsTotal > 1 ? (
          <span className="android-canvas-status-chip progress-meta-chip">{progressMeta.runningJobs} 并发 · {progressMeta.jobsCompleted}/{progressMeta.jobsTotal}</span>
        ) : null}
      </div>
    </header>
  );
}

function AndroidToolSegment({
  value,
  disabled,
  onChange,
}: {
  value: CanvasTool;
  disabled: boolean;
  onChange: (value: CanvasTool) => void;
}) {
  return (
    <div className="android-canvas-tool-segment" aria-label="画布工具">
      <SegmentButton active={value === "pan"} disabled={disabled} label="移动" onClick={() => onChange("pan")}>
        <Hand />
      </SegmentButton>
      <SegmentButton active={value === "mask"} disabled={disabled} label="蒙版" onClick={() => onChange("mask")}>
        <Brush />
      </SegmentButton>
      <SegmentButton active={value === "annotate"} disabled={disabled} label="标注" onClick={() => onChange("annotate")}>
        <Square />
      </SegmentButton>
    </div>
  );
}

function AndroidMaskControls({
  brushMode,
  brushSize,
  onSetBrushMode,
  onSetBrushSize,
  onImportMask,
  onResetMask,
}: {
  brushMode: BrushMode;
  brushSize: number;
  onSetBrushMode: (value: BrushMode) => void;
  onSetBrushSize: (value: number) => void;
  onImportMask: () => void;
  onResetMask: () => void;
}) {
  return (
    <div className="android-canvas-context-card mask">
      <div className="android-canvas-context-row">
        <DockIconButton title="画笔" active={brushMode === "paint"} onClick={() => onSetBrushMode("paint")}>
          <Brush />
        </DockIconButton>
        <DockIconButton title="橡皮" active={brushMode === "erase"} onClick={() => onSetBrushMode("erase")}>
          <Eraser />
        </DockIconButton>
        <DockIconButton title="导入蒙版图片" onClick={onImportMask}>
          <Upload />
        </DockIconButton>
        <button type="button" className="android-canvas-text-action danger" onClick={onResetMask}>
          清空
        </button>
      </div>
      <label className="android-canvas-slider-row">
        <span>大小</span>
        <input
          type="range"
          min={5}
          max={120}
          value={brushSize}
          onChange={(e) => onSetBrushSize(Number(e.target.value))}
        />
        <strong>{brushSize}</strong>
      </label>
    </div>
  );
}

function AndroidAnnotationControls({
  annotationKind,
  annotationColor,
  onSetKind,
  onSetColor,
  onClear,
}: {
  annotationKind: AnnotationKind;
  annotationColor: string;
  onSetKind: (value: AnnotationKind) => void;
  onSetColor: (value: string) => void;
  onClear: () => void;
}) {
  return (
    <div className="android-canvas-context-card annotate">
      <div className="android-canvas-context-row">
        <DockIconButton title="矩形" active={annotationKind === "rect"} onClick={() => onSetKind("rect")}>
          <Square />
        </DockIconButton>
        <DockIconButton title="箭头" active={annotationKind === "arrow"} onClick={() => onSetKind("arrow")}>
          <ArrowRight />
        </DockIconButton>
        <DockIconButton title="自由画" active={annotationKind === "freehand"} onClick={() => onSetKind("freehand")}>
          <Pencil />
        </DockIconButton>
        <DockIconButton title="文字" active={annotationKind === "text"} onClick={() => onSetKind("text")}>
          <TypeIcon />
        </DockIconButton>
        <button type="button" className="android-canvas-text-action danger" onClick={onClear}>
          清空
        </button>
      </div>
      <div className="android-canvas-color-row" aria-label="标注颜色">
        {ANNOTATION_COLORS.map((color) => (
          <button
            key={color}
            type="button"
            title={color}
            className={annotationColor === color ? "active" : ""}
            style={{ backgroundColor: color }}
            onClick={() => onSetColor(color)}
          />
        ))}
      </div>
    </div>
  );
}

function AndroidSourceStrip({
  currentImage,
  sources,
  onAdd,
  onCompare,
  onPreview,
  onRemove,
  onMove,
  onClear,
}: {
  currentImage: HistoryItem | null;
  sources: SourceImage[];
  onAdd: () => void;
  onCompare: () => void;
  onPreview: (index: number) => void;
  onRemove: (index: number) => void;
  onMove: (from: number, to: number) => void;
  onClear: () => void;
}) {
  return (
    <div className="android-canvas-source-strip">
      <div className="android-canvas-source-head">
        <span>参考图</span>
        <div className="flex items-center gap-2">
          <button type="button" onClick={onCompare}>
            对比主参考
          </button>
          <button type="button" onClick={onClear}>
            清空
          </button>
        </div>
      </div>
      <div className="android-canvas-source-list">
        {sources.map((source, index) => (
          <AndroidSourceTile
            key={`${source.path}-${index}`}
            source={source}
            index={index}
            total={sources.length}
            active={currentImage?.savedPath === source.path}
            onPreview={onPreview}
            onRemove={onRemove}
            onMove={onMove}
          />
        ))}
        <button type="button" className="android-canvas-source-add" onClick={onAdd} title="添加参考图">
          <ImagePlus />
        </button>
      </div>
    </div>
  );
}

function AndroidSourceTile({
  source,
  index,
  total,
  active,
  onPreview,
  onRemove,
  onMove,
}: {
  source: SourceImage;
  index: number;
  total: number;
  active: boolean;
  onPreview: (index: number) => void;
  onRemove: (index: number) => void;
  onMove: (from: number, to: number) => void;
}) {
  const objectURL = useBlobURL(source.imageBlob ?? null, source.imageB64 ?? null);
  const previewURL = source.previewUrl || objectURL;
  return (
    <div
      className="android-canvas-source-tile"
      data-active={active ? "true" : "false"}
      title={`${source.name}\n点击在画布查看`}
      onClick={() => onPreview(index)}
    >
      <div className="android-canvas-source-preview">
        {previewURL ? <img src={previewURL} alt={source.name} loading="lazy" decoding="async" /> : <span>{source.name.split(".").pop()?.toUpperCase() ?? "IMG"}</span>}
      </div>
      <div className="android-canvas-source-index">{index + 1}</div>
      <button type="button" className="android-canvas-source-remove" onClick={(event) => { event.stopPropagation(); onRemove(index); }} title="移除">
        <X />
      </button>
      {total > 1 ? (
        <div className="android-canvas-source-move">
          <button type="button" disabled={index === 0} onClick={(event) => { event.stopPropagation(); onMove(index, index - 1); }}>
            <RotateCcw />
          </button>
          <button type="button" disabled={index === total - 1} onClick={(event) => { event.stopPropagation(); onMove(index, index + 1); }}>
            <RotateCw />
          </button>
        </div>
      ) : null}
    </div>
  );
}

function AndroidCanvasDock({ children }: { children: ReactNode }) {
  return <div className="android-canvas-dock">{children}</div>;
}

function SegmentButton({
  active,
  disabled,
  label,
  onClick,
  children,
}: {
  active: boolean;
  disabled: boolean;
  label: string;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button type="button" className={active ? "active" : ""} disabled={disabled} onClick={onClick} title={label}>
      <span>{children}</span>
      <small>{label}</small>
    </button>
  );
}

function DockIconButton({
  active,
  danger,
  disabled,
  title,
  onClick,
  children,
}: {
  active?: boolean;
  danger?: boolean;
  disabled?: boolean;
  title: string;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      className={`android-canvas-icon-button ${active ? "active" : ""} ${danger ? "danger" : ""}`}
      disabled={disabled}
      title={title}
      aria-label={title}
      onClick={onClick}
    >
      {children}
    </button>
  );
}

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}
