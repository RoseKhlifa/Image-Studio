import { LoadCompatibilityState, SaveCompatibilityState } from "../platform/runtime/host.ts";
import type {
  BatchProcessConfig,
  BackgroundValue,
  CompletionNotificationConfig,
  CustomAspectRatio,
  EditSourceMode,
  CompletionSoundConfig,
  HistoryItem,
  ImageStyleValue,
  InputFidelityValue,
  KernelRuntimeMode,
  ModerationValue,
  OutputFormatValue,
  Preset,
  PromptTemplate,
  ProxyMode,
  ThemeMode,
  UpstreamProfile,
} from "../types/domain.ts";
import { ACTIVE_PROFILE_LS_KEY, PROFILES_LS_KEY, tryParseProfile } from "./profiles.ts";
import { normalizeProxyMode, persistProxyConfig } from "./proxy.ts";
import {
  normalizeCustomAspectRatios,
  persistCustomAspectRatios,
} from "./customAspectRatios.ts";
import {
  loadTrustedOutputRoots,
  persistHistoryFullImages,
  persistHistoryItems,
  pruneHistoryStorage,
} from "./storage.ts";
import {
  normalizeCompletionSoundConfig,
  persistCompletionSoundConfig,
} from "./completionSound.ts";
import {
  normalizeCompletionNotificationConfig,
  persistCompletionNotificationConfig,
} from "./completionNotification.ts";
import { normalizePromptTemplates } from "./promptTemplates.ts";
import { normalizeAutoRetryCount } from "../../../../shared/kernel/requestModel.js";

const SCHEMA_VERSION = 1;
const MARKER_KEY = "gptcodex.compatStateUpdatedAt";

export type CompatibilityState = {
  schemaVersion: number;
  client?: string;
  updatedAt: number;
  settings: {
    proxyMode?: ProxyMode;
    proxyURL?: string;
    theme?: ThemeMode;
    fontScale?: number;
    outputFormat?: OutputFormatValue;
    background?: BackgroundValue;
    outputCompression?: number;
    inputFidelity?: InputFidelityValue;
    imageStyle?: ImageStyleValue;
    moderation?: ModerationValue;
    userIdentifier?: string;
    partialImages?: number;
    protectStreamPreview?: boolean;
    autoRetryEnabled?: boolean;
    autoRetryCount?: number;
    promptTemplates?: PromptTemplate[];
    outputDir?: string;
    promptHistory?: string[];
    presets?: Preset[];
    customAspectRatios?: CustomAspectRatio[];
    kernelRuntimeMode?: KernelRuntimeMode;
    trustedOutputRoots?: string[];
    savePromptSuppressed?: boolean;
    keepLogs?: boolean;
    cleanupPreviewCacheOnExit?: boolean;
    ignoredReleaseTag?: string;
    completionSound?: CompletionSoundConfig;
    completionNotification?: CompletionNotificationConfig;
  };
  profiles: UpstreamProfile[];
  activeProfileId: string;
  history: HistoryItem[];
  historyFull?: Array<{ id: string; imageB64: string }>;
};

export type CompatibilityExportInput = {
  history: HistoryItem[];
  profiles: UpstreamProfile[];
  activeProfileId: string;
  proxyMode: ProxyMode;
  proxyURL: string;
  theme: ThemeMode;
  fontScale: number;
  outputFormat: OutputFormatValue;
  background: BackgroundValue;
  outputCompression: number;
  inputFidelity: InputFidelityValue;
  imageStyle: ImageStyleValue;
  moderation: ModerationValue;
  userIdentifier: string;
  partialImages: number;
  protectStreamPreview: boolean;
  autoRetryEnabled: boolean;
  autoRetryCount: number;
  promptTemplates: PromptTemplate[];
  promptHistory: string[];
  presets: Preset[];
  customAspectRatios: CustomAspectRatio[];
  kernelRuntimeMode: KernelRuntimeMode;
  keepLogs: boolean;
  cleanupPreviewCacheOnExit: boolean;
  ignoredReleaseTag: string;
  completionSound: CompletionSoundConfig;
  completionNotification: CompletionNotificationConfig;
};

let exportTimer: ReturnType<typeof setTimeout> | null = null;

export async function importCompatibilityStateIfNewer(): Promise<boolean> {
  const state = normalizeCompatibilityState(await LoadCompatibilityState());
  if (!state || state.updatedAt <= 0) return false;
  if (state.updatedAt <= readLocalMarker()) return false;

  applyCompatibilityLocalStorage(state);
  await persistCompatibilityHistory(state);
  writeLocalMarker(state.updatedAt);
  return true;
}

export function scheduleCompatibilityExport(input: CompatibilityExportInput): void {
  if (exportTimer) clearTimeout(exportTimer);
  const snapshot = cloneExportInput(input);
  exportTimer = setTimeout(() => {
    exportTimer = null;
    void exportCompatibilityStateNow(snapshot).catch((error) => {
      if (typeof console !== "undefined") console.warn("compat export failed", error);
    });
  }, 250);
}

export async function exportCompatibilityStateNow(input: CompatibilityExportInput): Promise<void> {
  const state = buildCompatibilityState(input);
  await SaveCompatibilityState(state as unknown as Record<string, unknown>);
  writeLocalMarker(state.updatedAt);
}

export function compatibilityExportFingerprint(input: CompatibilityExportInput): string {
  return JSON.stringify({
    profiles: input.profiles,
    activeProfileId: input.activeProfileId,
    proxyMode: input.proxyMode,
    proxyURL: input.proxyURL,
    theme: input.theme,
    fontScale: input.fontScale,
    outputFormat: input.outputFormat,
    background: input.background,
    outputCompression: input.outputCompression,
    inputFidelity: input.inputFidelity,
    imageStyle: input.imageStyle,
    moderation: input.moderation,
    userIdentifier: input.userIdentifier,
    partialImages: input.partialImages,
    protectStreamPreview: input.protectStreamPreview,
    autoRetryEnabled: input.autoRetryEnabled,
    autoRetryCount: input.autoRetryCount,
    promptTemplates: input.promptTemplates,
    promptHistory: input.promptHistory,
    presets: input.presets,
    customAspectRatios: input.customAspectRatios,
    kernelRuntimeMode: input.kernelRuntimeMode,
    keepLogs: input.keepLogs,
    cleanupPreviewCacheOnExit: input.cleanupPreviewCacheOnExit,
    ignoredReleaseTag: input.ignoredReleaseTag,
    completionSound: input.completionSound,
    completionNotification: input.completionNotification,
    outputDir: readLocalStorageString("gptcodex.outputDir"),
    trustedOutputRoots: loadTrustedOutputRoots(),
    savePromptSuppressed: readLocalStorageString("gptcodex.savePromptSuppressed") === "1",
    history: input.history.map(historyFingerprint),
  });
}

function buildCompatibilityState(input: CompatibilityExportInput): CompatibilityState {
  const history = input.history.map(toSerializableHistoryItem).filter((item): item is HistoryItem => item !== null);
  return {
    schemaVersion: SCHEMA_VERSION,
    client: "webview2",
    updatedAt: Date.now(),
    settings: {
      proxyMode: normalizeProxyMode(input.proxyMode),
      proxyURL: input.proxyURL.trim(),
      theme: normalizeTheme(input.theme),
      fontScale: normalizeFontScale(input.fontScale),
      outputFormat: normalizeOutputFormat(input.outputFormat),
      background: normalizeBackground(input.background),
      outputCompression: normalizeOutputCompression(input.outputCompression),
      inputFidelity: normalizeInputFidelity(input.inputFidelity),
      imageStyle: normalizeImageStyle(input.imageStyle),
      moderation: normalizeModeration(input.moderation),
      userIdentifier: normalizeUserIdentifier(input.userIdentifier),
      partialImages: normalizePartialImages(input.partialImages),
      protectStreamPreview: input.protectStreamPreview !== false,
      autoRetryEnabled: input.autoRetryEnabled !== false,
      autoRetryCount: normalizeAutoRetryCount(input.autoRetryCount),
      promptTemplates: normalizePromptTemplates(input.promptTemplates),
      outputDir: readLocalStorageString("gptcodex.outputDir"),
      promptHistory: cleanStringList(input.promptHistory, 50),
      presets: normalizePresets(input.presets),
      customAspectRatios: normalizeCustomAspectRatios(input.customAspectRatios),
      kernelRuntimeMode: normalizeKernelRuntimeMode(input.kernelRuntimeMode),
      trustedOutputRoots: loadTrustedOutputRoots(),
      savePromptSuppressed: readLocalStorageString("gptcodex.savePromptSuppressed") === "1",
      keepLogs: input.keepLogs === true,
      cleanupPreviewCacheOnExit: input.cleanupPreviewCacheOnExit === true,
      ignoredReleaseTag: readLocalStorageString("gptcodex.ignoredReleaseTag"),
      completionSound: normalizeCompletionSoundConfig(input.completionSound),
      completionNotification: normalizeCompletionNotificationConfig(input.completionNotification),
    },
    profiles: normalizeProfiles(input.profiles),
    activeProfileId: input.activeProfileId || "",
    history,
    historyFull: history
      .filter((item) => typeof item.imageB64 === "string" && item.imageB64.trim())
      .map((item) => ({ id: item.id, imageB64: item.imageB64 as string })),
  };
}

function applyCompatibilityLocalStorage(state: CompatibilityState): void {
  const profiles = normalizeProfiles(state.profiles);
  writeLocalStorageJSON(PROFILES_LS_KEY, profiles);
  if (state.activeProfileId) writeLocalStorageString(ACTIVE_PROFILE_LS_KEY, state.activeProfileId);
  else removeLocalStorage(ACTIVE_PROFILE_LS_KEY);

  const settings = state.settings ?? {};
  if (settings.proxyMode) persistProxyConfig(normalizeProxyMode(settings.proxyMode), settings.proxyURL ?? "");
  if (settings.theme) writeLocalStorageString("gptcodex.theme", normalizeTheme(settings.theme));
  if (typeof settings.fontScale === "number") writeLocalStorageString("gptcodex.fontScale", String(normalizeFontScale(settings.fontScale)));
  if (settings.outputFormat) writeLocalStorageString("gptcodex.outputFormat", normalizeOutputFormat(settings.outputFormat));
  if (settings.background) writeLocalStorageString("gptcodex.background", normalizeBackground(settings.background));
  if (typeof settings.outputCompression === "number") writeLocalStorageString("gptcodex.outputCompression", String(normalizeOutputCompression(settings.outputCompression)));
  if (settings.inputFidelity) writeLocalStorageString("gptcodex.inputFidelity", normalizeInputFidelity(settings.inputFidelity));
  if (settings.imageStyle) writeLocalStorageString("gptcodex.imageStyle", normalizeImageStyle(settings.imageStyle));
  if (settings.moderation) writeLocalStorageString("gptcodex.moderation", normalizeModeration(settings.moderation));
  if (settings.userIdentifier !== undefined) writeLocalStorageString("gptcodex.userIdentifier", normalizeUserIdentifier(settings.userIdentifier));
  if (typeof settings.partialImages === "number") writeLocalStorageString("gptcodex.partialImages", String(normalizePartialImages(settings.partialImages)));
  if (settings.protectStreamPreview === false) writeLocalStorageString("gptcodex.protectStreamPreview", "0");
  else removeLocalStorage("gptcodex.protectStreamPreview");
  if (settings.autoRetryEnabled === false) writeLocalStorageString("gptcodex.autoRetryEnabled", "0");
  else removeLocalStorage("gptcodex.autoRetryEnabled");
  if (typeof settings.autoRetryCount === "number") writeLocalStorageString("gptcodex.autoRetryCount", String(normalizeAutoRetryCount(settings.autoRetryCount)));
  writeLocalStorageJSON("gptcodex.promptTemplates", normalizePromptTemplates(settings.promptTemplates ?? []));
  if (settings.outputDir?.trim()) writeLocalStorageString("gptcodex.outputDir", settings.outputDir.trim());
  else removeLocalStorage("gptcodex.outputDir");
  if (settings.kernelRuntimeMode) writeLocalStorageString("gptcodex.kernelRuntimeMode", normalizeKernelRuntimeMode(settings.kernelRuntimeMode));
  writeLocalStorageJSON("gptcodex.promptHistory", cleanStringList(settings.promptHistory ?? [], 50));
  writeLocalStorageJSON("gptcodex.presets", normalizePresets(settings.presets ?? []));
  persistCustomAspectRatios(normalizeCustomAspectRatios(settings.customAspectRatios ?? []));
  writeLocalStorageJSON("gptcodex.trustedOutputRoots", cleanStringList(settings.trustedOutputRoots ?? [], 100));
  if (settings.savePromptSuppressed) writeLocalStorageString("gptcodex.savePromptSuppressed", "1");
  else removeLocalStorage("gptcodex.savePromptSuppressed");
  if (settings.keepLogs) writeLocalStorageString("gptcodex.keepLogs", "1");
  else removeLocalStorage("gptcodex.keepLogs");
  if (settings.cleanupPreviewCacheOnExit) writeLocalStorageString("gptcodex.cleanupPreviewCacheOnExit", "1");
  else removeLocalStorage("gptcodex.cleanupPreviewCacheOnExit");
  if (typeof settings.ignoredReleaseTag === "string" && settings.ignoredReleaseTag.trim()) {
    writeLocalStorageString("gptcodex.ignoredReleaseTag", settings.ignoredReleaseTag.trim());
  } else {
    removeLocalStorage("gptcodex.ignoredReleaseTag");
  }
  persistCompletionSoundConfig(normalizeCompletionSoundConfig(settings.completionSound));
  persistCompletionNotificationConfig(normalizeCompletionNotificationConfig(settings.completionNotification));
}

async function persistCompatibilityHistory(state: CompatibilityState): Promise<void> {
  const items = state.history.map(toSerializableHistoryItem).filter((item): item is HistoryItem => item !== null);
  await persistHistoryItems(items).catch(() => undefined);
  const fullImages = [
    ...items
      .filter((item) => typeof item.imageB64 === "string" && item.imageB64.trim())
      .map((item) => ({ id: item.id, imageB64: item.imageB64 as string })),
    ...(state.historyFull ?? []).filter((item) => item?.id && item.imageB64?.trim()),
  ];
  await persistHistoryFullImages(fullImages).catch(() => undefined);
  await pruneHistoryStorage(items.map((item) => item.id)).catch(() => undefined);
}

function normalizeCompatibilityState(raw: unknown): CompatibilityState | null {
  if (!raw || typeof raw !== "object") return null;
  const source = raw as Record<string, any>;
  return {
    schemaVersion: typeof source.schemaVersion === "number" ? source.schemaVersion : SCHEMA_VERSION,
    client: typeof source.client === "string" ? source.client : undefined,
    updatedAt: typeof source.updatedAt === "number" ? source.updatedAt : 0,
    settings: normalizeSettings(source.settings),
    profiles: normalizeProfiles(Array.isArray(source.profiles) ? source.profiles : []),
    activeProfileId: typeof source.activeProfileId === "string" ? source.activeProfileId : "",
    history: (Array.isArray(source.history) ? source.history : [])
      .map(toSerializableHistoryItem)
      .filter((item): item is HistoryItem => item !== null),
    historyFull: (Array.isArray(source.historyFull) ? source.historyFull : [])
      .map((item: any) => ({
        id: typeof item?.id === "string" ? item.id : "",
        imageB64: typeof item?.imageB64 === "string" ? item.imageB64 : "",
      }))
      .filter((item: { id: string; imageB64: string }) => item.id && item.imageB64),
  };
}

function normalizeSettings(raw: unknown): CompatibilityState["settings"] {
  const source = raw && typeof raw === "object" ? raw as Record<string, any> : {};
  return {
    proxyMode: normalizeProxyMode(source.proxyMode),
    proxyURL: typeof source.proxyURL === "string" ? source.proxyURL : "",
    theme: normalizeTheme(source.theme),
    fontScale: normalizeFontScale(source.fontScale),
    outputFormat: normalizeOutputFormat(source.outputFormat),
    background: normalizeBackground(source.background),
    outputCompression: normalizeOutputCompression(source.outputCompression),
    inputFidelity: normalizeInputFidelity(source.inputFidelity),
    imageStyle: normalizeImageStyle(source.imageStyle),
    moderation: normalizeModeration(source.moderation),
    userIdentifier: normalizeUserIdentifier(source.userIdentifier),
    partialImages: normalizePartialImages(source.partialImages),
    protectStreamPreview: source.protectStreamPreview !== false,
    autoRetryEnabled: source.autoRetryEnabled !== false,
    autoRetryCount: normalizeAutoRetryCount(source.autoRetryCount),
    promptTemplates: normalizePromptTemplates(source.promptTemplates ?? []),
    outputDir: typeof source.outputDir === "string" ? source.outputDir : "",
    promptHistory: cleanStringList(source.promptHistory ?? [], 50),
    presets: normalizePresets(source.presets ?? []),
    customAspectRatios: normalizeCustomAspectRatios(source.customAspectRatios ?? []),
    kernelRuntimeMode: normalizeKernelRuntimeMode(source.kernelRuntimeMode),
    trustedOutputRoots: cleanStringList(source.trustedOutputRoots ?? [], 100),
    savePromptSuppressed: source.savePromptSuppressed === true,
    keepLogs: source.keepLogs === true,
    cleanupPreviewCacheOnExit: source.cleanupPreviewCacheOnExit === true,
    ignoredReleaseTag: typeof source.ignoredReleaseTag === "string" ? source.ignoredReleaseTag.trim() : "",
    completionSound: normalizeCompletionSoundConfig(source.completionSound),
    completionNotification: normalizeCompletionNotificationConfig(source.completionNotification),
  };
}

function normalizeProfiles(raw: unknown[]): UpstreamProfile[] {
  return raw.map((item) => tryParseProfile(item)).filter((profile): profile is UpstreamProfile => profile !== null);
}

function toSerializableHistoryItem(raw: unknown): HistoryItem | null {
  if (!raw || typeof raw !== "object") return null;
  const item = raw as Record<string, any>;
  const id = typeof item.id === "string" ? item.id.trim() : "";
  const prompt = typeof item.prompt === "string" ? item.prompt : "";
  const createdAt = typeof item.createdAt === "number" ? item.createdAt : 0;
  if (!id || !createdAt) return null;
  return {
    id,
    imageId: stringOrUndefined(item.imageId),
    previewUrl: stringOrUndefined(item.previewUrl),
    fullUrl: stringOrUndefined(item.fullUrl),
    previewPath: stringOrUndefined(item.previewPath),
    thumbPath: stringOrUndefined(item.thumbPath),
    previewWidth: numberOrUndefined(item.previewWidth),
    previewHeight: numberOrUndefined(item.previewHeight),
    imageB64: stringOrUndefined(item.imageB64),
    imageBlob: null,
    previewBlob: null,
    previewOnly: item.previewOnly === true,
    prompt,
    revisedPrompt: stringOrUndefined(item.revisedPrompt),
    mode: item.mode === "edit" ? "edit" : "generate",
    size: normalizeSize(item.size),
    quality: normalizeQuality(item.quality),
    outputFormat: normalizeOutputFormat(item.outputFormat),
    parentId: stringOrUndefined(item.parentId),
    createdAt,
    seed: numberOrUndefined(item.seed),
    negativePrompt: stringOrUndefined(item.negativePrompt),
    background: backgroundOrUndefined(item.background),
    outputCompression: numberOrUndefined(item.outputCompression),
    inputFidelity: inputFidelityOrUndefined(item.inputFidelity),
    imageStyle: imageStyleOrUndefined(item.imageStyle),
    moderation: moderationOrUndefined(item.moderation),
    styleTag: stringOrUndefined(item.styleTag),
    batchIndex: numberOrUndefined(item.batchIndex),
    previewSlotIndex: numberOrUndefined(item.previewSlotIndex),
    elapsedSec: numberOrUndefined(item.elapsedSec),
    sourcePaths: cleanStringList(item.sourcePaths, 32),
    savedPath: stringOrUndefined(item.savedPath),
    rawPath: stringOrUndefined(item.rawPath),
  };
}

function historyFingerprint(item: HistoryItem) {
  return {
    id: item.id,
    imageId: item.imageId,
    savedPath: item.savedPath,
    previewPath: item.previewPath,
    thumbPath: item.thumbPath,
    rawPath: item.rawPath,
    prompt: item.prompt,
    revisedPrompt: item.revisedPrompt,
    mode: item.mode,
    size: item.size,
    quality: item.quality,
    outputFormat: item.outputFormat,
    createdAt: item.createdAt,
    seed: item.seed,
    negativePrompt: item.negativePrompt,
    background: item.background,
    outputCompression: item.outputCompression,
    inputFidelity: item.inputFidelity,
    imageStyle: item.imageStyle,
    moderation: item.moderation,
    styleTag: item.styleTag,
    batchIndex: item.batchIndex,
    previewSlotIndex: item.previewSlotIndex,
    elapsedSec: item.elapsedSec,
    sourcePaths: item.sourcePaths,
    imageB64: item.imageB64,
  };
}

function cloneExportInput(input: CompatibilityExportInput): CompatibilityExportInput {
  return {
    history: input.history.map((item) => ({ ...item, imageBlob: null, previewBlob: null })),
    profiles: input.profiles.map((profile) => ({ ...profile })),
    activeProfileId: input.activeProfileId,
    proxyMode: input.proxyMode,
    proxyURL: input.proxyURL,
    theme: input.theme,
    fontScale: input.fontScale,
    outputFormat: input.outputFormat,
    background: input.background,
    outputCompression: input.outputCompression,
    inputFidelity: input.inputFidelity,
    imageStyle: input.imageStyle,
    moderation: input.moderation,
    userIdentifier: input.userIdentifier,
    partialImages: input.partialImages,
    protectStreamPreview: input.protectStreamPreview,
    autoRetryEnabled: input.autoRetryEnabled,
    autoRetryCount: input.autoRetryCount,
    promptTemplates: input.promptTemplates.map((item) => ({ ...item })),
    promptHistory: [...input.promptHistory],
    presets: input.presets.map((preset) => ({ ...preset })),
    customAspectRatios: input.customAspectRatios.map((ratio) => ({ ...ratio })),
    kernelRuntimeMode: input.kernelRuntimeMode,
    keepLogs: input.keepLogs,
    cleanupPreviewCacheOnExit: input.cleanupPreviewCacheOnExit,
    ignoredReleaseTag: input.ignoredReleaseTag,
    completionSound: normalizeCompletionSoundConfig(input.completionSound),
    completionNotification: normalizeCompletionNotificationConfig(input.completionNotification),
  };
}

export function readIgnoredReleaseTag(): string {
  return readLocalStorageString("gptcodex.ignoredReleaseTag");
}

export function writeIgnoredReleaseTag(value: string): void {
  const trimmed = value.trim();
  if (trimmed) writeLocalStorageString("gptcodex.ignoredReleaseTag", trimmed);
  else removeLocalStorage("gptcodex.ignoredReleaseTag");
}

function normalizePresets(raw: unknown): Preset[] {
  if (!Array.isArray(raw)) return [];
  const out: Preset[] = [];
  for (const item of raw) {
    if (!item || typeof item !== "object") continue;
    const source = item as Record<string, any>;
    const id = typeof source.id === "string" ? source.id : "";
    const name = typeof source.name === "string" ? source.name.trim() : "";
    if (!id || !name) continue;
    out.push({
      id,
      name,
      size: normalizeSize(source.size),
      quality: normalizeQuality(source.quality),
      outputFormat: normalizeOutputFormat(source.outputFormat),
      negativePrompt: typeof source.negativePrompt === "string" ? source.negativePrompt : "",
      background: source.background === undefined ? undefined : normalizeBackground(source.background),
      outputCompression: source.outputCompression === undefined ? undefined : normalizeOutputCompression(source.outputCompression),
      inputFidelity: source.inputFidelity === undefined ? undefined : normalizeInputFidelity(source.inputFidelity),
      imageStyle: source.imageStyle === undefined ? undefined : normalizeImageStyle(source.imageStyle),
      moderation: source.moderation === undefined ? undefined : normalizeModeration(source.moderation),
      styleTag: typeof source.styleTag === "string" ? source.styleTag : undefined,
      kernelRuntimeMode: normalizeKernelRuntimeMode(source.kernelRuntimeMode),
      batchCount: normalizeBatchCount(source.batchCount),
    });
  }
  return out;
}

function normalizeTheme(value: unknown): ThemeMode {
  return value === "light" || value === "dark" || value === "system" ? value : "system";
}

function normalizeKernelRuntimeMode(value: unknown): KernelRuntimeMode {
  return value === "local" || value === "remote" || value === "auto" ? value : "auto";
}

function normalizeOutputFormat(value: unknown): OutputFormatValue {
  return value === "jpeg" || value === "webp" || value === "png" ? value : "png";
}

function normalizeBackground(value: unknown): BackgroundValue {
  return value === "opaque" || value === "transparent" || value === "auto" ? value : "auto";
}

function normalizeOutputCompression(value: unknown): number {
  if (value === null || value === undefined || value === "") return 100;
  const n = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(n)) return 100;
  return Math.max(0, Math.min(100, Math.round(n)));
}

function normalizeInputFidelity(value: unknown): InputFidelityValue {
  return value === "low" || value === "high" || value === "auto" ? value : "auto";
}

function normalizeImageStyle(value: unknown): ImageStyleValue {
  return value === "vivid" || value === "natural" || value === "default" ? value : "default";
}

function normalizeModeration(value: unknown): ModerationValue {
  return value === "auto" ? "auto" : "low";
}

function normalizeUserIdentifier(value: unknown): string {
  const trimmed = String(value ?? "").trim();
  if (!trimmed) return "";
  return Array.from(trimmed).slice(0, 64).join("");
}

function normalizePartialImages(value: unknown): number {
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric < 0) return 1;
  return Math.max(0, Math.min(3, Math.floor(numeric)));
}

function normalizeSize(value: unknown): HistoryItem["size"] {
  return typeof value === "string" && value.trim() ? value.trim() as HistoryItem["size"] : "1024x1024";
}

function normalizeQuality(value: unknown): HistoryItem["quality"] {
  return value === "auto" || value === "high" || value === "low" || value === "medium" || value === "standard" || value === "hd"
    ? value
    : "medium";
}

function normalizeFontScale(value: unknown): number {
  const n = typeof value === "number" ? value : Number(value);
  return Number.isFinite(n) && n > 0.5 && n < 2 ? n : 1;
}

function normalizeBatchCount(value: unknown): number {
  const n = typeof value === "number" ? value : Number(value);
  return Number.isFinite(n) && n > 0 ? Math.min(9, Math.floor(n)) : 1;
}

export function normalizeEditSourceMode(value: unknown): EditSourceMode {
  return value === "batch" ? "batch" : "manual";
}

export function normalizeBatchProcess(value: unknown): BatchProcessConfig {
  const source = value && typeof value === "object" ? value as Record<string, any> : {};
  const discoveredSources: BatchProcessConfig["discoveredSources"] = [];
  if (Array.isArray(source.discoveredSources)) {
    for (const item of source.discoveredSources) {
      const path = typeof item?.path === "string" ? item.path.trim() : "";
      const name = typeof item?.name === "string" ? item.name.trim() : "";
      if (!path || !name) continue;
      discoveredSources.push({
        path,
        name,
        size: Number.isFinite(Number(item?.size)) ? Math.max(0, Math.floor(Number(item.size))) : 0,
        previewUrl: typeof item?.previewUrl === "string" ? item.previewUrl : undefined,
        previewWidth: Number.isFinite(Number(item?.previewWidth)) ? Math.floor(Number(item.previewWidth)) : undefined,
        previewHeight: Number.isFinite(Number(item?.previewHeight)) ? Math.floor(Number(item.previewHeight)) : undefined,
      });
    }
  }
  return {
    enabled: source.enabled === true,
    inputDir: typeof source.inputDir === "string" ? source.inputDir.trim() : "",
    outputMode: source.outputMode === "custom_dir" ? "custom_dir" : "source_dir",
    outputDir: typeof source.outputDir === "string" ? source.outputDir.trim() : "",
    concurrency: Number.isFinite(Number(source.concurrency)) ? Math.max(1, Math.min(9, Math.floor(Number(source.concurrency)))) : 2,
    retryOnFailure: source.retryOnFailure === true,
    fileNamePrefix: typeof source.fileNamePrefix === "string" && source.fileNamePrefix.trim() ? source.fileNamePrefix.trim() : "processed-",
    autoAspectResolution: source.autoAspectResolution === "256" || source.autoAspectResolution === "512" || source.autoAspectResolution === "1k" || source.autoAspectResolution === "2k" || source.autoAspectResolution === "4k"
      ? source.autoAspectResolution
      : "",
    discoveredSources,
  };
}

function cleanStringList(raw: unknown, max: number): string[] {
  if (!Array.isArray(raw)) return [];
  return Array.from(new Set(
    raw.map((item) => typeof item === "string" ? item.trim() : "").filter(Boolean),
  )).slice(0, max);
}

function stringOrUndefined(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value : undefined;
}

function numberOrUndefined(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function moderationOrUndefined(value: unknown): ModerationValue | undefined {
  return value === "auto" || value === "low" ? value : undefined;
}

function backgroundOrUndefined(value: unknown): BackgroundValue | undefined {
  return value === "auto" || value === "opaque" || value === "transparent" ? value : undefined;
}

function inputFidelityOrUndefined(value: unknown): InputFidelityValue | undefined {
  return value === "auto" || value === "low" || value === "high" ? value : undefined;
}

function imageStyleOrUndefined(value: unknown): ImageStyleValue | undefined {
  return value === "default" || value === "vivid" || value === "natural" ? value : undefined;
}

function readLocalMarker(): number {
  const raw = readLocalStorageString(MARKER_KEY);
  const value = Number(raw);
  return Number.isFinite(value) ? value : 0;
}

function writeLocalMarker(value: number): void {
  writeLocalStorageString(MARKER_KEY, String(Math.max(0, Math.floor(value))));
}

function readLocalStorageString(key: string): string {
  try { return localStorage.getItem(key) ?? ""; } catch { return ""; }
}

function writeLocalStorageString(key: string, value: string): void {
  try { localStorage.setItem(key, value); } catch {}
}

function writeLocalStorageJSON(key: string, value: unknown): void {
  try { localStorage.setItem(key, JSON.stringify(value)); } catch {}
}

function removeLocalStorage(key: string): void {
  try { localStorage.removeItem(key); } catch {}
}
