import type { HistoryItem } from "../../types/domain";
import { historyPreviewSrc, isGIFImage, useBlobURL } from "../../lib/images";
import { DragExportHandle } from "./DragExportHandle";

export type BatchGridSlot =
  | { type: "result"; item: HistoryItem }
  | { type: "preview"; item: HistoryItem }
  | { type: "pending"; id: string };

export function BatchResultGrid({
  items,
  slots,
  currentId,
  onSelect,
  onClose,
  showClose = true,
  title,
  selectedIds,
  onToggleSelect,
  selectionMode = false,
  livePreview = false,
}: {
  items: HistoryItem[];
  slots?: BatchGridSlot[];
  currentId: string | null;
  onSelect: (item: HistoryItem) => void | Promise<void>;
  onClose: () => void;
  showClose?: boolean;
  title?: string;
  selectedIds?: Set<string>;
  onToggleSelect?: (item: HistoryItem) => void;
  selectionMode?: boolean;
  livePreview?: boolean;
}) {
  const gridSlots = slots ?? items.map((item) => ({ type: "result", item }) satisfies BatchGridSlot);
  const singlePendingPreview = livePreview && gridSlots.length === 1 && gridSlots[0]?.type === "pending";
  const singleLivePreview = livePreview && gridSlots.length === 1 && gridSlots[0]?.type !== "pending";
  const columns = singleLivePreview || singlePendingPreview ? 1 : gridSlots.length <= 4 ? 2 : 3;
  const rows = Math.min(3, Math.ceil(Math.max(gridSlots.length, 1) / columns));
  const scrollGrid = !singleLivePreview && !singlePendingPreview && gridSlots.length > 9;
  const fillGrid = !singleLivePreview && !singlePendingPreview && !scrollGrid;

  if (singlePendingPreview) {
    return (
      <div className="batch-grid-overlay live-preview-grid single-pending">
        <div className="batch-grid-head">
          <span className="batch-grid-title">{title ?? `本批结果 · ${items.length} 张`}</span>
          {showClose ? (
            <button type="button" className="batch-grid-close" onClick={onClose} title="返回当前图">
              返回当前图
            </button>
          ) : null}
        </div>
        <div className="batch-grid-pending-stage" aria-label="等待第一张预览">
          <div className="batch-grid-pending-stage-card">
            <span className="batch-grid-pending-ring large" />
            <strong className="batch-grid-pending-stage-title">等待第一张预览</strong>
            <span className="batch-grid-pending-stage-note">收到首帧后，这里会自动切成实时预览画面。</span>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className={`batch-grid-overlay ${livePreview ? "live-preview-grid" : ""} ${singleLivePreview ? "single-slot" : ""}`}>
      <div className={`batch-grid-head ${singleLivePreview ? "single-slot" : ""}`}>
        <span className="batch-grid-title">{title ?? `本批结果 · ${items.length} 张`}</span>
        {showClose ? (
          <button type="button" className="batch-grid-close" onClick={onClose} title="返回当前图">
            返回当前图
          </button>
        ) : null}
      </div>
      <div
        className={`batch-grid ${singleLivePreview ? "single-slot" : ""} ${fillGrid ? "fill-grid" : ""} ${scrollGrid ? "scroll-grid" : ""}`}
        style={{
          gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
          ...(fillGrid ? { gridTemplateRows: `repeat(${rows}, minmax(0, 1fr))` } : {}),
          ...(scrollGrid ? { gridAutoRows: "clamp(180px, 24vh, 260px)" } : {}),
        }}
      >
        {gridSlots.map((slot, index) => {
          if (slot.type === "pending") {
            return <PendingGridTile key={slot.id} index={index} singleLayout={singleLivePreview} />;
          }
          return (
            <BatchGridTile
              key={slot.item.id}
              item={slot.item}
              index={index}
              active={slot.type === "result" && slot.item.id === currentId}
              preview={slot.type === "preview"}
              onSelect={onSelect}
              selected={slot.type === "result" && !!selectedIds?.has(slot.item.id)}
              onToggleSelect={onToggleSelect}
              selectionMode={selectionMode}
              singleLayout={singleLivePreview}
            />
          );
        })}
      </div>
    </div>
  );
}

function BatchGridTile({
  item,
  index,
  active,
  preview,
  onSelect,
  selected,
  onToggleSelect,
  selectionMode,
  singleLayout,
}: {
  item: HistoryItem;
  index: number;
  active: boolean;
  preview: boolean;
  onSelect: (item: HistoryItem) => void | Promise<void>;
  selected: boolean;
  onToggleSelect?: (item: HistoryItem) => void;
  selectionMode: boolean;
  singleLayout: boolean;
}) {
  const previewURL = useBlobURL(isGIFImage(item) ? item.imageBlob : item.imageBlob ?? item.previewBlob ?? null, item.imageB64 ?? null);
  const src = historyPreviewSrc(item, previewURL, true);
  return (
    <div
      className={`batch-grid-tile ${active ? "active" : ""} ${preview ? "previewing" : ""} ${selected ? "selected" : ""} ${selectionMode ? "selection-mode" : ""}`}
      title={item.prompt}
    >
      <button
        type="button"
        className={`batch-grid-tile-button ${singleLayout ? "single-layout" : ""}`}
        onClick={() => {
          if (selectionMode && !preview) {
            onToggleSelect?.(item);
            return;
          }
          if (!preview) void onSelect(item);
        }}
        disabled={preview}
      >
        <span className="batch-grid-media">
          <img
            src={src}
            alt={item.prompt || `batch result ${index + 1}`}
            loading="eager"
            decoding="async"
            draggable={false}
          />
        </span>
        {singleLayout ? (
          <span className="batch-grid-single-note">
            {preview ? "流式预览会持续刷新，最终结果生成后会自动替换。" : "当前结果已就绪，生成完成后可继续在画布中处理。"}
          </span>
        ) : null}
        <span className="batch-grid-index">{index + 1}</span>
        {selectionMode && !preview ? <span className="batch-grid-check">{selected ? "已选" : "未选"}</span> : null}
        {preview ? <span className="batch-grid-meta">预览中</span> : null}
        {!preview && item.elapsedSec ? <span className="batch-grid-meta">{item.elapsedSec}s</span> : null}
      </button>
      {!preview ? <DragExportHandle item={item} className="batch-grid-drag-export" /> : null}
    </div>
  );
}

function PendingGridTile({ index, singleLayout }: { index: number; singleLayout: boolean }) {
  return (
    <div className="batch-grid-tile pending" aria-label={`等待第 ${index + 1} 张预览`}>
      <span className="batch-grid-index">{index + 1}</span>
      <span className="batch-grid-pending-ring" />
      <span className="batch-grid-pending-label">{singleLayout ? "等待第一张预览" : "等待预览"}</span>
      {singleLayout ? <span className="batch-grid-single-note pending-note">收到首帧后，这里会自动切成实时预览画面。</span> : null}
    </div>
  );
}
