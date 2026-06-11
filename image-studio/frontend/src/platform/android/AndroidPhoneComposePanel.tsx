import { useState } from "react";
import {
  FileText, ListPlus, RotateCw, Settings, Sparkles, X,
} from "lucide-react";
import { useStudioStore } from "../../state/studioStore";
import { OpenFile } from "../runtime/host";
import { Mode, SizeValue } from "../../types/domain";
import { availableQualityOptions, normalizeQualitySelection, STYLE_CHIPS } from "../../components/panel/panelOptions";
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
import { AndroidModeSwitch } from "./AndroidModeSwitch";
import { AndroidPhoneAdvancedSection } from "./AndroidPhoneAdvancedSection";
import { AndroidPhoneParameterSection } from "./AndroidPhoneParameterSection";
import { AndroidPhoneSourceSection } from "./AndroidPhoneSourceSection";
import { AndroidPromptTemplateModal } from "./AndroidPromptTemplateModal";
import { PromptTemplateManagerModal } from "../../components/panel/PromptTemplateManagerModal";
import { ParameterPresetsSection } from "../../components/panel/ParameterPresetsSection";
import {
  buildAndroidAspectSizeSelection,
  buildAndroidResolutionSizeSelection,
} from "./parameters/androidSizeSelection";
import { vibrateForPlatform } from "./bridge";
import { LoopGenerationSection } from "../../components/panel/LoopGenerationSection";
import { appendPromptTemplateText } from "../../lib/promptTemplateInsert";

export function AndroidPhoneComposePanel({
  onSubmitStart,
}: {
  onSubmitStart?: () => void;
} = {}) {
  const {
    apiKey, mode, prompt, background, imageStyle, inputFidelity, moderation, negativePrompt, outputCompression, size, quality, seed, styleTag,
    userIdentifier, partialImages,
    outputFormat, batchCount, editAutoAspectResolution, loopGeneration, sources, currentImage, errorMessage, errorCanRetry, errorRawPath,
    isRunning, lastPayload, isOptimizingPrompt, apiMode, requestPolicy, baseURL, profiles, imageModelID,
    customAspectRatios,
    setField, clearError, pushToast, selectSourceImage,
    removeSource, clearSources, viewSourceOnCanvas, openCustomAspectRatioModal, openCustomSizeModal, openUpstreamConfig, submit, cancel, retryLast, optimizePrompt,
    compareSourceOnCanvas,
  } = useStudioStore();
  const [templateOpen, setTemplateOpen] = useState(false);
  const [templateManagerOpen, setTemplateManagerOpen] = useState(false);
  const [parametersOpen, setParametersOpen] = useState(false);
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const promptTemplates = useStudioStore((s) => s.promptTemplates);
  const promptLen = prompt.length;
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
  const settingsExpanded = parametersOpen || advancedOpen;

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

  return (
    <div
      className={`control-panel android-phone-compose ${settingsExpanded ? "android-phone-compose-expanded" : ""} flex w-full flex-col gap-3 overflow-y-auto border-r-0 bg-[var(--bg)] px-3 py-3`}
      style={{ paddingLeft: "calc(env(safe-area-inset-left, 0px) + 12px)", paddingRight: "calc(env(safe-area-inset-right, 0px) + 12px)" }}
    >
      {errorMessage ? (
        <section className="platform-card border border-red-500/18 bg-red-500/10 p-3 text-xs text-red-700 dark:text-red-200">
          <div className="flex items-start gap-2">
            <div className="flex-1 whitespace-pre-wrap leading-relaxed">{errorMessage}</div>
            <button
              type="button"
              onClick={clearError}
              className="rounded-full p-1 text-red-400 transition-colors hover:bg-red-500/10 hover:text-red-300"
              title="关闭"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
          {(errorCanRetry && lastPayload && !isRunning) || errorRawPath ? (
            <div className="mt-2 flex flex-wrap items-center gap-2">
              {errorCanRetry && lastPayload && !isRunning ? (
                <button
                  type="button"
                  onClick={retryLast}
                  className="platform-pill inline-flex items-center gap-1 bg-red-500/15 px-2.5 py-1 text-[11px] transition-colors hover:bg-red-500/25"
                >
                  <RotateCw className="h-3 w-3" /> 重试
                </button>
              ) : null}
              {errorRawPath ? (
                <button
                  type="button"
                  onClick={() => OpenFile(errorRawPath).catch((e: any) => pushToast(`无法打开日志:${e?.message ?? e}`, "error"))}
                  className="platform-pill inline-flex items-center gap-1 px-2.5 py-1 text-[11px] ring-1 ring-red-500/30 transition-colors hover:bg-red-500/10"
                  title={errorRawPath}
                >
                  <FileText className="h-3 w-3" /> 查看日志
                </button>
              ) : null}
            </div>
          ) : null}
        </section>
      ) : null}

      {needsUpstreamSetup ? (
        <section className="platform-card android-phone-hero border border-[color:var(--accent)]/14 bg-[var(--accent-soft)] p-4">
          <div className="flex items-start gap-3">
            <div className="android-phone-hero-icon">
              <Settings className="h-4 w-4 text-[var(--accent)]" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="android-phone-kicker">启动前准备</div>
              <h2 className="mt-1 text-[20px] font-semibold tracking-[-0.02em] text-zinc-950 dark:text-zinc-50">
                先接入可用上游
              </h2>
              <p className="mt-2 text-[12px] leading-6 text-zinc-600 dark:text-zinc-300">
                保存中转站地址和 API Key 后，这里会切换成完整的移动端创作页。
              </p>
              <div className="mt-3 flex gap-2">
                <button
                  type="button"
                  onClick={() => openUpstreamConfig("app")}
                  className="liquid-primary-button inline-flex min-h-[42px] items-center gap-1.5 px-4 py-2 text-[12px] font-semibold text-white"
                >
                  <Settings className="h-3.5 w-3.5" /> 配置上游
                </button>
                <button
                  type="button"
                  onClick={() => openUpstreamConfig("app")}
                  className="platform-action-btn inline-flex min-h-[42px] items-center gap-1.5 border border-[color:var(--accent)]/20 bg-white/70 px-3 py-2 text-[12px] text-[var(--accent)] dark:bg-white/[0.05]"
                >
                  <Sparkles className="h-3.5 w-3.5" /> 去配置
                </button>
              </div>
            </div>
          </div>
        </section>
      ) : null}

      <section className={`platform-card android-phone-prompt ${needsUpstreamSetup ? "" : "android-phone-compose-sheet"} p-4`}>
        {!needsUpstreamSetup ? (
          <div className="android-phone-sheet-header">
            <div className="android-phone-hero-top">
              <div className="min-w-0">
                <div className="android-phone-kicker">{mode === "edit" ? "图生图工作流" : "文生图工作流"}</div>
                <h2 className="mt-1 text-[20px] font-semibold tracking-[-0.02em] text-zinc-950 dark:text-zinc-50">
                  {mode === "edit" ? "说明修改目标" : "描述画面"}
                </h2>
              </div>
              <div className="android-phone-mode-switch android-phone-mode-switch-compact">
                <AndroidModeSwitch mode={mode} onChange={handleModeChange} variant="phone" />
              </div>
            </div>
            <p className="android-phone-hero-copy mt-2 text-[12px] leading-5 text-zinc-500 dark:text-zinc-300">
              {mode === "edit"
                ? "先写改动重点，再补参考图和参数。"
                : "先写主体和镜头，参数在下面补。"}
            </p>
          </div>
        ) : null}
        <div className="mb-3">
          <ParameterPresetsSection variant="android" />
        </div>
        <div className="android-phone-prompt-head">
          <label className="android-phone-kicker">{mode === "edit" ? "修改要求" : "提示词"}</label>
          <span className="font-mono-token text-[11px] text-zinc-400 dark:text-zinc-500">{promptLen}</span>
        </div>
        <textarea
          value={prompt}
          placeholder={mode === "edit"
            ? "描述要修改的内容，例如换背景、改光线"
            : "描述主体、场景、光线、风格和镜头"}
          onChange={(e) => setField("prompt", e.target.value)}
          className="android-phone-prompt-input focus-ring"
        />
        <div className="mt-2 flex flex-wrap gap-2">
          <button
            type="button"
            onClick={() => { vibrateForPlatform(8); setTemplateManagerOpen(true); }}
            className="platform-pill android-phone-action-pill inline-flex min-h-[34px] items-center gap-1.5 px-3 text-[11px] text-zinc-500 hover:bg-[var(--accent-soft)] hover:text-[var(--accent)]"
          >
            <ListPlus className="h-3.5 w-3.5" /> 管理模板
          </button>
          {promptTemplates.map((item) => (
            <button
              key={item.id}
              type="button"
              onClick={() => setField("prompt", appendPromptTemplateText(prompt, item.text))}
              className="platform-pill android-phone-action-pill inline-flex min-h-[34px] items-center px-3 text-[11px] text-zinc-500 hover:bg-[var(--accent-soft)] hover:text-[var(--accent)]"
              title={item.text}
            >
              {item.label}
            </button>
          ))}
        </div>
        <div className="android-phone-action-row">
          <div className="android-phone-action-item relative">
            <button
              type="button"
              onClick={() => { vibrateForPlatform(8); setTemplateOpen(true); }}
              className={`platform-pill android-phone-action-pill inline-flex min-h-[38px] items-center gap-1.5 px-3 text-[11px] ${
                templateOpen
                  ? "bg-[var(--accent-soft)] text-[var(--accent)] ring-1 ring-[color:var(--accent)]/20"
                  : "text-zinc-500 hover:bg-[var(--accent-soft)] hover:text-[var(--accent)]"
              }`}
              title="prompt 模板与历史"
            >
              <ListPlus className="h-3.5 w-3.5" /> 模板
            </button>
          </div>
          <button
            type="button"
            onClick={handleOptimize}
            disabled={!optimizeReady || isOptimizingPrompt}
            className={`platform-pill android-phone-action-pill inline-flex min-h-[38px] items-center gap-1.5 px-3 text-[11px] ${
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

      {!needsUpstreamSetup ? (
        <AndroidPhoneParameterSection
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
          batchCount={batchCount}
          effectiveEditAutoAspectResolution={effectiveEditAutoAspectResolution}
          handleEditAutoAspectResolutionSelect={handleEditAutoAspectResolutionSelect}
          handleEditAutoAspectToggle={handleEditAutoAspectToggle}
          handleAspectSelect={handleAspectSelect}
          handleResolutionSelect={handleResolutionSelect}
          imageModelID={imageModelID}
          manualEditAutoAspectActive={manualEditAutoAspectActive}
          mode={mode}
          onOpenCustomAspectRatioModal={openCustomAspectRatioModal}
          onOpenCustomSizeModal={openCustomSizeModal}
          apiMode={apiMode}
          parametersOpen={parametersOpen}
          quality={normalizedQuality}
          requestPolicy={requestPolicy}
          setField={setField as any}
          setParametersOpen={setParametersOpen}
          styleTag={styleTag}
        />
      ) : null}

      {mode === "edit" ? (
        <AndroidPhoneSourceSection
          onCompareSource={(index) => void compareSourceOnCanvas(index)}
          clearSources={clearSources}
          currentImage={currentImage}
          editSourceLabel={editSourceLabel}
          onSelectSource={handleSelectSource}
          onViewSource={(index) => void viewSourceOnCanvas(index)}
          removeSource={removeSource}
          sources={sources}
        />
      ) : null}

      {!needsUpstreamSetup ? (
        <AndroidPhoneAdvancedSection
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
        />
      ) : null}

      {!needsUpstreamSetup ? (
        <LoopGenerationSection
          value={loopGeneration}
          onChange={(next) => setField("loopGeneration", next)}
        />
      ) : null}

      <div className="android-phone-sticky-cta" style={{ paddingLeft: "calc(env(safe-area-inset-left, 0px) + 12px)", paddingRight: "calc(env(safe-area-inset-right, 0px) + 12px)" }}>
        {needsUpstreamSetup ? (
          <button
            type="button"
            onClick={() => { vibrateForPlatform(10); openUpstreamConfig("app"); }}
            className="liquid-primary-button h-[54px] w-full text-[15px] font-semibold text-white"
          >
            配置上游
          </button>
        ) : isRunning ? (
          <button
            type="button"
            onClick={() => { vibrateForPlatform(10); cancel(); }}
            className="h-[54px] w-full rounded-[20px] border border-red-500/30 bg-red-500/10 text-[15px] font-medium text-red-500 transition-colors hover:bg-red-500/16"
          >
            取消生成
          </button>
        ) : (
          <button
            type="button"
            onClick={handleSubmit}
            disabled={!apiKey || !prompt.trim()}
            className="liquid-primary-button h-[54px] w-full text-[15px] font-semibold text-white disabled:cursor-not-allowed disabled:bg-zinc-300 disabled:text-zinc-500 dark:disabled:bg-zinc-800"
          >
            {loopGeneration.enabled ? "开始循环" : mode === "edit" ? "开始编辑" : "开始生成"}
          </button>
        )}
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
