import { Images, Layers3 } from "lucide-react";
import type React from "react";
import type { HistoryItem } from "../../types/domain";
import { HistoryPromptThumbnailStack } from "./HistoryPromptThumbnailStack";
import type { HistoryPromptGroup } from "./historyPromptGroups";
import { historyPromptGroupContains, historyPromptGroupIndexSummary, historyPromptGroupLabel } from "./historyPromptGroups";
import { qualityLabel, sizeLabel } from "./historyLabels";

export function HistoryPromptGroupCard({
  compareItemId,
  currentItemId,
  group,
  onOpenMenu,
  onOpenGroup,
  onSelect,
  onToggleCompare,
}: {
  compareItemId: string | null;
  currentItemId: string | null;
  group: HistoryPromptGroup;
  onOpenMenu: (item: HistoryItem, x: number, y: number) => void;
  onOpenGroup: () => void;
  onSelect: (item: HistoryItem) => void | Promise<void>;
  onToggleCompare: (item: HistoryItem | null) => void;
}) {
  const latest = group.representative;
  const isCurrentGroup = historyPromptGroupContains(group, currentItemId);
  const isCompareGroup = historyPromptGroupContains(group, compareItemId);
  const label = historyPromptGroupLabel(group);

  function handleSummaryClick(event: React.MouseEvent) {
    if (event.button === 2 || event.ctrlKey) return;
    if (event.shiftKey) {
      onToggleCompare(isCompareGroup ? null : latest);
      return;
    }
    void onSelect(latest);
  }

  function openGroup(event: React.MouseEvent) {
    event.preventDefault();
    event.stopPropagation();
    onOpenGroup();
  }

  function openLatestMenu(event: React.MouseEvent) {
    event.preventDefault();
    event.stopPropagation();
    onOpenMenu(latest, event.clientX, event.clientY);
  }

  return (
    <section className={`history-prompt-group ${isCurrentGroup ? "active" : ""} ${isCompareGroup ? "compare" : ""}`}>
      <button
        type="button"
        className="history-prompt-group-summary"
        title={label}
        onClick={handleSummaryClick}
        onContextMenu={openLatestMenu}
      >
        <span className="history-prompt-group-pile-button" onClick={openGroup} title="展开同提示词结果">
          <HistoryPromptThumbnailStack items={group.items} className="history-prompt-group-pile" />
        </span>
        <span className="history-prompt-group-copy">
          <span className="history-prompt-group-kicker">
            <Layers3 className="h-3.5 w-3.5" />
            同提示词
            <span>{group.items.length} 张</span>
            <span>{historyPromptGroupIndexSummary(group)}</span>
          </span>
          <strong>{label}</strong>
          <small>{sizeLabel(latest.size)} · {qualityLabel(latest.quality)}</small>
        </span>
      </button>
      <button
        type="button"
        className="history-prompt-group-expand"
        onClick={openGroup}
        title="展开同提示词结果"
      >
        <Images className="h-3.5 w-3.5" />
      </button>
    </section>
  );
}
