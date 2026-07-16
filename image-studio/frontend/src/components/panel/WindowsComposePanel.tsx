import { ChevronDown, ChevronRight } from "lucide-react";
import type { APIMode, AutoAspectResolutionPreset, BatchProcessConfig, EditSourceMode, Mode, QualityValue, RequestPolicy, SizeValue, SourceImage } from "../../types/domain";
import { DesktopComposeSections } from "./DesktopComposeSections";
import type { AspectPreset, AspectPresetOption, ResolutionPreset } from "./sizeCapabilities";

export function WindowsComposePanel({
  composeOpen,
  setComposeOpen,
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
  clearSources,
  currentImageSavedPath,
  editSourceMode,
  editAutoAspectComputedSizeLabel,
  editAutoAspectResolution,
  effectiveEditAutoAspectResolution,
  handleAspectSelect,
  handleEditAutoAspectResolutionSelect,
  handleEditAutoAspectToggle,
  handleResolutionSelect,
  imageModelID,
  onOpenCustomAspectRatioModal,
  onOpenCustomSizeModal,
  onRefreshBatchInputDir,
  mode,
  onPreviewSource,
  onRemoveSource,
  quality,
  qualityOptions,
  requestPolicy,
  selectSourceImage,
  setField,
  size,
  sources,
  apiMode,
}: {
  composeOpen: boolean;
  setComposeOpen: React.Dispatch<React.SetStateAction<boolean>>;
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
  clearSources: () => void;
  currentImageSavedPath?: string | null;
  editSourceMode: EditSourceMode;
  editAutoAspectComputedSizeLabel?: string | null;
  editAutoAspectResolution: AutoAspectResolutionPreset;
  effectiveEditAutoAspectResolution: Exclude<ResolutionPreset, "auto">;
  handleAspectSelect: (aspect: AspectPreset) => void;
  handleEditAutoAspectResolutionSelect: (resolution: Exclude<ResolutionPreset, "auto">) => void;
  handleEditAutoAspectToggle: (enabled: boolean) => void;
  handleResolutionSelect: (resolution: ResolutionPreset) => void;
  imageModelID: string;
  onOpenCustomAspectRatioModal: () => void;
  onOpenCustomSizeModal: () => void;
  onRefreshBatchInputDir: () => void;
  mode: Mode;
  onPreviewSource: (index: number) => void;
  onRemoveSource: (index: number) => void;
  quality: QualityValue;
  qualityOptions: Array<{ value: QualityValue; label: string }>;
  requestPolicy: RequestPolicy;
  selectSourceImage: () => void;
  setField: (key: "styleTag" | "quality" | "batchCount" | "size" | "editAutoAspectResolution", value: any) => void;
  size: SizeValue;
  sources: SourceImage[];
  apiMode: APIMode;
}) {
  const sourceLabel = mode === "edit"
    ? sources.length > 0
      ? `${sources.length} 张源图`
      : currentImageSavedPath
        ? "画板图作源图"
        : "未添加源图"
    : "文生图";
  const summary = [
    styleTag ? activeStyleLabel : "默认风格",
    activeAspectLabel,
    activeResolutionLabel,
    activeQualityLabel,
    `${batchCount} 张`,
    sourceLabel,
  ].join(" · ");

  return (
    <section className="platform-card windows-compose-panel">
      <button
        type="button"
        onClick={() => setComposeOpen((value) => !value)}
        className="windows-compose-toggle"
      >
        <span className="min-w-0">
          <span className="windows-compose-title">创作参数</span>
          <span className="windows-compose-summary">{summary}</span>
        </span>
        <span className="windows-compose-state">
          {composeOpen ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
          {composeOpen ? "收起" : "展开"}
        </span>
      </button>

      {composeOpen ? (
        <div className="windows-compose-body">
          <DesktopComposeSections
            activeAspect={activeAspect}
            aspectOptions={aspectOptions}
            activeResolution={activeResolution}
            exactSizeLabel={exactSizeLabel}
            allowCustomAspectRatios={allowCustomAspectRatios}
            allowPreciseSizeControl={allowPreciseSizeControl}
            apiMode={apiMode}
            availableResolutions={availableResolutions}
            batchCount={batchCount}
            batchProcess={batchProcess}
            chooseBatchInputDir={chooseBatchInputDir}
            chooseBatchInputFiles={chooseBatchInputFiles}
            chooseBatchOutputDir={chooseBatchOutputDir}
          clearSources={clearSources}
          currentImageSavedPath={currentImageSavedPath}
          editSourceMode={editSourceMode}
          editAutoAspectComputedSizeLabel={editAutoAspectComputedSizeLabel}
          editAutoAspectResolution={editAutoAspectResolution}
          effectiveEditAutoAspectResolution={effectiveEditAutoAspectResolution}
          handleAspectSelect={handleAspectSelect}
          handleEditAutoAspectResolutionSelect={handleEditAutoAspectResolutionSelect}
          handleEditAutoAspectToggle={handleEditAutoAspectToggle}
          handleResolutionSelect={handleResolutionSelect}
            imageModelID={imageModelID}
            onOpenCustomAspectRatioModal={onOpenCustomAspectRatioModal}
            onOpenCustomSizeModal={onOpenCustomSizeModal}
            onRefreshBatchInputDir={onRefreshBatchInputDir}
            mode={mode}
            onPreviewSource={onPreviewSource}
            onRemoveSource={onRemoveSource}
            quality={quality}
            qualityOptions={qualityOptions}
            requestPolicy={requestPolicy}
            selectSourceImage={selectSourceImage}
            setField={setField}
            size={size}
            sources={sources}
            styleTag={styleTag}
            usesFluentUI
          />
        </div>
      ) : null}
    </section>
  );
}
