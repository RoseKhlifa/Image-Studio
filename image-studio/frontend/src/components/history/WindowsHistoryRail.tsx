import {
  ChevronDown, ChevronRight, Clock3, CopyPlus, Filter, Image as ImageIcon, Loader2,
  Search, Settings2, Split, Trash2,
} from "lucide-react";
import type { APIMode, HistoryItem } from "../../types/domain";
import { ContextMenu } from "../common/ContextMenu";
import { RawResponseModal } from "./RawResponseModal";
import type { DateFilter, ModeFilter } from "./HistoryRail";
import type { HistoryPromptEntry, HistoryPromptGroup } from "./historyPromptGroups";
import { HistoryTile } from "./HistoryTile";
import { WindowsHistoryPromptGroup } from "./WindowsHistoryPromptGroup";
import type { MenuItem } from "../common/ContextMenu";

export function WindowsHistoryRail({
  activeProfileId,
  apiKey,
  apiMode,
  baseURL,
  buildMenu,
  closeMenu,
  closeRaw,
  compareB,
  currentImage,
  dateF,
  deleteHistoryItem,
  filtered,
  generateCount,
  editCount,
  entries,
  history,
  historyHasMore,
  historyClearing,
  historyLoading,
  historyFiltersActive,
  historyRailCollapsed,
  isTestingKey,
  menu,
  modeF,
  openHistoryTimeline,
  openMenu,
  openUpstreamConfig,
  profiles,
  q,
  rawPath,
  reuseAsSource,
  selectCurrent,
  setActiveProfile,
  setCompareB,
  setDateF,
  setHistoryRailCollapsed,
  setModeF,
  setQ,
  testAPIKey,
  onClearAllHistory,
  onOpenPromptGroup,
}: {
  activeProfileId: string;
  apiKey: string;
  apiMode: APIMode;
  baseURL: string;
  buildMenu: (item: HistoryItem) => MenuItem[];
  closeMenu: () => void;
  closeRaw: () => void;
  compareB: HistoryItem | null;
  currentImage: HistoryItem | null;
  dateF: DateFilter;
  deleteHistoryItem: (id: string) => void | Promise<void>;
  filtered: HistoryItem[];
  generateCount: number;
  editCount: number;
  entries: HistoryPromptEntry[];
  history: HistoryItem[];
  historyHasMore: boolean;
  historyClearing: boolean;
  historyLoading: boolean;
  historyFiltersActive: boolean;
  historyRailCollapsed: boolean;
  isTestingKey: boolean;
  menu: { item: HistoryItem; x: number; y: number } | null;
  modeF: ModeFilter;
  openHistoryTimeline: () => void;
  openMenu: (item: HistoryItem, x: number, y: number) => void;
  openUpstreamConfig: (source?: "app" | "settings") => void;
  profiles: Array<{ id: string; name: string; apiMode: APIMode }>;
  q: string;
  rawPath: string | null;
  reuseAsSource: (item: HistoryItem) => void | Promise<void>;
  selectCurrent: (item: HistoryItem) => void | Promise<void>;
  setActiveProfile: (id: string) => void | Promise<void>;
  setCompareB: (item: HistoryItem | null) => void;
  setDateF: (value: DateFilter) => void;
  setHistoryRailCollapsed: (value: boolean) => void;
  setModeF: (value: ModeFilter) => void;
  setQ: (value: string) => void;
  testAPIKey: () => void | Promise<void>;
  onClearAllHistory: () => void;
  onOpenPromptGroup: (group: HistoryPromptGroup) => void;
}) {
  const latest = filtered[0] ?? null;
  const list = historyRailCollapsed ? [] : entries.slice(0, 18);
  const historyCountLabel = historyHasMore ? `${history.length}+` : `${history.length}`;

  return (
    <aside className="history-rail windows-history-rail box-border flex shrink-0 flex-col overflow-y-auto border-l border-[var(--border)] bg-[var(--inspector)]">
      <div className="windows-history-stack">
        <section className="platform-card windows-history-upstream">
          <div className="windows-history-card-head">
            <div className="windows-history-title-row">
              <span className="windows-history-title">上游</span>
              <span className={`windows-status-dot ${apiKey && baseURL ? "ready" : "missing"}`} />
              <span className={apiKey && baseURL ? "text-[var(--accent)]" : "text-red-400"}>
                {apiKey && baseURL ? "已配置" : "未配置"}
              </span>
            </div>
            <span className="windows-history-muted">当前连接</span>
          </div>

          {profiles.length > 0 ? (
            <select
              value={activeProfileId}
              onChange={(event) => {
                const id = event.target.value;
                if (id === "__manage__") {
                  openUpstreamConfig("app");
                  return;
                }
                if (id) void setActiveProfile(id);
              }}
              className="focus-ring windows-history-select"
              title="切换上游配置 / 管理"
            >
              {profiles.map((profile) => (
                <option key={profile.id} value={profile.id}>
                  {profile.name} · {profile.apiMode === "responses" ? "Responses" : "Images"}
                </option>
              ))}
              <option value="__manage__">管理配置...</option>
            </select>
          ) : (
            <p className="windows-history-description">还没有上游配置，先建一条再开始生成。</p>
          )}

          <div className="windows-history-actions">
            <button type="button" onClick={() => openUpstreamConfig("app")} className="platform-action-btn">
              上游配置
            </button>
            <button
              type="button"
              onClick={() => void testAPIKey()}
              disabled={!apiKey.trim() || !baseURL.trim() || isTestingKey}
              className="platform-action-btn"
            >
              {isTestingKey ? "检查中..." : "测试"}
            </button>
          </div>
          <span className="windows-history-api-mode">
            {apiMode === "responses" ? "Responses API" : "Images API"}
          </span>
        </section>

        <section className="platform-card windows-history-summary">
          <div className="windows-history-card-head">
            <div>
              <div className="windows-history-title">历史</div>
              <div className="windows-history-count">{filtered.length}{filtered.length !== history.length ? ` / ${historyCountLabel}` : historyHasMore ? "+" : ""} 项</div>
            </div>
            <div className="windows-history-card-actions">
              <button
                type="button"
                onClick={onClearAllHistory}
                className="platform-pill windows-history-clear"
                disabled={historyClearing || (history.length === 0 && !historyHasMore)}
                title="删除全部历史"
              >
                {historyClearing ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Trash2 className="h-3.5 w-3.5" />}
                全部删除
              </button>
              <button
                type="button"
                onClick={() => setHistoryRailCollapsed(!historyRailCollapsed)}
                className="platform-pill windows-history-collapse"
              >
                {historyRailCollapsed ? <ChevronRight className="h-3.5 w-3.5" /> : <ChevronDown className="h-3.5 w-3.5" />}
                {historyRailCollapsed ? "展开" : "折叠"}
              </button>
            </div>
          </div>

          <div className="windows-history-stats">
            <button type="button" className={modeF === "all" ? "active" : ""} onClick={() => setModeF("all")}>
              <ImageIcon className="h-3.5 w-3.5" /> 全部 <strong>{historyCountLabel}</strong>
            </button>
            <button type="button" className={modeF === "generate" ? "active" : ""} onClick={() => setModeF("generate")}>
              <CopyPlus className="h-3.5 w-3.5" /> 文生图 <strong>{generateCount}</strong>
            </button>
            <button type="button" className={modeF === "edit" ? "active" : ""} onClick={() => setModeF("edit")}>
              <Settings2 className="h-3.5 w-3.5" /> 图生图 <strong>{editCount}</strong>
            </button>
          </div>

          <label className="windows-history-search">
            <Search className="h-3.5 w-3.5" />
            <input value={q} onChange={(event) => setQ(event.target.value)} placeholder="搜索 prompt..." />
          </label>

          <div className="windows-history-filter-row">
            <button type="button" className={dateF === "all" ? "active" : ""} onClick={() => setDateF("all")}>全部</button>
            <button type="button" className={dateF === "today" ? "active" : ""} onClick={() => setDateF("today")}>今天</button>
            <button type="button" className={dateF === "week" ? "active" : ""} onClick={() => setDateF("week")}>本周</button>
          </div>
        </section>

        {compareB ? (
          <button type="button" onClick={() => setCompareB(null)} className="platform-pill windows-compare-exit">
            <Split className="h-3.5 w-3.5" /> 退出对比
          </button>
        ) : null}

        {!historyRailCollapsed && latest ? (
          <section className="platform-card windows-history-feature">
            <div className="windows-history-section-head">
              <span><Clock3 className="h-3.5 w-3.5" /> 最近作品</span>
              <button type="button" onClick={openHistoryTimeline}>完整历史</button>
            </div>
            <HistoryTile
              item={latest}
              isCurrent={currentImage?.id === latest.id}
              isCompare={compareB?.id === latest.id}
              onSelect={selectCurrent}
              onToggleCompare={(next) => setCompareB(next)}
              onReuse={reuseAsSource}
              onDelete={deleteHistoryItem}
              onOpenMenu={(x, y) => openMenu(latest, x, y)}
              variant="windowsFeature"
            />
          </section>
        ) : null}

        {!historyRailCollapsed ? (
          <section className="platform-card windows-history-results">
            <div className="windows-history-section-head">
              <span><Filter className="h-3.5 w-3.5" /> 结果</span>
              <span>{list.length}{entries.length > list.length ? ` / ${entries.length}` : ""}</span>
            </div>

            {list.length === 0 ? (
              <div className="windows-history-empty">
                {historyFiltersActive ? "没有匹配项" : "还没有结果"}
              </div>
            ) : (
              <div className="windows-history-list">
                {list.map((entry) => (
                  <WindowsHistoryEntry
                    key={entry.key}
                    entry={entry}
                    currentItemId={currentImage?.id ?? null}
                    compareItemId={compareB?.id ?? null}
                    onDelete={deleteHistoryItem}
                    onOpenMenu={openMenu}
                    onOpenPromptGroup={onOpenPromptGroup}
                    onReuse={reuseAsSource}
                    onSelect={selectCurrent}
                    onToggleCompare={(next) => setCompareB(next)}
                  />
                ))}
              </div>
            )}

            {historyLoading ? (
              <div className="windows-history-empty">
                <Loader2 className="h-4 w-4 animate-spin" />
                正在加载更多历史...
              </div>
            ) : null}

            {historyHasMore || entries.length > list.length ? (
              <button type="button" onClick={openHistoryTimeline} className="windows-history-more">
                查看更多历史
              </button>
            ) : null}
          </section>
        ) : null}
      </div>

      {menu ? <ContextMenu x={menu.x} y={menu.y} items={buildMenu(menu.item)} onClose={closeMenu} /> : null}
      {rawPath ? <RawResponseModal path={rawPath} onClose={closeRaw} /> : null}
    </aside>
  );
}

function WindowsHistoryEntry({
  compareItemId,
  currentItemId,
  entry,
  onDelete,
  onOpenMenu,
  onOpenPromptGroup,
  onReuse,
  onSelect,
  onToggleCompare,
}: {
  compareItemId: string | null;
  currentItemId: string | null;
  entry: HistoryPromptEntry;
  onDelete: (id: string) => void | Promise<void>;
  onOpenMenu: (item: HistoryItem, x: number, y: number) => void;
  onOpenPromptGroup: (group: HistoryPromptGroup) => void;
  onReuse: (item: HistoryItem) => void | Promise<void>;
  onSelect: (item: HistoryItem) => void | Promise<void>;
  onToggleCompare: (item: HistoryItem | null) => void;
}) {
  if (entry.kind === "group") {
    return (
      <WindowsHistoryPromptGroup
        group={entry.group}
        currentItemId={currentItemId}
        compareItemId={compareItemId}
        onOpenMenu={onOpenMenu}
        onOpenGroup={() => onOpenPromptGroup(entry.group)}
        onSelect={onSelect}
        onToggleCompare={onToggleCompare}
      />
    );
  }

  return (
    <WindowsHistoryRow
      item={entry.item}
      isCurrent={currentItemId === entry.item.id}
      isCompare={compareItemId === entry.item.id}
      onDelete={onDelete}
      onOpenMenu={(x, y) => onOpenMenu(entry.item, x, y)}
      onReuse={onReuse}
      onSelect={onSelect}
      onToggleCompare={onToggleCompare}
    />
  );
}

function WindowsHistoryRow({
  item,
  isCompare,
  isCurrent,
  onDelete,
  onOpenMenu,
  onReuse,
  onSelect,
  onToggleCompare,
}: {
  item: HistoryItem;
  isCompare: boolean;
  isCurrent: boolean;
  onDelete: (id: string) => void | Promise<void>;
  onOpenMenu: (x: number, y: number) => void;
  onReuse: (item: HistoryItem) => void | Promise<void>;
  onSelect: (item: HistoryItem) => void | Promise<void>;
  onToggleCompare: (item: HistoryItem | null) => void;
}) {
  return (
    <HistoryTile
      item={item}
      isCurrent={isCurrent}
      isCompare={isCompare}
      onSelect={onSelect}
      onToggleCompare={onToggleCompare}
      onReuse={onReuse}
      onDelete={onDelete}
      onOpenMenu={onOpenMenu}
      variant="windowsList"
    />
  );
}
