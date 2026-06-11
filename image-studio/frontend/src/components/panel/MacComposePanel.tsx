import type { APIMode, AutoAspectResolutionPreset, BatchProcessConfig, EditSourceMode, Mode, QualityValue, RequestPolicy, SourceImage } from "../../types/domain";
import {
  type AspectPreset,
  type AspectPresetOption,
  type ResolutionPreset,
} from "./sizeCapabilities";
import { MacComposeSources } from "./MacComposeSources";
import { MacComposeStyleAndSize } from "./MacComposeStyleAndSize";

export function MacComposePanel({
  macComposeOpen,
  setMacComposeOpen,
  styleTag,
  activeStyleLabel,
  activeAspect,
  activeAspectLabel,
  aspectOptions,
  activeResolution,
  activeResolutionLabel,
  exactSizeLabel,
  activeQualityLabel,
  allowCustomAspectRatios,
  allowPreciseSizeControl,
  availableResolutions,
  batchCount,
  batchProcess,
  chooseBatchInputDir,
  chooseBatchInputFiles,
  chooseBatchOutputDir,
  mode,
  sources,
  currentImage,
  editSourceMode,
  editAutoAspectComputedSizeLabel,
  editAutoAspectResolution,
  effectiveEditAutoAspectResolution,
  apiMode,
  requestPolicy,
  imageModelID,
  setField,
  handleAspectSelect,
  handleEditAutoAspectResolutionSelect,
  handleEditAutoAspectToggle,
  handleResolutionSelect,
  onOpenCustomAspectRatioModal,
  onOpenCustomSizeModal,
  refreshBatchInputDir,
  selectSourceImage,
  clearSources,
  compareSourceOnCanvas,
  viewSourceOnCanvas,
  quality,
  qualityOptions,
  Seg,
  SegItem,
}: {
  macComposeOpen: boolean;
  setMacComposeOpen: React.Dispatch<React.SetStateAction<boolean>>;
  styleTag: string;
  activeStyleLabel: string;
  activeAspect: AspectPreset | null;
  activeAspectLabel: string;
  aspectOptions: AspectPresetOption[];
  activeResolution: ResolutionPreset | null;
  activeResolutionLabel: string;
  exactSizeLabel?: string | null;
  activeQualityLabel: string;
  allowCustomAspectRatios: boolean;
  allowPreciseSizeControl: boolean;
  availableResolutions: ResolutionPreset[];
  batchCount: number;
  batchProcess: BatchProcessConfig;
  chooseBatchInputDir: () => void;
  chooseBatchInputFiles: () => void;
  chooseBatchOutputDir: () => void;
  mode: Mode;
  sources: SourceImage[];
  currentImage: { savedPath?: string } | null;
  editSourceMode: EditSourceMode;
  editAutoAspectComputedSizeLabel?: string | null;
  editAutoAspectResolution: AutoAspectResolutionPreset;
  effectiveEditAutoAspectResolution: Exclude<ResolutionPreset, "auto">;
  apiMode: APIMode;
  requestPolicy: RequestPolicy;
  imageModelID: string;
  setField: (key: string, value: any) => void;
  handleAspectSelect: (aspect: AspectPreset) => void;
  handleEditAutoAspectResolutionSelect: (resolution: Exclude<ResolutionPreset, "auto">) => void;
  handleEditAutoAspectToggle: (enabled: boolean) => void;
  handleResolutionSelect: (resolution: ResolutionPreset) => void;
  onOpenCustomAspectRatioModal: () => void;
  onOpenCustomSizeModal: () => void;
  refreshBatchInputDir: () => void;
  selectSourceImage: () => void;
  clearSources: () => void;
  compareSourceOnCanvas: (index: number) => void;
  viewSourceOnCanvas: (index: number) => void;
  quality: QualityValue;
  qualityOptions: Array<{ value: QualityValue; label: string }>;
  Seg: (props: { children: React.ReactNode }) => React.ReactNode;
  SegItem: (props: { active: boolean; onClick: () => void; children: React.ReactNode }) => React.ReactNode;
}) {
  return (
    <section className="platform-card rounded-[22px] border border-black/[0.05] bg-white/70 p-4.5 shadow-[var(--shadow-card)] dark:border-white/[0.06] dark:bg-white/[0.03]">
      <button
        type="button"
        onClick={() => setMacComposeOpen((v) => !v)}
        className="flex w-full items-center justify-between text-left"
      >
        <div>
          <div className="text-[11px] uppercase tracking-[0.12em] text-zinc-400 dark:text-zinc-500">创作参数</div>
          <div className="mt-1.5 text-[13px] leading-6 text-zinc-600 dark:text-zinc-300">
            {styleTag ? `风格 ${activeStyleLabel}` : "默认风格"} · {activeAspectLabel} · {activeResolutionLabel} · {activeQualityLabel} · {batchCount} 张
          </div>
        </div>
        <span className="shrink-0 pl-3 text-[12px] text-zinc-500 dark:text-zinc-400">{macComposeOpen ? "收起 ▾" : "展开 ▸"}</span>
      </button>
      {macComposeOpen && (
        <div className="mt-4 flex flex-col gap-[18px]">
          <MacComposeStyleAndSize
            activeAspect={activeAspect}
            aspectOptions={aspectOptions}
            activeResolution={activeResolution}
            batchAutoAspectActive={mode === "edit" && editSourceMode === "batch" && batchProcess.autoAspectResolution !== ""}
            manualEditAutoAspectActive={mode === "edit" && editSourceMode === "manual" && editAutoAspectResolution !== ""}
            editAutoAspectComputedSizeLabel={editAutoAspectComputedSizeLabel}
            effectiveEditAutoAspectResolution={effectiveEditAutoAspectResolution}
            exactSizeLabel={exactSizeLabel}
            allowCustomAspectRatios={allowCustomAspectRatios}
            allowPreciseSizeControl={allowPreciseSizeControl}
            apiMode={apiMode}
            availableResolutions={availableResolutions}
            batchCount={batchCount}
            mode={mode}
            handleAspectSelect={handleAspectSelect}
            handleEditAutoAspectResolutionSelect={handleEditAutoAspectResolutionSelect}
            handleEditAutoAspectToggle={handleEditAutoAspectToggle}
            handleResolutionSelect={handleResolutionSelect}
            imageModelID={imageModelID}
            onOpenCustomAspectRatioModal={onOpenCustomAspectRatioModal}
            onOpenCustomSizeModal={onOpenCustomSizeModal}
            quality={quality}
            qualityOptions={qualityOptions}
            requestPolicy={requestPolicy}
            setField={setField}
            styleTag={styleTag}
            Seg={Seg}
            SegItem={SegItem}
          />

          {mode === "edit" && (
            <MacComposeSources
              batchProcess={batchProcess}
              chooseBatchInputDir={chooseBatchInputDir}
              chooseBatchInputFiles={chooseBatchInputFiles}
              chooseBatchOutputDir={chooseBatchOutputDir}
              clearSources={clearSources}
              compareSourceOnCanvas={compareSourceOnCanvas}
              currentImageSavedPath={currentImage?.savedPath ?? null}
              editSourceMode={editSourceMode}
              refreshBatchInputDir={refreshBatchInputDir}
              setField={setField}
              selectSourceImage={selectSourceImage}
              viewSourceOnCanvas={viewSourceOnCanvas}
              sources={sources}
            />
          )}
        </div>
      )}
    </section>
  );
}
