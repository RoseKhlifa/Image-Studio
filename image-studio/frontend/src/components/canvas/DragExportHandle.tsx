import type { HistoryItem } from "../../types/domain";
import {
  buildHistoryItemDragExport,
  shouldUseNativeFileDrag,
  writeImageFileDragData,
  writeInternalHistoryItemDragData,
} from "../../lib/dragExport.ts";
import { BeginNativeFileDrag } from "../../platform/runtime/host";
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
  const spec = buildHistoryItemDragExport(item, sourceURL);
  if (!spec) return null;

  const classes = ["image-drag-export", className].filter(Boolean).join(" ");
  const label = "拖到文件夹复制";

  return (
    <a
      href={spec.href}
      download={spec.fileName}
      draggable
      className={classes}
      title={`${label} · ${spec.fileName}`}
      aria-label={`${label} · ${spec.fileName}`}
      onMouseDown={(event) => event.stopPropagation()}
      onClick={(event) => {
        event.preventDefault();
        event.stopPropagation();
      }}
      onDragStart={(event) => {
        event.stopPropagation();
        if (shouldUseNativeFileDrag(targetPlatform, item.savedPath)) {
          event.preventDefault();
          console.debug("[drag-export] native-file-drag", item.savedPath);
          void BeginNativeFileDrag(item.savedPath).catch((error) => {
            console.error("[drag-export] native-file-drag failed", error);
          });
          return;
        }
        event.dataTransfer.effectAllowed = "copy";
        writeInternalHistoryItemDragData(event.dataTransfer, item);
        writeImageFileDragData(event.dataTransfer, spec);
      }}
    >
      拖出复制
    </a>
  );
}
