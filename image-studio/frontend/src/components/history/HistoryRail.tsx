import { Suspense, lazy, useDeferredValue, useEffect, useMemo, useRef, useState } from "react";
import {
  ChevronDown, ChevronRight, Clock3, CopyPlus, Filter, GalleryVerticalEnd,
  Image as ImageIcon, ListFilter, Loader2, RotateCcw, Search, Settings2, Split, Trash2,
} from "lucide-react";
import { useStudioStore } from "../../state/studioStore";
import type { HistoryItem, Mode } from "../../types/domain";
import { ContextMenu } from "../common/ContextMenu";
import { RawResponseModal } from "./RawResponseModal";
import { usePlatform } from "../../platform/context";
import { AndroidHistoryActionSheet } from "../../platform/android/history/AndroidHistoryActionSheet";
import { AndroidHistoryPromptGroup } from "../../platform/android/history/AndroidHistoryPromptGroup";
import { AndroidHistoryTile } from "../../platform/android/history/AndroidHistoryTile";
import {
  isHistoryInDateFilter,
  matchesHistorySearch,
  type RelativeHistoryDateFilter,
} from "./historyFilters";
import { HistoryPromptGroupCard } from "./HistoryPromptGroupCard";
import { buildHistoryPromptEntries, type HistoryPromptGroup } from "./historyPromptGroups";
import { HistoryPromptGroupModal } from "./HistoryPromptGroupModal";
import { HistoryTile } from "./HistoryTile";
import { useHistoryContextMenu } from "./useHistoryContextMenu";
import { WindowsHistoryRail } from "./WindowsHistoryRail";
import { qualityLabel, sizeLabel } from "./historyLabels";
import { toPreviewOnlyHistoryItem } from "../../state/studioStore.runtime";

export type ModeFilter = "all" | Mode;
export type DateFilter = RelativeHistoryDateFilter;

export function HistoryRail() {
  const {
    history, currentImage, reuseAsSource, deleteHistoryItem, clearHistory, setField,
    compareB, setCompareB, pushToast, fullscreen,
    applyHistoryParams, regenerateFromHistory,
    openResultDetail, apiKey, baseURL, apiMode,
    profiles, activeProfileId, setActiveProfile,
    openUpstreamConfig, openHistoryTimeline, testAPIKey, isTestingKey,
    historyRailCollapsed, historyHasMore, historyLoading, loadMoreHistory, setHistoryRailCollapsed,
  } = useStudioStore();

  const [q, setQ] = useState("");
  const deferredQ = useDeferredValue(q);
  const [modeF, setModeF] = useState<ModeFilter>("all");
  const [dateF, setDateF] = useState<DateFilter>("all");
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [historyClearing, setHistoryClearing] = useState(false);
  const [activePromptGroup, setActivePromptGroup] = useState<HistoryPromptGroup | null>(null);
  const { isAndroidPhone, isAndroidPad, isMac, isWindows, usesFluentUI, usesAndroidUI, usesAppleUI } = usePlatform();
  // 防快速连点产生竞态:每次点击递增 epoch,后台 materialize 全图 resolve
  // 时跟当前 epoch 比对,过时的就丢弃。之前的写法是先 await 再 setField,
  // 慢的请求会在用户已经点了另一张图之后把画布盖回去。
  const selectEpochRef = useRef(0);

  const filtered = useMemo(() => {
    return history.filter((h) => {
      if (modeF !== "all" && h.mode !== modeF) return false;
      if (!isHistoryInDateFilter(h.createdAt, dateF)) return false;
      return matchesHistorySearch(h, deferredQ);
    });
  }, [history, deferredQ, modeF, dateF]);
  const promptEntries = useMemo(() => buildHistoryPromptEntries(filtered), [filtered]);
  const visibleDesktopEntries = isWindows ? promptEntries : promptEntries.slice(0, 6);
  const desktopHistoryCollapsed = !isWindows && historyRailCollapsed;
  const androidHistoryEntries = promptEntries.slice(0, isAndroidPad ? 48 : 24);
  const latestHistory = filtered[0] ?? null;
  const generateCount = history.filter((item) => item.mode === "generate").length;
  const editCount = history.length - generateCount;
  const desktopFilterThreshold = isMac ? 8 : 4;
  const showHistoryFilters = !isMac && (history.length > desktopFilterThreshold || q.trim().length > 0 || modeF !== "all" || dateF !== "all");
  const historyFiltersActive = q.trim().length > 0 || modeF !== "all" || dateF !== "all";
  const showPhoneFilterToggle = isAndroidPhone && (history.length > 4 || historyFiltersActive);
  const showFilterControls = !isAndroidPhone ? showHistoryFilters : (filtersOpen || historyFiltersActive);
  const historyCountLabel = historyHasMore ? `${history.length}+` : `${history.length}`;

  useEffect(() => {
    if (!historyHasMore || historyLoading) return;
    if (historyFiltersActive) {
      void loadMoreHistory();
    }
  }, [historyFiltersActive, historyHasMore, historyLoading, loadMoreHistory]);

  async function clearAllHistory() {
    if (historyClearing) return;
    if (history.length === 0 && !historyHasMore) {
      pushToast("当前没有可删除的历史", "info");
      return;
    }
    if (!window.confirm("确定删除全部历史记录吗?\n\n此操作会清空本地数据库中的所有历史，且无法撤销。")) return;
    setHistoryClearing(true);
    try {
      const removed = await clearHistory();
      setActivePromptGroup(null);
      pushToast(removed > 0 ? `已删除全部 ${removed} 条历史` : "已清空全部历史", "success");
    } catch (error) {
      pushToast(`清空历史失败:${error instanceof Error ? error.message : String(error)}`, "error", 5000);
    } finally {
      setHistoryClearing(false);
    }
  }

  async function selectCurrent(h: HistoryItem) {
    const myEpoch = ++selectEpochRef.current;
    // 2) 关键:从历史栏选图 = 显式单图选择,退出批量结果网格 overlay。否则
    //    刚生成完 9 张批量,grid 一直罩在画板上,用户在历史栏怎么点都只是
    //    切 grid 里的高亮项,视觉上像「卡在第一张」。grid 可以从工具栏的
    //    openResultGrid 重新打开。
    if (useStudioStore.getState().resultGridOpen) {
      useStudioStore.getState().closeResultGrid();
    }
    const previewItem = toPreviewOnlyHistoryItem(h);
    setField("currentImage", previewItem);
    try {
      const full = await useStudioStore.getState().materializeCurrentImage?.(h);
      if (selectEpochRef.current === myEpoch && full && useStudioStore.getState().currentImage?.id === h.id) {
        setField("currentImage", full.previewOnly ? toPreviewOnlyHistoryItem(full) : full);
      }
    } catch {
      // 读不出来就维持当前状态,用户可以再点一次
    }
  }

  const {
    buildMenu,
    closeMenu,
    closeRaw,
    menu,
    openMenu,
    rawPath,
  } = useHistoryContextMenu({
    currentImageId: currentImage?.id ?? null,
    compareItemId: compareB?.id ?? null,
    onOpenDetail: openResultDetail,
    onApplyParams: applyHistoryParams,
    onRegenerate: (item) => void regenerateFromHistory(item),
    onReuseAsSource: (item) => void reuseAsSource(item),
    onToggleCompare: (item) => setCompareB(compareB?.id === item.id ? null : item),
    onDelete: (item) => {
      if (window.confirm(`确定删除此历史项?\n\n${item.prompt?.slice(0, 60) || "(无 prompt)"}`)) {
        deleteHistoryItem(item.id);
      }
    },
    pushToast,
  });

  if (fullscreen) return null;

  if (isWindows) {
    return (
      <>
        <WindowsHistoryRail
          activeProfileId={activeProfileId}
          apiKey={apiKey}
          apiMode={apiMode}
          baseURL={baseURL}
          buildMenu={buildMenu}
          closeMenu={closeMenu}
          closeRaw={closeRaw}
          compareB={compareB}
          currentImage={currentImage}
          dateF={dateF}
          deleteHistoryItem={deleteHistoryItem}
          editCount={editCount}
          entries={promptEntries}
          filtered={filtered}
          generateCount={generateCount}
          history={history}
          historyHasMore={historyHasMore}
          historyLoading={historyLoading}
          historyClearing={historyClearing}
          historyFiltersActive={historyFiltersActive}
          historyRailCollapsed={historyRailCollapsed}
          isTestingKey={isTestingKey}
          menu={menu}
          modeF={modeF}
          openHistoryTimeline={openHistoryTimeline}
          openMenu={openMenu}
          openUpstreamConfig={openUpstreamConfig}
          profiles={profiles}
          q={q}
          rawPath={rawPath}
          reuseAsSource={reuseAsSource}
          selectCurrent={selectCurrent}
          setActiveProfile={setActiveProfile}
          setCompareB={setCompareB}
          setDateF={setDateF}
          setHistoryRailCollapsed={setHistoryRailCollapsed}
          setModeF={setModeF}
          setQ={setQ}
          testAPIKey={testAPIKey}
          onOpenPromptGroup={setActivePromptGroup}
          onClearAllHistory={() => void clearAllHistory()}
        />
        <HistoryPromptGroupModal
          group={activePromptGroup}
          currentItemId={currentImage?.id ?? null}
          compareItemId={compareB?.id ?? null}
          onClose={() => setActivePromptGroup(null)}
          onSelect={(item) => void selectCurrent(item)}
          onReuse={reuseAsSource}
          onToggleCompare={(next) => setCompareB(next)}
          onOpenMenu={(item, x, y) => openMenu(item, x, y)}
        />
      </>
    );
  }

  if (isAndroidPhone || isAndroidPad) {
    return (
      <aside
        className={`history-rail android-history-page ${isAndroidPad ? "android-history-page-pad" : ""} box-border flex w-full shrink-0 flex-col overflow-y-auto border-0 bg-[var(--bg)]`}
        data-android-history-layout={isAndroidPad ? "pad" : "phone"}
      >
        <section className="android-history-hero">
          <div>
            <div className="android-history-kicker">本地图库</div>
            <h2>历史作品</h2>
            <p>按时间回看生成结果，直接复用参数、设为源图或继续变体。</p>
          </div>
          <div className="android-history-total">
            <span>{historyCountLabel}</span>
            <small>张</small>
          </div>
        </section>

        <section className="android-history-stats" aria-label="历史统计">
          <button
            type="button"
            className={`android-history-stat ${modeF === "all" ? "active" : ""}`}
            onClick={() => setModeF("all")}
          >
            <ImageIcon className="h-4 w-4" />
            <span>全部</span>
            <strong>{historyCountLabel}</strong>
          </button>
          <button
            type="button"
            className={`android-history-stat ${modeF === "generate" ? "active" : ""}`}
            onClick={() => setModeF("generate")}
          >
            <CopyPlus className="h-4 w-4" />
            <span>文生图</span>
            <strong>{generateCount}</strong>
          </button>
          <button
            type="button"
            className={`android-history-stat ${modeF === "edit" ? "active" : ""}`}
            onClick={() => setModeF("edit")}
          >
            <Settings2 className="h-4 w-4" />
            <span>图生图</span>
            <strong>{editCount}</strong>
          </button>
        </section>

        <section className="android-history-filter-card">
          <label className="android-history-search">
            <Search className="h-4 w-4" />
            <input
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="搜索提示词 / 优化后提示词"
            />
          </label>
          <div className="android-history-filter-row">
            <button
              type="button"
              className={dateF === "all" ? "active" : ""}
              onClick={() => setDateF("all")}
            >
              全部日期
            </button>
            <button
              type="button"
              className={dateF === "today" ? "active" : ""}
              onClick={() => setDateF("today")}
            >
              今天
            </button>
            <button
              type="button"
              className={dateF === "week" ? "active" : ""}
              onClick={() => setDateF("week")}
            >
              本周
            </button>
          </div>
        </section>

        {compareB ? (
          <button
            type="button"
            onClick={() => setCompareB(null)}
            className="android-history-compare-exit"
          >
            <Split className="h-4 w-4" /> 退出对比
          </button>
        ) : null}

        {history.length > 0 && latestHistory ? (
          <section className="android-history-feature-card">
            <div className="android-history-section-head">
              <span><Clock3 className="h-4 w-4" /> 最近作品</span>
              <small>{new Date(latestHistory.createdAt).toLocaleDateString()}</small>
            </div>
            <div className="android-history-feature">
              <AndroidHistoryTile
                item={latestHistory}
                isCurrent={currentImage?.id === latestHistory.id}
                isCompare={compareB?.id === latestHistory.id}
                onSelect={selectCurrent}
                onToggleCompare={(next) => setCompareB(next)}
                onOpenMenu={(x, y) => openMenu(latestHistory, x, y)}
                variant="feature"
              />
              <button
                type="button"
                className="android-history-feature-copy"
                onClick={() => void selectCurrent(latestHistory)}
              >
                <strong>{latestHistory.prompt || "(无 prompt)"}</strong>
                <span>{sizeLabel(latestHistory.size)} · {qualityLabel(latestHistory.quality)} · {latestHistory.mode === "edit" ? "图生图" : "文生图"}</span>
              </button>
            </div>
          </section>
        ) : null}

        <section className="android-history-results-card">
          <div className="android-history-section-head">
            <span><ListFilter className="h-4 w-4" /> 结果</span>
            <small>{filtered.length}{filtered.length !== history.length ? ` / ${historyCountLabel}` : historyHasMore ? "+" : ""}</small>
          </div>

          {androidHistoryEntries.length === 0 ? (
            <div className="android-history-empty">
              <div className="android-history-empty-icon"><ImageIcon className="h-5 w-5" /></div>
              <strong>{historyFiltersActive ? "没有匹配项" : "还没有历史结果"}</strong>
              <span>{historyFiltersActive ? "换个关键词或清除筛选条件。" : "生成后的图片会自动出现在这里。"}</span>
            </div>
          ) : (
            <div className="android-history-grid">
              {androidHistoryEntries.map((entry) => {
                if (entry.kind === "group") {
                  return (
                    <AndroidHistoryPromptGroup
                      key={entry.key}
                      group={entry.group}
                      currentItemId={currentImage?.id ?? null}
                      compareItemId={compareB?.id ?? null}
                      onSelect={selectCurrent}
                      onToggleCompare={(next) => setCompareB(next)}
                      onOpenMenu={(item, x, y) => openMenu(item, x, y)}
                      onOpenGroup={() => setActivePromptGroup(entry.group)}
                    />
                  );
                }
                const h = entry.item;
                return (
                  <AndroidHistoryTile
                    key={h.id}
                    item={h}
                    isCurrent={currentImage?.id === h.id}
                    isCompare={compareB?.id === h.id}
                    onSelect={selectCurrent}
                    onToggleCompare={(next) => setCompareB(next)}
                    onOpenMenu={(x, y) => openMenu(h, x, y)}
                  />
                );
              })}
            </div>
          )}

          {historyLoading ? (
            <div className="android-history-empty">
              <Loader2 className="h-5 w-5 animate-spin" />
              <strong>正在加载更多历史</strong>
              <span>稍等一下，旧记录正在补齐。</span>
            </div>
          ) : null}

          {historyHasMore || promptEntries.length > androidHistoryEntries.length ? (
            <button type="button" className="android-history-more" onClick={openHistoryTimeline}>
              查看更多历史
            </button>
          ) : null}
        </section>

        <section className="android-history-quick-actions">
          <button type="button" onClick={() => latestHistory && void regenerateFromHistory(latestHistory)} disabled={!latestHistory}>
            <RotateCcw className="h-4 w-4" /> 重跑最近
          </button>
          <button type="button" onClick={() => void clearAllHistory()} disabled={historyClearing || (history.length === 0 && !historyHasMore)}>
            {historyClearing ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />} 全部删除
          </button>
          <button
            type="button"
            className="danger"
            onClick={() => {
              if (!currentImage) return;
              if (window.confirm(`确定删除当前历史项?\n\n${currentImage.prompt?.slice(0, 60) || "(无 prompt)"}`)) {
                void deleteHistoryItem(currentImage.id);
              }
            }}
            disabled={!currentImage}
          >
            <Trash2 className="h-4 w-4" /> 删除当前
          </button>
        </section>

        {menu && <AndroidHistoryActionSheet item={menu.item} items={buildMenu(menu.item)} onClose={closeMenu} />}
        {rawPath && <RawResponseModal path={rawPath} onClose={closeRaw} />}
        <HistoryPromptGroupModal
          group={activePromptGroup}
          currentItemId={currentImage?.id ?? null}
          compareItemId={compareB?.id ?? null}
          onClose={() => setActivePromptGroup(null)}
          onSelect={(item) => void selectCurrent(item)}
          onReuse={reuseAsSource}
          onToggleCompare={(next) => setCompareB(next)}
          onOpenMenu={(item, x, y) => openMenu(item, x, y)}
        />
      </aside>
    );
  }

  return (
    <aside className={`history-rail box-border flex w-[332px] shrink-0 flex-col overflow-y-auto border-l border-[var(--border)] bg-[var(--inspector)] px-4 py-4 backdrop-blur-2xl ${usesAppleUI ? "liquid-sidebar" : ""} ${usesAndroidUI && !isAndroidPhone ? "android-surface-pane" : ""} ${isAndroidPad ? "android-pad-history" : ""}`}>
      <div className={`history-rail-stack ${isAndroidPad ? "android-pad-history-stack" : "history-rail-stack-compact"}`}>
      <div className={`platform-card history-rail-summary-card border border-black/[0.05] bg-white/70 shadow-[var(--shadow-card)] dark:border-white/[0.06] dark:bg-white/[0.03] ${isAndroidPhone ? "p-2.5" : "p-3.5"} ${usesFluentUI ? "rounded-[12px]" : "rounded-[20px]"}`}>
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-1.5">
            <h3 className="text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-500 dark:text-zinc-200">
              上游
            </h3>
            <span className={`h-1.5 w-1.5 rounded-full ${apiKey && baseURL ? "bg-[var(--accent)] shadow-[0_0_6px_rgb(0_122_255_/_0.55)]" : "bg-red-500"}`} />
            <span className={`text-[11px] font-medium ${apiKey && baseURL ? "text-[var(--accent)]" : "text-red-400"}`}>
              {apiKey && baseURL ? "已配置" : "未配置"}
            </span>
          </div>
          <span className="text-[11px] text-zinc-500 dark:text-zinc-400">当前连接</span>
        </div>

        {profiles.length > 0 ? (
          <div className="mt-3">
            <select
              value={activeProfileId}
              onChange={(e) => {
                const id = e.target.value;
                if (id === "__manage__") {
                  openUpstreamConfig("app");
                  return;
                }
                if (id) void setActiveProfile(id);
              }}
              className={`focus-ring w-full border border-black/[0.08] bg-[var(--surface)] px-3 py-2.5 text-[12px] font-medium text-zinc-800 dark:border-white/[0.08] dark:text-zinc-100 ${usesFluentUI ? "rounded-[8px]" : "rounded-[16px]"}`}
              title="切换上游配置 / 管理"
            >
              {profiles.map((profile) => (
                <option key={profile.id} value={profile.id}>
                  {profile.name} · {profile.apiMode === "responses" ? "Responses" : "Images"}
                </option>
              ))}
              <option value="__manage__">⚙ 管理配置...</option>
            </select>
          </div>
        ) : (
          <p className="mt-3 text-[12px] leading-relaxed text-zinc-500 dark:text-zinc-300">
            还没有上游配置，先建一条再开始生成。
          </p>
        )}

          <div className={`mt-2 flex ${isAndroidPhone ? "gap-1" : "gap-1.5"} ${isMac ? "items-stretch" : ""}`}>
          <button
            onClick={() => openUpstreamConfig("app")}
            className={`platform-action-btn flex-1 inline-flex min-h-[34px] items-center justify-center gap-1.5 border border-black/[0.08] px-3 text-[12px] font-medium text-zinc-700 transition-colors hover:border-[color:var(--accent)]/35 hover:text-[var(--accent)] dark:border-white/[0.08] dark:text-zinc-300 ${isAndroidPhone ? "py-1.5" : isMac ? "py-2.5" : "py-2"} ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
          >
            上游配置
          </button>
          <button
            onClick={testAPIKey}
            disabled={!apiKey.trim() || !baseURL.trim() || isTestingKey}
            title="验证当前配置是否可连通"
            className={`platform-action-btn inline-flex min-h-[34px] min-w-[84px] items-center justify-center gap-1.5 border border-black/[0.08] px-3 text-[12px] font-medium text-zinc-700 transition-colors hover:border-[color:var(--accent)]/35 hover:text-[var(--accent)] disabled:cursor-not-allowed disabled:opacity-50 dark:border-white/[0.08] dark:text-zinc-300 ${isAndroidPhone ? "py-1.5" : isMac ? "py-2.5" : "py-2"} ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
          >
            {isTestingKey ? "检查中..." : isAndroidPhone ? "连通性" : "测试"}
          </button>
        </div>

        {!isAndroidPhone ? (
          <div className="mt-2 flex items-center justify-between gap-2">
            <p className="min-w-0 text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-300">
              {apiMode === "responses" ? "Responses API" : "Images API"}
            </p>
          </div>
        ) : null}
      </div>

      <div className={`platform-card history-rail-summary-card border border-black/[0.05] bg-white/70 shadow-[var(--shadow-card)] dark:border-white/[0.06] dark:bg-white/[0.03] ${isAndroidPhone ? "p-2.5" : "p-3.5"} ${usesFluentUI ? "rounded-[12px]" : "rounded-[20px]"}`}>
        <div className={`flex items-center justify-between ${isMac ? "gap-2.5" : "gap-2"}`}>
          <h3 className={`${isMac ? "text-[12px]" : "text-[11px]"} font-semibold uppercase tracking-[0.12em] text-zinc-500 dark:text-zinc-200`}>
            历史 <span className="font-mono-token text-zinc-500 dark:text-zinc-400">({filtered.length}{filtered.length !== history.length ? `/${historyCountLabel}` : historyHasMore ? "+" : ""})</span>
          </h3>
          <div className={`history-rail-header-actions flex items-center ${isMac ? "gap-1.5 flex-wrap justify-end" : "gap-2"} shrink-0`}>
            {showPhoneFilterToggle ? (
              <button
                type="button"
                onClick={() => setFiltersOpen((v) => !v)}
                className={`platform-pill inline-flex min-h-[30px] items-center gap-1.5 px-2.5 py-1 text-[11px] font-medium transition-colors ${
                  filtersOpen || historyFiltersActive
                    ? "bg-[var(--accent-soft)] text-[var(--accent)] ring-1 ring-[color:var(--accent)]/20"
                    : "text-zinc-500 hover:bg-[var(--accent-soft)] hover:text-[var(--accent)] dark:text-zinc-300"
                } ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
              >
                <Filter className="h-3 w-3" /> 筛选
              </button>
            ) : null}
            {!isAndroidPhone && !isWindows && (historyHasMore || filtered.length > 6) ? (
              <button
                type="button"
                onClick={openHistoryTimeline}
                className={`history-rail-header-btn platform-pill inline-flex min-h-[30px] items-center justify-center gap-1.5 ${isMac ? "min-w-[78px] px-2.5 py-1.5 text-[12px]" : "px-2.5 py-1 text-[11px]"} font-medium text-zinc-500 transition-colors hover:text-[var(--accent)] ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
              >
                <GalleryVerticalEnd className="h-3 w-3" /> 更多
              </button>
            ) : null}
            {!isAndroidPhone && !isWindows ? (
              <button
                type="button"
                onClick={() => setHistoryRailCollapsed(!historyRailCollapsed)}
                className={`history-rail-header-btn platform-pill inline-flex min-h-[30px] items-center justify-center gap-1.5 ${isMac ? "min-w-[78px] px-2.5 py-1.5 text-[12px]" : "px-2.5 py-1 text-[11px]"} font-medium text-zinc-500 transition-colors hover:text-[var(--accent)] ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
              >
                {historyRailCollapsed ? <ChevronRight className="h-3 w-3" /> : <ChevronDown className="h-3 w-3" />}
                {historyRailCollapsed ? "展开" : "折叠"}
              </button>
            ) : null}
          </div>
        </div>

        {showFilterControls && (
          <>
            <input
              placeholder="搜索 prompt..."
              value={q}
              onChange={(e) => setQ(e.target.value)}
              className={`focus-ring ${isAndroidPhone ? "mt-1.5" : "mt-3"} w-full border border-black/[0.08] bg-[var(--surface)] px-3 py-2.5 text-[12px] text-zinc-900 placeholder:text-zinc-400 dark:border-white/[0.08] dark:text-zinc-100 dark:placeholder:text-zinc-500 ${usesFluentUI ? "rounded-[10px]" : "rounded-[16px]"}`}
            />
            <div className={`mt-2 flex ${isAndroidPhone ? "gap-1" : "gap-1.5"}`}>
              <select
                value={modeF}
                onChange={(e) => setModeF(e.target.value as ModeFilter)}
                className={`focus-ring flex-1 border border-black/[0.08] bg-[var(--surface)] px-3 ${isAndroidPhone ? "py-1.5" : "py-2.5"} text-[12px] text-zinc-700 dark:border-white/[0.08] dark:text-zinc-300 ${usesFluentUI ? "rounded-[10px]" : "rounded-[16px]"}`}
              >
                <option value="all">全部模式</option>
                <option value="generate">文生图</option>
                <option value="edit">图生图</option>
              </select>
              <select
                value={dateF}
                onChange={(e) => setDateF(e.target.value as DateFilter)}
                className={`focus-ring flex-1 border border-black/[0.08] bg-[var(--surface)] px-3 ${isAndroidPhone ? "py-1.5" : "py-2.5"} text-[12px] text-zinc-700 dark:border-white/[0.08] dark:text-zinc-300 ${usesFluentUI ? "rounded-[10px]" : "rounded-[16px]"}`}
              >
                <option value="all">全部日期</option>
                <option value="today">今天</option>
                <option value="week">本周</option>
              </select>
            </div>
          </>
        )}

        {!isAndroidPhone && !isMac && (
          <p className="mt-2 text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-300">
            {isWindows
              ? "点击查看 · Shift+点击对比 · 双击设源图 · 右键更多"
              : isAndroidPad
              ? "点缩略图查看，Shift 可对比，双击可设为源图。"
              : "点击查看 · Shift+点击对比 · 双击设源图 · 更多菜单"}
          </p>
        )}

        {isAndroidPad && filtered.length > 0 && (
          <p className="mt-2 text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-400">
            历史单独收纳，回溯参数、继续变体都从这里进入。
          </p>
        )}
      </div>

      {compareB && (
        <button
          onClick={() => setCompareB(null)}
          className={`platform-pill inline-flex items-center justify-center gap-1.5 border border-[color:var(--accent)]/20 bg-[var(--accent-soft)] px-2.5 py-2 text-xs text-[var(--accent)] transition-colors hover:opacity-90 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
        >
          <Split className="w-3 h-3" /> 退出对比
        </button>
      )}

      {isMac && !desktopHistoryCollapsed && visibleDesktopEntries.length > 0 ? (
        <p className="text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-400">
          单击查看 · 双击设源图 · Shift+点击对比 · 更多菜单查看完整操作
        </p>
      ) : null}

      {!desktopHistoryCollapsed && history.length > 0 ? (
        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => latestHistory && void regenerateFromHistory(latestHistory)}
            disabled={!latestHistory}
            className={`platform-pill inline-flex min-h-[34px] flex-1 items-center justify-center gap-1.5 border border-black/[0.08] bg-[var(--surface)] px-3 text-[12px] text-zinc-600 transition-colors hover:border-[color:var(--accent)]/35 hover:text-[var(--accent)] dark:border-white/[0.08] dark:text-zinc-300 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
          >
            <RotateCcw className="h-3.5 w-3.5" /> 重跑最近
          </button>
          <button
            type="button"
            onClick={() => void clearAllHistory()}
            disabled={historyClearing || (history.length === 0 && !historyHasMore)}
            className={`platform-pill inline-flex min-h-[34px] flex-1 items-center justify-center gap-1.5 border border-black/[0.08] bg-[var(--surface)] px-3 text-[12px] text-zinc-600 transition-colors hover:border-red-400/40 hover:text-red-400 dark:border-white/[0.08] dark:text-zinc-300 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
          >
            {historyClearing ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Trash2 className="h-3.5 w-3.5" />} 全部删除
          </button>
        </div>
      ) : null}

      {desktopHistoryCollapsed ? null : visibleDesktopEntries.length === 0 ? (
        <div className={`platform-card border border-black/[0.05] bg-white/70 text-center text-[12px] text-zinc-500 shadow-[var(--shadow-card)] dark:border-white/[0.06] dark:bg-white/[0.03] dark:text-zinc-300 ${isAndroidPhone ? "py-4" : "py-8"} ${usesFluentUI ? "rounded-[12px]" : "rounded-[20px]"}`}>
          {q || modeF !== "all" || dateF !== "all" ? "没有匹配项" : "还没有结果"}
        </div>
      ) : (
        <>
          <div className="grid grid-cols-2 gap-2.5">
            {visibleDesktopEntries.map((entry) => {
              if (entry.kind === "group") {
                return (
                  <HistoryPromptGroupCard
                    key={entry.key}
                    group={entry.group}
                    currentItemId={currentImage?.id ?? null}
                    compareItemId={compareB?.id ?? null}
                    onSelect={selectCurrent}
                    onToggleCompare={(next) => setCompareB(next)}
                    onOpenMenu={(item, x, y) => openMenu(item, x, y)}
                    onOpenGroup={() => setActivePromptGroup(entry.group)}
                  />
                );
              }
              const h = entry.item;
              return (
                <HistoryTile
                  key={h.id}
                  item={h}
                  isCurrent={currentImage?.id === h.id}
                  isCompare={compareB?.id === h.id}
                  onSelect={selectCurrent}
                  onToggleCompare={(next) => setCompareB(next)}
                  onReuse={reuseAsSource}
                  onDelete={deleteHistoryItem}
                  onOpenMenu={(x, y) => openMenu(h, x, y)}
                />
              );
            })}
          </div>
          {historyLoading ? (
            <div className="platform-card border border-black/[0.05] bg-white/70 px-3 py-3 text-center text-[12px] text-zinc-500 shadow-[var(--shadow-card)] dark:border-white/[0.06] dark:bg-white/[0.03] dark:text-zinc-300">
              <span className="inline-flex items-center gap-2">
                <Loader2 className="h-4 w-4 animate-spin" />
                正在加载更多历史...
              </span>
            </div>
          ) : null}
        </>
      )}

      {menu && <ContextMenu x={menu.x} y={menu.y} items={buildMenu(menu.item)} onClose={closeMenu} />}
      {rawPath && <RawResponseModal path={rawPath} onClose={closeRaw} />}
      <HistoryPromptGroupModal
        group={activePromptGroup}
        currentItemId={currentImage?.id ?? null}
        compareItemId={compareB?.id ?? null}
        onClose={() => setActivePromptGroup(null)}
        onSelect={(item) => void selectCurrent(item)}
        onReuse={reuseAsSource}
        onToggleCompare={(next) => setCompareB(next)}
        onOpenMenu={(item, x, y) => openMenu(item, x, y)}
      />
      </div>
    </aside>
  );
}
