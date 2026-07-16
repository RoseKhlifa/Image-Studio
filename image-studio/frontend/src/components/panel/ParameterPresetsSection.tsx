import { useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { FolderCog, Plus, Save } from "lucide-react";
import { findMatchingPresetId, nextDefaultPresetName, normalizeSelectedPresetId, pickPresetStateSnapshot } from "../../lib/presets";
import { usePlatform } from "../../platform/context";
import { useStudioStore } from "../../state/studioStore";
import type { Preset } from "../../types/domain";
import { PresetManagerModal } from "./PresetManagerModal";
import { PresetSaveModal } from "./PresetSaveModal";
import { STYLE_CHIPS } from "./panelOptions";

const QUALITY_LABELS: Record<string, string> = {
  auto: "Auto",
  low: "低",
  medium: "中",
  high: "高",
  standard: "标准",
  hd: "HD",
};

export function ParameterPresetsSection({
  variant = "desktop",
}: {
  variant?: "desktop" | "android";
}) {
  const state = useStudioStore();
  const {
    presets,
    selectedPresetId,
    setField,
    savePreset,
    overwritePreset,
    updatePreset,
    applyPreset,
    deletePreset,
  } = state;
  const { usesFluentUI } = usePlatform();
  const [managerOpen, setManagerOpen] = useState(false);
  const [saveModalMode, setSaveModalMode] = useState<"save-current" | "new" | null>(null);
  const currentSnapshot = pickPresetStateSnapshot(state);
  const matchedPresetId = findMatchingPresetId(presets, currentSnapshot);
  const normalizedSelectedPresetId = normalizeSelectedPresetId(presets, selectedPresetId);
  const selectedPreset = presets.find((preset) => preset.id === normalizedSelectedPresetId) ?? null;
  const currentPreset = selectedPreset ?? presets.find((preset) => preset.id === matchedPresetId) ?? null;
  const isAndroid = variant === "android";
  const cardRadius = isAndroid ? "rounded-[18px]" : usesFluentUI ? "rounded-[10px]" : "rounded-[16px]";
  const buttonRadius = usesFluentUI ? "rounded-[8px]" : "rounded-full";
  const suggestedName = useMemo(() => nextDefaultPresetName(presets), [presets]);

  useEffect(() => {
    if (normalizedSelectedPresetId === selectedPresetId) return;
    setField("selectedPresetId", normalizedSelectedPresetId);
  }, [normalizedSelectedPresetId, selectedPresetId, setField]);

  function handleApplyPreset(id: string) {
    setField("selectedPresetId", id);
    applyPreset(id);
  }

  function handleSaveAsNewPreset(name: string) {
    const id = savePreset(name);
    if (id) {
      setField("selectedPresetId", id);
    }
    return id;
  }

  function handleOverwritePreset(id: string) {
    return overwritePreset(id);
  }

  function handleUpdatePreset(id: string, patch: Partial<Omit<Preset, "id">>) {
    return updatePreset(id, patch);
  }

  const summary = buildPresetSummary({
    matchedPresetId,
    matchedPresetName: presets.find((preset) => preset.id === matchedPresetId)?.name ?? null,
    selectedPreset,
  });
  const actionGridClassName = isAndroid
    ? "grid-cols-1 gap-2.5"
    : usesFluentUI
      ? "windows-preset-actions"
      : "grid-cols-3 gap-2";

  return (
    <>
      <section className={`windows-preset-card border border-black/[0.06] bg-[var(--surface)] px-3 py-3 dark:border-white/[0.08] ${cardRadius}`}>
        <div className="windows-preset-head flex items-center justify-between gap-2">
          <div className="windows-preset-kicker flex items-center gap-1.5">
            <span className="windows-preset-title text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-500 dark:text-zinc-300">
              参数预设
            </span>
            <span className={`h-1.5 w-1.5 rounded-full ${presets.length > 0 ? "bg-[var(--accent)] shadow-[0_0_6px_rgb(0_122_255_/_0.45)]" : "bg-zinc-300 dark:bg-zinc-600"}`} />
            <span className={`text-[11px] font-medium ${presets.length > 0 ? "text-[var(--accent)]" : "text-zinc-400 dark:text-zinc-500"}`}>
              {presets.length > 0 ? `已保存 ${presets.length} 条` : "未保存"}
            </span>
          </div>
          <span className="windows-preset-caption text-[11px] text-zinc-500 dark:text-zinc-400">当前方案</span>
        </div>

        <div className="mt-2">
          <div className="windows-preset-summary flex items-center gap-2">
            <div className="min-w-0 flex-1">
              <div className="text-[13px] font-medium text-zinc-900 dark:text-zinc-100">{summary.title}</div>
              <div className="mt-1 text-[11px] leading-5 text-zinc-500 dark:text-zinc-400">{summary.detail}</div>
            </div>
            {selectedPreset ? <PresetBadge variant="selected">已选</PresetBadge> : null}
            {!selectedPreset && matchedPresetId ? <PresetBadge variant="matched">匹配</PresetBadge> : null}
          </div>
        </div>

        {presets.length > 0 ? (
          <div className="mt-3">
            <select
              value={normalizedSelectedPresetId ?? ""}
              onChange={(e) => {
                const id = e.target.value;
                if (!id) {
                  setField("selectedPresetId", null);
                  return;
                }
                handleApplyPreset(id);
              }}
              className={`focus-ring w-full border border-black/[0.08] bg-white/70 px-3 py-2.5 text-[12px] font-medium text-zinc-800 dark:border-white/[0.08] dark:bg-black/10 dark:text-zinc-100 ${usesFluentUI ? "rounded-[8px]" : "rounded-[16px]"}`}
              title="切换预设"
            >
              <option value="">选择预设...</option>
              {presets.map((preset) => (
              <option key={preset.id} value={preset.id}>
                  {preset.name} · {preset.editAutoAspectResolution ? `按源图+${preset.editAutoAspectResolution.toUpperCase()}` : (preset.size === "auto" ? "Auto" : preset.size)} · {QUALITY_LABELS[preset.quality] ?? preset.quality}
              </option>
              ))}
            </select>
          </div>
        ) : (
          <div className={`windows-preset-empty mt-3 border border-dashed border-black/[0.12] px-3 py-3 text-zinc-500 dark:border-white/[0.1] dark:text-zinc-400 ${cardRadius} ${isAndroid ? "text-[12px]" : "text-[11px]"}`}>
            还没有保存任何预设。先调好参数，再新建一条。
          </div>
        )}

        <div className={`mt-3 grid ${actionGridClassName}`}>
          <ActionButton icon={<Save className="h-3.5 w-3.5" />} onClick={() => setSaveModalMode("save-current")}>
            保存当前预设
          </ActionButton>
          <ActionButton icon={<Plus className="h-3.5 w-3.5" />} onClick={() => setSaveModalMode("new")}>
            新建预设
          </ActionButton>
          <ActionButton icon={<FolderCog className="h-3.5 w-3.5" />} onClick={() => setManagerOpen(true)}>
            预设管理
          </ActionButton>
        </div>
      </section>

      <PresetSaveModal
        open={saveModalMode !== null}
        mode={saveModalMode ?? "new"}
        currentPreset={currentPreset ? { id: currentPreset.id, name: currentPreset.name } : null}
        suggestedName={suggestedName}
        onClose={() => setSaveModalMode(null)}
        onOverwritePreset={handleOverwritePreset}
        onSaveAsNewPreset={handleSaveAsNewPreset}
      />

      <PresetManagerModal
        open={managerOpen}
        presets={presets}
        selectedPresetId={normalizedSelectedPresetId}
        onClose={() => setManagerOpen(false)}
        onDeletePreset={(id) => {
          if (normalizedSelectedPresetId === id) setField("selectedPresetId", null);
          deletePreset(id);
        }}
        onUpdatePreset={handleUpdatePreset}
      />
    </>
  );
}

function ActionButton({
  children,
  icon,
  onClick,
}: {
  children: ReactNode;
  icon: ReactNode;
  onClick: () => void;
}) {
  const { usesFluentUI } = usePlatform();

  return (
    <button
      type="button"
      onClick={onClick}
      className={`windows-preset-action inline-flex min-h-[38px] items-center justify-center gap-1.5 border border-black/[0.08] px-3 text-[12px] font-medium text-zinc-600 transition-colors hover:border-[color:var(--accent)]/30 hover:text-[var(--accent)] dark:border-white/[0.08] dark:text-zinc-300 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
    >
      {icon}
      {children}
    </button>
  );
}

function PresetBadge({
  children,
  variant,
}: {
  children: ReactNode;
  variant: "matched" | "selected";
}) {
  const { usesFluentUI } = usePlatform();
  const colorClasses = variant === "selected"
    ? "border-[color:var(--accent)]/20 bg-white/70 text-[var(--accent)] dark:bg-black/20"
    : "border-emerald-400/20 bg-emerald-500/10 text-emerald-600 dark:text-emerald-300";

  return (
    <span className={`shrink-0 border px-1.5 py-0.5 text-[10px] font-medium ${colorClasses} ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}>
      {children}
    </span>
  );
}

function buildPresetSummary({
  matchedPresetId,
  matchedPresetName,
  selectedPreset,
}: {
  matchedPresetId: string | null;
  matchedPresetName: string | null;
  selectedPreset: Preset | null;
}) {
  if (selectedPreset && selectedPreset.id === matchedPresetId) {
    return {
      title: `当前使用「${selectedPreset.name}」`,
      detail: "当前参数与已选预设完全一致，可直接覆盖保存。",
    };
  }
  if (selectedPreset) {
    return {
      title: `已选「${selectedPreset.name}」`,
      detail: `当前选中方案：${describePreset(selectedPreset)}`,
    };
  }
  if (matchedPresetName) {
    return {
      title: `当前匹配「${matchedPresetName}」`,
      detail: "当前参数正好匹配一条已有预设，保存时可直接覆盖。",
    };
  }
  return {
    title: "还没有选中预设",
    detail: "可以从下拉里直接切换，或把当前参数保存成新的预设。",
  };
}

function describePreset(preset: Preset): string {
  const styleLabel = preset.styleTag
    ? STYLE_CHIPS.find((item) => item.id === preset.styleTag)?.label ?? preset.styleTag
    : "默认风格";
  return [
    preset.editAutoAspectResolution
      ? `按源图比例 + ${preset.editAutoAspectResolution.toUpperCase()}`
      : preset.size === "auto"
        ? "Auto 尺寸"
        : preset.size,
    `质量 ${QUALITY_LABELS[preset.quality] ?? preset.quality}`,
    preset.outputFormat ? preset.outputFormat.toUpperCase() : "PNG",
    `风格 ${styleLabel}`,
    `${preset.batchCount} 张`,
  ].join(" · ");
}
