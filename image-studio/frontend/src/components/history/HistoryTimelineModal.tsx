import { useEffect, useMemo, useRef, useState } from "react";
import { CalendarDays, Loader2, Search } from "lucide-react";
import { Modal } from "../common/Modal";
import { useStudioStore } from "../../state/studioStore";
import { useStudioFields } from "../../state/useStudioFields";
import type { HistoryItem, Mode } from "../../types/domain";
import { usePlatform } from "../../platform/context";
import { ContextMenu } from "../common/ContextMenu";
import { RawResponseModal } from "./RawResponseModal";
import {
  historyDayKey,
  isHistoryInDateFilter,
  matchesHistorySearch,
  type TimelineHistoryDateFilter,
} from "./historyFilters";
import { TimelineHistoryItem } from "./TimelineHistoryItem";
import { TimelinePromptStackGroup } from "./TimelinePromptStackGroup";
import { buildHistoryPromptEntries, type HistoryPromptEntry } from "./historyPromptGroups";
import { useHistoryContextMenu } from "./useHistoryContextMenu";
import { toPreviewOnlyHistoryItem } from "../../state/studioStore.runtime";

type ModeFilter = "all" | Mode;
type DateFilter = TimelineHistoryDateFilter;

export function HistoryTimelineModal() {
  const {
    historyTimelineOpen,
    closeHistoryTimeline,
    history,
    historyHasMore,
    historyLoading,
    loadMoreHistory,
    setCompareB,
    deleteHistoryItem,
    reuseAsSource,
    materializeCurrentImage,
    setField,
    applyHistoryParams,
    regenerateFromHistory,
    openResultDetail,
    pushToast,
  } = useStudioFields([
    "historyTimelineOpen", "closeHistoryTimeline", "history", "historyHasMore",
    "historyLoading", "loadMoreHistory", "setCompareB", "deleteHistoryItem",
    "reuseAsSource", "materializeCurrentImage",
    "setField", "applyHistoryParams", "regenerateFromHistory", "openResultDetail",
    "pushToast",
  ]);
  const currentImageId = useStudioStore((state) => state.currentImage?.id ?? null);
  const compareItemId = useStudioStore((state) => state.compareB?.id ?? null);
  const { usesFluentUI } = usePlatform();
  const [query, setQuery] = useState("");
  const [modeFilter, setModeFilter] = useState<ModeFilter>("all");
  const [dateFilter, setDateFilter] = useState<DateFilter>("all");
  const [pickedDate, setPickedDate] = useState("");
  const [expandedPromptGroups, setExpandedPromptGroups] = useState<Set<string>>(() => new Set());
  const listRef = useRef<HTMLDivElement | null>(null);
  const {
    buildMenu,
    closeMenu,
    closeRaw,
    menu,
    openMenu,
    rawPath,
  } = useHistoryContextMenu({
    currentImageId,
    compareItemId,
    onOpenDetail: openResultDetail,
    onApplyParams: applyHistoryParams,
    onRegenerate: (item) => void regenerateFromHistory(item),
    onReuseAsSource: (item) => void reuseAsSource(item),
    onToggleCompare: (item) => setCompareB(compareItemId === item.id ? null : item),
    onDelete: (item) => void deleteHistoryItem(item.id),
    pushToast,
  });

  const filtered = useMemo(() => {
    return history.filter((item) => {
      if (modeFilter !== "all" && item.mode !== modeFilter) return false;
      if (!isHistoryInDateFilter(item.createdAt, dateFilter, pickedDate)) return false;
      return matchesHistorySearch(item, query);
    });
  }, [history, query, modeFilter, dateFilter, pickedDate]);

  const groups = useMemo(() => {
    const map = new Map<string, HistoryPromptEntry[]>();
    for (const entry of buildHistoryPromptEntries(filtered)) {
      const representative = entry.kind === "group" ? entry.group.representative : entry.item;
      const key = historyDayKey(representative.createdAt);
      const bucket = map.get(key) ?? [];
      bucket.push(entry);
      map.set(key, bucket);
    }
    return Array.from(map.entries());
  }, [filtered]);

  function togglePromptGroup(key: string) {
    setExpandedPromptGroups((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  async function selectHistory(item: HistoryItem) {
    setField("currentImage", toPreviewOnlyHistoryItem(item));
    closeHistoryTimeline();
    const full = await materializeCurrentImage(item).catch(() => null);
    if (full && useStudioStore.getState().currentImage?.id === item.id) {
      setField("currentImage", full.previewOnly ? toPreviewOnlyHistoryItem(full) : full);
    }
  }

  useEffect(() => {
    if (!historyTimelineOpen) return;
    if (query.trim() || modeFilter !== "all" || dateFilter !== "all") {
      void loadMoreHistory();
    }
  }, [dateFilter, historyTimelineOpen, historyHasMore, historyLoading, loadMoreHistory, modeFilter, query]);

  function handleScroll() {
    const node = listRef.current;
    if (!node || historyLoading || !historyHasMore) return;
    const distanceToBottom = node.scrollHeight - node.scrollTop - node.clientHeight;
    if (distanceToBottom < 240) {
      void loadMoreHistory();
    }
  }

  if (!historyTimelineOpen) return null;

  return (
    <Modal open onClose={closeHistoryTimeline} title="更多历史" width={920}>
      <div className="flex flex-col gap-4">
        <div className="grid grid-cols-[minmax(0,1fr)_140px_140px] gap-2">
          <label className={`flex items-center gap-2 border border-black/[0.08] bg-[var(--surface)] px-3 py-2.5 dark:border-white/[0.08] ${usesFluentUI ? "rounded-[10px]" : "rounded-[16px]"}`}>
            <Search className="h-3.5 w-3.5 text-zinc-400" />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="搜索 prompt / revised prompt..."
              className="min-w-0 flex-1 bg-transparent text-[12px] text-zinc-800 outline-none placeholder:text-zinc-400 dark:text-zinc-100"
            />
          </label>
          <select
            value={modeFilter}
            onChange={(e) => setModeFilter(e.target.value as ModeFilter)}
            className={`focus-ring border border-black/[0.08] bg-[var(--surface)] px-3 py-2.5 text-[12px] text-zinc-700 dark:border-white/[0.08] dark:text-zinc-300 ${usesFluentUI ? "rounded-[10px]" : "rounded-[16px]"}`}
          >
            <option value="all">全部模式</option>
            <option value="generate">文生图</option>
            <option value="edit">图生图</option>
          </select>
          <select
            value={dateFilter}
            onChange={(e) => setDateFilter(e.target.value as DateFilter)}
            className={`focus-ring border border-black/[0.08] bg-[var(--surface)] px-3 py-2.5 text-[12px] text-zinc-700 dark:border-white/[0.08] dark:text-zinc-300 ${usesFluentUI ? "rounded-[10px]" : "rounded-[16px]"}`}
          >
            <option value="all">全部日期</option>
            <option value="today">今天</option>
            <option value="week">近 7 天</option>
            <option value="pick">指定日期</option>
          </select>
        </div>

        {dateFilter === "pick" && (
          <input
            type="date"
            value={pickedDate}
            onChange={(e) => setPickedDate(e.target.value)}
            className={`focus-ring w-[220px] border border-black/[0.08] bg-[var(--surface)] px-3 py-2.5 text-[12px] text-zinc-700 dark:border-white/[0.08] dark:text-zinc-300 ${usesFluentUI ? "rounded-[10px]" : "rounded-[16px]"}`}
          />
        )}

        <div ref={listRef} className="max-h-[68vh] overflow-y-auto pr-1" onScroll={handleScroll}>
          {groups.length === 0 ? (
            <div className={`border border-dashed border-black/[0.08] py-12 text-center text-[13px] text-zinc-500 dark:border-white/[0.08] dark:text-zinc-300 ${usesFluentUI ? "rounded-[12px]" : "rounded-[20px]"}`}>
              没有匹配的历史记录
            </div>
          ) : (
            <div className="flex flex-col gap-5">
              {groups.map(([day, entries]) => (
                <section key={day} className="flex flex-col gap-3">
                  <div className="sticky top-0 z-10 -mx-1 flex items-center gap-2 bg-[var(--bg)]/90 px-1 py-1 backdrop-blur-sm">
                    <CalendarDays className="h-4 w-4 text-[var(--accent)]" />
                    <div className="text-[13px] font-semibold text-zinc-800 dark:text-zinc-100">{day}</div>
                    <div className="text-[11px] text-zinc-500 dark:text-zinc-400">{entries.length} 组</div>
                  </div>
                  <div className="flex flex-col gap-3">
                    {entries.map((entry) => (
                      <TimelineHistoryEntry
                        key={entry.key}
                        entry={entry}
                        currentItemId={currentImageId}
                        compareItemId={compareItemId}
                        expanded={entry.kind === "group" && expandedPromptGroups.has(entry.key)}
                        onSelect={(item) => void selectHistory(item)}
                        onDelete={(item) => void deleteHistoryItem(item.id)}
                        onReuse={(item) => void reuseAsSource(item)}
                        onToggleCompare={(item) => setCompareB(item && compareItemId !== item.id ? item : null)}
                        onOpenMenu={openMenu}
                        onToggleExpanded={() => togglePromptGroup(entry.key)}
                        usesFluentUI={usesFluentUI}
                      />
                    ))}
                  </div>
                </section>
              ))}
            </div>
          )}
          {historyLoading ? (
            <div className="flex items-center justify-center gap-2 py-4 text-[12px] text-zinc-500 dark:text-zinc-300">
              <Loader2 className="h-4 w-4 animate-spin" />
              正在加载更多历史...
            </div>
          ) : null}
          {historyHasMore && !historyLoading ? (
            <div className="flex justify-center py-3">
              <button
                type="button"
                onClick={() => void loadMoreHistory()}
                className={`inline-flex min-h-[34px] items-center justify-center rounded-full border border-black/[0.08] bg-[var(--surface)] px-4 text-[12px] font-medium text-zinc-600 transition-colors hover:border-[color:var(--accent)]/35 hover:text-[var(--accent)] dark:border-white/[0.08] dark:text-zinc-300 ${usesFluentUI ? "rounded-[10px]" : "rounded-full"}`}
              >
                加载更多历史
              </button>
            </div>
          ) : null}
        </div>
      </div>
      {menu ? (
        <ContextMenu
          x={menu.x}
          y={menu.y}
          items={buildMenu(menu.item)}
          onClose={closeMenu}
        />
      ) : null}
      {rawPath ? <RawResponseModal path={rawPath} onClose={closeRaw} /> : null}
    </Modal>
  );
}

function TimelineHistoryEntry({
  compareItemId,
  currentItemId,
  entry,
  expanded,
  onDelete,
  onOpenMenu,
  onReuse,
  onSelect,
  onToggleCompare,
  onToggleExpanded,
  usesFluentUI,
}: {
  compareItemId: string | null;
  currentItemId: string | null;
  entry: HistoryPromptEntry;
  expanded: boolean;
  onDelete: (item: HistoryItem) => void;
  onOpenMenu: (item: HistoryItem, x: number, y: number) => void;
  onReuse: (item: HistoryItem) => void;
  onSelect: (item: HistoryItem) => void;
  onToggleCompare: (item: HistoryItem | null) => void;
  onToggleExpanded: () => void;
  usesFluentUI: boolean;
}) {
  if (entry.kind === "item") {
    const item = entry.item;
    return (
      <TimelineHistoryItem
        item={item}
        isCurrent={currentItemId === item.id}
        isCompare={compareItemId === item.id}
        onSelect={() => onSelect(item)}
        onDelete={() => onDelete(item)}
        onReuse={() => onReuse(item)}
        onToggleCompare={() => onToggleCompare(item)}
        onOpenMenu={(x, y) => onOpenMenu(item, x, y)}
      />
    );
  }

  return (
    <TimelinePromptStackGroup
      group={entry.group}
      currentItemId={currentItemId}
      compareItemId={compareItemId}
      expanded={expanded}
      onSelect={onSelect}
      onReuse={onReuse}
      onToggleCompare={onToggleCompare}
      onOpenMenu={onOpenMenu}
      onToggleExpanded={onToggleExpanded}
      usesFluentUI={usesFluentUI}
    />
  );
}
