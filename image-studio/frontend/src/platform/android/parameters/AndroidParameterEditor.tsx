import type { QualityValue } from "../../../types/domain";
import { availableQualityOptions } from "../../../components/panel/panelOptions";
import {
  RESOLUTION_PRESETS,
  sizeCapabilityHint,
  type AspectPreset,
  type AspectPresetOption,
  type ResolutionPreset,
} from "../../../components/panel/sizeCapabilities";
import { ANDROID_BATCH_COUNT_OPTIONS } from "./parameterOptions";
import {
  AndroidAspectGrid,
  AndroidDiscreteSlider,
  AndroidParameterBlock,
  AndroidParameterEditorShell,
  AndroidParameterSummary,
  AndroidSegmentedChoices,
  AndroidStyleChips,
  buildAndroidParameterSummaryItems,
} from "./AndroidParameterPrimitives";

export function AndroidParameterEditor({
  activeAspect,
  activeAspectLabel,
  aspectOptions,
  activeResolution,
  activeResolutionLabel,
  editAutoAspectComputedSizeLabel,
  exactSizeLabel,
  activeQualityLabel,
  activeStyleLabel,
  allowCustomAspectRatios,
  allowPreciseSizeControl,
  availableResolutions,
  apiMode,
  batchCount,
  effectiveEditAutoAspectResolution,
  handleEditAutoAspectResolutionSelect,
  handleEditAutoAspectToggle,
  handleAspectSelect,
  handleResolutionSelect,
  imageModelID,
  manualEditAutoAspectActive,
  mode,
  onOpenCustomAspectRatioModal,
  onOpenCustomSizeModal,
  quality,
  requestPolicy,
  setField,
  styleTag,
}: {
  activeAspect: AspectPreset | null;
  activeAspectLabel: string;
  aspectOptions: AspectPresetOption[];
  activeResolution: ResolutionPreset | null;
  activeResolutionLabel: string;
  editAutoAspectComputedSizeLabel?: string | null;
  exactSizeLabel?: string | null;
  activeQualityLabel: string;
  activeStyleLabel: string;
  allowCustomAspectRatios: boolean;
  allowPreciseSizeControl: boolean;
  availableResolutions: ResolutionPreset[];
  apiMode: "responses" | "images";
  batchCount: number;
  effectiveEditAutoAspectResolution: Exclude<ResolutionPreset, "auto">;
  handleEditAutoAspectResolutionSelect: (resolution: Exclude<ResolutionPreset, "auto">) => void;
  handleEditAutoAspectToggle: (enabled: boolean) => void;
  handleAspectSelect: (aspect: AspectPreset) => void;
  handleResolutionSelect: (resolution: ResolutionPreset) => void;
  imageModelID: string;
  manualEditAutoAspectActive: boolean;
  mode: "generate" | "edit";
  onOpenCustomAspectRatioModal: () => void;
  onOpenCustomSizeModal: () => void;
  quality: string;
  requestPolicy: "openai" | "compat";
  setField: (key: "quality" | "styleTag" | "batchCount", value: any) => void;
  styleTag: string;
}) {
  const resolutionHint = sizeCapabilityHint({ apiMode, requestPolicy, imageModelID });
  const summaryItems = buildAndroidParameterSummaryItems({
    activeAspectLabel,
    activeResolutionLabel,
    activeQualityLabel,
    batchCount,
  });
  const showRegularSizeControls = !manualEditAutoAspectActive;
  const editAutoResolutionOptions = RESOLUTION_PRESETS.filter(
    (item): item is { value: Exclude<ResolutionPreset, "auto">; label: string } =>
      item.value !== "auto" && availableResolutions.includes(item.value),
  );

  return (
    <AndroidParameterEditorShell
      summary={(
        <AndroidParameterSummary
          batchCount={batchCount}
          items={summaryItems}
          title={styleTag ? activeStyleLabel : "默认风格"}
        />
      )}
    >
      <AndroidParameterBlock
        title="风格"
        trailing={styleTag ? (
          <button type="button" onClick={() => setField("styleTag", "")}>清除</button>
        ) : null}
      >
        <AndroidStyleChips
          value={styleTag}
          onChange={(next) => setField("styleTag", next)}
        />
      </AndroidParameterBlock>

      {mode === "edit" ? (
        <AndroidParameterBlock title="源图尺寸策略">
          <AndroidSegmentedChoices
            columns={2}
            options={[
              { value: "manual", label: "沿用当前比例", hint: "继续使用普通比例和分辨率预设" },
              { value: "auto", label: "按源图自动适配", hint: "比例跟随源图，单独控制分辨率档位" },
            ]}
            value={manualEditAutoAspectActive ? "auto" : "manual"}
            onChange={(next) => handleEditAutoAspectToggle(next === "auto")}
          />
          {manualEditAutoAspectActive ? (
            <>
              <div className="mt-3">
                <AndroidSegmentedChoices
                  columns={3}
                  options={editAutoResolutionOptions}
                  value={effectiveEditAutoAspectResolution}
                  onChange={handleEditAutoAspectResolutionSelect}
                />
              </div>
              <p className="android-parameter-note mt-2">
                {editAutoAspectComputedSizeLabel
                  ? `当前会按源图比例自动换算，目标分辨率档位 ${effectiveEditAutoAspectResolution.toUpperCase()}，实际提交尺寸 ${editAutoAspectComputedSizeLabel}。`
                  : `开启后会按当前源图比例自动换算尺寸，并统一使用 ${effectiveEditAutoAspectResolution.toUpperCase()} 分辨率档位。`}
              </p>
            </>
          ) : (
            <p className="android-parameter-note mt-2">
              关闭后使用下面的普通比例和分辨率预设。
            </p>
          )}
        </AndroidParameterBlock>
      ) : null}

      {showRegularSizeControls ? (
        <>
          <AndroidParameterBlock title="画幅比例">
            <AndroidAspectGrid
              items={aspectOptions}
              onManageCustom={allowCustomAspectRatios ? onOpenCustomAspectRatioModal : undefined}
              value={activeAspect}
              onChange={handleAspectSelect}
            />
          </AndroidParameterBlock>

          <AndroidDiscreteSlider
            label="分辨率"
            value={activeResolution}
            displayValue={exactSizeLabel ?? undefined}
            options={RESOLUTION_PRESETS.filter((item) => availableResolutions.includes(item.value))}
            onChange={handleResolutionSelect}
            note={resolutionHint}
          />
        </>
      ) : (
        <AndroidParameterBlock title="尺寸控制">
          <p className="android-parameter-note">
            当前普通图生图已开启“按源图比例自动适配”，比例会跟随源图自动计算，分辨率由上面的源图尺寸策略统一控制。
          </p>
        </AndroidParameterBlock>
      )}

      {allowPreciseSizeControl ? (
        <AndroidParameterBlock
          title="精确尺寸"
          trailing={(
            <button type="button" onClick={onOpenCustomSizeModal}>
              {exactSizeLabel ? "修改" : "设置"}
            </button>
          )}
        >
          <p className="android-parameter-note">
            {exactSizeLabel
              ? `当前精确尺寸 ${exactSizeLabel}。点击比例或分辨率预设后会切回预设档位。`
              : "需要精确像素时可直接输入宽高，自定义 size 会原样下发给上游。"}
          </p>
        </AndroidParameterBlock>
      ) : null}

      <AndroidParameterBlock title="画面质量">
        <AndroidSegmentedChoices
          columns={2}
          options={availableQualityOptions(imageModelID).map((item) => ({ ...item, hint: qualityHint(item.value) }))}
          value={quality as QualityValue}
          onChange={(next) => setField("quality", next)}
        />
      </AndroidParameterBlock>

      <AndroidParameterBlock title="出图张数">
        <AndroidSegmentedChoices
          columns={3}
          options={ANDROID_BATCH_COUNT_OPTIONS}
          value={batchCount}
          onChange={(next) => setField("batchCount", next)}
        />
        <p className="android-parameter-note mt-2">
          多张会并行请求，完成后可在画板按网格挑图；实际效果仍受上游并发限制影响。
        </p>
      </AndroidParameterBlock>
    </AndroidParameterEditorShell>
  );
}

function qualityHint(value: QualityValue) {
  switch (value) {
    case "low":
      return "更快";
    case "medium":
      return "均衡";
    case "high":
      return "细节";
    case "standard":
      return "默认";
    case "hd":
      return "增强";
    default:
      return "上游";
  }
}
