import type { APIMode, QualityValue, RequestPolicy } from "../../types/domain";
import { Check } from "lucide-react";
import { STYLE_CHIPS } from "./panelOptions";
import {
  type AspectPreset,
  type AspectPresetOption,
  RESOLUTION_PRESETS,
  type ResolutionPreset,
  sizeCapabilityHint,
} from "./sizeCapabilities";

export function MacComposeStyleAndSize({
  activeAspect,
  aspectOptions,
  activeResolution,
  batchAutoAspectActive = false,
  manualEditAutoAspectActive = false,
  editAutoAspectComputedSizeLabel,
  effectiveEditAutoAspectResolution,
  exactSizeLabel,
  allowCustomAspectRatios,
  allowPreciseSizeControl,
  apiMode,
  availableResolutions,
  batchCount,
  mode,
  handleAspectSelect,
  handleEditAutoAspectResolutionSelect,
  handleEditAutoAspectToggle,
  handleResolutionSelect,
  imageModelID,
  onOpenCustomAspectRatioModal,
  onOpenCustomSizeModal,
  quality,
  qualityOptions,
  requestPolicy,
  setField,
  styleTag,
  Seg,
  SegItem,
}: {
  activeAspect: AspectPreset | null;
  aspectOptions: AspectPresetOption[];
  activeResolution: ResolutionPreset | null;
  batchAutoAspectActive?: boolean;
  manualEditAutoAspectActive?: boolean;
  editAutoAspectComputedSizeLabel?: string | null;
  effectiveEditAutoAspectResolution: Exclude<ResolutionPreset, "auto">;
  exactSizeLabel?: string | null;
  allowCustomAspectRatios: boolean;
  allowPreciseSizeControl: boolean;
  apiMode: APIMode;
  availableResolutions: ResolutionPreset[];
  batchCount: number;
  mode: "generate" | "edit";
  handleAspectSelect: (aspect: AspectPreset) => void;
  handleEditAutoAspectResolutionSelect: (resolution: Exclude<ResolutionPreset, "auto">) => void;
  handleEditAutoAspectToggle: (enabled: boolean) => void;
  handleResolutionSelect: (resolution: ResolutionPreset) => void;
  imageModelID: string;
  onOpenCustomAspectRatioModal: () => void;
  onOpenCustomSizeModal: () => void;
  quality: QualityValue;
  qualityOptions: Array<{ value: QualityValue; label: string }>;
  requestPolicy: RequestPolicy;
  setField: (key: string, value: any) => void;
  styleTag: string;
  Seg: (props: { children: React.ReactNode }) => React.ReactNode;
  SegItem: (props: { active: boolean; onClick: () => void; children: React.ReactNode }) => React.ReactNode;
}) {
  const hideRegularSizeControls = batchAutoAspectActive || manualEditAutoAspectActive;
  const editAutoResolutionOptions = RESOLUTION_PRESETS.filter(
    (item): item is { value: Exclude<ResolutionPreset, "auto">; label: string } =>
      item.value !== "auto" && availableResolutions.includes(item.value),
  );

  return (
    <>
      <div>
        <div className="mb-1.5 flex items-center justify-between">
          <span className="text-[12px] text-zinc-500">风格</span>
          {styleTag ? (
            <button onClick={() => setField("styleTag", "")} className="text-[12px] text-[var(--accent)] hover:opacity-80">清除</button>
          ) : null}
        </div>
        <div className="mac-style-chips">
          {STYLE_CHIPS.map((style) => {
            const active = styleTag === style.id;
            return (
              <button
                key={style.id}
                type="button"
                aria-pressed={active}
                onClick={() => setField("styleTag", active ? "" : style.id)}
                className={`mac-style-pill ${active ? "active" : ""}`}
              >
                <span>{style.label}</span>
              </button>
            );
          })}
        </div>
      </div>

      {mode === "edit" ? (
        <div>
          <div className="mb-2 text-[12px] text-zinc-500">源图尺寸策略</div>
          <Seg>
            <SegItem active={!manualEditAutoAspectActive} onClick={() => handleEditAutoAspectToggle(false)}>
              沿用当前比例
            </SegItem>
            <SegItem active={manualEditAutoAspectActive} onClick={() => handleEditAutoAspectToggle(true)}>
              按源图比例自动适配
            </SegItem>
          </Seg>
          {manualEditAutoAspectActive ? (
            <>
              <div className="mt-2 grid grid-cols-5 gap-2.5">
                {editAutoResolutionOptions.map((item) => (
                  <button
                    key={item.value}
                    type="button"
                    onClick={() => handleEditAutoAspectResolutionSelect(item.value)}
                    className={`rounded-[14px] border px-2 py-2.5 text-[12px] font-semibold transition-colors ${
                      effectiveEditAutoAspectResolution === item.value
                        ? "border-[color:var(--accent)]/35 bg-white text-[var(--accent)] shadow-sm dark:bg-zinc-900"
                        : "border-black/[0.08] bg-white/70 text-zinc-600 hover:border-[color:var(--accent)]/30 hover:text-zinc-900 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-zinc-300"
                    }`}
                  >
                    {item.label}
                  </button>
                ))}
              </div>
              <p className="mt-1.5 text-[10px] leading-relaxed text-zinc-500 dark:text-zinc-400">
                {editAutoAspectComputedSizeLabel
                  ? `当前会按源图比例自动换算，目标分辨率档位 ${effectiveEditAutoAspectResolution.toUpperCase()}，实际提交尺寸 ${editAutoAspectComputedSizeLabel}。`
                  : `开启后会按当前源图比例自动换算尺寸，并统一使用 ${effectiveEditAutoAspectResolution.toUpperCase()} 分辨率档位。`}
              </p>
            </>
          ) : (
            <p className="mt-1.5 text-[10px] leading-relaxed text-zinc-500 dark:text-zinc-400">
              关闭后使用下面的普通比例和分辨率预设。
            </p>
          )}
        </div>
      ) : null}

      {!hideRegularSizeControls ? (
        <>
          <div>
            <div className="mb-2 flex items-center justify-between">
              <span className="text-[12px] text-zinc-500">比例</span>
              {allowCustomAspectRatios ? (
                <button
                  type="button"
                  onClick={onOpenCustomAspectRatioModal}
                  className="text-[12px] text-[var(--accent)] transition-opacity hover:opacity-80"
                >
                  自定义比例
                </button>
              ) : null}
            </div>
            <div className="grid grid-cols-3 gap-2.5">
              {aspectOptions.map((aspect) => {
                const active = activeAspect === aspect.value;
                return (
                  <button
                    key={aspect.value}
                    onClick={() => handleAspectSelect(aspect.value)}
                    title={aspect.auto ? "让上游决定尺寸 / 比例" : aspect.label}
                    className={`flex min-h-[64px] flex-col items-center justify-center gap-1.5 rounded-[14px] px-2 py-2 ring-1 transition-colors ${
                      active
                        ? "bg-[var(--accent-soft)] ring-[color:var(--accent)]/35"
                        : "ring-black/[0.08] dark:ring-white/[0.08] hover:ring-[color:var(--accent)]/30"
                    }`}
                  >
                    <span
                      className={`block rounded-sm border-2 ${aspect.auto ? "border-dashed" : ""} ${
                        active ? "border-[var(--accent)]" : "border-zinc-400 dark:border-zinc-600"
                      }`}
                      style={{ width: aspect.w, height: aspect.h }}
                    />
                    <span className={`text-[10px] ${active ? "text-[var(--accent)]" : "text-zinc-500"}`}>{aspect.label}</span>
                  </button>
                );
              })}
            </div>
          </div>

          <div>
            <div className="mb-2 flex items-center justify-between">
              <span className="text-[12px] text-zinc-500">分辨率</span>
              {allowPreciseSizeControl ? (
                <button
                  type="button"
                  onClick={onOpenCustomSizeModal}
                  className="text-[12px] text-[var(--accent)] transition-opacity hover:opacity-80"
                >
                  {exactSizeLabel ? "修改精确尺寸" : "精确尺寸"}
                </button>
              ) : null}
            </div>
            <Seg>
              {RESOLUTION_PRESETS.filter((item) => availableResolutions.includes(item.value)).map((item) => (
                <SegItem
                  key={item.value}
                  active={activeResolution === item.value}
                  onClick={() => handleResolutionSelect(item.value)}
                >
                  {item.label}
                </SegItem>
              ))}
            </Seg>
            {exactSizeLabel ? (
              <p className="mt-1.5 text-[10px] leading-relaxed text-zinc-500 dark:text-zinc-400">
                当前精确尺寸 {exactSizeLabel}。点击比例或分辨率预设后会切回预设档位。
              </p>
            ) : null}
            {sizeCapabilityHint({ apiMode, requestPolicy, imageModelID }) ? (
              <p className="mt-1.5 text-[10px] leading-relaxed text-zinc-500 dark:text-zinc-400">
                {sizeCapabilityHint({ apiMode, requestPolicy, imageModelID })}
              </p>
            ) : null}
          </div>
        </>
      ) : (
        <div className="rounded-[14px] border border-[color:var(--accent)]/16 bg-[var(--accent-soft)]/55 px-3 py-2 text-[11px] leading-5 text-zinc-600 dark:text-zinc-300">
          {batchAutoAspectActive
            ? "当前批处理已开启“按源图比例自动适配”，本批任务的比例与分辨率由“批处理图生图”区统一控制。这里的普通比例/分辨率预设已暂时隐藏，避免出现两套尺寸入口。"
            : "当前普通图生图已开启“按源图比例自动适配”，比例会跟随源图自动计算，分辨率由上面的源图尺寸策略统一控制。这里的普通比例/分辨率预设已暂时隐藏，避免出现两套尺寸入口。"}
        </div>
      )}

      <div>
        <div className="mb-2 text-[12px] text-zinc-500">质量</div>
        <Seg>
          {qualityOptions.map((tier) => (
            <SegItem
              key={tier.value}
              active={quality === tier.value}
              onClick={() => setField("quality", tier.value as QualityValue)}
            >
              {tier.label}
            </SegItem>
          ))}
        </Seg>
      </div>

      <div>
        <div className="mb-1.5 flex items-center justify-between">
          <span className="text-[12px] text-zinc-500">出图张数</span>
          <span className="rounded-full bg-black/[0.04] px-2 py-0.5 text-[11px] font-medium text-zinc-500 dark:bg-white/[0.06] dark:text-zinc-300">
            当前 {batchCount} 张
          </span>
        </div>
        <div className="grid grid-cols-3 gap-2.5">
          {[1, 2, 4, 6, 8, 9].map((count) => (
            <button
              key={count}
              type="button"
              aria-pressed={batchCount === count}
              onClick={() => setField("batchCount", count)}
              title={`同一提示词发起 ${count} 次请求`}
              className={`relative flex min-h-[48px] min-w-0 items-center justify-center rounded-[16px] border px-2 text-center transition-all ${
                batchCount === count
                  ? "border-[color:var(--accent)] bg-[var(--accent)] text-white ring-2 ring-[color:var(--accent)]/35 shadow-[0_10px_24px_rgb(0_122_255_/_0.24)]"
                  : "border-black/[0.08] bg-white/45 text-zinc-600 hover:border-[color:var(--accent)]/35 hover:text-zinc-900 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-zinc-300 dark:hover:text-zinc-100"
              }`}
            >
              <span className="flex min-w-0 flex-col items-center leading-none">
                <span className="text-[15px] font-semibold tabular-nums">{count}</span>
                <span className={`mt-1 text-[10px] font-medium ${batchCount === count ? "text-white/80" : "text-zinc-400 dark:text-zinc-500"}`}>
                  张
                </span>
              </span>
              {batchCount === count ? (
                <span className="absolute right-1.5 top-1.5 flex h-4 w-4 items-center justify-center rounded-full bg-white/18">
                  <Check className="h-3 w-3" />
                </span>
              ) : null}
            </button>
          ))}
        </div>
      </div>
    </>
  );
}
