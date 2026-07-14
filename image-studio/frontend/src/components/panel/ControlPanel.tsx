import { useState } from "react";
import { useStudioStore } from "../../state/studioStore";
import { SizeValue, QualityValue, Mode } from "../../types/domain";
import { usePlatform } from "../../platform/context";
import { ChooseDirectory } from "../../platform/runtime/host";
import { AndroidPhoneComposePanel } from "../../platform/android/AndroidPhoneComposePanel";
import { AndroidPadComposePanel } from "../../platform/android/AndroidPadComposePanel";
import { DesktopAdvancedPanel } from "./DesktopAdvancedPanel";
import { ErrorNotice } from "./ErrorNotice";
import { DesktopComposeSections } from "./DesktopComposeSections";
import { LoopGenerationSection } from "./LoopGenerationSection";
import { MacAdvancedPanel } from "./MacAdvancedPanel";
import { MacComposePanel } from "./MacComposePanel";
import { availableQualityOptions, normalizeQualitySelection, STYLE_CHIPS } from "./panelOptions";
import { PromptEditorSection } from "./PromptEditorSection";
import { Section, Seg, SegItem } from "./panelChrome";
import { SubmitBar } from "./SubmitBar";
import { WindowsComposePanel } from "./WindowsComposePanel";
import {
  RESOLUTION_PRESETS,
  aspectPresetLabel,
  availableResolutionPresets,
  buildAspectSizeSelection,
  buildReferenceAspectRatio,
  buildReferenceResolutionSizeSelection,
  buildResolutionSizeSelection,
  deriveExactSizeSelection,
  deriveAspectPreset,
  deriveResolutionPreset,
  formatSizeValue,
  listAspectPresetOptions,
  normalizeResolutionSelection,
  normalizeSizeSelection,
  supportsCustomAspectRatios,
  supportsPreciseSizeControl,
} from "./sizeCapabilities";

export function ControlPanel({
  onAndroidSubmitStart,
}: {
  onAndroidSubmitStart?: () => void;
} = {}) {
  const {
    apiKey, mode, prompt, background, imageStyle, inputFidelity, moderation, negativePrompt, outputCompression, size, quality, seed, styleTag,
    userIdentifier, partialImages,
    outputFormat, batchCount, editSourceMode, editAutoAspectResolution, batchProcess, loopGeneration,
    sources, currentImage,
    errorMessage, errorCanRetry, errorRawPath, isRunning, lastPayload, isTestingKey, isOptimizingPrompt, isInferringPrompt,
    apiMode, requestPolicy, baseURL, profiles, aiProfileId, imageModelID,
    customAspectRatios,
    setField, clearError, pushToast,
    selectSourceImage, chooseBatchInputDir, chooseBatchInputFiles, refreshBatchInputDir, removeSource, clearSources, viewSourceOnCanvas,
    compareSourceOnCanvas,
    openCustomAspectRatioModal,
    openCustomSizeModal,
    openUpstreamConfig,
    submit, cancel, retryLast, optimizePrompt, inferPromptFromCanvas,
  } = useStudioStore();
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [promptPopover, setPromptPopover] = useState(false);
  const [macComposeOpen, setMacComposeOpen] = useState(false);
  const [windowsComposeOpen, setWindowsComposeOpen] = useState(false);
  const { isAndroid, isAndroidPhone, isAndroidPad, isMac, isWindows, usesAndroidUI, usesAppleUI, usesFluentUI } = usePlatform();

  if (isAndroidPhone) {
    return <AndroidPhoneComposePanel onSubmitStart={onAndroidSubmitStart} />;
  }

  if (isAndroidPad) {
    return <AndroidPadComposePanel onSubmitStart={onAndroidSubmitStart} />;
  }

  const promptLen = prompt.length;
  const aiProfile = profiles.find((profile) => profile.id === aiProfileId && profile.apiMode === "responses");
  const aiReady = !!aiProfile?.baseURL.trim();
  const capabilityInput = { apiMode, requestPolicy, imageModelID };
  const normalizedSize = normalizeSizeSelection(size, capabilityInput, customAspectRatios);
  const normalizedQuality = normalizeQualitySelection(quality, imageModelID);
  const qualityOptions = availableQualityOptions(imageModelID);
  const allowCustomAspectRatios = supportsCustomAspectRatios(capabilityInput);
  const allowPreciseSizeControl = supportsPreciseSizeControl(capabilityInput);
  const referenceDimensions = sources[0]?.previewWidth && sources[0]?.previewHeight
    ? { width: sources[0].previewWidth, height: sources[0].previewHeight }
    : currentImage?.previewWidth && currentImage?.previewHeight
      ? { width: currentImage.previewWidth, height: currentImage.previewHeight }
      : null;
  const referenceAspectRatio = referenceDimensions
    ? buildReferenceAspectRatio(referenceDimensions.width, referenceDimensions.height, customAspectRatios)
    : null;
  const sizingAspectRatios = referenceAspectRatio && !customAspectRatios.some((item) => item.id === referenceAspectRatio.id)
    ? [...customAspectRatios, referenceAspectRatio]
    : customAspectRatios;
  const activeStyleLabel = STYLE_CHIPS.find((item) => item.id === styleTag)?.label ?? styleTag;
  const aspectOptions = listAspectPresetOptions(capabilityInput, sizingAspectRatios);
  const exactSize = deriveExactSizeSelection(normalizedSize, capabilityInput, sizingAspectRatios);
  const derivedAspect = deriveAspectPreset(normalizedSize, sizingAspectRatios);
  const derivedResolution = deriveResolutionPreset(normalizedSize);
  const normalizedEditAutoAspectResolution = normalizeResolutionSelection(editAutoAspectResolution || "1k", capabilityInput);
  const effectiveEditAutoAspectResolution = normalizedEditAutoAspectResolution === "auto"
    ? "1k"
    : normalizedEditAutoAspectResolution;
  const manualEditAutoAspectActive = mode === "edit" && editSourceMode === "manual" && editAutoAspectResolution !== "";
  const editAutoAspectComputedSizeLabel = manualEditAutoAspectActive && referenceDimensions
    ? formatSizeValue(buildReferenceResolutionSizeSelection(
        effectiveEditAutoAspectResolution,
        referenceDimensions,
        capabilityInput,
        customAspectRatios,
      ))
    : null;
  const activeAspect = exactSize ? null : derivedAspect;
  const activeResolution = exactSize ? null : derivedResolution;
  const activeAspectLabel = manualEditAutoAspectActive
    ? "按源图自动适配"
    : exactSize
      ? "精确尺寸"
      : aspectPresetLabel(derivedAspect, sizingAspectRatios);
  const activeResolutionLabel = manualEditAutoAspectActive
    ? effectiveEditAutoAspectResolution.toUpperCase()
    : exactSize
    ? exactSize.label
    : (RESOLUTION_PRESETS.find((item) => item.value === derivedResolution)?.label ?? derivedResolution);
  const activeQualityLabel = qualityOptions.find((item) => item.value === normalizedQuality)?.label ?? normalizedQuality;
  const availableResolutions = availableResolutionPresets(capabilityInput);
  const optimizeReady = !!(
    prompt.trim() && aiReady
  );
  const compactMacCompose = isMac;
  const compactWindowsCompose = isWindows;
  const advancedSummary = [
    negativePrompt.trim() ? "已填负向提示词" : "无负向限制",
    outputFormat.toUpperCase(),
    `背景 ${background}`,
    outputFormat === "png" ? null : `压缩 ${outputCompression}`,
    inputFidelity === "auto" ? null : `保真 ${inputFidelity}`,
    imageStyle === "default" ? null : `图风 ${imageStyle}`,
    `审核 ${moderation}`,
    `预览 ${partialImages === 0 ? "仅最终图" : `${partialImages} 帧`}`,
    userIdentifier.trim() ? "用户标识 已填" : null,
    seed > 0 ? `Seed ${seed}` : "随机 Seed",
  ].filter(Boolean).join(" · ");
  const submitLabel = loopGeneration.enabled
    ? (mode === "edit" ? "循环编辑" : "循环生成")
    : (mode === "edit" ? "编辑" : "生成");

  function handleAspectSelect(aspect: typeof activeAspect) {
    setField("size", buildAspectSizeSelection(
      aspect ?? derivedAspect,
      derivedResolution,
      capabilityInput,
      customAspectRatios,
    ));
  }

  function handleResolutionSelect(resolution: typeof activeResolution) {
    const referenceAspectPreset = referenceDimensions
      ? deriveAspectPreset(
          `${referenceDimensions.width}x${referenceDimensions.height}` as SizeValue,
          sizingAspectRatios,
        )
      : null;
    setField("size", buildResolutionSizeSelection(
      derivedAspect,
      resolution ?? derivedResolution,
      capabilityInput,
      sizingAspectRatios,
      referenceAspectPreset,
    ));
  }

  function handleEditAutoAspectToggle(enabled: boolean) {
    setField(
      "editAutoAspectResolution",
      enabled ? effectiveEditAutoAspectResolution : "",
    );
  }

  function handleEditAutoAspectResolutionSelect(resolution: typeof effectiveEditAutoAspectResolution) {
    setField("editAutoAspectResolution", resolution);
  }

  async function chooseBatchOutputDir() {
    const chosen = await ChooseDirectory("选择批处理输出目录").catch(() => "");
    if (!chosen) return;
    setField("batchProcess" as any, { ...batchProcess, outputDir: chosen, outputMode: "custom_dir" });
  }

  return (
    <div className={`control-panel box-border flex shrink-0 flex-col overflow-y-auto border-r border-[var(--border)] bg-[var(--sidebar)] backdrop-blur-2xl ${usesAppleUI ? "liquid-sidebar" : ""} ${usesAndroidUI ? "android-surface-pane" : ""} ${isMac ? "w-[408px] gap-5 px-6 py-5" : "w-[372px] gap-4 px-5 py-4"} ${usesFluentUI ? "pt-3" : ""}`}>
      <section className={`platform-card ${isMac ? "px-5 py-5" : "px-4 py-4"}`}>
        <div className="flex items-start justify-between gap-3">
          <div>
            <h2
              className={`text-zinc-900 dark:text-zinc-100 ${usesFluentUI ? "text-[18px] font-semibold tracking-[0]" : "text-[20px] font-semibold tracking-[-0.02em]"}`}
              style={{ fontFamily: "var(--title-font)" }}
            >
              图像工作台
            </h2>
            {!isAndroid && (
              <p className={`${isMac ? "mt-1 text-[12px] leading-6" : "mt-0.5 text-[11px] leading-relaxed"} text-zinc-500 dark:text-zinc-400`}>
                保持界面简洁，把注意力留给 prompt、参考图和结果。
              </p>
            )}
            {isMac && (
              <div className="mt-3">
                <div className="mb-2 text-[11px] uppercase tracking-[0.12em] text-zinc-400 dark:text-zinc-500">模式</div>
                <Seg>
                  {(["generate", "edit"] as Mode[]).map((m) => (
                    <SegItem
                      key={m}
                      active={mode === m}
                      onClick={() => setField("mode", m)}
                    >
                      {m === "generate" ? "📝 文生图" : "🖼 图生图"}
                    </SegItem>
                  ))}
                </Seg>
              </div>
            )}
          </div>
          {!isMac && (
            <div className={`platform-pill bg-[var(--accent-soft)] px-2.5 py-1 text-[11px] font-medium text-[var(--accent)] ${usesFluentUI ? "rounded-[8px]" : "rounded-2xl"}`}>
              {mode === "edit" ? "图生图" : "文生图"}
            </div>
          )}
        </div>
      </section>

      {errorMessage ? (
        <ErrorNotice
          errorMessage={errorMessage}
          errorRawPath={errorRawPath}
          showRetry={!!(errorCanRetry && lastPayload && !isRunning)}
          onRetry={retryLast}
          onClear={clearError}
          onPushToast={pushToast}
        />
      ) : null}

      {!isMac && (
        <Section label="模式">
          <Seg>
            {(["generate", "edit"] as Mode[]).map((m) => (
              <SegItem
                key={m}
                active={mode === m}
                onClick={() => setField("mode", m)}
              >
                {m === "generate" ? "📝 文生图" : "🖼 图生图"}
              </SegItem>
            ))}
          </Seg>
        </Section>
      )}

      <PromptEditorSection
        mode={mode}
        prompt={prompt}
        promptLen={promptLen}
        promptPopover={promptPopover}
        setPromptPopover={setPromptPopover}
        optimizeReady={optimizeReady}
        inferReady={!!currentImage && aiReady}
        isOptimizingPrompt={isOptimizingPrompt}
        isInferringPrompt={isInferringPrompt}
        onSetPrompt={(value) => setField("prompt", value)}
        onOptimizePrompt={optimizePrompt}
        onInferPrompt={inferPromptFromCanvas}
      />

      {!compactMacCompose && !compactWindowsCompose ? (
        <DesktopComposeSections
          activeAspect={activeAspect}
          aspectOptions={aspectOptions}
          activeResolution={activeResolution}
          exactSizeLabel={exactSize?.label ?? null}
          apiMode={apiMode}
          availableResolutions={availableResolutions}
          batchCount={batchCount}
          batchProcess={batchProcess}
          chooseBatchInputDir={chooseBatchInputDir}
          chooseBatchInputFiles={chooseBatchInputFiles}
          chooseBatchOutputDir={chooseBatchOutputDir}
          clearSources={clearSources}
          currentImageSavedPath={currentImage?.savedPath ?? null}
          editSourceMode={editSourceMode}
          editAutoAspectResolution={editAutoAspectResolution}
          effectiveEditAutoAspectResolution={effectiveEditAutoAspectResolution}
          editAutoAspectComputedSizeLabel={editAutoAspectComputedSizeLabel}
          handleAspectSelect={handleAspectSelect}
          handleEditAutoAspectResolutionSelect={handleEditAutoAspectResolutionSelect}
          handleEditAutoAspectToggle={handleEditAutoAspectToggle}
          handleResolutionSelect={handleResolutionSelect}
          imageModelID={imageModelID}
          allowCustomAspectRatios={allowCustomAspectRatios}
          allowPreciseSizeControl={allowPreciseSizeControl}
          onOpenCustomAspectRatioModal={openCustomAspectRatioModal}
          onOpenCustomSizeModal={openCustomSizeModal}
          onRefreshBatchInputDir={refreshBatchInputDir}
          usesFluentUI={usesFluentUI}
          mode={mode}
          onPreviewSource={(index) => void viewSourceOnCanvas(index)}
          onRemoveSource={removeSource}
          quality={normalizedQuality}
          qualityOptions={qualityOptions}
          requestPolicy={requestPolicy}
          selectSourceImage={selectSourceImage}
          setField={setField as any}
          size={size}
          sources={sources}
          styleTag={styleTag}
        />
      ) : null}

      {compactWindowsCompose ? (
        <WindowsComposePanel
          composeOpen={windowsComposeOpen}
          setComposeOpen={setWindowsComposeOpen}
          styleTag={styleTag}
          activeStyleLabel={activeStyleLabel}
          activeAspect={activeAspect}
          activeAspectLabel={activeAspectLabel}
          aspectOptions={aspectOptions}
          activeResolution={activeResolution}
          activeResolutionLabel={activeResolutionLabel}
          exactSizeLabel={exactSize?.label ?? null}
          activeQualityLabel={activeQualityLabel}
          availableResolutions={availableResolutions}
          batchCount={batchCount}
          batchProcess={batchProcess}
          chooseBatchInputDir={chooseBatchInputDir}
          chooseBatchInputFiles={chooseBatchInputFiles}
          chooseBatchOutputDir={chooseBatchOutputDir}
          clearSources={clearSources}
          currentImageSavedPath={currentImage?.savedPath ?? null}
          editSourceMode={editSourceMode}
          editAutoAspectResolution={editAutoAspectResolution}
          effectiveEditAutoAspectResolution={effectiveEditAutoAspectResolution}
          editAutoAspectComputedSizeLabel={editAutoAspectComputedSizeLabel}
          handleAspectSelect={handleAspectSelect}
          handleEditAutoAspectResolutionSelect={handleEditAutoAspectResolutionSelect}
          handleEditAutoAspectToggle={handleEditAutoAspectToggle}
          handleResolutionSelect={handleResolutionSelect}
          imageModelID={imageModelID}
          allowCustomAspectRatios={allowCustomAspectRatios}
          allowPreciseSizeControl={allowPreciseSizeControl}
          onOpenCustomAspectRatioModal={openCustomAspectRatioModal}
          onOpenCustomSizeModal={openCustomSizeModal}
          onRefreshBatchInputDir={refreshBatchInputDir}
          mode={mode}
          onPreviewSource={(index) => void viewSourceOnCanvas(index)}
          onRemoveSource={removeSource}
          quality={normalizedQuality}
          qualityOptions={qualityOptions}
          requestPolicy={requestPolicy}
          selectSourceImage={selectSourceImage}
          setField={setField as any}
          size={size}
          sources={sources}
          apiMode={apiMode}
        />
      ) : null}

      {compactMacCompose && (
          <MacComposePanel
          macComposeOpen={macComposeOpen}
          setMacComposeOpen={setMacComposeOpen}
          styleTag={styleTag}
          activeStyleLabel={activeStyleLabel}
          activeAspect={activeAspect}
          activeAspectLabel={activeAspectLabel}
          aspectOptions={aspectOptions}
          activeResolution={activeResolution}
          activeResolutionLabel={activeResolutionLabel}
          exactSizeLabel={exactSize?.label ?? null}
          activeQualityLabel={activeQualityLabel}
          availableResolutions={availableResolutions}
          batchCount={batchCount}
          batchProcess={batchProcess}
          chooseBatchInputDir={chooseBatchInputDir}
          chooseBatchInputFiles={chooseBatchInputFiles}
          chooseBatchOutputDir={chooseBatchOutputDir}
          mode={mode}
          sources={sources}
          currentImage={currentImage}
          editSourceMode={editSourceMode}
          editAutoAspectResolution={editAutoAspectResolution}
          effectiveEditAutoAspectResolution={effectiveEditAutoAspectResolution}
          editAutoAspectComputedSizeLabel={editAutoAspectComputedSizeLabel}
          apiMode={apiMode}
          requestPolicy={requestPolicy}
          imageModelID={imageModelID}
          setField={setField as any}
          handleAspectSelect={handleAspectSelect}
          handleEditAutoAspectResolutionSelect={handleEditAutoAspectResolutionSelect}
          handleEditAutoAspectToggle={handleEditAutoAspectToggle}
          handleResolutionSelect={handleResolutionSelect}
          allowCustomAspectRatios={allowCustomAspectRatios}
          allowPreciseSizeControl={allowPreciseSizeControl}
          onOpenCustomAspectRatioModal={openCustomAspectRatioModal}
          onOpenCustomSizeModal={openCustomSizeModal}
          selectSourceImage={selectSourceImage}
          refreshBatchInputDir={refreshBatchInputDir}
            clearSources={clearSources}
            compareSourceOnCanvas={(index) => void compareSourceOnCanvas(index)}
            viewSourceOnCanvas={(index) => void viewSourceOnCanvas(index)}
          quality={normalizedQuality}
          qualityOptions={qualityOptions}
          Seg={Seg as any}
          SegItem={SegItem as any}
        />
      )}

      {/* 高级参数(可折叠)*/}
      {isMac ? (
        <MacAdvancedPanel
          advancedOpen={advancedOpen}
          advancedSummary={advancedSummary}
          background={background}
          imageStyle={imageStyle}
          inputFidelity={inputFidelity}
          moderation={moderation}
          negativePrompt={negativePrompt}
          outputCompression={outputCompression}
          outputFormat={outputFormat}
          userIdentifier={userIdentifier}
          partialImages={partialImages}
          seed={seed}
          setAdvancedOpen={setAdvancedOpen}
          setField={setField as any}
        />
      ) : (
        <DesktopAdvancedPanel
          advancedOpen={advancedOpen}
          advancedSummary={advancedSummary}
          background={background}
          imageStyle={imageStyle}
          inputFidelity={inputFidelity}
          moderation={moderation}
          negativePrompt={negativePrompt}
          outputCompression={outputCompression}
          outputFormat={outputFormat}
          userIdentifier={userIdentifier}
          partialImages={partialImages}
          seed={seed}
          setAdvancedOpen={setAdvancedOpen}
          setField={setField as any}
        />
      )}

      <LoopGenerationSection
        value={loopGeneration}
        onChange={(next) => setField("loopGeneration", next)}
      />

      <SubmitBar
        apiKey={apiKey}
        baseURL={baseURL}
        prompt={prompt}
        isRunning={isRunning}
        submitLabel={submitLabel}
        onOpenUpstreamConfig={() => openUpstreamConfig("app")}
        onCancel={cancel}
        onSubmit={submit}
      />
    </div>
  );
}
