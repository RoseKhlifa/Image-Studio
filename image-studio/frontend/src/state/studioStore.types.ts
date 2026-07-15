import type { GenerateOptionsLike } from "../platform/runtime/hostTypes";
import type { SavePromptRequest } from "../lib/savePromptState";
import type {
  Annotation,
  AppUpdateInfo,
  APIMode,
  AutoAspectResolutionPreset,
  BatchProcessConfig,
  BackgroundValue,
  BatchProcessSourceImage,
  EditSourceMode,
  HistoryItem,
  ImageStyleValue,
  InputFidelityValue,
  KernelRuntimeMode,
  LoopGenerationConfig,
  ModerationValue,
  Mode,
  OutputFormatValue,
  CustomAspectRatio,
  CompletionSoundConfig,
  CompletionNotificationConfig,
  Preset,
  ProgressInfo,
  PromptTemplate,
  ProxyMode,
  QualityValue,
  RequestPolicy,
  SizeValue,
  SourceImage,
  StreamPreview,
  StreamPreviewMap,
  SystemNotificationPermissionState,
  ThemeMode,
  Toast,
  UpstreamProfile,
  Workspace,
} from "../types/domain";
import type { RunningJobMeta } from "./workspaceRuntime";

export interface ModeConfig {
  baseURL: string;
  apiKey: string;
  textModelID: string;
  imageModelID: string;
  concurrencyLimit: number;
}

export interface PromptOptimizeRequest {
  apiKey: string;
  prompt: string;
  mode: Mode | "describe";
  baseURL: string;
  textModelID: string;
  proxyMode: ProxyMode;
  proxyURL: string;
  allowInsecureConnection?: boolean;
  imagePaths: string[];
  imagePath: string;
  sourceImages?: Array<{
    path?: string;
    name?: string;
    mimeType?: string | null;
    imageB64?: string | null;
    imageBlob?: Blob | null;
  }>;
}

export interface Stroke {
  points: number[];
  size: number;
  erase?: boolean;
}

export interface UndoEntry {
  label: string;
  undo: (s: StudioState) => Partial<StudioState>;
  redo: (s: StudioState) => Partial<StudioState>;
}

export interface StudioState {
  apiKey: string;
  mode: Mode;
  prompt: string;
  negativePrompt: string;
  size: SizeValue;
  quality: QualityValue;
  outputFormat: OutputFormatValue;
  seed: number;
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
  kernelRuntimeMode: KernelRuntimeMode;
  baseURL: string;
  textModelID: string;
  proxyMode: ProxyMode;
  proxyURL: string;
  imageModelID: string;
  reasoningEffort: import("../types/domain").ReasoningEffortValue;
  apiMode: APIMode;
  responsesTransport: import("../types/domain").ResponsesTransport;
  requestPolicy: RequestPolicy;
  imagesNewAPICompat: boolean;
  noPromptRevision: boolean;
  profiles: UpstreamProfile[];
  activeProfileId: string;
  aiProfileId: string;
  sources: SourceImage[];
  editSourceMode: EditSourceMode;
  editAutoAspectResolution: AutoAspectResolutionPreset;
  batchProcess: BatchProcessConfig;
  runningJobs: string[];
  jobsTotal: number;
  jobsCompleted: number;
  progress: ProgressInfo | null;
  streamPreview: StreamPreview | null;
  streamPreviews: StreamPreviewMap;
  lastLogLine: string;
  errorMessage: string | null;
  errorCanRetry: boolean;
  errorRawPath: string | null;
  isRunning: boolean;
  lastPayload: GenerateOptionsLike | null;
  runningJobMeta: Record<string, RunningJobMeta>;
  currentImage: HistoryItem | null;
  history: HistoryItem[];
  historyHasMore: boolean;
  historyLoading: boolean;
  historyCursorBeforeDayStart: number | null;
  batchResults: HistoryItem[];
  resultGridOpen: boolean;
  historyRailCollapsed: boolean;
  historyTimelineOpen: boolean;
  tool: "pan" | "mask" | "annotate";
  brushSize: number;
  brushMode: "paint" | "erase";
  annotationKind: "rect" | "arrow" | "freehand" | "text";
  annotationColor: string;
  selectedAnnotationId: string | null;
  maskDataURL: string | null;
  strokes: Stroke[];
  annotations: Annotation[];
  compareB: HistoryItem | null;
  compareSplit: number;
  toasts: Toast[];
  recentDurations: number[];
  viewZoom: number;
  canvasViewResetTick: number;
  fullscreen: boolean;
  promptHistory: string[];
  promptTemplates: PromptTemplate[];
  batchCount: number;
  loopGeneration: LoopGenerationConfig;
  presets: Preset[];
  customAspectRatios: CustomAspectRatio[];
  theme: ThemeMode;
  fontScale: number;
  workspaces: Workspace[];
  activeWorkspaceId: string;
  selectedPresetId: string | null;
  styleTag: string;
  undoStack: UndoEntry[];
  redoStack: UndoEntry[];
  setField: <K extends keyof StudioState>(key: K, value: StudioState[K]) => void;
  setFullscreen: (value: boolean) => Promise<void>;
  toggleFullscreen: () => Promise<void>;
  setAPIKey: (v: string) => Promise<void>;
  clearError: () => void;
  createProfile: (input: {
    name?: string;
    apiMode: APIMode;
    responsesTransport?: import("../types/domain").ResponsesTransport;
    baseURL?: string;
    requestPolicy?: RequestPolicy;
    imagesNewAPICompat?: boolean;
    allowInsecureConnection?: boolean;
    textModelID?: string;
    imageModelID?: string;
    reasoningEffort?: import("../types/domain").ReasoningEffortValue;
    concurrencyLimit?: number;
    apiKey?: string;
    setActive?: boolean;
  }) => Promise<string>;
  updateProfile: (id: string, patch: Partial<Omit<UpstreamProfile, "id" | "createdAt">> & { apiKey?: string }) => Promise<boolean>;
  deleteProfile: (id: string) => Promise<void>;
  duplicateProfile: (id: string) => Promise<string | null>;
  setActiveProfile: (id: string) => Promise<void>;
  setAIProfile: (id: string) => Promise<boolean>;
  selectSourceImage: () => Promise<void>;
  chooseBatchInputDir: () => Promise<void>;
  chooseBatchInputFiles: () => Promise<void>;
  refreshBatchInputDir: () => Promise<void>;
  viewSourceOnCanvas: (index: number) => Promise<void>;
  compareSourceOnCanvas: (index: number) => Promise<void>;
  removeSource: (index: number) => void;
  clearSources: () => void;
  reorderSources: (from: number, to: number) => void;
  submit: () => Promise<void>;
  cancel: () => Promise<void>;
  reuseAsSource: (item: HistoryItem) => Promise<void>;
  applyHistoryParams: (item: HistoryItem) => void;
  regenerateFromHistory: (item: HistoryItem) => Promise<void>;
  deleteHistoryItem: (id: string) => Promise<void>;
  clearHistory: () => Promise<number>;
  saveCurrentImageAs: () => Promise<void>;
  bootstrap: () => Promise<void>;
  importMaskImage: () => Promise<void>;
  setMaskDataURL: (v: string | null) => void;
  pushStroke: (s: Stroke) => void;
  resetMask: () => void;
  addAnnotation: (a: Annotation) => void;
  removeAnnotation: (id: string) => void;
  updateAnnotation: (id: string, patch: Partial<Annotation>) => void;
  clearAnnotations: () => void;
  undo: () => void;
  redo: () => void;
  setCompareB: (item: HistoryItem | null) => void;
  setCompareSplit: (v: number) => void;
  openResultGrid: () => void;
  closeResultGrid: () => void;
  selectBatchResult: (item: HistoryItem) => Promise<void>;
  stepBatchResult: (delta: -1 | 1) => Promise<void>;
  importImageFile: (file: File) => Promise<void>;
  pushToast: (text: string, kind?: Toast["kind"], ttl?: number, action?: Toast["action"]) => void;
  dismissToast: (id: string) => void;
  resultDetail: HistoryItem | null;
  openResultDetail: (item: HistoryItem) => Promise<void>;
  closeResultDetail: () => void;
  savePromptRequest: SavePromptRequest | null;
  savePromptQueue: SavePromptRequest[];
  savePromptSuppressed: boolean;
  keepLogs: boolean;
  cleanupPreviewCacheOnExit: boolean;
  completionSound: CompletionSoundConfig;
  completionNotification: CompletionNotificationConfig;
  completionNotificationPermission: SystemNotificationPermissionState;
  ignoredReleaseTag: string;
  appUpdate: AppUpdateInfo | null;
  appUpdateModalOpen: boolean;
  enqueueSavePrompt: (request: SavePromptRequest) => void;
  closeSavePrompt: () => void;
  setSavePromptSuppressed: (value: boolean) => void;
  setKeepLogs: (value: boolean) => Promise<void>;
  setCleanupPreviewCacheOnExit: (value: boolean) => Promise<void>;
  ignoreAppUpdate: (releaseTag: string) => void;
  dismissAppUpdateModal: () => void;
  setCompletionSoundEnabled: (value: boolean) => void;
  setCompletionSoundMode: (value: CompletionSoundConfig["mode"]) => void;
  setCompletionSoundCustom: (input: { name: string; dataURL: string }) => void;
  resetCompletionSoundCustom: () => void;
  previewCompletionSound: () => Promise<void>;
  setCompletionNotificationEnabled: (value: boolean) => Promise<SystemNotificationPermissionState>;
  requestCompletionNotificationPermission: () => Promise<SystemNotificationPermissionState>;
  materializeCurrentImage: (item: HistoryItem) => Promise<HistoryItem>;
  retryLast: () => Promise<void>;
  setHistoryRailCollapsed: (collapsed: boolean) => void;
  loadMoreHistory: () => Promise<void>;
  openHistoryTimeline: () => void;
  closeHistoryTimeline: () => void;
  pruneHistoryOlderThanDays: (days: number) => Promise<number>;
  savePreset: (name: string) => string | null;
  overwritePreset: (id: string) => boolean;
  updatePreset: (id: string, patch: Partial<Omit<Preset, "id">>) => boolean;
  applyPreset: (id: string) => void;
  deletePreset: (id: string) => void;
  addPromptTemplate: (label: string, text: string) => string | null;
  updatePromptTemplate: (id: string, patch: Partial<Pick<PromptTemplate, "label" | "text">>) => boolean;
  deletePromptTemplate: (id: string) => void;
  exportHistory: () => Promise<void>;
  importHistory: () => Promise<void>;
  setTheme: (t: ThemeMode) => void;
  setFontScale: (v: number) => void;
  setProxyConfig: (mode: ProxyMode, url?: string) => void;
  customAspectRatioModalOpen: boolean;
  openCustomAspectRatioModal: () => void;
  closeCustomAspectRatioModal: () => void;
  addCustomAspectRatio: (width: number, height: number) => boolean;
  deleteCustomAspectRatio: (id: string) => void;
  customSizeModalOpen: boolean;
  openCustomSizeModal: () => void;
  closeCustomSizeModal: () => void;
  applyCustomSize: (width: number, height: number) => boolean;
  settingsOpen: boolean;
  openSettings: () => void;
  closeSettings: () => void;
  testAPIKey: () => Promise<void>;
  isTestingKey: boolean;
  isOptimizingPrompt: boolean;
  isInferringPrompt: boolean;
  optimizePrompt: () => Promise<void>;
  inferPromptFromCanvas: () => Promise<void>;
  upstreamModalOpen: boolean;
  upstreamReturnTarget: "app" | "settings";
  openUpstreamConfig: (returnTarget?: "app" | "settings") => void;
  closeUpstreamConfig: () => void;
  starPromptOpen: boolean;
  starPromptSource: "auto" | "manual";
  openStarPrompt: () => void;
  dismissStarPrompt: () => void;
  newWorkspace: (name?: string) => void;
  switchWorkspace: (id: string) => void;
  closeWorkspace: (id: string) => void;
  renameWorkspace: (id: string, name: string) => void;
  rotateCurrent: (degrees: number) => Promise<void>;
  flipCurrent: (horizontal: boolean) => Promise<void>;
  cropToRect: (x: number, y: number, w: number, h: number) => Promise<void>;
}
