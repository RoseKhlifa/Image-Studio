import { useState } from "react";
import {
  CheckCircle2, ImagePlus, ListPlus, Sparkles,
} from "lucide-react";
import { useStudioStore } from "../../state/studioStore";
import { availableQualityOptions, normalizeQualitySelection, STYLE_CHIPS } from "../../components/panel/panelOptions";
import type { Mode, SizeValue } from "../../types/domain";
import { AndroidModeSwitch } from "./AndroidModeSwitch";
import { usePlatform } from "../context";
import { vibrateForPlatform } from "./bridge";
import { AndroidAdvancedSection } from "./AndroidAdvancedSection";
import { AndroidPadParameterSection } from "./AndroidPadParameterSection";
import { AndroidPadSourceSection } from "./AndroidPadSourceSection";
import { AndroidPromptTemplateModal } from "./AndroidPromptTemplateModal";
import { PromptTemplateManagerModal } from "../../components/panel/PromptTemplateManagerModal";
import { ParameterPresetsSection } from "../../components/panel/ParameterPresetsSection";
import { appendPromptTemplateText } from "../../lib/promptTemplateInsert";
import {
  aspectPresetLabel,
  availableResolutionPresets,
  buildReferenceAspectRatio,
  buildReferenceResolutionSizeSelection,
  deriveExactSizeSelection,
  deriveAspectPreset,
  deriveResolutionPreset,
  formatSizeValue,
  listAspectPresetOptions,
  normalizeResolutionSelection,
  normalizeSizeSelection,
  supportsCustomAspectRatios,
  supportsPreciseSizeControl,
} from "../../components/panel/sizeCapabilities";
import {
  buildAndroidAspectSizeSelection,
  buildAndroidResolutionSizeSelection,
} from "./parameters/androidSizeSelection";
import { LoopGenerationSection } from "../../components/panel/LoopGenerationSection";

export function AndroidPadComposePanel({
  onSubmitStart,
}: {
  onSubmitStart?: () => void;
} = {}) {
  const {
    apiKey, mode, prompt, background, imageStyle, inputFidelity, moderation, negativePrompt, outputCompression, size, quality, seed, styleTag, outputFormat,
    userIdentifier, partialImages,
    batchCount, editAutoAspectResolution, loopGeneration, sources, currentImage, isRunning, isOptimizingPrompt, apiMode, requestPolicy, baseURL, imageModelID,
    profiles, customAspectRatios, setField, selectSourceImage, removeSource, clearSources, viewSourceOnCanvas,
    compareSourceOnCanvas,
    openCustomAspectRatioModal, openCustomSizeModal,
    openUpstreamConfig, submit, cancel, optimizePrompt,
  } = useStudioStore();
  const [templateOpen, setTemplateOpen] = useState(false);
  const [templateManagerOpen, setTemplateManagerOpen] = useState(false);
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const promptTemplates = useStudioStore((s) => s.promptTemplates);
  const promptLen = prompt.length;
  const { androidOrientation, androidWidthClass } = usePlatform();
  const isMediumPad = androidWidthClass === "medium";
  const isLandscapePad = androidOrientation === "landscape";
  const needsUpstreamSetup = !apiKey.trim() || !baseURL.trim();
  const hasUsableResponsesProfile = profiles.some(
    (p) => p.apiMode === "responses" && p.baseURL.trim(),
  );
  const optimizeReady = !!(
    prompt.trim() && (hasUsableResponsesProfile || (apiKey.trim() && baseURL.trim()))
  );
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
  const activeStyleLabel = STYLE_CHIPS.find((item) => item.id === styleTag)?.label ?? "默认风格";
  const aspectOptions = listAspectPresetOptions(capabilityInput, sizingAspectRatios);
  const exactSize = deriveExactSizeSelection(normalizedSize, capabilityInput, sizingAspectRatios);
  const derivedAspect = deriveAspectPreset(normalizedSize, sizingAspectRatios);
  const derivedResolution = deriveResolutionPreset(normalizedSize);
  const normalizedEditAutoAspectResolution = normalizeResolutionSelection(editAutoAspectResolution || "1k", capabilityInput);
  const effectiveEditAutoAspectResolution = normalizedEditAutoAspectResolution === "auto"
    ? "1k"
    : normalizedEditAutoAspectResolution;
  const manualEditAutoAspectActive = mode === "edit" && editAutoAspectResolution !== "";
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
  const availableResolutions = availableResolutionPresets(capabilityInput);
  const activeAspectLabel = manualEditAutoAspectActive
    ? "按源图自动适配"
    : exactSize
      ? "精确尺寸"
      : aspectPresetLabel(derivedAspect, sizingAspectRatios);
  const activeResolutionLabel = manualEditAutoAspectActive
    ? effectiveEditAutoAspectResolution.toUpperCase()
    : exactSize
      ? exactSize.label
      : (derivedResolution === "auto" ? "自动" : derivedResolution.toUpperCase());
  const activeQualityLabel = qualityOptions.find((item) => item.value === normalizedQuality)?.label ?? normalizedQuality;
  const editSourceLabel = sources.length > 0 ? `${sources.length} 张已添加` : currentImage?.savedPath ? "使用当前画板" : "未添加";

  const handleAspectSelect = (aspect: typeof activeAspect) => {
    setField("size", buildAndroidAspectSizeSelection(
      aspect ?? derivedAspect,
      derivedResolution,
      capabilityInput,
      sizingAspectRatios,
    ));
  };

  const handleResolutionSelect = (resolution: typeof activeResolution) => {
    const referenceAspectPreset = referenceDimensions
      ? deriveAspectPreset(
          `${referenceDimensions.width}x${referenceDimensions.height}` as SizeValue,
          sizingAspectRatios,
        )
      : null;
    setField("size", buildAndroidResolutionSizeSelection(
      derivedAspect,
      resolution ?? derivedResolution,
      capabilityInput,
      sizingAspectRatios,
      referenceAspectPreset,
    ));
  };

  const handleEditAutoAspectToggle = (enabled: boolean) => {
    setField("editAutoAspectResolution", enabled ? effectiveEditAutoAspectResolution : "");
  };

  const handleEditAutoAspectResolutionSelect = (resolution: typeof effectiveEditAutoAspectResolution) => {
    setField("editAutoAspectResolution", resolution);
  };

  const handleModeChange = (next: Mode) => {
    vibrateForPlatform(12);
    setField("mode", next);
  };

  const handleSubmit = () => {
    vibrateForPlatform(15);
    void submit().then(() => {
      if (useStudioStore.getState().isRunning) onSubmitStart?.();
    });
  };

  const handleOptimize = () => {
    vibrateForPlatform(10);
    optimizePrompt();
  };

  const handleSelectSource = () => {
    vibrateForPlatform(8);
    selectSourceImage();
  };

  const submitButton = needsUpstreamSetup ? (
    <button
      type="button"
      onClick={() => { vibrateForPlatform(10); openUpstreamConfig("app"); }}
      className="liquid-primary-button h-[52px] w-full text-[15px] font-semibold text-white"
    >
      配置上游
    </button>
  ) : isRunning ? (
    <button
      type="button"
      onClick={() => { vibrateForPlatform(10); cancel(); }}
      className="h-[52px] w-full rounded-[20px] border border-red-500/30 bg-red-500/10 text-[15px] font-medium text-red-500 transition-colors hover:bg-red-500/16"
    >
      取消生成
    </button>
  ) : (
    <button
      type="button"
      onClick={handleSubmit}
      disabled={!apiKey || !prompt.trim()}
      className="liquid-primary-button h-[52px] w-full text-[15px] font-semibold text-white disabled:cursor-not-allowed disabled:bg-zinc-300 disabled:text-zinc-500 dark:disabled:bg-zinc-800"
    >
      {loopGeneration.enabled ? "开始循环" : mode === "edit" ? "开始编辑" : "开始生成"}
    </button>
  );

  return (
    <div
      className="control-panel android-pad-compose flex w-full flex-col gap-4 overflow-y-auto border-r border-[var(--border)] bg-[var(--bg)] px-4 py-4"
      data-mode={mode}
      data-orientation={androidOrientation}
      style={{ paddingLeft: "calc(env(safe-area-inset-left, 0px) + 16px)", paddingRight: "calc(env(safe-area-inset-right, 0px) + 16px)" }}
    >
      <section className="platform-card android-pad-overview p-4">
        <div className="android-pad-overview-row">
          <div className="android-pad-hero-copy">
            <div className="android-phone-kicker">{mode === "edit" ? "图生图工作流" : "文生图工作流"}</div>
            <h2 className="mt-1 text-[17px] font-semibold tracking-[-0.02em] text-zinc-950 dark:text-zinc-50">
              图像工作区
            </h2>
            <p className="mt-1 text-[12px] leading-6 text-zinc-500 dark:text-zinc-300">
              {isLandscapePad
                ? "横屏下聚焦参数与提示词，画布从左侧导航进入。"
                : isMediumPad
                ? "中等宽度下保留 rail 导航，把主要操作压在单列主区域里。"
                : "参数在左，画布在中，大屏下保持一眼可扫的多窗格结构。"}
            </p>
          </div>
          <div className="android-pad-mode-switch">
            <AndroidModeSwitch mode={mode} onChange={handleModeChange} variant="pad" />
          </div>
        </div>
        <div className="android-inline-metrics mt-3">
          <span>{mode === "edit" ? "图生图" : "文生图"}</span>
          <span>{activeQualityLabel}</span>
          <span>{activeAspectLabel}</span>
          <span>{batchCount} 张</span>
          {!needsUpstreamSetup ? <span>上游已连接</span> : <span>待配置上游</span>}
        </div>
      </section>

      <div className="android-pad-compose-grid">
        <section className="platform-card android-pad-prompt p-5">
          <div className="mt-3">
            <ParameterPresetsSection variant="android" />
          </div>
          <div className="android-pad-section-head mt-3">
            <label className="android-phone-kicker">{mode === "edit" ? "修改要求" : "提示词"}</label>
            <span className="font-mono-token text-[11px] text-zinc-400 dark:text-zinc-500">{promptLen}</span>
          </div>
          <div className="android-pad-prompt-input-shell mt-3">
            <textarea
              value={prompt}
              placeholder={mode === "edit"
                ? "描述如何修改源图，例如：把背景换成夜景、保留主体姿态"
                : "描述你想生成的画面内容、光线、构图、风格、镜头感"}
              onChange={(e) => setField("prompt", e.target.value)}
              className="focus-ring android-pad-prompt-textarea min-h-[170px] w-full resize-none border border-black/[0.08] bg-[var(--surface)] px-4 py-3 text-[15px] leading-7 text-zinc-900 placeholder:text-zinc-400 dark:border-white/[0.08] dark:text-zinc-100 dark:placeholder:text-zinc-500"
            />
            <div className="mt-3 flex flex-wrap gap-2">
              <button
                type="button"
                onClick={() => { vibrateForPlatform(8); setTemplateManagerOpen(true); }}
                className="platform-pill inline-flex min-h-[36px] items-center gap-1.5 px-3 text-[12px] text-zinc-500 hover:bg-[var(--accent-soft)] hover:text-[var(--accent)]"
              >
                <ListPlus className="h-3.5 w-3.5" /> 管理模板
              </button>
              {promptTemplates.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => setField("prompt", appendPromptTemplateText(prompt, item.text))}
                  className="platform-pill inline-flex min-h-[36px] items-center px-3 text-[12px] text-zinc-500 hover:bg-[var(--accent-soft)] hover:text-[var(--accent)]"
                  title={item.text}
                >
                  {item.label}
                </button>
              ))}
            </div>
          </div>
          <div className="android-pad-action-row mt-3">
            <div className="relative android-pad-action-slot">
              <button
                type="button"
                onClick={() => { vibrateForPlatform(8); setTemplateOpen(true); }}
                className={`platform-pill inline-flex min-h-[40px] items-center gap-1.5 px-3 text-[12px] ${
                  templateOpen
                    ? "bg-[var(--accent-soft)] text-[var(--accent)] ring-1 ring-[color:var(--accent)]/20"
                    : "text-zinc-500 hover:bg-[var(--accent-soft)] hover:text-[var(--accent)]"
                }`}
              >
                <ListPlus className="h-3.5 w-3.5" /> 模板
              </button>
            </div>
            <button
              type="button"
              onClick={handleOptimize}
              disabled={!optimizeReady || isOptimizingPrompt}
              className={`platform-pill inline-flex min-h-[40px] items-center gap-1.5 px-3 text-[12px] ${
                isOptimizingPrompt
                  ? "bg-[var(--accent-soft)] text-[var(--accent)]"
                  : "text-zinc-500 hover:bg-[var(--accent-soft)] hover:text-[var(--accent)]"
              } disabled:cursor-not-allowed disabled:opacity-50`}
            >
              <Sparkles className={`h-3.5 w-3.5 ${isOptimizingPrompt ? "animate-pulse" : ""}`} />
              {isOptimizingPrompt ? "优化中..." : "AI 优化"}
            </button>
          </div>
        </section>

        <div className="android-pad-side-stack">
          <AndroidPadParameterSection
            activeAspect={activeAspect}
            activeAspectLabel={activeAspectLabel}
            aspectOptions={aspectOptions}
            activeResolution={activeResolution}
            activeResolutionLabel={activeResolutionLabel}
            editAutoAspectComputedSizeLabel={editAutoAspectComputedSizeLabel}
            exactSizeLabel={exactSize?.label ?? null}
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
            isMediumPad={isMediumPad}
            manualEditAutoAspectActive={manualEditAutoAspectActive}
            mode={mode}
            needsUpstreamSetup={needsUpstreamSetup}
            onOpenCustomAspectRatioModal={openCustomAspectRatioModal}
            onOpenCustomSizeModal={openCustomSizeModal}
            onOpenUpstream={() => { vibrateForPlatform(8); openUpstreamConfig("app"); }}
            quality={normalizedQuality}
            requestPolicy={requestPolicy}
            setField={setField as any}
            styleTag={styleTag}
          />

          <div className="android-pad-right-stack">
            {mode === "edit" ? (
              <AndroidPadSourceSection
                onCompareSource={(index) => void compareSourceOnCanvas(index)}
                clearSources={clearSources}
                currentImage={currentImage}
              editSourceLabel={editSourceLabel}
              onSelectSource={handleSelectSource}
              onViewSource={(index) => void viewSourceOnCanvas(index)}
              removeSource={removeSource}
              sources={sources}
            />
            ) : (
              <section className="platform-card android-pad-source-placeholder">
                <ImagePlus className="h-4 w-4" />
                <span>
                  <span className="android-phone-kicker">参考图</span>
                  <strong>无需参考图</strong>
                </span>
                <CheckCircle2 className="h-4 w-4" />
              </section>
            )}

            <AndroidAdvancedSection
              advancedOpen={advancedOpen}
              background={background}
              imageStyle={imageStyle}
              inputFidelity={inputFidelity}
              moderation={moderation}
              negativePrompt={negativePrompt}
              outputCompression={outputCompression}
              outputFormat={outputFormat}
              partialImages={partialImages}
              seed={seed}
              userIdentifier={userIdentifier}
              setAdvancedOpen={setAdvancedOpen}
              setField={setField as any}
              surface="pad"
            />

            <LoopGenerationSection
              value={loopGeneration}
              onChange={(next) => setField("loopGeneration", next)}
            />

            <div className="android-pad-side-cta">
              {submitButton}
            </div>
          </div>
        </div>
      </div>

      <div className="android-pad-cta" style={{ paddingLeft: "calc(env(safe-area-inset-left, 0px) + 16px)", paddingRight: "calc(env(safe-area-inset-right, 0px) + 16px)" }}>
        {submitButton}
      </div>
      <AndroidPromptTemplateModal
        open={templateOpen}
        onClose={() => setTemplateOpen(false)}
        onManageTemplates={() => setTemplateManagerOpen(true)}
        onPick={(text) => {
          const current = useStudioStore.getState().prompt;
          setField("prompt", appendPromptTemplateText(current, text));
        }}
      />
      <PromptTemplateManagerModal open={templateManagerOpen} onClose={() => setTemplateManagerOpen(false)} />
    </div>
  );
}
