import type React from "react";
import { Clock3, Ellipsis, Images, Layers3 } from "lucide-react";
import { Modal } from "../common/Modal";
import { historyPreviewSrc, useBlobURL } from "../../lib/images";
import {
  buildHistoryItemDragExport,
  shouldUseNativeFileDrag,
  writeImageFileDragData,
  writeInternalHistoryItemDragData,
} from "../../lib/dragExport.ts";
import { BeginNativeFileDrag } from "../../platform/runtime/host";
import type { HistoryItem } from "../../types/domain";
import { usePlatform } from "../../platform/context";
import { HistoryModeBadge } from "./HistoryModeBadge";
import { HistoryPromptThumbnailStack } from "./HistoryPromptThumbnailStack";
import type { HistoryPromptGroup } from "./historyPromptGroups";
import { historyPromptGroupContains, historyPromptGroupLabel } from "./historyPromptGroups";
import { qualityLabel, sizeLabel } from "./historyLabels";

export function HistoryPromptGroupModal({
  compareItemId,
  currentItemId,
  group,
  onClose,
  onOpenMenu,
  onReuse,
  onSelect,
  onToggleCompare,
}: {
  compareItemId: string | null;
  currentItemId: string | null;
  group: HistoryPromptGroup | null;
  onClose: () => void;
  onOpenMenu: (item: HistoryItem, x: number, y: number) => void;
  onReuse: (item: HistoryItem) => void | Promise<void>;
  onSelect: (item: HistoryItem) => void | Promise<void>;
  onToggleCompare: (item: HistoryItem | null) => void;
}) {
  const { isAndroidPhone, usesFluentUI } = usePlatform();
  if (!group) return null;

  const latest = group.representative;
  const label = historyPromptGroupLabel(group);
  const isCurrentGroup = historyPromptGroupContains(group, currentItemId);
  const isCompareGroup = historyPromptGroupContains(group, compareItemId);

  function openLatestMenu(event: React.MouseEvent) {
    event.preventDefault();
    event.stopPropagation();
    onOpenMenu(latest, event.clientX, event.clientY);
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="同提示词历史"
      width={isAndroidPhone ? 390 : 760}
      cardClassName="history-prompt-modal-card"
      bodyClassName="history-prompt-modal-body"
    >
      <div className={`history-prompt-modal ${usesFluentUI ? "fluent" : ""}`}>
        <section className={`history-prompt-modal-hero ${isCurrentGroup ? "active" : ""} ${isCompareGroup ? "compare" : ""}`}>
          <HistoryPromptThumbnailStack items={group.items} className="history-prompt-modal-pile" />
          <div className="history-prompt-modal-copy">
            <span className="history-prompt-modal-kicker">
              <Layers3 className="h-3.5 w-3.5" />
              同提示词
              <span>{group.items.length} 张</span>
            </span>
            <strong>{label}</strong>
            <div className="history-prompt-modal-meta">
              <span><Clock3 className="h-3.5 w-3.5" /> {new Date(latest.createdAt).toLocaleString()}</span>
              <span>{sizeLabel(latest.size)}</span>
              <span>{qualityLabel(latest.quality)}</span>
            </div>
            <div className="history-prompt-modal-actions">
              <button type="button" className="history-prompt-modal-action-primary" onClick={() => void onSelect(latest)}>查看最新</button>
              <button type="button" className="history-prompt-modal-action-secondary" onClick={openLatestMenu}>
                <Ellipsis className="h-3.5 w-3.5" />
                更多
              </button>
            </div>
          </div>
        </section>

        <div className="history-prompt-modal-grid" aria-label="同提示词缩略图">
          {group.items.map((item, index) => (
            <HistoryPromptModalThumbnail
              key={item.id}
              item={item}
              index={index}
              isCurrent={currentItemId === item.id}
              isCompare={compareItemId === item.id}
              onOpenMenu={(x, y) => onOpenMenu(item, x, y)}
              onReuse={onReuse}
              onSelect={onSelect}
              onToggleCompare={onToggleCompare}
            />
          ))}
        </div>
      </div>
    </Modal>
  );
}

function HistoryPromptModalThumbnail({
  index,
  isCompare,
  isCurrent,
  item,
  onOpenMenu,
  onReuse,
  onSelect,
  onToggleCompare,
}: {
  index: number;
  isCompare: boolean;
  isCurrent: boolean;
  item: HistoryItem;
  onOpenMenu: (x: number, y: number) => void;
  onReuse: (item: HistoryItem) => void | Promise<void>;
  onSelect: (item: HistoryItem) => void | Promise<void>;
  onToggleCompare: (item: HistoryItem | null) => void;
}) {
  const previewURL = useBlobURL(item.previewBlob ?? item.imageBlob ?? null, item.imageB64 ?? null);
  const imageSrc = historyPreviewSrc(item, previewURL);
  const displayIndex = typeof item.batchIndex === "number" ? item.batchIndex + 1 : index + 1;
  const dragSpec = buildHistoryItemDragExport(item);
  const { targetPlatform } = usePlatform();

  function openMenu(event: React.MouseEvent) {
    event.preventDefault();
    event.stopPropagation();
    onOpenMenu(event.clientX, event.clientY);
  }

  function handleSelect(event: React.MouseEvent) {
    if (event.button === 2 || event.ctrlKey) return;
    if (event.shiftKey) {
      onToggleCompare(isCompare ? null : item);
      return;
    }
    void onSelect(item);
  }

  function handleDragStart(event: React.DragEvent<HTMLButtonElement>) {
    if (!dragSpec) {
      event.preventDefault();
      return;
    }
    event.stopPropagation();
    if (shouldUseNativeFileDrag(targetPlatform, item.savedPath)) {
      event.preventDefault();
      void BeginNativeFileDrag(item.savedPath).catch((error) => {
        console.error("[drag-export] native-file-drag failed", error);
      });
      return;
    }
    event.dataTransfer.effectAllowed = "copy";
    writeInternalHistoryItemDragData(event.dataTransfer, item);
    writeImageFileDragData(event.dataTransfer, dragSpec);
  }

  return (
    <button
      type="button"
      className={`history-prompt-modal-thumb ${isCurrent ? "active" : ""} ${isCompare ? "compare" : ""}`}
      title={item.prompt}
      draggable={!!dragSpec}
      onClick={handleSelect}
      onDoubleClick={() => void onReuse(item)}
      onContextMenu={openMenu}
      onDragStart={handleDragStart}
    >
      <img src={imageSrc} alt={item.prompt} loading="eager" decoding="async" draggable={false} />
      <HistoryModeBadge mode={item.mode} className="history-prompt-modal-thumb-mode" />
      <span className="history-prompt-modal-thumb-index">#{displayIndex}</span>
      {isCompare ? <span className="history-prompt-modal-thumb-compare">B</span> : null}
      <span
        className="history-prompt-modal-thumb-menu"
        onClick={openMenu}
        onContextMenu={openMenu}
        title="更多"
      >
        <Ellipsis className="h-3.5 w-3.5" />
      </span>
    </button>
  );
}
