import { useState } from "react";
import { Settings } from "lucide-react";
import {
  type AspectPreset,
  type AspectPresetOption,
  type ResolutionPreset,
} from "../../../components/panel/sizeCapabilities";
import { Modal } from "../../../components/common/Modal";
import { vibrateForPlatform } from "../bridge";
import {
  AndroidParameterSummary,
  buildAndroidParameterSummaryItems,
} from "./AndroidParameterPrimitives";
import { AndroidParameterEditor } from "./AndroidParameterEditor";

export function AndroidPadParameterSection({
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
  isMediumPad,
  manualEditAutoAspectActive,
  mode,
  needsUpstreamSetup,
  onOpenCustomAspectRatioModal,
  onOpenCustomSizeModal,
  onOpenUpstream,
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
  isMediumPad: boolean;
  manualEditAutoAspectActive: boolean;
  mode: "generate" | "edit";
  needsUpstreamSetup: boolean;
  onOpenCustomAspectRatioModal: () => void;
  onOpenCustomSizeModal: () => void;
  onOpenUpstream: () => void;
  quality: string;
  requestPolicy: "openai" | "compat";
  setField: (key: "quality" | "batchCount" | "styleTag", value: any) => void;
  styleTag: string;
}) {
  const [parametersOpen, setParametersOpen] = useState(false);

  const openParameters = () => {
    vibrateForPlatform(8);
    setParametersOpen(true);
  };
  const summaryItems = buildAndroidParameterSummaryItems({
    activeAspectLabel,
    activeResolutionLabel,
    activeQualityLabel,
    batchCount,
  });

  return (
    <section className={`platform-card android-parameter-card android-pad-parameter-card ${isMediumPad ? "medium" : "expanded"}`}>
      <div className="android-pad-parameter-head">
        <AndroidParameterSummary
          batchCount={batchCount}
          items={summaryItems}
          title={activeStyleLabel}
        />
        <div className="android-pad-parameter-actions">
          <button
            type="button"
            onClick={openParameters}
            className="android-parameter-upstream-button"
          >
            编辑参数
          </button>
          {needsUpstreamSetup ? (
          <button
            type="button"
            onClick={onOpenUpstream}
            className="android-parameter-upstream-button"
          >
            <Settings className="h-4 w-4" />
            打开设置
          </button>
          ) : null}
        </div>
      </div>

      <Modal
        open={parametersOpen}
        onClose={() => setParametersOpen(false)}
        title="创作参数"
        width={780}
      >
        <AndroidParameterEditor
          activeAspect={activeAspect}
          activeAspectLabel={activeAspectLabel}
          aspectOptions={aspectOptions}
          activeResolution={activeResolution}
          activeResolutionLabel={activeResolutionLabel}
          editAutoAspectComputedSizeLabel={editAutoAspectComputedSizeLabel}
          exactSizeLabel={exactSizeLabel}
          activeQualityLabel={activeQualityLabel}
          activeStyleLabel={activeStyleLabel}
          allowCustomAspectRatios={allowCustomAspectRatios}
          allowPreciseSizeControl={allowPreciseSizeControl}
          availableResolutions={availableResolutions}
          apiMode={apiMode}
          batchCount={batchCount}
          effectiveEditAutoAspectResolution={effectiveEditAutoAspectResolution}
          handleEditAutoAspectResolutionSelect={handleEditAutoAspectResolutionSelect}
          handleEditAutoAspectToggle={handleEditAutoAspectToggle}
          handleAspectSelect={handleAspectSelect}
          handleResolutionSelect={handleResolutionSelect}
          imageModelID={imageModelID}
          manualEditAutoAspectActive={manualEditAutoAspectActive}
          mode={mode}
          onOpenCustomAspectRatioModal={onOpenCustomAspectRatioModal}
          onOpenCustomSizeModal={onOpenCustomSizeModal}
          quality={quality}
          requestPolicy={requestPolicy}
          setField={setField}
          styleTag={styleTag}
        />
      </Modal>
    </section>
  );
}
