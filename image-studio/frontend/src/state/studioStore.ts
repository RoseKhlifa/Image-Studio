import { create } from "zustand";
import {
  DEFAULT_AUTO_RETRY_COUNT,
  normalizeAutoRetryCount,
} from "../../../../shared/kernel/requestModel.js";
import {
  BuildBatchOutputPath,
  EventsOn,
  EventsOff,
  Generate as wailsGenerate,
  Edit as wailsEdit,
  OptimizePrompt as wailsOptimizePrompt,
  Cancel as wailsCancel,
  GetOutputDir,
  DeleteStoredAPIKey,
  GetStoredAPIKey,
  SetStoredAPIKey,
  RegisterMediaAsset,
  RegisterImportedImageAsset,
  SetKeepLogsEnabled,
  SetCleanupPreviewCacheOnExitEnabled,
  SetOutputDir,
  CheckForAppUpdate,
  WriteAppUpdateProbe,
  probeCurrentUpstream,
  setKernelRuntimeMode,
} from "../platform/runtime/host";
import type { backend } from "../../wailsjs/go/models";
import {
  APIMode,
  AppUpdateInfo,
  BatchProcessResultLink,
  BatchProcessSourceImage,
  BackgroundValue,
  EditSourceMode,
  HistoryItem,
  ImageStyleValue,
  InputFidelityValue,
  KernelRuntimeMode,
  LoopGenerationConfig,
  ModerationValue,
  Mode,
  OutputFormatValue,
  Preset,
  ProgressInfo,
  PromptTemplate,
  QualityValue,
  RequestPolicy,
  SizeValue,
  SourceImage,
  ThemeMode,
  Toast,
  UpstreamProfile,
  Workspace,
} from "../types/domain";
import {
  clearLegacyAPIKeys,
  loadLegacyModeAPIKey,
  loadLegacySharedAPIKey,
  loadTrustedOutputRoots,
  persistHistoryItem,
  persistHistoryItems,
  rememberTrustedOutputRoot,
  loadAllHistory,
  loadHistoryPage,
} from "../lib/storage";
import {
  compatibilityExportFingerprint,
  importCompatibilityStateIfNewer,
  readIgnoredReleaseTag,
  scheduleCompatibilityExport,
  writeIgnoredReleaseTag,
} from "../lib/compatState";
import { normalizeAppUpdateInfo } from "../lib/appUpdate.ts";
import { appVersion } from "../lib/version.ts";
import { normalizeSavePromptRequest, type SavePromptRequest } from "../lib/savePromptState";
import {
  readSavePromptSuppressed,
  writeSavePromptSuppressed,
} from "../lib/savePromptPreference";
import {
  normalizeCompletionSoundConfig,
  playCompletionSound,
  persistCompletionSoundConfig,
  readCompletionSoundConfig,
  shouldPlayCompletionSound,
} from "../lib/completionSound";
import {
  normalizeCompletionNotificationConfig,
  persistCompletionNotificationConfig,
  readCompletionNotificationConfig,
  readSystemNotificationPermission,
  requestSystemNotificationPermission,
  shouldSendCompletionNotification,
  showSystemNotification,
} from "../lib/completionNotification";
import {
  persistPromptTemplates,
  readStoredPromptTemplates,
} from "../lib/promptTemplates";
import {
  loadCustomAspectRatios,
  makeCustomAspectRatio,
  MAX_CUSTOM_ASPECT_RATIOS,
  persistCustomAspectRatios,
} from "../lib/customAspectRatios.ts";
import {
  cleanBaseURL,
} from "../lib/security";
import { loadProxyConfig, normalizeProxyMode, persistProxyConfig } from "../lib/proxy";
import {
  duplicateProfile as cloneProfile,
  genProfileId,
  keyringUserFor,
  pickActiveProfile,
} from "../lib/profiles";
import { isMac, readRuntimePlatformState } from "../platform";
import { dispatchFullscreenResize, setNativeFullscreen } from "../platform/nativeFullscreen";
import {
  activeRuntimePatch,
  apiModeLabel,
  defaultBatchProcessConfig,
  defaultLoopGenerationConfig,
  normalizeAutoAspectResolutionPreset,
  normalizeBatchCount,
  normalizeBatchProcessConcurrency,
  normalizeBatchProcessConfig,
  normalizeConcurrencyLimit,
  normalizeLoopGenerationConcurrency,
  normalizeLoopGenerationConfig,
  normalizeLoopGenerationCount,
  patchWorkspaceRuntime,
  workspaceRuntimeFromState,
  workspaceRunningCount,
  type APIModeValue,
  type RunningJobMeta,
  type WorkspacePatch,
} from "./workspaceRuntime";
import { getStreamPreviewDisableReason } from "./streamPreviewPolicy";
import {
  buildAspectSizeSelection,
  buildExactSizeValue,
  buildReferenceResolutionSizeSelection,
  buildCustomAspectValue,
  deriveAspectPreset,
  deriveResolutionPreset,
  formatSizeValue,
  isBuiltInAspectRatio,
  normalizeResolutionSelection,
  normalizeSizeSelection,
  supportsPreciseSizeControl,
} from "../components/panel/sizeCapabilities";
import { normalizeQualitySelection } from "../components/panel/panelOptions";
import { buildMacWorkspacePreview, buildWindowsRightRailPreview, readPreviewScenario } from "../app/dev/previewData";
import {
  applyTheme,
  augmentPromptWithAnnotations,
  buildMaskPNGDataURL,
  clearLegacyModeLocalStorage,
  genId,
  imageDims,
  loadModeConfig,
  loadStoredActiveProfileId,
  loadStoredProfiles,
  MAX_HISTORY_ITEMS,
  persistActiveProfileId,
  persistProfiles,
  persistTrimmedHistory,
  registerTrustedOutputRoots,
  stripDataURLPrefix,
  tempDataURLFromB64,
  trimHistory,
} from "./studioStore.shared";
import type { ModeConfig, PromptOptimizeRequest, Stroke, StudioState, UndoEntry } from "./studioStore.types";
import {
  cryptoIDFallback,
  ensureFullHistoryItem as ensureFullHistoryItemRuntime,
  historyItemsByIds,
  materializeHistoryItem as materializeHistoryItemRuntime,
  STYLE_SUFFIXES,
  withMediaAssetRef,
} from "./studioStore.runtime";
import { createMediaActions } from "./studioStore.media";
import { createProfileActions } from "./studioStore.profiles";
import { createWorkspaceActions } from "./studioStore.workspaces";
import { createImageActions } from "./studioStore.images";
import { saveHistoryItemToDirectory, saveHistoryItemToDirectoryAs } from "../lib/saveResultImage";
import {
  currentImageIdForWorkspaceSnapshot,
  removeStreamPreview,
  restoreCurrentImageAfterPreviewError,
  streamPreviewStatePatch,
  type StreamPreviewPayload,
} from "./studioStore.streamPreview";
import type { GenerateOptionsLike } from "../platform/runtime/hostTypes";

type RuntimeGenerateOptions = GenerateOptionsLike & {
  sourceImages?: Array<SourceImage | BatchProcessSourceImage>;
};

type JobSnapshot = {
  workspaceId: string;
  apiMode: APIModeValue;
  batchIndex: number;
  previewSlotIndex?: number;
  size: SizeValue;
  quality: QualityValue;
  outputFormat: OutputFormatValue;
  sources: SourceImage[];
  currentImage: HistoryItem | null;
  styleTag: string;
  loopGeneration: LoopGenerationConfig;
  editSourceMode: EditSourceMode;
  batchProcessLink?: BatchProcessResultLink;
};

type LaunchOneJobHooks = {
  onSettled?: (status: "success" | "error") => void;
};

type LoopRunController = {
  workspaceId: string;
  totalJobs: number;
  maxConcurrent: number;
  launchedJobs: number;
  nextPreviewSlotIndex: number;
  mode: Mode;
  payload: RuntimeGenerateOptions;
  snapshotBase: Omit<JobSnapshot, "batchIndex">;
  batchSources?: BatchProcessSourceImage[];
  batchOutputDir?: string;
  batchOutputPrefix?: string;
  batchAutoAspectResolution?: Exclude<import("../types/domain").BatchProcessAutoAspectResolution, ""> | "";
  batchCapabilityInput?: {
    apiMode: APIMode;
    requestPolicy: RequestPolicy;
    imageModelID?: string;
  };
  batchCustomAspectRatios?: import("../types/domain").CustomAspectRatio[];
  stopped: boolean;
};

const loopRunControllers = new Map<string, LoopRunController>();

function stopLoopRun(workspaceId: string): void {
  const controller = loopRunControllers.get(workspaceId);
  if (controller) controller.stopped = true;
  loopRunControllers.delete(workspaceId);
}

function directoryFromPath(path: string): string {
  const normalized = path.trim();
  const idx = Math.max(normalized.lastIndexOf("/"), normalized.lastIndexOf("\\"));
  return idx >= 0 ? normalized.slice(0, idx) : "";
}

function launchQueuedLoopJobs(controller: LoopRunController): void {
  if (controller.stopped) return;
  const state = useStudioStore.getState();
  const workspaceExists = state.activeWorkspaceId === controller.workspaceId
    || state.workspaces.some((workspace) => workspace.id === controller.workspaceId);
  if (!workspaceExists) {
    stopLoopRun(controller.workspaceId);
    return;
  }
  const runtime = workspaceRuntimeFromState(state, controller.workspaceId);
  const batchQueueMode = Array.isArray(controller.batchSources) && controller.batchSources.length > 0;
  if (!batchQueueMode && runtime.jobsTotal === 0) {
    stopLoopRun(controller.workspaceId);
    return;
  }

  while (!controller.stopped && controller.launchedJobs < controller.totalJobs) {
    const latestState = useStudioStore.getState();
    const latestRuntime = workspaceRuntimeFromState(latestState, controller.workspaceId);
    if ((!batchQueueMode && latestRuntime.jobsTotal === 0) || latestRuntime.runningJobs.length >= controller.maxConcurrent) break;
    const batchIndex = controller.launchedJobs;
    controller.launchedJobs += 1;
    const previewSlotIndex = controller.nextPreviewSlotIndex;
    controller.nextPreviewSlotIndex = (controller.nextPreviewSlotIndex + 1) % Math.max(1, controller.maxConcurrent);
    const payloadSeed = controller.payload.seed ? controller.payload.seed + batchIndex : 0;
    const batchSource = controller.batchSources?.[batchIndex];
    const batchPayloadSize = batchSource
      && controller.snapshotBase.editSourceMode === "batch"
      && controller.batchAutoAspectResolution
      && controller.batchCapabilityInput
      ? buildReferenceResolutionSizeSelection(
          controller.batchAutoAspectResolution,
          batchSource.width && batchSource.height ? { width: batchSource.width, height: batchSource.height } : null,
          controller.batchCapabilityInput,
          controller.batchCustomAspectRatios ?? [],
        )
      : controller.payload.size;
    const nextPayload: RuntimeGenerateOptions = {
      ...controller.payload,
      seed: payloadSeed,
      size: batchPayloadSize,
      imagePaths: batchSource ? [batchSource.path] : controller.payload.imagePaths,
      sourceImages: batchSource ? [batchSource] : controller.payload.sourceImages,
    };
    void launchOneJob(controller.mode, nextPayload, {
      ...controller.snapshotBase,
      batchIndex,
      previewSlotIndex,
      sources: batchSource ? [batchSource as SourceImage] : controller.snapshotBase.sources,
      batchProcessLink: batchSource
        ? {
            sourcePath: batchSource.path,
            outputDir: controller.batchOutputDir || "",
            outputNamePrefix: controller.batchOutputPrefix || "processed-",
          }
        : undefined,
    }, {
      onSettled: (status) => {
        const current = loopRunControllers.get(controller.workspaceId);
        if (!current || current !== controller) return;
        if (status === "error" && !batchQueueMode) {
          stopLoopRun(controller.workspaceId);
          return;
        }
        const currentState = useStudioStore.getState();
        const currentRuntime = workspaceRuntimeFromState(currentState, controller.workspaceId);
        if (batchQueueMode) {
          if (current.launchedJobs >= current.totalJobs && currentRuntime.runningJobs.length === 0) {
            stopLoopRun(controller.workspaceId);
            return;
          }
        } else if (currentRuntime.jobsTotal === 0) {
          stopLoopRun(controller.workspaceId);
          return;
        }
        launchQueuedLoopJobs(controller);
      },
    });
  }
}

const KEEP_LOGS_KEY = "gptcodex.keepLogs";
const CLEANUP_PREVIEW_CACHE_ON_EXIT_KEY = "gptcodex.cleanupPreviewCacheOnExit";
const AUTO_RETRY_ENABLED_KEY = "gptcodex.autoRetryEnabled";
const AUTO_RETRY_COUNT_KEY = "gptcodex.autoRetryCount";
const PROTECT_STREAM_PREVIEW_KEY = "gptcodex.protectStreamPreview";
const INITIAL_HISTORY_LOAD = 18;
const HISTORY_MEDIA_HYDRATE_CONCURRENCY = 4;

let deferredHistoryLoadPromise: Promise<void> | null = null;

function readKeepLogs(): boolean {
  try {
    return localStorage.getItem(KEEP_LOGS_KEY) === "1";
  } catch {
    return false;
  }
}

function writeKeepLogs(value: boolean): void {
  try {
    if (value) localStorage.setItem(KEEP_LOGS_KEY, "1");
    else localStorage.removeItem(KEEP_LOGS_KEY);
  } catch {
    // localStorage can be unavailable in tests/previews.
  }
}

function readCleanupPreviewCacheOnExit(): boolean {
  try {
    return localStorage.getItem(CLEANUP_PREVIEW_CACHE_ON_EXIT_KEY) === "1";
  } catch {
    return false;
  }
}

function writeCleanupPreviewCacheOnExit(value: boolean): void {
  try {
    if (value) localStorage.setItem(CLEANUP_PREVIEW_CACHE_ON_EXIT_KEY, "1");
    else localStorage.removeItem(CLEANUP_PREVIEW_CACHE_ON_EXIT_KEY);
  } catch {
    // localStorage can be unavailable in tests/previews.
  }
}

function readProtectStreamPreview(): boolean {
  try {
    return localStorage.getItem(PROTECT_STREAM_PREVIEW_KEY) !== "0";
  } catch {
    return true;
  }
}

function readAutoRetryEnabled(): boolean {
  try {
    return localStorage.getItem(AUTO_RETRY_ENABLED_KEY) !== "0";
  } catch {
    return true;
  }
}

function writeAutoRetryEnabled(value: boolean): void {
  try {
    if (value) localStorage.removeItem(AUTO_RETRY_ENABLED_KEY);
    else localStorage.setItem(AUTO_RETRY_ENABLED_KEY, "0");
  } catch {
    // localStorage can be unavailable in tests/previews.
  }
}

function readAutoRetryCount(): number {
  try {
    return normalizeAutoRetryCount(localStorage.getItem(AUTO_RETRY_COUNT_KEY));
  } catch {
    return DEFAULT_AUTO_RETRY_COUNT;
  }
}

function writeAutoRetryCount(value: number): void {
  try {
    localStorage.setItem(AUTO_RETRY_COUNT_KEY, String(normalizeAutoRetryCount(value)));
  } catch {
    // localStorage can be unavailable in tests/previews.
  }
}

function writeProtectStreamPreview(value: boolean): void {
  try {
    if (value) localStorage.removeItem(PROTECT_STREAM_PREVIEW_KEY);
    else localStorage.setItem(PROTECT_STREAM_PREVIEW_KEY, "0");
  } catch {
    // localStorage can be unavailable in tests/previews.
  }
}

function normalizeBackgroundValue(value: unknown): BackgroundValue {
  return value === "opaque" || value === "transparent" || value === "auto" ? value : "auto";
}

function normalizeOutputCompressionValue(value: unknown): number {
  if (value === null || value === undefined || value === "") return 100;
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) return 100;
  return Math.max(0, Math.min(100, Math.round(numeric)));
}

function normalizeInputFidelityValue(value: unknown): InputFidelityValue {
  return value === "low" || value === "high" || value === "auto" ? value : "auto";
}

function normalizeImageStyleValue(value: unknown): ImageStyleValue {
  return value === "vivid" || value === "natural" || value === "default" ? value : "default";
}

function normalizeUserIdentifierValue(value: unknown): string {
  const trimmed = String(value ?? "").trim();
  if (!trimmed) return "";
  return Array.from(trimmed).slice(0, 64).join("");
}

function normalizePartialImagesValue(value: unknown): number {
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric < 0) return 1;
  return Math.max(0, Math.min(3, Math.floor(numeric)));
}

async function writeBase64ToTempFile(b64: string, _name: string): Promise<string> {
  // Backend doesn't currently expose a "write temp file from b64" binding,
  // but reuseAsSource needs a path for edit mode. Workaround: use SaveImageAs
  // with a fixed name into the user config dir would prompt the user. Instead,
  // we re-purpose the savedPath field that comes back with every result — it's
  // already on disk under UserConfigDir/image-studio/images. So callers should
  // use item.savedPath; this helper exists for parity and is currently unused.
  void b64;
  return "";
}

function needsHistoryPreviewHydration(item: HistoryItem): boolean {
  return !!item.savedPath
    && !item.savedPath.startsWith("memory://")
    && !item.previewBlob
    && !item.imageB64;
}

async function mapWithConcurrency<T, R>(
  items: T[],
  concurrency: number,
  task: (item: T, index: number) => Promise<R>,
): Promise<R[]> {
  if (items.length === 0) return [];
  const results = new Array<R>(items.length);
  let cursor = 0;
  const worker = async () => {
    while (cursor < items.length) {
      const index = cursor;
      cursor += 1;
      results[index] = await task(items[index], index);
    }
  };
  const workerCount = Math.min(Math.max(1, concurrency), items.length);
  await Promise.all(Array.from({ length: workerCount }, () => worker()));
  return results;
}

function mergeHistoryMediaRef(current: HistoryItem | null, nextById: Map<string, HistoryItem>): HistoryItem | null {
  if (!current) return current;
  const next = nextById.get(current.id);
  return next ? withMediaAssetRef(current, next) : current;
}

function batchSavePromptRequestFromState(state: StudioState, workspaceId: string): SavePromptRequest | null {
  const workspace = state.workspaces.find((entry) => entry.id === workspaceId);
  const items = historyItemsByIds(state.history, workspace?.batchResultIds ?? []).sort((a, b) => {
    const aIndex = typeof a.batchIndex === "number" ? a.batchIndex : Number.MAX_SAFE_INTEGER;
    const bIndex = typeof b.batchIndex === "number" ? b.batchIndex : Number.MAX_SAFE_INTEGER;
    if (aIndex !== bIndex) return aIndex - bIndex;
    if (a.createdAt !== b.createdAt) return a.createdAt - b.createdAt;
    return a.id.localeCompare(b.id);
  });
  if (items.length === 0) return null;
  return {
    kind: "batch",
    items,
    workspaceId,
  };
}

function maybeEnqueueBatchSavePrompt(
  store: typeof useStudioStore,
  workspaceId: string,
  completedNow: number,
  totalNow: number,
): void {
  if (totalNow <= 1 || completedNow !== totalNow) return;
  const batchRequest = batchSavePromptRequestFromState(store.getState(), workspaceId);
  if (batchRequest) {
    store.getState().enqueueSavePrompt(batchRequest);
  }
}

async function hydrateHistoryPreviewRefs(items: HistoryItem[]): Promise<HistoryItem[]> {
  return mapWithConcurrency(items, HISTORY_MEDIA_HYDRATE_CONCURRENCY, async (item) => {
    if (!needsHistoryPreviewHydration(item)) return item;
    try {
      const ref = item.thumbPath
        ? await RegisterMediaAsset(item.savedPath!, item.thumbPath)
        : await RegisterImportedImageAsset(item.savedPath!);
      return withMediaAssetRef(item, ref);
    } catch {
      return item;
    }
  });
}

async function backfillHistoryPreviewRefs(items: HistoryItem[]): Promise<void> {
  const hydrated = await hydrateHistoryPreviewRefs(items);
  const changed = hydrated.filter((item, index) => item !== items[index]);
  if (changed.length === 0) return;
  const changedById = new Map(changed.map((item) => [item.id, item]));
  useStudioStore.setState((state) => ({
    history: state.history.map((item) => changedById.get(item.id) ?? item),
    batchResults: state.batchResults.map((item) => {
      const next = changedById.get(item.id);
      return next ? withMediaAssetRef(item, next) : item;
    }),
    currentImage: mergeHistoryMediaRef(state.currentImage, changedById),
    compareB: mergeHistoryMediaRef(state.compareB, changedById),
    resultDetail: mergeHistoryMediaRef(state.resultDetail, changedById),
  }));
  await persistHistoryItems(changed).catch(() => undefined);
}

const mediaActions = createMediaActions({
  getState: () => useStudioStore.getState(),
  setState: (patch) => {
    if (typeof patch === "function") {
      useStudioStore.setState((state) => patch(state));
      return;
    }
    useStudioStore.setState(patch);
  },
});

const profileActions = createProfileActions({
  getState: () => useStudioStore.getState(),
  setState: (patch) => {
    if (typeof patch === "function") {
      useStudioStore.setState((state) => patch(state));
      return;
    }
    useStudioStore.setState(patch);
  },
});

const workspaceActions = createWorkspaceActions({
  getState: () => useStudioStore.getState(),
  setState: (patch) => {
    if (typeof patch === "function") {
      useStudioStore.setState((state) => patch(state));
      return;
    }
    useStudioStore.setState(patch);
  },
});

const imageActions = createImageActions({
  getState: () => useStudioStore.getState(),
  setState: (patch) => {
    if (typeof patch === "function") {
      useStudioStore.setState((state) => patch(state));
      return;
    }
    useStudioStore.setState(patch);
  },
});

export const useStudioStore = create<StudioState>((set, get) => ({
  apiKey: "",
  mode: "generate",
  prompt: "",
  negativePrompt: "",
  size: "1024x1024",
  quality: "medium",
  outputFormat: "png",
  seed: 0,
  background: "auto",
  outputCompression: 100,
  inputFidelity: "auto",
  imageStyle: "default",
  moderation: "low",
  userIdentifier: "",
  partialImages: 1,
  protectStreamPreview: true,
  autoRetryEnabled: true,
  autoRetryCount: DEFAULT_AUTO_RETRY_COUNT,
  kernelRuntimeMode: "auto",
  baseURL: "",
  textModelID: "",
  imageModelID: "",
  reasoningEffort: "xhigh",
  proxyMode: "system",
  proxyURL: "",
  apiMode: "responses",
  responsesTransport: "sse",
  requestPolicy: "openai",
  imagesNewAPICompat: false,
  noPromptRevision: true,
  profiles: [],
  activeProfileId: "",
  sources: [],

  runningJobs: [],
  jobsTotal: 0,
  jobsCompleted: 0,
  progress: null,
  streamPreview: null,
  streamPreviews: {},
  lastLogLine: "",
  errorMessage: null,
  errorCanRetry: false,
  errorRawPath: null,
  isRunning: false,
  lastPayload: null,
  runningJobMeta: {},

  currentImage: null,
  history: [],
  historyHasMore: false,
  historyLoading: false,
  historyCursorBeforeDayStart: null,
  batchResults: [],
  resultGridOpen: false,
  historyRailCollapsed: false,
  historyTimelineOpen: false,

  tool: "pan",
  brushSize: 30,
  brushMode: "paint",
  annotationKind: "rect",
  annotationColor: "#ff4d4d",
  selectedAnnotationId: null,
  maskDataURL: null,
  strokes: [],
  annotations: [],
  undoStack: [],
  redoStack: [],

  compareB: null,
  compareSplit: 0.5,

  toasts: [],
  recentDurations: [],
  viewZoom: 1,
  canvasViewResetTick: 0,
  fullscreen: false,
  starPromptOpen: false,
  starPromptSource: "auto",
  savePromptRequest: null,
  savePromptQueue: [],
  savePromptSuppressed: readSavePromptSuppressed(),
  keepLogs: readKeepLogs(),
  cleanupPreviewCacheOnExit: readCleanupPreviewCacheOnExit(),
  completionSound: readCompletionSoundConfig(),
  completionNotification: readCompletionNotificationConfig(),
  completionNotificationPermission: readSystemNotificationPermission(),
  ignoredReleaseTag: readIgnoredReleaseTag(),
  appUpdate: null,
  appUpdateModalOpen: false,
  promptHistory: [],
  promptTemplates: [],
  batchCount: 1,
  editSourceMode: "manual",
  editAutoAspectResolution: "",
  batchProcess: defaultBatchProcessConfig(),
  loopGeneration: defaultLoopGenerationConfig(),
  presets: [],
  customAspectRatios: [],
  theme: "system",
  fontScale: 1,
  customAspectRatioModalOpen: false,
  openCustomAspectRatioModal: () => set({ customAspectRatioModalOpen: true }),
  closeCustomAspectRatioModal: () => set({ customAspectRatioModalOpen: false }),
  addCustomAspectRatio: (width, height) => {
    const nextRatio = makeCustomAspectRatio(width, height);
    if (!nextRatio) {
      get().pushToast("请输入有效的宽高比例", "warn");
      return false;
    }
    if (isBuiltInAspectRatio(nextRatio.width, nextRatio.height)) {
      get().pushToast("这个比例已经内置了", "warn");
      return false;
    }
    const existing = get().customAspectRatios;
    if (existing.some((item) => item.id === nextRatio.id)) {
      get().pushToast(`比例 ${nextRatio.label} 已存在`, "warn");
      return false;
    }
    if (existing.length >= MAX_CUSTOM_ASPECT_RATIOS) {
      get().pushToast(`最多保存 ${MAX_CUSTOM_ASPECT_RATIOS} 个自定义比例`, "warn");
      return false;
    }
    const next = [...existing, nextRatio];
    persistCustomAspectRatios(next);
    set({ customAspectRatios: next });
    get().pushToast(`已添加比例 ${nextRatio.label}`, "success");
    return true;
  },
  deleteCustomAspectRatio: (id) => {
    const state = get();
    const removed = state.customAspectRatios.find((item) => item.id === id);
    if (!removed) return;
    const next = state.customAspectRatios.filter((item) => item.id !== id);
    persistCustomAspectRatios(next);
    const patch: Partial<StudioState> = { customAspectRatios: next };
    if (deriveAspectPreset(state.size, state.customAspectRatios) === buildCustomAspectValue(id)) {
      const resolution = deriveResolutionPreset(state.size);
      patch.size = buildAspectSizeSelection(
        "1:1",
        resolution === "auto" ? "1k" : resolution,
        {
          apiMode: state.apiMode,
          requestPolicy: state.requestPolicy,
          imageModelID: state.imageModelID,
        },
        next,
      );
    }
    set(patch);
    get().pushToast(`已删除比例 ${removed.label}`, "success");
  },
  customSizeModalOpen: false,
  openCustomSizeModal: () => set({ customSizeModalOpen: true }),
  closeCustomSizeModal: () => set({ customSizeModalOpen: false }),
  applyCustomSize: (width, height) => {
    const state = get();
    if (!supportsPreciseSizeControl({
      apiMode: state.apiMode,
      requestPolicy: state.requestPolicy,
      imageModelID: state.imageModelID,
    })) {
      state.pushToast("当前模型链路不支持精确尺寸自定义", "warn");
      return false;
    }
    const nextSize = buildExactSizeValue(width, height);
    if (!nextSize) {
      state.pushToast("请输入 64 到 3840 之间的整数尺寸，并满足最长边 3840、宽高比不超过 3:1、总像素不超过 8294400", "warn");
      return false;
    }
    set({ size: nextSize, customSizeModalOpen: false });
    get().pushToast(`已应用精确尺寸 ${formatSizeValue(nextSize)}`, "success");
    return true;
  },
  settingsOpen: false,
  openSettings: () => set({ settingsOpen: true, upstreamModalOpen: false }),
  closeSettings: () => set({ settingsOpen: false }),
  isTestingKey: false,
  isOptimizingPrompt: false,
  upstreamModalOpen: false,
  upstreamReturnTarget: "app",
  openUpstreamConfig: (returnTarget = "app") => set({
    upstreamModalOpen: true,
    upstreamReturnTarget: returnTarget,
    settingsOpen: false,
  }),
  closeUpstreamConfig: () => {
    const { upstreamReturnTarget } = get();
    set({
      upstreamModalOpen: false,
      settingsOpen: upstreamReturnTarget === "settings",
      upstreamReturnTarget: "app",
    });
  },
  openStarPrompt: () => {
    if (isMac) return;
    set({ starPromptOpen: true, starPromptSource: "manual" });
  },
  dismissStarPrompt: () => {
    set({ starPromptOpen: false });
    try { localStorage.setItem("gptcodex.starPrompted", "1"); } catch {}
  },
  enqueueSavePrompt: (request) => {
    if (get().savePromptSuppressed) return;
    const normalized = normalizeSavePromptRequest(request);
    if (!normalized) return;
    const hydrated = normalized.kind === "batch"
      ? {
          ...normalized,
          items: historyItemsByIds(normalized.items, normalized.items.map((item) => item.id)),
        }
      : normalized;
    if (hydrated.kind === "batch" && hydrated.items.length === 0) return;
    set((state) => {
      if (!state.savePromptRequest) return { savePromptRequest: hydrated };
      return { savePromptQueue: [...state.savePromptQueue, hydrated].slice(-12) };
    });
  },
  closeSavePrompt: () => {
    set((state) => {
      const [next, ...rest] = state.savePromptQueue;
      return {
        savePromptRequest: next ?? null,
        savePromptQueue: rest,
      };
    });
  },
  setSavePromptSuppressed: (value) => {
    writeSavePromptSuppressed(value);
    set(value ? { savePromptSuppressed: true, savePromptRequest: null, savePromptQueue: [] } : { savePromptSuppressed: false });
  },
  setKeepLogs: async (value) => {
    writeKeepLogs(value);
    set({ keepLogs: value });
    await SetKeepLogsEnabled(value).catch(() => undefined);
  },
  setCleanupPreviewCacheOnExit: async (value) => {
    writeCleanupPreviewCacheOnExit(value);
    set({ cleanupPreviewCacheOnExit: value });
    await SetCleanupPreviewCacheOnExitEnabled(value).catch(() => undefined);
  },
  ignoreAppUpdate: (releaseTag) => {
    const trimmed = releaseTag.trim();
    writeIgnoredReleaseTag(trimmed);
    set((state) => ({
      ignoredReleaseTag: trimmed,
      appUpdate: state.appUpdate?.releaseTag === trimmed ? null : state.appUpdate,
      appUpdateModalOpen: false,
    }));
  },
  dismissAppUpdateModal: () => {
    set({ appUpdateModalOpen: false });
  },
  setCompletionSoundEnabled: (value) => {
    const next = normalizeCompletionSoundConfig({ ...get().completionSound, enabled: value });
    persistCompletionSoundConfig(next);
    set({ completionSound: next });
  },
  setCompletionSoundMode: (value) => {
    const next = normalizeCompletionSoundConfig({ ...get().completionSound, mode: value });
    persistCompletionSoundConfig(next);
    set({ completionSound: next });
  },
  setCompletionSoundCustom: (input) => {
    const next = normalizeCompletionSoundConfig({
      ...get().completionSound,
      mode: "custom",
      customName: input.name,
      customDataURL: input.dataURL,
    });
    persistCompletionSoundConfig(next);
    set({ completionSound: next });
  },
  resetCompletionSoundCustom: () => {
    const next = normalizeCompletionSoundConfig({
      ...get().completionSound,
      mode: "default",
      customName: "",
      customDataURL: "",
    });
    persistCompletionSoundConfig(next);
    set({ completionSound: next });
  },
  previewCompletionSound: async () => {
    await playCompletionSound(get().completionSound, { force: true });
  },
  setCompletionNotificationEnabled: async (value) => {
    if (!value) {
      const next = normalizeCompletionNotificationConfig({ ...get().completionNotification, enabled: false });
      persistCompletionNotificationConfig(next);
      const permission = readSystemNotificationPermission();
      set({
        completionNotification: next,
        completionNotificationPermission: permission,
      });
      return permission;
    }
    const permission = await requestSystemNotificationPermission();
    set({ completionNotificationPermission: permission });
    if (permission !== "granted") return permission;
    const next = normalizeCompletionNotificationConfig({ ...get().completionNotification, enabled: true });
    persistCompletionNotificationConfig(next);
    set({ completionNotification: next });
    return permission;
  },
  requestCompletionNotificationPermission: async () => {
    const permission = await requestSystemNotificationPermission();
    set({ completionNotificationPermission: permission });
    return permission;
  },
  workspaces: [],
  activeWorkspaceId: "",
  selectedPresetId: null,
  styleTag: "",

  setField: (key, value) => {
    // 上游字段(apiKey / baseURL / textModelID / imageModelID / apiMode)是
    // active profile 的派生镜像,直接 set 顶层不持久化,改完下次启动就丢。
    // 这些字段必须走 updateProfile / setActiveProfile 这两个 action。开发期
    // 抓一下,生产期还是 set 一下顶层让 UI 不爆炸。
    if (key === "apiMode" || key === "baseURL" || key === "apiKey" ||
        key === "textModelID" || key === "imageModelID" || key === "reasoningEffort") {
      if (typeof console !== "undefined") {
        console.warn(`setField("${String(key)}", ...) 不写持久化;改这个字段请用 updateProfile / setActiveProfile`);
      }
      set({ [key]: value } as any);
      return;
    }
    // 其他全局偏好字段
    const normalizedValue = key === "batchCount"
      ? normalizeBatchCount(value)
      : key === "batchProcess"
        ? normalizeBatchProcessConfig(value)
      : key === "editAutoAspectResolution"
        ? normalizeAutoAspectResolutionPreset(value)
      : key === "background"
        ? normalizeBackgroundValue(value)
        : key === "outputCompression"
          ? normalizeOutputCompressionValue(value)
          : key === "inputFidelity"
            ? normalizeInputFidelityValue(value)
            : key === "imageStyle"
              ? normalizeImageStyleValue(value)
              : key === "userIdentifier"
                ? normalizeUserIdentifierValue(value)
                  : key === "partialImages"
                    ? normalizePartialImagesValue(value)
                    : key === "protectStreamPreview"
                      ? value !== false
                    : key === "autoRetryEnabled"
                      ? value !== false
                      : key === "autoRetryCount"
                        ? normalizeAutoRetryCount(value)
      : key === "loopGeneration"
        ? normalizeLoopGenerationConfig(value)
        : value;
    set({ [key]: normalizedValue } as any);
    if (key === "currentImage") {
      const item = normalizedValue as HistoryItem | null;
      const workspace = get().workspaces.find((w) => w.id === get().activeWorkspaceId);
      set({
        compareB: null,
        resultGridOpen: false,
        workspaces: patchWorkspaceRuntime(get().workspaces, get().activeWorkspaceId, {
          currentImageId: currentImageIdForWorkspaceSnapshot(item, get().streamPreview, get().streamPreviews, workspace?.currentImageId ?? null),
          resultGridOpen: false,
        }),
      });
    } else if (key === "batchCount") {
      const value = normalizedValue as number;
      set({
        workspaces: get().workspaces.map((w) => (
          w.id === get().activeWorkspaceId ? { ...w, batchCount: value } : w
        )),
      });
    } else if (key === "selectedPresetId") {
      set({
        workspaces: patchWorkspaceRuntime(get().workspaces, get().activeWorkspaceId, {
          selectedPresetId: normalizedValue as string | null,
        }),
      });
    } else if (key === "mode") {
      const value = normalizedValue as Mode;
      if (value === "edit" && get().sources.length === 0 && get().currentImage?.savedPath && get().size !== "auto") {
        set({ size: "auto" });
      }
    } else if (key === "loopGeneration") {
      const value = normalizedValue as LoopGenerationConfig;
      set({
        workspaces: get().workspaces.map((w) => (
          w.id === get().activeWorkspaceId ? { ...w, loopGeneration: value } : w
        )),
      });
    } else if (key === "editSourceMode") {
      const value = normalizedValue as EditSourceMode;
      set({
        workspaces: get().workspaces.map((w) => (
          w.id === get().activeWorkspaceId ? { ...w, editSourceMode: value } : w
        )),
      });
    } else if (key === "editAutoAspectResolution") {
      set({
        workspaces: patchWorkspaceRuntime(get().workspaces, get().activeWorkspaceId, {
          editAutoAspectResolution: normalizedValue as StudioState["editAutoAspectResolution"],
        }),
      });
    } else if (key === "batchProcess") {
      const value = normalizedValue as StudioState["batchProcess"];
      set({
        workspaces: get().workspaces.map((w) => (
          w.id === get().activeWorkspaceId ? { ...w, batchProcess: value } : w
        )),
      });
    } else if (key === "errorMessage") {
      set({ workspaces: patchWorkspaceRuntime(get().workspaces, get().activeWorkspaceId, { errorMessage: value as string | null }) });
    } else if (key === "errorCanRetry") {
      set({ workspaces: patchWorkspaceRuntime(get().workspaces, get().activeWorkspaceId, { errorCanRetry: value as boolean }) });
    } else if (key === "errorRawPath") {
      set({ workspaces: patchWorkspaceRuntime(get().workspaces, get().activeWorkspaceId, { errorRawPath: value as string | null }) });
    } else if (key === "lastPayload") {
      set({ workspaces: patchWorkspaceRuntime(get().workspaces, get().activeWorkspaceId, { lastPayload: value as GenerateOptionsLike | null }) });
    }
    if (key === "kernelRuntimeMode") {
      try { localStorage.setItem("gptcodex.kernelRuntimeMode", String(value)); } catch {}
      setKernelRuntimeMode(value as KernelRuntimeMode);
    } else if (key === "outputFormat") {
      try { localStorage.setItem("gptcodex.outputFormat", String(value)); } catch {}
    } else if (key === "background") {
      try { localStorage.setItem("gptcodex.background", String(value)); } catch {}
    } else if (key === "outputCompression") {
      try { localStorage.setItem("gptcodex.outputCompression", String(value)); } catch {}
    } else if (key === "inputFidelity") {
      try { localStorage.setItem("gptcodex.inputFidelity", String(value)); } catch {}
    } else if (key === "imageStyle") {
      try { localStorage.setItem("gptcodex.imageStyle", String(value)); } catch {}
    } else if (key === "moderation") {
      try { localStorage.setItem("gptcodex.moderation", String(value)); } catch {}
    } else if (key === "userIdentifier") {
      try { localStorage.setItem("gptcodex.userIdentifier", String(value)); } catch {}
    } else if (key === "partialImages") {
      try { localStorage.setItem("gptcodex.partialImages", String(value)); } catch {}
    } else if (key === "protectStreamPreview") {
      writeProtectStreamPreview(value !== false);
    } else if (key === "autoRetryEnabled") {
      writeAutoRetryEnabled(value !== false);
    } else if (key === "autoRetryCount") {
      writeAutoRetryCount(Number(value));
    }
  },
  setFullscreen: async (value) => {
    const next = !!value;
    set({ fullscreen: next });
    dispatchFullscreenResize();
    try {
      await setNativeFullscreen(next);
    } catch (error: any) {
      const platform = readRuntimePlatformState();
      const message = platform.isAndroid
        ? `Android 原生全屏切换失败:${error?.message ?? error}`
        : `原生全屏切换失败:${error?.message ?? error}`;
      get().pushToast(message, "error", 6000);
    } finally {
      dispatchFullscreenResize();
      set((state) => ({ canvasViewResetTick: state.canvasViewResetTick + 1 }));
    }
  },
  toggleFullscreen: async () => {
    await get().setFullscreen(!get().fullscreen);
  },

  setAPIKey: async (v) => {
    const trimmed = v.trim();
    const activeId = get().activeProfileId;
    if (!activeId) {
      // 没有 active profile,设 key 没意义;留个 warning 方便排查。
      if (typeof console !== "undefined") console.warn("setAPIKey: 没有 active profile,丢弃");
      return;
    }
    // 顶层镜像立即更新,UI 立即响应;keyring 写入异步
    set({ apiKey: trimmed });
    await SetStoredAPIKey(keyringUserFor(activeId), trimmed);
  },

  createProfile: async (input) => profileActions.createProfile(input),
  updateProfile: async (id, patch) => profileActions.updateProfile(id, patch),
  deleteProfile: async (id) => profileActions.deleteProfile(id),
  duplicateProfile: async (id) => profileActions.duplicateProfile(id),
  setActiveProfile: async (id) => profileActions.setActiveProfile(id),

  clearError: () => {
    const wsId = get().activeWorkspaceId;
    set({
      errorMessage: null,
      errorCanRetry: false,
      errorRawPath: null,
      workspaces: patchWorkspaceRuntime(get().workspaces, wsId, {
        errorMessage: null,
        errorCanRetry: false,
        errorRawPath: null,
      }),
    });
  },

  selectSourceImage: async () => imageActions.selectSourceImage(),
  chooseBatchInputDir: async () => imageActions.chooseBatchInputDir(),
  chooseBatchInputFiles: async () => imageActions.chooseBatchInputFiles(),
  refreshBatchInputDir: async () => imageActions.refreshBatchInputDir(),
  removeSource: (index) => imageActions.removeSource(index),
  clearSources: () => imageActions.clearSources(),
  reorderSources: (from, to) => imageActions.reorderSources(from, to),

  submit: async () => {
    const s = get();
    if (s.isRunning) return;
    if (!s.apiKey.trim()) {
      set({ errorMessage: "请填写 API Key", errorCanRetry: false, errorRawPath: null });
      return;
    }
    if (!s.prompt.trim()) {
      set({ errorMessage: "请填写提示词", errorCanRetry: false, errorRawPath: null });
      return;
    }
    if (!s.baseURL.trim()) {
      set({ errorMessage: "请在右侧工作栏顶部的「上游配置」中填入你的中转站地址(必须兼容 OpenAI Responses API + image_generation 工具)", errorCanRetry: false, errorRawPath: null });
      return;
    }
    const cleanedBaseURL = cleanBaseURL(s.baseURL);
    const batchCount = normalizeBatchCount(s.batchCount);
    const batchProcess = normalizeBatchProcessConfig(s.batchProcess);
    const loopGeneration = normalizeLoopGenerationConfig(s.loopGeneration);
    const batchProcessActive = batchProcess.enabled || !!batchProcess.inputDir.trim() || batchProcess.discoveredSources.length > 0;
    const batchProcessEnabled = s.mode === "edit" && s.editSourceMode === "batch" && batchProcessActive;
    const loopEnabled = !batchProcessEnabled && loopGeneration.enabled;
    const requestedJobCount = loopEnabled
      ? normalizeLoopGenerationCount(loopGeneration.totalCount)
      : batchProcessEnabled
        ? batchProcess.discoveredSources.length
        : batchCount;
    const requestedConcurrency = loopEnabled
      ? Math.min(requestedJobCount, normalizeLoopGenerationConcurrency(loopGeneration.concurrency))
      : batchProcessEnabled
        ? Math.min(requestedJobCount, normalizeBatchProcessConcurrency(batchProcess.concurrency))
        : batchCount;
    const runtimePlatform = readRuntimePlatformState();
    if (loopEnabled && loopGeneration.autoSave && !loopGeneration.autoSaveDir.trim()) {
      set({ errorMessage: "请先为循环出图配置自动另存为路径", errorCanRetry: false, errorRawPath: null });
      return;
    }
    if (batchProcessEnabled) {
      if (runtimePlatform.isAndroid) {
        set({ errorMessage: "批处理模式暂仅支持桌面端", errorCanRetry: false, errorRawPath: null });
        return;
      }
      if (batchProcess.discoveredSources.length === 0) {
        set({ errorMessage: "请先选择批处理输入目录，或直接选择多张图片加入队列", errorCanRetry: false, errorRawPath: null });
        return;
      }
      if (batchProcess.outputMode === "custom_dir" && !batchProcess.outputDir.trim()) {
        set({ errorMessage: "请先选择独立输出路径", errorCanRetry: false, errorRawPath: null });
        return;
      }
    }
    const activeProfile = s.profiles.find((p) => p.id === s.activeProfileId);
    const responsesTransport = activeProfile?.responsesTransport ?? s.responsesTransport;
    if (s.apiMode === "responses" && responsesTransport === "websocket" && s.kernelRuntimeMode === "remote" && !runtimePlatform.isAndroid) {
      set({
        errorMessage: "当前远程内核模式暂不支持 Responses WebSocket mode，请切回本地内核或关闭该开关。",
        errorCanRetry: false,
        errorRawPath: null,
      });
      return;
    }
    const concurrencyLimit = normalizeConcurrencyLimit(activeProfile?.concurrencyLimit ?? 0);
    const fallbackProfile = activeProfile?.fallbackProfileId
      ? s.profiles.find((profile) => profile.id === activeProfile.fallbackProfileId) ?? null
      : null;
    const fallbackProfileKey = fallbackProfile
      ? await GetStoredAPIKey(keyringUserFor(fallbackProfile.id)).catch(() => "")
      : "";
    if (concurrencyLimit > 0) {
      const activeCount = workspaceRunningCount(s, s.apiMode);
      const available = concurrencyLimit - activeCount;
      const requiredConcurrency = requestedConcurrency;
      if (available < requiredConcurrency) {
        const apiLabel = s.apiMode === "responses" ? "Responses API" : "Images API";
        set({
          errorMessage: batchProcessEnabled
            ? `${apiLabel} 并发限制 ${concurrencyLimit},当前还可提交 ${Math.max(0, available)} 个,批处理并发需要 ${requiredConcurrency} 个。`
            : loopEnabled
            ? `${apiLabel} 并发限制 ${concurrencyLimit},当前还可提交 ${Math.max(0, available)} 个,循环模式并发需要 ${requiredConcurrency} 个。`
            : `${apiLabel} 并发限制 ${concurrencyLimit},当前还可提交 ${Math.max(0, available)} 个,本次需要 ${batchCount} 个。`,
          errorCanRetry: false,
          errorRawPath: null,
        });
        return;
      }
    }
    let editSourcePaths: string[] = [];
    if (s.mode === "edit" && !batchProcessEnabled) {
      editSourcePaths = s.sources.map((src) => src.path).filter(Boolean);
      if (editSourcePaths.length === 0 && s.currentImage) {
        const materialized = await materializeHistoryItem(s.currentImage).catch(() => null);
        if (materialized?.savedPath) {
          editSourcePaths = [materialized.savedPath];
        }
      }
      if (editSourcePaths.length === 0) {
        set({
          errorMessage: runtimePlatform.isAndroid
            ? "图生图模式需要先从相册或历史添加源图"
            : "图生图模式需要先添加源图(或从文件管理器拖图到画板)",
          errorCanRetry: false,
          errorRawPath: null,
        });
        return;
      }
    }

    const workspaceId = s.activeWorkspaceId;
    const clearCurrentForNewRun = s.mode === "generate";
    stopLoopRun(workspaceId);
    const runPatch = {
      errorMessage: null,
      errorCanRetry: false,
      errorRawPath: null,
      progress: null,
      streamPreview: null,
      streamPreviews: {},
      lastLogLine: "",
      isRunning: true,
      jobsTotal: requestedJobCount,
      jobsCompleted: 0,
      runningJobs: [],
    };
    set({
      ...runPatch,
      batchCount,
      batchResults: [],
      resultGridOpen: requestedJobCount > 1,
      compareB: null,
      currentImage: clearCurrentForNewRun ? null : s.currentImage,
      maskDataURL: null,
      annotations: [],
      strokes: [],
      workspaces: patchWorkspaceRuntime(s.workspaces, workspaceId, {
        ...runPatch,
        currentImageId: clearCurrentForNewRun ? null : s.currentImage?.id ?? null,
        batchResultIds: [],
        resultGridOpen: requestedJobCount > 1,
      }),
    });

    const maskDataURL = s.mode === "edit"
      ? buildMaskPNGDataURL(s.strokes, s.currentImage?.imageB64 ? imageDims(s.currentImage.imageB64) : null)
      : null;
    const maskB64 = maskDataURL ? stripDataURLPrefix(maskDataURL) : "";
    let augmentedPrompt = augmentPromptWithAnnotations(s.prompt, s.annotations, s.currentImage?.imageB64 ? imageDims(s.currentImage.imageB64) : null);
    // Append style chip suffix if the user picked one (other than "全部").
    const styleSuffix = STYLE_SUFFIXES[s.styleTag];
    if (styleSuffix) {
      augmentedPrompt = `${augmentedPrompt}, ${styleSuffix}`;
    }

    const normalizedBaseSize = normalizeSizeSelection(s.size, {
      apiMode: s.apiMode,
      requestPolicy: s.requestPolicy,
      imageModelID: s.imageModelID,
    }, s.customAspectRatios);
    const normalizedEditAutoAspectResolution = normalizeResolutionSelection(
      s.editAutoAspectResolution || "1k",
      {
        apiMode: s.apiMode,
        requestPolicy: s.requestPolicy,
        imageModelID: s.imageModelID,
      },
    );
    const effectiveEditAutoAspectResolution = normalizedEditAutoAspectResolution === "auto"
      ? "1k"
      : normalizedEditAutoAspectResolution;
    const editReferenceDimensions = s.sources[0]?.previewWidth && s.sources[0]?.previewHeight
      ? { width: s.sources[0].previewWidth, height: s.sources[0].previewHeight }
      : s.currentImage?.previewWidth && s.currentImage?.previewHeight
        ? { width: s.currentImage.previewWidth, height: s.currentImage.previewHeight }
        : null;
    const resolvedSize = s.mode === "edit" && !batchProcessEnabled && s.editAutoAspectResolution !== ""
      ? buildReferenceResolutionSizeSelection(
          effectiveEditAutoAspectResolution,
          editReferenceDimensions,
          {
            apiMode: s.apiMode,
            requestPolicy: s.requestPolicy,
            imageModelID: s.imageModelID,
          },
          s.customAspectRatios,
        )
      : normalizedBaseSize;
    const resolvedQuality = normalizeQualitySelection(s.quality, s.imageModelID);
    const streamPreviewDisableReason = getStreamPreviewDisableReason({
      enabled: s.protectStreamPreview,
      isAndroid: runtimePlatform.isAndroid,
      requestedConcurrency,
      resolvedSize,
    });
    const forceDisableStreamPreview = streamPreviewDisableReason !== null;

    const basePayload: GenerateOptionsLike = {
      apiKey: s.apiKey,
      mode: s.mode,
      requestedJobId: "",
      prompt: augmentedPrompt,
      size: resolvedSize,
      quality: resolvedQuality,
      outputFormat: s.outputFormat,
      imagePaths: editSourcePaths,
      imagePath: "",
      maskB64: maskB64,
      seed: s.seed,
      negativePrompt: s.negativePrompt,
      background: s.background,
      outputCompression: s.outputCompression,
      inputFidelity: s.inputFidelity,
      imageStyle: s.imageStyle,
      moderation: s.moderation,
      userIdentifier: s.userIdentifier,
      baseURL: cleanedBaseURL,
      textModelID: s.textModelID,
      imageModelID: s.imageModelID,
      reasoningEffort: s.reasoningEffort,
      proxyMode: s.proxyMode,
      proxyURL: s.proxyURL,
      responsesTransport: s.responsesTransport,
      requestPolicy: s.requestPolicy,
      imagesNewAPICompat: s.imagesNewAPICompat,
      apiMode: s.apiMode,
      noPromptRevision: true,
      concurrencyLimit,
      partialImages: s.partialImages,
      autoRetryCount: s.autoRetryCount,
      fallbackProfile: fallbackProfile && fallbackProfileKey.trim() && fallbackProfile.baseURL.trim()
        ? {
            baseURL: cleanBaseURL(fallbackProfile.baseURL),
            apiKey: fallbackProfileKey.trim(),
            textModelID: fallbackProfile.textModelID,
            imageModelID: fallbackProfile.imageModelID,
            reasoningEffort: fallbackProfile.reasoningEffort,
            apiMode: fallbackProfile.apiMode,
            responsesTransport: fallbackProfile.responsesTransport ?? "sse",
            requestPolicy: fallbackProfile.requestPolicy,
            imagesNewAPICompat: fallbackProfile.imagesNewAPICompat === true,
          }
        : undefined,
      autoRetryEnabled: batchProcessEnabled ? batchProcess.retryOnFailure : s.autoRetryEnabled,
      disablePreview: s.partialImages === 0 || (loopEnabled && !loopGeneration.livePreview) || forceDisableStreamPreview,
    };
    const remotePayload: RuntimeGenerateOptions = {
      ...basePayload,
      sourceImages: s.mode === "edit" && !batchProcessEnabled ? s.sources : undefined,
    };
    const persistedPayload = basePayload;

    if (s.prompt.trim()) {
      const ph = [s.prompt, ...get().promptHistory.filter((p) => p !== s.prompt)].slice(0, 50);
      set({ promptHistory: ph });
      try { localStorage.setItem("gptcodex.promptHistory", JSON.stringify(ph)); } catch {}
    }
    set({
      lastPayload: persistedPayload,
      workspaces: patchWorkspaceRuntime(get().workspaces, workspaceId, { lastPayload: persistedPayload }),
    });
    if (forceDisableStreamPreview && s.partialImages > 0 && (!loopEnabled || loopGeneration.livePreview)) {
      get().pushToast(
        streamPreviewDisableReason === "android_large_size"
          ? `当前大尺寸任务已自动关闭流式预览，优先保证最终图完整。`
          : runtimePlatform.isAndroid
            ? `并发 ${requestedConcurrency} 路任务已自动关闭流式预览，优先保证最终图完整。`
            : `高并发(${requestedConcurrency})已自动关闭流式预览，优先保证最终图完整。`,
        "info",
        5000,
      );
    }

    const snapshotBase = {
      workspaceId,
      apiMode: s.apiMode,
      size: resolvedSize,
      quality: resolvedQuality,
      outputFormat: s.outputFormat,
      sources: s.sources,
      currentImage: s.currentImage,
      styleTag: s.styleTag,
      loopGeneration,
      editSourceMode: s.editSourceMode,
    } as const;

    if (batchProcessEnabled) {
      const controller: LoopRunController = {
        workspaceId,
        totalJobs: batchProcess.discoveredSources.length,
        maxConcurrent: requestedConcurrency,
        launchedJobs: 0,
        nextPreviewSlotIndex: 0,
        mode: s.mode,
        payload: remotePayload,
        snapshotBase,
        batchSources: batchProcess.discoveredSources,
        batchOutputDir: batchProcess.outputMode === "custom_dir" ? batchProcess.outputDir : "",
        batchOutputPrefix: batchProcess.fileNamePrefix,
        batchAutoAspectResolution: batchProcess.autoAspectResolution,
        batchCapabilityInput: {
          apiMode: s.apiMode,
          requestPolicy: s.requestPolicy,
          imageModelID: s.imageModelID,
        },
        batchCustomAspectRatios: s.customAspectRatios,
        stopped: false,
      };
      loopRunControllers.set(workspaceId, controller);
      launchQueuedLoopJobs(controller);
      return;
    }

    if (loopEnabled) {
      const controller: LoopRunController = {
        workspaceId,
        totalJobs: requestedJobCount,
        maxConcurrent: requestedConcurrency,
        launchedJobs: 0,
        nextPreviewSlotIndex: 0,
        mode: s.mode,
        payload: remotePayload,
        snapshotBase,
        stopped: false,
      };
      loopRunControllers.set(workspaceId, controller);
      launchQueuedLoopJobs(controller);
      return;
    }

    for (let i = 0; i < batchCount; i++) {
      const jobSeed = s.seed ? s.seed + i : 0;
      const p: RuntimeGenerateOptions = { ...remotePayload, seed: jobSeed };
      void launchOneJob(s.mode, p, {
        ...snapshotBase,
        batchIndex: i,
        previewSlotIndex: i,
      });
    }
  },

  cancel: async () => {
    const s = get();
    const workspaceId = s.activeWorkspaceId;
    stopLoopRun(workspaceId);
    const ids = [...s.runningJobs];
    // Cancel every concurrent job in the batch.
    for (const id of ids) {
      try { await wailsCancel(id); } catch { /* ignore */ }
      EventsOff(`progress:${id}`, `log:${id}`, `preview:${id}`, `result:${id}`, `error:${id}`);
    }
    const nextMeta = { ...get().runningJobMeta };
    for (const id of ids) delete nextMeta[id];
    const runPatch = {
      isRunning: false,
      runningJobs: [],
      progress: null,
      streamPreview: null,
      streamPreviews: {},
      jobsTotal: 0,
      jobsCompleted: 0,
    };
    set({
      ...runPatch,
      runningJobMeta: nextMeta,
      workspaces: patchWorkspaceRuntime(get().workspaces, workspaceId, runPatch),
    });
  },

  applyHistoryParams: (item) => imageActions.applyHistoryParams(item),
  regenerateFromHistory: async (item) => imageActions.regenerateFromHistory(item),
  viewSourceOnCanvas: async (index) => imageActions.viewSourceOnCanvas(index),
  compareSourceOnCanvas: async (index) => imageActions.compareSourceOnCanvas(index),
  reuseAsSource: async (item) => imageActions.reuseAsSource(item),
  deleteHistoryItem: async (id) => imageActions.deleteHistoryItem(id),
  saveCurrentImageAs: async () => imageActions.saveCurrentImageAs(),

  bootstrap: async () => {
    const previewScenario = readPreviewScenario();
    if (previewScenario === "mac-workspace" || previewScenario === "windows-right-rail") {
      const workspaceId = genId();
      const preview = previewScenario === "windows-right-rail"
        ? buildWindowsRightRailPreview(workspaceId)
        : buildMacWorkspacePreview(workspaceId);
      await SetKeepLogsEnabled(readKeepLogs()).catch(() => undefined);
      await SetCleanupPreviewCacheOnExitEnabled(readCleanupPreviewCacheOnExit()).catch(() => undefined);
      applyTheme(previewScenario === "windows-right-rail" ? "light" : "dark");
      document.documentElement.style.setProperty("--font-scale", "1");
      setKernelRuntimeMode("auto");
      const workspaceState = preview.workspace;
      set({
        apiKey: "sk-preview",
        mode: workspaceState.mode,
        prompt: workspaceState.prompt,
        negativePrompt: workspaceState.negativePrompt,
        size: workspaceState.size,
        quality: workspaceState.quality,
        outputFormat: workspaceState.outputFormat,
        seed: workspaceState.seed,
        background: workspaceState.background,
        outputCompression: workspaceState.outputCompression,
        inputFidelity: workspaceState.inputFidelity,
        imageStyle: workspaceState.imageStyle,
        moderation: workspaceState.moderation,
        userIdentifier: "",
        partialImages: workspaceState.partialImages,
        autoRetryEnabled: true,
        autoRetryCount: DEFAULT_AUTO_RETRY_COUNT,
        kernelRuntimeMode: "auto",
        baseURL: preview.profile.baseURL,
        textModelID: preview.profile.textModelID,
        imageModelID: preview.profile.imageModelID,
        reasoningEffort: preview.profile.reasoningEffort,
        proxyMode: "system",
        proxyURL: "",
        apiMode: preview.profile.apiMode,
        responsesTransport: preview.profile.responsesTransport ?? "sse",
        requestPolicy: preview.profile.requestPolicy,
        noPromptRevision: true,
        profiles: [preview.profile],
        activeProfileId: preview.profile.id,
        sources: preview.sources,
        runningJobs: [],
        jobsTotal: 0,
        jobsCompleted: 0,
        progress: null,
        streamPreview: null,
        streamPreviews: {},
        lastLogLine: "",
        errorMessage: null,
        errorRawPath: null,
        isRunning: false,
        lastPayload: null,
        runningJobMeta: {},
        currentImage: preview.currentImage,
        history: preview.history,
        historyHasMore: false,
        historyLoading: false,
        historyCursorBeforeDayStart: null,
        batchResults: [],
        resultGridOpen: false,
        historyRailCollapsed: false,
        historyTimelineOpen: false,
        tool: "pan",
        brushSize: 24,
        brushMode: "paint",
        annotationKind: "rect",
        annotationColor: "#ff4d4d",
        selectedAnnotationId: null,
        maskDataURL: null,
        strokes: [],
        annotations: [],
        compareB: null,
        compareSplit: 0.5,
        toasts: [],
        recentDurations: preview.history.map((item) => item.elapsedSec ?? 0).filter((value) => value > 0),
        viewZoom: 1,
        canvasViewResetTick: 0,
        fullscreen: false,
        promptHistory: [],
        promptTemplates: preview.promptTemplates ?? [],
        batchCount: workspaceState.batchCount,
        selectedPresetId: workspaceState.selectedPresetId ?? null,
        editSourceMode: workspaceState.editSourceMode,
        editAutoAspectResolution: normalizeAutoAspectResolutionPreset(workspaceState.editAutoAspectResolution),
        batchProcess: normalizeBatchProcessConfig(workspaceState.batchProcess),
        loopGeneration: normalizeLoopGenerationConfig(workspaceState.loopGeneration),
        presets: preview.presets ?? [],
        customAspectRatios: [],
        theme: previewScenario === "windows-right-rail" ? "light" : "dark",
        fontScale: 1,
        workspaces: [workspaceState],
        activeWorkspaceId: workspaceId,
        styleTag: preview.currentImage.styleTag ?? "",
        undoStack: [],
        redoStack: [],
        resultDetail: null,
        settingsOpen: false,
        isTestingKey: false,
        isOptimizingPrompt: false,
        customAspectRatioModalOpen: false,
        customSizeModalOpen: false,
        upstreamModalOpen: false,
        upstreamReturnTarget: "app",
        starPromptOpen: false,
        starPromptSource: "auto",
        savePromptRequest: null,
        savePromptQueue: [],
        savePromptSuppressed: readSavePromptSuppressed(),
        keepLogs: readKeepLogs(),
        cleanupPreviewCacheOnExit: readCleanupPreviewCacheOnExit(),
        completionSound: readCompletionSoundConfig(),
        completionNotification: readCompletionNotificationConfig(),
        completionNotificationPermission: readSystemNotificationPermission(),
        ignoredReleaseTag: readIgnoredReleaseTag(),
        appUpdate: null,
        appUpdateModalOpen: false,
      });
      return;
    }

    await importCompatibilityStateIfNewer().catch((error) => {
      if (typeof console !== "undefined") console.warn("compat import failed", error);
      return false;
    });
    const initialHistoryPage = await loadHistoryPage({ limit: INITIAL_HISTORY_LOAD });
    const items = trimHistory(initialHistoryPage.items);
    const historyHasMore = !!initialHistoryPage.nextCursor;
    let promptHistory: string[] = [];
    let promptTemplates: PromptTemplate[] = [];
    let presets: Preset[] = [];
    const customAspectRatios = loadCustomAspectRatios();
    let theme: ThemeMode = "system";
    let fontScale = 1;
    try {
      const raw = localStorage.getItem("gptcodex.promptHistory");
      if (raw) promptHistory = JSON.parse(raw);
    } catch {}
    try {
      const raw = localStorage.getItem("gptcodex.presets");
      if (raw) presets = JSON.parse(raw);
    } catch {}
    promptTemplates = readStoredPromptTemplates();
    try {
      const raw = localStorage.getItem("gptcodex.theme");
      if (raw === "system" || raw === "light" || raw === "dark") theme = raw;
    } catch {}
    try {
      const raw = localStorage.getItem("gptcodex.fontScale");
      const n = Number(raw);
      if (!Number.isNaN(n) && n > 0.5 && n < 2) fontScale = n;
    } catch {}
    let kernelRuntimeMode: KernelRuntimeMode = "auto";
    try {
      const v = localStorage.getItem("gptcodex.kernelRuntimeMode");
      if (v === "auto" || v === "local" || v === "remote") kernelRuntimeMode = v;
    } catch {}
    const keepLogs = readKeepLogs();
    const cleanupPreviewCacheOnExit = readCleanupPreviewCacheOnExit();
    const completionSound = readCompletionSoundConfig();
    const completionNotification = readCompletionNotificationConfig();
    const completionNotificationPermission = readSystemNotificationPermission();
    const ignoredReleaseTag = readIgnoredReleaseTag();
    const updateInfo = normalizeAppUpdateInfo(await CheckForAppUpdate().catch(() => null));
    const shouldShowUpdate = !!updateInfo?.hasUpdate && updateInfo.releaseTag !== ignoredReleaseTag;
    const noPromptRevision = true;
    const proxyConfig = loadProxyConfig();
    let outputFormat: OutputFormatValue = "png";
    try {
      const v = localStorage.getItem("gptcodex.outputFormat");
      if (v === "png" || v === "jpeg" || v === "webp") outputFormat = v;
    } catch {}
    let background: BackgroundValue = "auto";
    try {
      background = normalizeBackgroundValue(localStorage.getItem("gptcodex.background"));
    } catch {}
    let outputCompression = 100;
    try {
      outputCompression = normalizeOutputCompressionValue(localStorage.getItem("gptcodex.outputCompression"));
    } catch {}
    let inputFidelity: InputFidelityValue = "auto";
    try {
      inputFidelity = normalizeInputFidelityValue(localStorage.getItem("gptcodex.inputFidelity"));
    } catch {}
    let imageStyle: ImageStyleValue = "default";
    try {
      imageStyle = normalizeImageStyleValue(localStorage.getItem("gptcodex.imageStyle"));
    } catch {}
    let moderation: ModerationValue = "low";
    try {
      const v = localStorage.getItem("gptcodex.moderation");
      if (v === "auto" || v === "low") moderation = v;
    } catch {}
    let userIdentifier = "";
    try {
      userIdentifier = normalizeUserIdentifierValue(localStorage.getItem("gptcodex.userIdentifier"));
    } catch {}
    let partialImages = 1;
    try {
      partialImages = normalizePartialImagesValue(localStorage.getItem("gptcodex.partialImages"));
    } catch {}
    const protectStreamPreview = readProtectStreamPreview();
    const autoRetryEnabled = readAutoRetryEnabled();
    const autoRetryCount = readAutoRetryCount();
    // ---- v0.1.6 profile 列表加载 / 迁移 -----------------------------------
    // 1) 优先读新格式 gptcodex.profiles。
    // 2) 缺失时尝试从老 gptcodex.{responses,images}.* + 老 keyring 项合成 0-2
    //    个 profile,顺手清理老 localStorage 键。
    let profiles = loadStoredProfiles();
    let activeProfileId = loadStoredActiveProfileId();
    if (profiles.length === 0) {
      // 检测老格式
      let legacyApiMode: APIMode = "responses";
      try {
        const v = localStorage.getItem("gptcodex.apiMode");
        if (v === "images" || v === "responses") legacyApiMode = v;
      } catch {}
      const legacyResponses = loadModeConfig("responses");
      const legacyImages = loadModeConfig("images");
      // 沿用 v0.1.5 那套 legacy-shared 字段(更老的 gptcodex.baseURL 等)
      const legacyBaseURL  = (() => { try { return localStorage.getItem("gptcodex.baseURL") ?? ""; } catch { return ""; } })();
      const legacyTextID   = (() => { try { return localStorage.getItem("gptcodex.textModelID") ?? ""; } catch { return ""; } })();
      const legacyImageID  = (() => { try { return localStorage.getItem("gptcodex.imageModelID") ?? ""; } catch { return ""; } })();
      if (legacyApiMode === "responses" && legacyBaseURL && !legacyResponses.baseURL) {
        legacyResponses.baseURL = cleanBaseURL(legacyBaseURL);
        legacyResponses.textModelID = legacyTextID;
        legacyResponses.imageModelID = legacyImageID;
      } else if (legacyApiMode === "images" && legacyBaseURL && !legacyImages.baseURL) {
        legacyImages.baseURL = cleanBaseURL(legacyBaseURL);
        legacyImages.imageModelID = legacyImageID;
      }
      const legacySharedKey = loadLegacySharedAPIKey();
      const legacyResponsesKey = await GetStoredAPIKey("responses").catch(() => "")
        || loadLegacyModeAPIKey("responses")
        || (legacyApiMode === "responses" ? legacySharedKey : "");
      const legacyImagesKey = await GetStoredAPIKey("images").catch(() => "")
        || loadLegacyModeAPIKey("images")
        || (legacyApiMode === "images" ? legacySharedKey : "");
      const synth: UpstreamProfile[] = [];
      if (legacyResponses.baseURL || legacyResponsesKey) {
        const id = genProfileId();
        synth.push({
          id,
          name: "Responses · 默认",
          apiMode: "responses",
          responsesTransport: "sse",
          requestPolicy: "openai",
          imagesNewAPICompat: false,
          baseURL: legacyResponses.baseURL,
          textModelID: legacyResponses.textModelID,
          imageModelID: legacyResponses.imageModelID,
          reasoningEffort: "xhigh",
          concurrencyLimit: normalizeConcurrencyLimit(legacyResponses.concurrencyLimit),
          createdAt: Date.now(),
          lastUsedAt: legacyApiMode === "responses" ? Date.now() : undefined,
        });
        if (legacyResponsesKey) {
          try { await SetStoredAPIKey(keyringUserFor(id), legacyResponsesKey); } catch {}
        }
      }
      if (legacyImages.baseURL || legacyImagesKey) {
        const id = genProfileId();
        synth.push({
          id,
          name: "Images · 默认",
          apiMode: "images",
          responsesTransport: "sse",
          requestPolicy: "openai",
          imagesNewAPICompat: false,
          baseURL: legacyImages.baseURL,
          textModelID: legacyImages.textModelID,
          imageModelID: legacyImages.imageModelID,
          reasoningEffort: "xhigh",
          concurrencyLimit: normalizeConcurrencyLimit(legacyImages.concurrencyLimit),
          createdAt: Date.now(),
          lastUsedAt: legacyApiMode === "images" ? Date.now() : undefined,
        });
        if (legacyImagesKey) {
          try { await SetStoredAPIKey(keyringUserFor(id), legacyImagesKey); } catch {}
        }
      }
      if (synth.length > 0) {
        profiles = synth;
        // active = 跟老 apiMode 对应的那个
        const matching = synth.find((p) => p.apiMode === legacyApiMode);
        activeProfileId = (matching ?? synth[0]).id;
        persistProfiles(profiles);
        persistActiveProfileId(activeProfileId);
        // 清掉老的 keyring 项 + localStorage 键(避免下次启动重复迁移)
        try { await DeleteStoredAPIKey("responses"); } catch {}
        try { await DeleteStoredAPIKey("images"); } catch {}
        clearLegacyAPIKeys();
        clearLegacyModeLocalStorage();
      }
    }

    // 决定 active profile 与对应顶层镜像。空列表 → 全置空,后面会自动弹首次配置。
    const activeProfile = pickActiveProfile(profiles, activeProfileId);
    if (activeProfile && activeProfile.id !== activeProfileId) {
      activeProfileId = activeProfile.id;
      persistActiveProfileId(activeProfileId);
    }
    const apiMode: APIMode = activeProfile?.apiMode ?? "responses";
    const responsesTransport = activeProfile?.responsesTransport ?? "sse";
    const requestPolicy: RequestPolicy = activeProfile?.requestPolicy ?? "openai";
    const imagesNewAPICompat = activeProfile?.imagesNewAPICompat === true;
    const baseURL = activeProfile?.baseURL ?? "";
    const textModelID = activeProfile?.textModelID ?? "";
    const imageModelID = activeProfile?.imageModelID ?? "";
    const reasoningEffort = activeProfile?.reasoningEffort ?? "xhigh";
    const activeKey = activeProfile
      ? await GetStoredAPIKey(keyringUserFor(activeProfile.id)).catch(() => "")
      : "";
    // Apply theme + font scale to root immediately.
    applyTheme(theme);
    document.documentElement.style.setProperty("--font-scale", String(fontScale));
    setKernelRuntimeMode(kernelRuntimeMode);
    await SetKeepLogsEnabled(keepLogs).catch(() => undefined);
    await SetCleanupPreviewCacheOnExitEnabled(cleanupPreviewCacheOnExit).catch(() => undefined);
    // 用户自定义输出目录 —— 推给 backend,并记为可信输出根。
    const trustedRoots = new Set(loadTrustedOutputRoots());
    try {
      const customOutput = localStorage.getItem("gptcodex.outputDir");
      if (customOutput && customOutput.trim()) {
        await SetOutputDir(customOutput).catch(() => undefined);
        trustedRoots.add(customOutput.trim());
      }
    } catch {}
    const effectiveOutput = await GetOutputDir().catch(() => "");
    if (effectiveOutput) trustedRoots.add(effectiveOutput);
    for (const root of trustedRoots) rememberTrustedOutputRoot(root);
    await registerTrustedOutputRoots(Array.from(trustedRoots));
    // Make sure there's always at least one workspace.
    const wsId = genId();
    const initialWorkspace: Workspace = {
      id: wsId,
      name: "图片 1",
      prompt: "",
      negativePrompt: "",
      mode: "generate",
      size: "1024x1024",
      quality: "medium",
      outputFormat,
      seed: 0,
      background,
      outputCompression,
      inputFidelity,
      imageStyle,
      moderation,
      userIdentifier,
      partialImages,
      batchCount: 1,
      selectedPresetId: null,
      editSourceMode: "manual",
      editAutoAspectResolution: "",
      batchProcess: defaultBatchProcessConfig(),
      loopGeneration: defaultLoopGenerationConfig(),
      sources: [],
      currentImageId: null,
      batchResultIds: [],
      resultGridOpen: false,
      runningJobIds: [],
      jobsTotal: 0,
      jobsCompleted: 0,
      progress: null,
      streamPreview: null,
      streamPreviews: {},
      lastLogLine: "",
      errorMessage: null,
      errorRawPath: null,
      lastPayload: null,
    };
    const runtimePlatform = readRuntimePlatformState();
    const shouldAutoOpenSettings = runtimePlatform.isAndroid
      ? false
      : !activeProfile || !activeKey.trim() || !baseURL.trim();
    set({
      apiKey: activeKey, history: items, promptHistory, promptTemplates, presets, customAspectRatios, theme, fontScale,
      historyHasMore,
      historyLoading: false,
      historyCursorBeforeDayStart: initialHistoryPage.nextCursor?.beforeDayStart ?? null,
      apiMode, responsesTransport, requestPolicy, imagesNewAPICompat, baseURL, textModelID, imageModelID, reasoningEffort, kernelRuntimeMode, noPromptRevision,
      proxyMode: proxyConfig.mode,
      proxyURL: proxyConfig.url,
      outputFormat,
      background,
      outputCompression,
      inputFidelity,
      imageStyle,
      moderation,
      userIdentifier,
      partialImages,
      autoRetryCount,
      protectStreamPreview,
      autoRetryEnabled,
      profiles,
      activeProfileId,
      workspaces: [initialWorkspace],
      activeWorkspaceId: wsId,
      selectedPresetId: initialWorkspace.selectedPresetId ?? null,
      editSourceMode: initialWorkspace.editSourceMode,
      editAutoAspectResolution: normalizeAutoAspectResolutionPreset(initialWorkspace.editAutoAspectResolution),
      batchProcess: normalizeBatchProcessConfig(initialWorkspace.batchProcess),
      loopGeneration: normalizeLoopGenerationConfig(initialWorkspace.loopGeneration),
      // Android 走首页 hero 引导，不用启动即弹设置；桌面仍保留首次引导。
      settingsOpen: shouldAutoOpenSettings,
      customAspectRatioModalOpen: false,
      customSizeModalOpen: false,
      upstreamModalOpen: false,
      upstreamReturnTarget: shouldAutoOpenSettings ? "settings" : "app",
      savePromptRequest: null,
      savePromptQueue: [],
      savePromptSuppressed: readSavePromptSuppressed(),
      keepLogs,
      cleanupPreviewCacheOnExit,
      completionSound,
      completionNotification,
      completionNotificationPermission,
      ignoredReleaseTag,
      appUpdate: shouldShowUpdate ? updateInfo : null,
      appUpdateModalOpen: shouldShowUpdate,
    });
    void WriteAppUpdateProbe({
      appVersion,
      currentVersion: updateInfo?.currentVersion ?? appVersion,
      latestVersion: updateInfo?.latestVersion ?? "",
      releaseTag: updateInfo?.releaseTag ?? "",
      releaseURL: updateInfo?.releaseURL ?? "",
      ignoredReleaseTag,
      updateInfoAvailable: !!updateInfo,
      hasUpdate: updateInfo?.hasUpdate === true,
      shouldShowUpdate,
      appUpdateModalOpen: shouldShowUpdate,
    }).catch(() => undefined);
    enableCompatibilityExport();
    void backfillHistoryPreviewRefs(items);
  },

  setMaskDataURL: (v) => set({ maskDataURL: v }),

  pushStroke: (stroke) => {
    const before = get().strokes;
    const after = [...before, stroke];
    const entry: UndoEntry = {
      label: "stroke",
      undo: (s) => ({ strokes: s.strokes.slice(0, -1) }),
      redo: () => ({ strokes: [...get().strokes, stroke] }),
    };
    set({
      strokes: after,
      undoStack: [...get().undoStack, entry],
      redoStack: [],
    });
  },

  resetMask: () => {
    const before = get().strokes;
    if (before.length === 0) return;
    const entry: UndoEntry = {
      label: "clear-mask",
      undo: () => ({ strokes: before, maskDataURL: get().maskDataURL }),
      redo: () => ({ strokes: [], maskDataURL: null }),
    };
    set({
      strokes: [],
      maskDataURL: null,
      undoStack: [...get().undoStack, entry],
      redoStack: [],
    });
  },

  addAnnotation: (a) => {
    const entry: UndoEntry = {
      label: "annotation",
      undo: (s) => ({ annotations: s.annotations.filter((x) => x.id !== a.id) }),
      redo: () => ({ annotations: [...get().annotations, a] }),
    };
    set({
      annotations: [...get().annotations, a],
      undoStack: [...get().undoStack, entry],
      redoStack: [],
    });
  },

  removeAnnotation: (id) => {
    const target = get().annotations.find((a) => a.id === id);
    if (!target) return;
    const entry: UndoEntry = {
      label: "remove-annotation",
      undo: (s) => ({ annotations: [...s.annotations, target] }),
      redo: () => ({ annotations: get().annotations.filter((x) => x.id !== id) }),
    };
    set({
      annotations: get().annotations.filter((a) => a.id !== id),
      selectedAnnotationId: get().selectedAnnotationId === id ? null : get().selectedAnnotationId,
      undoStack: [...get().undoStack, entry],
      redoStack: [],
    });
  },

  updateAnnotation: (id, patch) => {
    set({
      annotations: get().annotations.map((a) => (a.id === id ? { ...a, ...patch } : a)),
    });
  },

  clearAnnotations: () => {
    const before = get().annotations;
    if (before.length === 0) return;
    const entry: UndoEntry = {
      label: "clear-annotations",
      undo: () => ({ annotations: before }),
      redo: () => ({ annotations: [] }),
    };
    set({
      annotations: [],
      undoStack: [...get().undoStack, entry],
      redoStack: [],
    });
  },

  undo: () => {
    const stack = get().undoStack;
    if (stack.length === 0) return;
    const entry = stack[stack.length - 1];
    const patch = entry.undo(get());
    set({
      ...(patch as any),
      undoStack: stack.slice(0, -1),
      redoStack: [...get().redoStack, entry],
    });
  },

  redo: () => {
    const stack = get().redoStack;
    if (stack.length === 0) return;
    const entry = stack[stack.length - 1];
    const patch = entry.redo(get());
    set({
      ...(patch as any),
      redoStack: stack.slice(0, -1),
      undoStack: [...get().undoStack, entry],
    });
  },

  setCompareB: (item) => mediaActions.setCompareB(item),
  setCompareSplit: (v) => mediaActions.setCompareSplit(v),
  openResultGrid: () => mediaActions.openResultGrid(),
  closeResultGrid: () => mediaActions.closeResultGrid(),
  selectBatchResult: async (item) => mediaActions.selectBatchResult(item),
  stepBatchResult: async (delta) => mediaActions.stepBatchResult(delta),
  pushToast: (text, kind = "info", ttl = 3500, action) => mediaActions.pushToast(text, kind, ttl, action),
  dismissToast: (id) => mediaActions.dismissToast(id),
  resultDetail: null,
  openResultDetail: async (item) => mediaActions.openResultDetail(item),
  closeResultDetail: () => mediaActions.closeResultDetail(),
  materializeCurrentImage: async (item) => mediaActions.materializeCurrentImage(item),
  loadMoreHistory: async () => {
    if (deferredHistoryLoadPromise) return deferredHistoryLoadPromise;
    if (!get().historyHasMore || get().historyLoading) return;
    set({ historyLoading: true });
    deferredHistoryLoadPromise = (async () => {
      try {
        const currentHistory = get().history;
        const cursorBeforeDayStart = get().historyCursorBeforeDayStart;
        const nextPage = await loadHistoryPage({
          cursor: typeof cursorBeforeDayStart === "number" ? { beforeDayStart: cursorBeforeDayStart } : null,
          limit: INITIAL_HISTORY_LOAD,
        });
        const merged = trimHistory([...currentHistory, ...nextPage.items]);
        set({
          history: merged,
          historyHasMore: !!nextPage.nextCursor && merged.length < MAX_HISTORY_ITEMS,
          historyCursorBeforeDayStart: nextPage.nextCursor?.beforeDayStart ?? null,
        });
        void backfillHistoryPreviewRefs(nextPage.items);
      } catch (error) {
        if (typeof console !== "undefined") console.warn("load more history failed", error);
      } finally {
        deferredHistoryLoadPromise = null;
        set({ historyLoading: false });
      }
    })();
    return deferredHistoryLoadPromise;
  },
  setHistoryRailCollapsed: (collapsed) => {
    mediaActions.setHistoryRailCollapsed(collapsed);
  },
  openHistoryTimeline: () => {
    mediaActions.openHistoryTimeline();
  },
  closeHistoryTimeline: () => mediaActions.closeHistoryTimeline(),
  pruneHistoryOlderThanDays: async (days) => mediaActions.pruneHistoryOlderThanDays(days),
  rotateCurrent: async (degrees) => mediaActions.rotateCurrent(degrees),
  flipCurrent: async (horizontal) => mediaActions.flipCurrent(horizontal),
  cropToRect: async (x, y, w, h) => mediaActions.cropToRect(x, y, w, h),
  savePreset: (name) => mediaActions.savePreset(name),
  overwritePreset: (id) => mediaActions.overwritePreset(id),
  updatePreset: (id, patch) => mediaActions.updatePreset(id, patch),
  applyPreset: (id) => mediaActions.applyPreset(id),
  deletePreset: (id) => mediaActions.deletePreset(id),
  exportHistory: async () => mediaActions.exportHistory(),

  setTheme: (t) => {
    set({ theme: t });
    try { localStorage.setItem("gptcodex.theme", t); } catch {}
    applyTheme(t);
  },

  setFontScale: (v) => {
    set({ fontScale: v });
    try { localStorage.setItem("gptcodex.fontScale", String(v)); } catch {}
    document.documentElement.style.setProperty("--font-scale", String(v));
  },

  setProxyConfig: (mode, url) => {
    const normalizedMode = normalizeProxyMode(mode);
    const nextURL = (url ?? get().proxyURL).trim();
    set({ proxyMode: normalizedMode, proxyURL: nextURL });
    persistProxyConfig(normalizedMode, nextURL);
  },

  testAPIKey: async () => {
    const s = get();
    if (!s.apiKey.trim()) {
      s.pushToast("先填入 API Key", "warn");
      return;
    }
    if (!s.baseURL.trim()) {
      s.pushToast("先在「上游配置」里填入中转站地址", "warn", 5000);
      return;
    }
    const cleanedBaseURL = cleanBaseURL(s.baseURL);
    if (s.isTestingKey) return;
    set({ isTestingKey: true });
    s.pushToast("正在测试连接...", "info", 8000);
    try {
      const result = await probeCurrentUpstream(
        cleanedBaseURL,
        s.apiKey.trim(),
        s.proxyMode,
        s.proxyURL,
        s.apiMode,
        s.responsesTransport,
      );
      set({ isTestingKey: false });
      if (result.responsesTransport === "websocket" && result.responsesTransportOK === false) {
        s.pushToast(`基础连接 OK，但 Responses WebSocket 不可用:${result.responsesTransportError || "未返回具体原因"}`, "warn", 7000);
      } else if (result.responsesTransport === "websocket") {
        s.pushToast("连接 OK · /v1/models 可访问，Responses WebSocket 可用", "success");
      } else {
        s.pushToast("连接 OK · 上游 models 列表可访问", "success");
      }
    } catch (e: any) {
      set({ isTestingKey: false });
      s.pushToast(`连接失败:${e?.message ?? e}`, "error", 6000);
    }
  },

  optimizePrompt: async () => {
    const s = get();
    if (s.isRunning || s.isOptimizingPrompt) return;
    // prompt 优化必须走 Responses(它要文本模型),如果用户 active 的是 Images
    // profile,要回头找一个 Responses profile 来跑;它的 key 还是从 keyring 拿。
    let optimizeAPIKey = s.apiKey;
    let optimizeBaseURL = s.baseURL;
    let optimizeTextModelID = s.textModelID;
    if (s.apiMode !== "responses") {
      const responsesProfile = s.profiles.find((p) => p.apiMode === "responses" && p.baseURL);
      if (responsesProfile) {
        optimizeBaseURL = responsesProfile.baseURL;
        optimizeTextModelID = responsesProfile.textModelID;
        const k = await GetStoredAPIKey(keyringUserFor(responsesProfile.id)).catch(() => "");
        if (k) optimizeAPIKey = k;
      }
    }
    optimizeAPIKey = optimizeAPIKey.trim();
    optimizeBaseURL = cleanBaseURL(optimizeBaseURL);
    optimizeTextModelID = optimizeTextModelID.trim();
    if (!optimizeAPIKey) {
      s.pushToast("先填入 API Key", "warn");
      return;
    }
    if (!optimizeBaseURL) {
      s.pushToast("先在上游配置里填入可用于 AI 优化的 Responses API 地址", "warn", 5000);
      return;
    }
    if (!s.prompt.trim()) {
      s.pushToast("先输入 prompt", "warn");
      return;
    }
    const sourcePaths = s.mode === "edit"
      ? s.sources.map((src) => src.path).filter(Boolean)
      : [];
    if (s.mode === "edit" && sourcePaths.length === 0 && s.currentImage?.savedPath) {
      sourcePaths.push(s.currentImage.savedPath);
    }
    set({ isOptimizingPrompt: true, errorMessage: null, errorCanRetry: false, errorRawPath: null });
    try {
      const optimized = await wailsOptimizePrompt({
        apiKey: optimizeAPIKey,
        prompt: s.prompt,
        mode: s.mode,
        baseURL: optimizeBaseURL,
        textModelID: optimizeTextModelID,
        proxyMode: s.proxyMode,
        proxyURL: s.proxyURL,
        imagePaths: sourcePaths,
        imagePath: "",
      } satisfies PromptOptimizeRequest);
      const trimmed = optimized.trim();
      if (!trimmed) {
        throw new Error("上游没有返回可用的优化结果");
      }
      set({ prompt: trimmed });
      s.pushToast("已优化提示词", "success");
    } catch (e: any) {
      const msg = `优化失败:${e?.message ?? e}`;
      set({ errorMessage: msg, errorCanRetry: false, errorRawPath: null });
      s.pushToast(msg, "error", 6000);
    } finally {
      set({ isOptimizingPrompt: false });
    }
  },

  newWorkspace: (name) => workspaceActions.newWorkspace(name),
  switchWorkspace: (id) => workspaceActions.switchWorkspace(id),
  closeWorkspace: (id) => workspaceActions.closeWorkspace(id),
  renameWorkspace: (id, name) => workspaceActions.renameWorkspace(id, name),

  importHistory: async () => mediaActions.importHistory(),

  addPromptTemplate: (label, text) => {
    const trimmedLabel = label.trim().slice(0, 40);
    const trimmedText = text.trim();
    if (!trimmedLabel || !trimmedText) return null;
    const next: PromptTemplate = {
      id: genId(),
      label: trimmedLabel,
      text: trimmedText,
      createdAt: Date.now(),
      updatedAt: Date.now(),
    };
    const list = [...get().promptTemplates, next];
    persistPromptTemplates(list);
    set({ promptTemplates: list });
    return next.id;
  },

  updatePromptTemplate: (id, patch) => {
    const index = get().promptTemplates.findIndex((item) => item.id === id);
    if (index < 0) return false;
    const current = get().promptTemplates[index];
    const label = patch.label !== undefined ? patch.label.trim().slice(0, 40) : current.label;
    const text = patch.text !== undefined ? patch.text.trim() : current.text;
    if (!label || !text) return false;
    const list = get().promptTemplates.map((item, itemIndex) => itemIndex === index ? {
      ...item,
      label,
      text,
      updatedAt: Date.now(),
    } : item);
    persistPromptTemplates(list);
    set({ promptTemplates: list });
    return true;
  },

  deletePromptTemplate: (id) => {
    const list = get().promptTemplates.filter((item) => item.id !== id);
    if (list.length === get().promptTemplates.length) return;
    persistPromptTemplates(list);
    set({ promptTemplates: list });
  },

  retryLast: async () => {
    const s = get();
    if (!s.lastPayload || s.isRunning) return;
    set({ errorMessage: null, errorCanRetry: false, errorRawPath: null });
    // Re-invoke submit, which will rebuild the payload from current state.
    // (We don't reuse lastPayload verbatim so any tweaks the user made
    // after the failure — different seed, different prompt — take effect.)
    await get().submit();
  },

  importImageFile: async (file) => imageActions.importImageFile(file),
}));

// Fire one job (concurrent member of a batch). Registers its own EventsOn
// callbacks; updates store.runningJobs / jobsCompleted as the run progresses.
// `snapshot` is the store state at submit time — captures size/quality/sources
// so per-job result writes still see the originating context.
async function launchOneJob(
  mode: string,
  payload: RuntimeGenerateOptions,
  snapshot: JobSnapshot,
  hooks: LaunchOneJobHooks = {},
): Promise<void> {
  const store = useStudioStore;
  const jobId = cryptoIDFallback();
  let offProgress = () => {};
  let offLog = () => {};
  let offPreview = () => {};
  let offResult = () => {};
  let offError = () => {};
  const cleanup = () => { offProgress(); offLog(); offPreview(); offResult(); offError(); };
  let settled = false;
  const settle = (status: "success" | "error") => {
    if (settled) return;
    settled = true;
    hooks.onSettled?.(status);
  };
  try {
    store.setState((state) => {
      const runtime = workspaceRuntimeFromState(state, snapshot.workspaceId);
      const runningJobs = runtime.runningJobs.includes(jobId)
        ? runtime.runningJobs
        : [...runtime.runningJobs, jobId];
      const patch: WorkspacePatch = { runningJobs };
      return {
        runningJobMeta: {
          ...state.runningJobMeta,
          [jobId]: { workspaceId: snapshot.workspaceId, apiMode: snapshot.apiMode },
        },
        workspaces: patchWorkspaceRuntime(state.workspaces, snapshot.workspaceId, patch),
        ...(state.activeWorkspaceId === snapshot.workspaceId ? activeRuntimePatch(patch) : {}),
      } as Partial<StudioState>;
    });

    const removeFromRunning = () => {
      let completed = 0;
      let total = 0;
      store.setState((state) => {
        const runtime = workspaceRuntimeFromState(state, snapshot.workspaceId);
        const remaining = runtime.runningJobs.filter((id) => id !== jobId);
        const prunedPreview = removeStreamPreview(runtime.streamPreviews, jobId);
        completed = runtime.jobsCompleted + 1;
        total = runtime.jobsTotal;
        const patch: WorkspacePatch = {
          runningJobs: remaining,
          jobsCompleted: completed,
          jobsTotal: remaining.length === 0 ? 0 : runtime.jobsTotal,
          progress: remaining.length === 0 ? null : runtime.progress,
          streamPreview: remaining.length === 0 ? null : prunedPreview.streamPreview,
          streamPreviews: remaining.length === 0 ? {} : prunedPreview.streamPreviews,
          lastLogLine: remaining.length === 0 ? "" : runtime.lastLogLine,
        };
        const nextMeta = { ...state.runningJobMeta };
        delete nextMeta[jobId];
        return {
          runningJobMeta: nextMeta,
          workspaces: patchWorkspaceRuntime(state.workspaces, snapshot.workspaceId, patch),
          ...(state.activeWorkspaceId === snapshot.workspaceId ? activeRuntimePatch(patch) : {}),
        } as Partial<StudioState>;
      });
      return { completed, total };
    };

    offProgress = EventsOn(`progress:${jobId}`, (p: ProgressInfo) => {
      const patch: WorkspacePatch = { progress: p };
      store.setState((state) => ({
        workspaces: patchWorkspaceRuntime(state.workspaces, snapshot.workspaceId, patch),
        ...(state.activeWorkspaceId === snapshot.workspaceId ? activeRuntimePatch(patch) : {}),
      } as Partial<StudioState>));
    });
    offLog = EventsOn(`log:${jobId}`, (line: string) => {
      const patch: WorkspacePatch = { lastLogLine: line };
      store.setState((state) => ({
        workspaces: patchWorkspaceRuntime(state.workspaces, snapshot.workspaceId, patch),
        ...(state.activeWorkspaceId === snapshot.workspaceId ? activeRuntimePatch(patch) : {}),
      } as Partial<StudioState>));
    });
    if (!payload.disablePreview) {
      offPreview = EventsOn(`preview:${jobId}`, (preview: StreamPreviewPayload) => {
        store.setState((state) => (
          streamPreviewStatePatch(state, jobId, preview, {
            workspaceId: snapshot.workspaceId,
            mode: mode === "edit" ? "edit" : "generate",
            prompt: payload.prompt,
            size: snapshot.size,
            quality: snapshot.quality,
            outputFormat: snapshot.outputFormat,
            currentImage: snapshot.currentImage,
            batchIndex: snapshot.batchIndex,
            previewSlotIndex: snapshot.previewSlotIndex,
          }) ?? {}
        ));
      });
    }

    const startedAt = Date.now();
    offResult = EventsOn(`result:${jobId}`, (r: any) => {
      cleanup();
      void (async () => {
        try {
          const elapsedSec = (Date.now() - startedAt) / 1000;
          const rd = [elapsedSec, ...store.getState().recentDurations].slice(0, 5);
          const willNotify = typeof document !== "undefined" && document.visibilityState !== "visible";
          const parentId = mode === "edit" ? (snapshot.sources[0]?.path || snapshot.currentImage?.savedPath) : undefined;
          const itemID = cryptoIDFallback();
          const fallbackB64 = typeof r.imageB64 === "string" ? r.imageB64 : "";
          const previewItem: HistoryItem = {
            id: itemID,
            imageId: r.imageId || undefined,
            previewUrl: r.previewUrl || undefined,
            thumbPath: r.thumbPath || undefined,
            previewWidth: typeof r.previewWidth === "number" ? r.previewWidth : undefined,
            previewHeight: typeof r.previewHeight === "number" ? r.previewHeight : undefined,
            imageB64: fallbackB64 || undefined,
            imageBlob: null,
            previewBlob: null,
            previewOnly: true,
            prompt: r.prompt,
            revisedPrompt: r.revisedPrompt,
            mode: r.mode as Mode,
            size: snapshot.size,
            quality: snapshot.quality,
            outputFormat: snapshot.outputFormat,
            parentId,
            createdAt: Date.now(),
            seed: payload.seed || undefined,
            negativePrompt: payload.negativePrompt || undefined,
            background: normalizeBackgroundValue(payload.background),
            outputCompression: normalizeOutputCompressionValue(payload.outputCompression),
            inputFidelity: normalizeInputFidelityValue(payload.inputFidelity),
            imageStyle: normalizeImageStyleValue(payload.imageStyle),
            moderation: payload.moderation === "auto" ? "auto" : "low",
            styleTag: snapshot.styleTag || undefined,
            batchIndex: snapshot.batchIndex,
            previewSlotIndex: snapshot.previewSlotIndex,
            elapsedSec: Number(elapsedSec.toFixed(1)),
            savedPath: r.savedPath,
            rawPath: r.rawPath,
          };
          const activeItem: HistoryItem = {
            ...previewItem,
            fullUrl: r.fullUrl || (r.imageId ? `/media/full/${r.imageId}` : undefined),
            previewOnly: false,
          };
          const historyItem: HistoryItem = {
            ...previewItem,
            previewOnly: true,
          };
          const { completed: completedNow, total: totalNow } = removeFromRunning();
          const currentItem = totalNow > 1 ? historyItem : activeItem;
          const trimmed = trimHistory([historyItem, ...store.getState().history]);
          store.setState((state) => {
            const workspace = state.workspaces.find((w) => w.id === snapshot.workspaceId);
            const existingBatchIDs = state.activeWorkspaceId === snapshot.workspaceId
              ? state.batchResults.map((b) => b.id)
              : workspace?.batchResultIds ?? [];
            const gridWasOpen = state.activeWorkspaceId === snapshot.workspaceId
              ? state.resultGridOpen
              : workspace?.resultGridOpen ?? false;
            const nextBatchIDs = existingBatchIDs.includes(historyItem.id)
              ? existingBatchIDs
              : [...existingBatchIDs, historyItem.id];
            const nextGridOpen = gridWasOpen;
            const batchResults = state.activeWorkspaceId === snapshot.workspaceId
              ? [...state.batchResults, historyItem]
              : state.batchResults;
            return {
              history: trimmed,
              recentDurations: rd,
              workspaces: patchWorkspaceRuntime(state.workspaces, snapshot.workspaceId, {
                currentImageId: historyItem.id,
                batchResultIds: nextBatchIDs,
                resultGridOpen: nextGridOpen,
              }),
              ...(state.activeWorkspaceId === snapshot.workspaceId
                ? {
                    currentImage: currentItem,
                    batchResults,
                    resultGridOpen: nextGridOpen,
                    maskDataURL: null,
                    annotations: [],
                    tool: "pan",
                  }
                : {}),
            } as Partial<StudioState>;
          });
          persistTrimmedHistory(trimmed);
          persistHistoryItem(historyItem).catch(() => undefined);
          const loopMode = snapshot.loopGeneration.enabled;
          const isFinalLoopResult = loopMode && completedNow === totalNow;
          const shouldPlaySound = shouldPlayCompletionSound({
            config: store.getState().completionSound,
            completedNow,
            totalNow,
          });
          const shouldNotifyCompletion = shouldSendCompletionNotification({
            config: store.getState().completionNotification,
            completedNow,
            totalNow,
            windowHidden: willNotify,
          });
          if (shouldPlaySound) {
            void playCompletionSound(store.getState().completionSound);
          }
          if (shouldNotifyCompletion && (!loopMode || isFinalLoopResult)) {
            const notificationBody = (r.revisedPrompt ?? r.prompt ?? payload.prompt ?? "").trim()
              || (historyItem.mode === "edit" ? "图片编辑任务已完成" : "图片生成任务已完成");
            showSystemNotification("Image Studio · 已完成", notificationBody, () => {
              store.getState().openResultDetail(historyItem);
            });
          }
          if (!loopMode) {
            store.getState().pushToast(
              totalNow > 1
                ? `已完成 (${completedNow}/${totalNow}) · ${elapsedSec.toFixed(0)}s`
                : `已${historyItem.mode === "edit" ? "编辑" : "生成"} · ${elapsedSec.toFixed(0)}s`,
              "success",
              6000,
              { label: "查看详情", onClick: () => store.getState().openResultDetail(historyItem) },
            );
            if (totalNow <= 1) {
              store.getState().enqueueSavePrompt({ kind: "single", item: historyItem });
            }
            maybeEnqueueBatchSavePrompt(store, snapshot.workspaceId, completedNow, totalNow);
          } else if (isFinalLoopResult) {
            store.getState().pushToast(
              `循环出图完成 · ${completedNow} 张`,
              "success",
              6000,
              { label: "查看详情", onClick: () => store.getState().openResultDetail(historyItem) },
            );
          }
          settle("success");
          if (snapshot.batchProcessLink?.sourcePath) {
            const targetDirectory = snapshot.batchProcessLink.outputDir.trim()
              || directoryFromPath(snapshot.batchProcessLink.sourcePath);
            void BuildBatchOutputPath(
              snapshot.batchProcessLink.sourcePath,
              targetDirectory,
              snapshot.batchProcessLink.outputNamePrefix,
            ).then((targetPath) => {
              const suggestedName = targetPath.split(/[\\/]/).pop() || `${snapshot.batchProcessLink?.outputNamePrefix || "processed-"}image.png`;
              return saveHistoryItemToDirectoryAs(historyItem, targetDirectory, suggestedName);
            }).catch((error: any) => {
              store.getState().pushToast(`批处理保存失败:${error?.message ?? error}`, "warn", 6000);
            });
          }
          if (loopMode && snapshot.loopGeneration.autoSave && snapshot.loopGeneration.autoSaveDir.trim()) {
            void saveHistoryItemToDirectory(historyItem, snapshot.loopGeneration.autoSaveDir).catch((error: any) => {
              store.getState().pushToast(`自动另存为失败:${error?.message ?? error}`, "warn", 6000);
            });
          }
          // 首次成功生图 → 延迟 2s 弹 GitHub Star 引导。localStorage 标志一旦
          // 写入就再也不弹(无论用户点 star 还是关闭)。延迟是为了让用户先看
          // 到图,然后再被礼貌打扰。
          try {
            if (!isMac
                && localStorage.getItem("gptcodex.starPrompted") !== "1"
                && !store.getState().starPromptOpen) {
              setTimeout(() => {
                const snapshot = store.getState();
                const overlayBusy =
                  snapshot.upstreamModalOpen ||
                  snapshot.resultDetail !== null ||
                  document.querySelector('[role="dialog"]') !== null;
                if (!overlayBusy && localStorage.getItem("gptcodex.starPrompted") !== "1") {
                  store.setState({ starPromptOpen: true, starPromptSource: "auto" });
                }
              }, 3500);
            }
          } catch { /* localStorage 不可用 → 静默跳过 */ }
        } catch (err: any) {
          const patch: WorkspacePatch = {
            errorMessage: `处理结果失败:${err?.message ?? err}`,
            errorCanRetry: true,
            errorRawPath: null,
          };
          store.setState((state) => ({
            workspaces: patchWorkspaceRuntime(state.workspaces, snapshot.workspaceId, patch),
            ...(state.activeWorkspaceId === snapshot.workspaceId ? activeRuntimePatch(patch) : {}),
          } as Partial<StudioState>));
          const { completed: completedNow, total: totalNow } = removeFromRunning();
          maybeEnqueueBatchSavePrompt(store, snapshot.workspaceId, completedNow, totalNow);
          settle("error");
        }
      })();
    });
    offError = EventsOn(`error:${jobId}`, (e: { message: string; rawPath?: string }) => {
      cleanup();
      store.setState((state) => {
        const runtime = workspaceRuntimeFromState(state, snapshot.workspaceId);
        const prunedPreview = removeStreamPreview(runtime.streamPreviews, jobId);
        const patch: WorkspacePatch = {
          errorMessage: e?.message ?? "未知错误",
          errorCanRetry: true,
          errorRawPath: (typeof e?.rawPath === "string" && e.rawPath) ? e.rawPath : null,
          streamPreview: prunedPreview.streamPreview,
          streamPreviews: prunedPreview.streamPreviews,
        };
        return {
          workspaces: patchWorkspaceRuntime(state.workspaces, snapshot.workspaceId, patch),
          ...(state.activeWorkspaceId === snapshot.workspaceId
            ? {
                ...activeRuntimePatch(patch),
                currentImage: restoreCurrentImageAfterPreviewError(state, jobId, {
                  workspaceId: snapshot.workspaceId,
                  mode: mode === "edit" ? "edit" : "generate",
                  prompt: payload.prompt,
                  size: snapshot.size,
                  quality: snapshot.quality,
                  outputFormat: snapshot.outputFormat,
                  currentImage: snapshot.currentImage,
                }),
              }
            : {}),
        } as Partial<StudioState>;
      });
      const { completed: completedNow, total: totalNow } = removeFromRunning();
      maybeEnqueueBatchSavePrompt(store, snapshot.workspaceId, completedNow, totalNow);
      settle("error");
    });
    const started = mode === "edit"
      ? await wailsEdit({ ...payload, requestedJobId: jobId })
      : await wailsGenerate({ ...payload, requestedJobId: jobId });
    if (started.jobId && started.jobId !== jobId) {
      cleanup();
      throw new Error(`job id 不一致: expected ${jobId}, got ${started.jobId}`);
    }
  } catch (e: any) {
    cleanup();
    const patch: WorkspacePatch = {
      errorMessage: `提交失败:${e?.message ?? e}`,
      errorCanRetry: true,
      errorRawPath: null,
    };
    let completedNow = 0;
    let totalNow = 0;
    store.setState((state) => {
      const runtime = workspaceRuntimeFromState(state, snapshot.workspaceId);
      completedNow = runtime.jobsCompleted + 1;
      totalNow = runtime.jobsTotal;
      const nextMeta = { ...state.runningJobMeta };
      delete nextMeta[jobId];
      const remaining = runtime.runningJobs.filter((id) => id !== jobId);
      const prunedPreview = removeStreamPreview(runtime.streamPreviews, jobId);
      const nextPatch: WorkspacePatch = {
        ...patch,
        runningJobs: remaining,
        jobsTotal: remaining.length === 0 ? 0 : runtime.jobsTotal,
        jobsCompleted: remaining.length === 0 ? 0 : runtime.jobsCompleted,
        progress: remaining.length === 0 ? null : runtime.progress,
        streamPreview: remaining.length === 0 ? null : prunedPreview.streamPreview,
        streamPreviews: remaining.length === 0 ? {} : prunedPreview.streamPreviews,
        lastLogLine: remaining.length === 0 ? "" : runtime.lastLogLine,
      };
      return {
        runningJobMeta: nextMeta,
        workspaces: patchWorkspaceRuntime(state.workspaces, snapshot.workspaceId, nextPatch),
        ...(state.activeWorkspaceId === snapshot.workspaceId ? activeRuntimePatch(nextPatch) : {}),
      } as Partial<StudioState>;
    });
    maybeEnqueueBatchSavePrompt(store, snapshot.workspaceId, completedNow, totalNow);
    settle("error");
  }
}

export { tempDataURLFromB64, writeBase64ToTempFile };

let compatibilityExportEnabled = false;
let compatibilityFingerprint = "";

function enableCompatibilityExport() {
  const state = useStudioStore.getState();
  compatibilityExportEnabled = true;
  compatibilityFingerprint = compatibilityExportFingerprint(state);
  scheduleCompatibilityExport(state);
}

useStudioStore.subscribe((state) => {
  if (!compatibilityExportEnabled) return;
  const next = compatibilityExportFingerprint(state);
  if (next === compatibilityFingerprint) return;
  compatibilityFingerprint = next;
  scheduleCompatibilityExport(state);
});

async function materializeHistoryItem(item: HistoryItem): Promise<HistoryItem> {
  return materializeHistoryItemRuntime(item, {
    setState: (fn) => useStudioStore.setState((state) => fn(state)),
  });
}

async function ensureFullHistoryItem(item: HistoryItem | null): Promise<HistoryItem | null> {
  return ensureFullHistoryItemRuntime(item, {
    setState: (fn) => useStudioStore.setState((state) => fn(state)),
  });
}
