import type { HistoryItem } from "../../types/domain";
import {
  buildHistoryItemDragExport,
  shouldUseNativeFileDrag,
  writeHistoryItemFileDragData,
} from "../../lib/dragExport.ts";
import { useNativeFileDrag } from "../common/useNativeFileDrag";
import { usePlatform } from "../../platform/context";

export function DragExportHandle({
  item,
  className = "",
  sourceURL,
}: {
  item: HistoryItem;
  className?: string;
  sourceURL?: string | null;
}) {
  const { targetPlatform } = usePlatform();
  const nativeDrag = useNativeFileDrag(item.savedPath);
  const spec = buildHistoryItemDragExport(item, sourceURL);
  if (!spec) return null;

  const classes = ["image-drag-export", className].filter(Boolean).join(" ");
  const label = "拖到文件夹复制";

  return (
    <a
      href={spec.href}
      download={spec.fileName}
      draggable={!nativeDrag.native}
      className={classes}
      title={`${label} · ${spec.fileName}`}
      aria-label={`${label} · ${spec.fileName}`}
      onMouseDown={(event) => {
        event.stopPropagation();
        nativeDrag.onMouseDown(event);
      }}
      onClick={(event) => {
        event.preventDefault();
        event.stopPropagation();
      }}
      onDragStart={(event) => {
        event.stopPropagation();
        if (shouldUseNativeFileDrag(targetPlatform, item.savedPath)) {
          event.preventDefault();
          return;
        }
        event.dataTransfer.effectAllowed = "copy";
        writeHistoryItemFileDragData(event.dataTransfer, item, spec);
      }}
    >
      拖出复制
    </a>
  );
}
