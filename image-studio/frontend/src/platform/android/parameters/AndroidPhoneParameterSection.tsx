import type { Dispatch, SetStateAction } from "react";
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

export function AndroidPhoneParameterSection({
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
  parametersOpen,
  quality,
  requestPolicy,
  setField,
  setParametersOpen,
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
  parametersOpen: boolean;
  quality: string;
  requestPolicy: "openai" | "compat";
  setField: (key: "quality" | "styleTag" | "batchCount", value: any) => void;
  setParametersOpen: Dispatch<SetStateAction<boolean>>;
  styleTag: string;
}) {
  const toggleParameters = () => {
    vibrateForPlatform(8);
    setParametersOpen((current) => !current);
  };
  const summaryItems = buildAndroidParameterSummaryItems({
    activeAspectLabel,
    activeResolutionLabel,
    activeQualityLabel,
    batchCount,
  });

  return (
    <section className="platform-card android-parameter-card android-phone-parameter-card">
      <AndroidParameterSummary
        batchCount={batchCount}
        items={summaryItems}
        onClick={toggleParameters}
        open={parametersOpen}
        title={styleTag ? activeStyleLabel : "默认风格"}
      />

      <Modal
        open={parametersOpen}
        onClose={() => setParametersOpen(false)}
        title="创作参数"
        width={720}
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
