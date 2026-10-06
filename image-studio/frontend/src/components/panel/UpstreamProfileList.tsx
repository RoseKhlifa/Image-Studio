import { ClipboardPaste, Copy, Download, Plus, RefreshCw, Trash2, Upload } from "lucide-react";
import type { UpstreamProfile } from "../../types/domain";
import { usePlatform } from "../../platform/context";

export function UpstreamProfileList({
  profiles,
  selectedId,
  activeProfileId,
  aiProfileId,
  draftId,
  draftAPIMode,
  isAndroidPhone,
  canSyncCodexConfig,
  isSyncingCodexConfig,
  onSelectProfile,
  onHandleNew,
  onHandleDuplicate,
  onHandleDelete,
  onHandleExport,
  onHandleImport,
  onHandleQuickImport,
  onHandleSetActive,
  onHandleSetAI,
  onHandleSyncCodex,
}: {
  profiles: UpstreamProfile[];
  selectedId: string;
  activeProfileId: string;
  aiProfileId: string;
  draftId?: string;
  draftAPIMode?: UpstreamProfile["apiMode"];
  isAndroidPhone: boolean;
  canSyncCodexConfig: boolean;
  isSyncingCodexConfig: boolean;
  onSelectProfile: (id: string) => void;
  onHandleNew: () => void | Promise<void>;
  onHandleDuplicate: () => void | Promise<void>;
  onHandleDelete: () => void | Promise<void>;
  onHandleExport: () => void | Promise<void>;
  onHandleImport: () => void | Promise<void>;
  onHandleQuickImport: () => void | Promise<void>;
  onHandleSetActive: () => void | Promise<void>;
  onHandleSetAI: () => void | Promise<void>;
  onHandleSyncCodex: () => void | Promise<void>;
}) {
  const { usesFluentUI } = usePlatform();

  return (
    <aside className={`upstream-profile-list flex min-w-0 shrink-0 flex-col gap-2 ${isAndroidPhone ? "w-full" : "w-[240px]"}`}>
      <div className={`flex-1 overflow-y-auto border border-black/[0.08] bg-[var(--surface)] p-1.5 dark:border-white/[0.06] ${usesFluentUI ? "rounded-[10px]" : "rounded-[16px]"}`} style={{ maxHeight: isAndroidPhone ? 172 : 460 }}>
        {profiles.length === 0 ? (
          <p className="px-2 py-3 text-[11px] text-zinc-500">还没有配置,点下方「+ 新建」开始。</p>
        ) : (
          <div className={`flex ${isAndroidPhone ? "gap-2 overflow-x-auto pb-1" : "flex-col"}`}>
            {profiles.map((p) => {
              const isSel = p.id === selectedId;
              const isActive = p.id === activeProfileId;
              const isAI = p.id === aiProfileId;
              return (
                <button
                  key={p.id}
                  type="button"
                  onClick={() => onSelectProfile(p.id)}
                  className={`platform-card group flex min-w-0 items-center gap-2 px-2.5 py-2 text-left transition-colors ${
                    isSel
                      ? "border-[color:var(--accent)]/35 bg-[var(--accent-soft)] text-[var(--accent)]"
                      : "border-transparent text-zinc-700 hover:bg-black/[0.04] dark:text-zinc-300 dark:hover:bg-white/[0.04]"
                  } ${isAndroidPhone ? "min-w-[208px]" : "mb-1 w-full"} ${usesFluentUI ? "rounded-[8px]" : "rounded-[12px]"}`}
                >
                  <span
                    title={isActive ? "当前生图渠道" : "选择后可设为生图渠道"}
                    className={`h-2 w-2 shrink-0 rounded-full ${isActive ? "bg-[var(--accent)] shadow-[0_0_5px_rgb(0_122_255_/_0.6)]" : "bg-zinc-300 dark:bg-zinc-700"}`}
                  />
                  <span className="min-w-0 flex-1 truncate break-words text-[13px] font-medium [overflow-wrap:anywhere]">{p.name}</span>
                  {isActive ? <span className="shrink-0 text-[9px] font-medium text-[var(--accent)]">生图</span> : null}
                  {isAI ? <span className="shrink-0 text-[9px] font-medium text-emerald-600 dark:text-emerald-400">AI</span> : null}
                  <span className="shrink-0 text-[9px] uppercase tracking-wider opacity-70">
                    {(p.provider ?? "openai") === "openai" ? (p.apiMode === "responses" ? "R" : "I") : p.provider === "google" ? "G" : "X"}
                  </span>
                </button>
              );
            })}
          </div>
        )}
      </div>
      {canSyncCodexConfig ? (
        <button
          type="button"
          onClick={() => void onHandleSyncCodex()}
          disabled={isSyncingCodexConfig}
          className={`platform-action-btn inline-flex items-center justify-center gap-1 border border-[color:var(--accent)]/28 bg-[var(--accent-soft)] px-2.5 py-1.5 text-[11px] font-medium text-[var(--accent)] transition-colors hover:bg-[color:var(--accent)]/15 disabled:cursor-not-allowed disabled:opacity-60 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
        >
          <RefreshCw className={`h-3 w-3 ${isSyncingCodexConfig ? "animate-spin" : ""}`} />
          {isSyncingCodexConfig ? "同步中..." : "同步 Codex 配置"}
        </button>
      ) : null}
      <div className="flex flex-wrap gap-1.5">
        <button
          type="button"
          onClick={() => void onHandleNew()}
          className={`platform-action-btn inline-flex flex-1 items-center justify-center gap-1 border border-black/[0.08] px-2.5 py-1.5 text-[11px] text-zinc-700 transition-colors hover:border-[color:var(--accent)]/35 hover:text-[var(--accent)] dark:border-white/[0.08] dark:text-zinc-300 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
        >
          <Plus className="h-3 w-3" /> 新建
        </button>
        <button
          type="button"
          onClick={() => void onHandleDuplicate()}
          disabled={!selectedId}
          title="复制当前选中"
          className={`platform-action-btn inline-flex items-center justify-center gap-1 border border-black/[0.08] px-2.5 py-1.5 text-[11px] text-zinc-700 transition-colors hover:border-[color:var(--accent)]/35 hover:text-[var(--accent)] disabled:cursor-not-allowed disabled:opacity-40 dark:border-white/[0.08] dark:text-zinc-300 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
        >
          <Copy className="h-3 w-3" />
        </button>
        <button
          type="button"
          onClick={() => void onHandleImport()}
          title="导入上游配置"
          className={`platform-action-btn inline-flex items-center justify-center gap-1 border border-black/[0.08] px-2.5 py-1.5 text-[11px] text-zinc-700 transition-colors hover:border-[color:var(--accent)]/35 hover:text-[var(--accent)] dark:border-white/[0.08] dark:text-zinc-300 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
        >
          <Upload className="h-3 w-3" />
        </button>
        <button
          type="button"
          onClick={() => void onHandleQuickImport()}
          title="粘贴 JSON 快捷导入"
          className={`platform-action-btn inline-flex items-center justify-center gap-1 border border-black/[0.08] px-2.5 py-1.5 text-[11px] text-zinc-700 transition-colors hover:border-[color:var(--accent)]/35 hover:text-[var(--accent)] dark:border-white/[0.08] dark:text-zinc-300 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
        >
          <ClipboardPaste className="h-3 w-3" />
        </button>
        <button
          type="button"
          onClick={() => void onHandleExport()}
          disabled={profiles.length === 0}
          title="导出上游配置"
          className={`platform-action-btn inline-flex items-center justify-center gap-1 border border-black/[0.08] px-2.5 py-1.5 text-[11px] text-zinc-700 transition-colors hover:border-[color:var(--accent)]/35 hover:text-[var(--accent)] disabled:cursor-not-allowed disabled:opacity-40 dark:border-white/[0.08] dark:text-zinc-300 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
        >
          <Download className="h-3 w-3" />
        </button>
        <button
          type="button"
          onClick={() => void onHandleDelete()}
          disabled={!selectedId}
          title="删除当前选中(连同凭据)"
          className={`platform-action-btn inline-flex items-center justify-center gap-1 border border-black/[0.08] px-2.5 py-1.5 text-[11px] text-zinc-500 transition-colors hover:border-red-400/45 hover:text-red-400 disabled:cursor-not-allowed disabled:opacity-40 dark:border-white/[0.08] ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
        >
          <Trash2 className="h-3 w-3" />
        </button>
      </div>
      <div className="grid grid-cols-2 gap-1.5">
        <button
          type="button"
          onClick={() => void onHandleSetActive()}
          disabled={!draftId || draftId === activeProfileId}
          className={`platform-action-btn inline-flex min-w-0 items-center justify-center border border-[color:var(--accent)]/30 bg-[var(--accent-soft)] px-2 py-1.5 text-[11px] font-medium text-[var(--accent)] transition-colors hover:bg-[color:var(--accent)]/15 disabled:cursor-default disabled:opacity-45 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
        >
          {draftId === activeProfileId ? "当前生图" : "设为生图"}
        </button>
        <button
          type="button"
          onClick={() => void onHandleSetAI()}
          disabled={!draftId || draftAPIMode !== "responses" || draftId === aiProfileId}
          title={draftAPIMode === "responses" ? "用于 AI 优化与图片反推" : "AI 渠道必须使用 Responses API"}
          className={`platform-action-btn inline-flex min-w-0 items-center justify-center border border-emerald-500/30 bg-emerald-500/10 px-2 py-1.5 text-[11px] font-medium text-emerald-700 transition-colors hover:bg-emerald-500/15 disabled:cursor-default disabled:opacity-45 dark:text-emerald-300 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
        >
          {draftId === aiProfileId ? "当前 AI" : "设为 AI"}
        </button>
      </div>
    </aside>
  );
}
