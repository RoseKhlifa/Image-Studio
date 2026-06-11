// Domain types shared across components. Backend-emitted shapes live in
// wailsjs/go/models.ts; this file owns the frontend-only state.

export type Mode = "generate" | "edit";

// 上游 API 形态 —— Responses (`/v1/responses` + SSE) 或标准 Images API。
// 老代码里以前是顶层全局二选一,v0.1.6 起降级成 profile 的字段。
export type APIMode = "responses" | "images";
export type RequestPolicy = "openai" | "compat";
export type ReasoningEffortValue = "low" | "medium" | "high" | "xhigh";
export type ResponsesTransport = "sse" | "websocket";
export type AutoRetryCountValue = number;

// UpstreamProfile 是一组完整可用于生成的上游配置。用户可以保存多个,例如
// 「gptcodex 主号 / gptcodex 备号 / OpenAI 直连」,在 UI 里下拉切换 active。
//
// 注意 apiKey 不在这里 —— 它走系统凭据存储(Keychain / Credential Manager /
// Secret Service),用 profile.id 作为 keyring "user" 寻址。JSON 导出 /
// localStorage 里都不会出现明文 key。
export interface UpstreamProfile {
  id: string;
  name: string;
  apiMode: APIMode;
  responsesTransport?: ResponsesTransport;
  requestPolicy: RequestPolicy;
  imagesNewAPICompat?: boolean;
  baseURL: string;
  textModelID: string;
  imageModelID: string;
  reasoningEffort: ReasoningEffortValue;
  // 0 = 不限。同一 profile 跨所有 workspace 共享并发计数。
  concurrencyLimit: number;
  // 失败重试路由到的备用 profile。空 = 不自动切备用上游。
  fallbackProfileId?: string;
  createdAt: number;
  // 最近一次被 setActive / 提交生成 时更新;用于把最近使用过的 profile 在
  // 下拉里排到前面,以及下次启动默认 active。
  lastUsedAt?: number;
}

export interface PromptTemplate {
  id: string;
  label: string;
  text: string;
  createdAt: number;
  updatedAt: number;
}

export type SizeValue = "auto" | `${number}x${number}`;
export type QualityValue = "auto" | "high" | "medium" | "low" | "standard" | "hd";
export type KernelRuntimeMode = "auto" | "local" | "remote";
export type ProxyMode = "none" | "system" | "custom";
// 让上游做编码;落盘扩展名 jpeg → .jpg,其他原样。
export type OutputFormatValue = "png" | "jpeg" | "webp";
export type BackgroundValue = "auto" | "opaque" | "transparent";
export type InputFidelityValue = "auto" | "low" | "high";
export type ImageStyleValue = "default" | "vivid" | "natural";
export type ModerationValue = "low" | "auto";
export type ThemeMode = "system" | "light" | "dark";
export type CompletionSoundMode = "default" | "custom";
export type EditSourceMode = "manual" | "batch";
export type BatchProcessOutputMode = "source_dir" | "custom_dir";
export type AutoAspectResolutionPreset = "" | "256" | "512" | "1k" | "2k" | "4k";
export type BatchProcessAutoAspectResolution = AutoAspectResolutionPreset;
export type SystemNotificationPermissionState = "default" | "granted" | "denied" | "unsupported";

export interface CompletionSoundConfig {
  enabled: boolean;
  mode: CompletionSoundMode;
  customName: string;
  customDataURL: string;
}

export interface CompletionNotificationConfig {
  enabled: boolean;
}

export interface SizeOption { value: SizeValue; label: string; }
export interface QualityOption { value: QualityValue; label: string; }
export interface OutputFormatOption { value: OutputFormatValue; label: string; }
export interface BackgroundOption { value: BackgroundValue; label: string; }
export interface InputFidelityOption { value: InputFidelityValue; label: string; }
export interface ImageStyleOption { value: ImageStyleValue; label: string; }
export interface ModerationOption { value: ModerationValue; label: string; }
export interface ReasoningEffortOption { value: ReasoningEffortValue; label: string; }

export interface CustomAspectRatio {
  id: string;
  label: string;
  width: number;
  height: number;
  createdAt: number;
}

export const SIZE_OPTIONS: SizeOption[] = [
  { value: "auto",      label: "自适应 auto" },
  { value: "1024x1024", label: "方形 1024×1024" },
  { value: "1536x1024", label: "横版 1536×1024" },
  { value: "1024x1536", label: "竖版 1024×1536" },
  { value: "2048x2048", label: "2K 方形 2048×2048" },
  { value: "2048x1360", label: "2K 横版 2048×1360" },
  { value: "1360x2048", label: "2K 竖版 1360×2048" },
  { value: "2048x1152", label: "2K 横版 2048×1152" },
  { value: "1152x2048", label: "2K 竖版 1152×2048" },
  { value: "2880x2880", label: "4K 方形 2880×2880" },
  { value: "3456x2304", label: "4K 横版 3456×2304" },
  { value: "2304x3456", label: "4K 竖版 2304×3456" },
  { value: "3840x2160", label: "4K 横版 3840×2160" },
  { value: "2160x3840", label: "4K 竖版 2160×3840" },
];

export const QUALITY_OPTIONS: QualityOption[] = [
  { value: "auto",   label: "自适应 auto" },
  { value: "high",   label: "高质量 high" },
  { value: "medium", label: "中等 medium" },
  { value: "low",    label: "快速草稿 low" },
];

export const OUTPUT_FORMAT_OPTIONS: OutputFormatOption[] = [
  { value: "png",  label: "PNG" },
  { value: "jpeg", label: "JPEG" },
  { value: "webp", label: "WebP" },
];

export const BACKGROUND_OPTIONS: BackgroundOption[] = [
  { value: "auto", label: "自动 auto" },
  { value: "opaque", label: "纯色 opaque" },
  { value: "transparent", label: "透明 transparent" },
];

export const INPUT_FIDELITY_OPTIONS: InputFidelityOption[] = [
  { value: "auto", label: "默认 auto" },
  { value: "low", label: "低保真 low" },
  { value: "high", label: "高保真 high" },
];

export const IMAGE_STYLE_OPTIONS: ImageStyleOption[] = [
  { value: "default", label: "默认" },
  { value: "vivid", label: "vivid" },
  { value: "natural", label: "natural" },
];

export const MODERATION_OPTIONS: ModerationOption[] = [
  { value: "low",  label: "宽松 low" },
  { value: "auto", label: "标准 auto" },
];

export const REASONING_EFFORT_OPTIONS: ReasoningEffortOption[] = [
  { value: "xhigh", label: "最高 xhigh" },
  { value: "high", label: "高 high" },
  { value: "medium", label: "中 medium" },
  { value: "low", label: "低 low" },
];

export interface SourceImage {
  // Local path on disk (path is what the Go backend ultimately reads).
  path: string;
  name: string;
  size: number;       // bytes; 0 when unknown (e.g. reused-from-history)
  previewUrl?: string;
  previewWidth?: number;
  previewHeight?: number;
  imageBlob?: Blob | null;
  // Legacy/browser fallback for canvas preview. Wails source previews should
  // prefer previewUrl so selected files do not cross the bridge as base64.
  imageB64?: string;
}

export interface BatchProcessSourceImage {
  path: string;
  name: string;
  size: number;
  width?: number;
  height?: number;
  previewUrl?: string;
  previewWidth?: number;
  previewHeight?: number;
}

export interface BatchProcessConfig {
  enabled: boolean;
  inputDir: string;
  outputMode: BatchProcessOutputMode;
  outputDir: string;
  concurrency: number;
  retryOnFailure: boolean;
  fileNamePrefix: string;
  autoAspectResolution: BatchProcessAutoAspectResolution;
  discoveredSources: BatchProcessSourceImage[];
}

export interface HistoryItem {
  id: string;
  imageId?: string;
  previewUrl?: string;
  fullUrl?: string;
  previewPath?: string;
  thumbPath?: string;
  previewWidth?: number;
  previewHeight?: number;
  // Legacy/import/browser fallback. New Wails result records keep this empty so
  // full images do not live in persistent Zustand/history/batch state.
  imageB64?: string;
  imageBlob?: Blob | null;
  previewBlob?: Blob | null;
  previewOnly?: boolean;
  prompt: string;
  revisedPrompt?: string;
  mode: Mode;
  size: SizeValue;
  quality: QualityValue;
  outputFormat?: OutputFormatValue;
  parentId?: string;       // id of the source image (when mode === "edit")
  createdAt: number;       // unix ms

  // Extended params — captured at submit time so the item can be exactly
  // reproduced via "重新生成" or "应用参数" from the right-click menu.
  seed?: number;
  negativePrompt?: string;
  background?: BackgroundValue;
  outputCompression?: number;
  inputFidelity?: InputFidelityValue;
  imageStyle?: ImageStyleValue;
  moderation?: ModerationValue;
  styleTag?: string;
  batchIndex?: number;
  previewSlotIndex?: number;
  elapsedSec?: number;     // generation duration in seconds

  sourcePaths?: string[];
  savedPath?: string;
  rawPath?: string;
}

export interface ProgressInfo {
  stage: string;
  elapsed: number;
  bytes: number;
}

export interface StreamPreview {
  jobId: string;
  imageId?: string;
  previewUrl?: string;
  previewWidth?: number;
  previewHeight?: number;
  imageB64?: string;
  revisedPrompt?: string;
  partialImageIndex?: number;
  batchIndex?: number;
  previewSlotIndex?: number;
  updatedAt: number;
}

export type StreamPreviewMap = Record<string, StreamPreview>;

export interface LoopGenerationConfig {
  enabled: boolean;
  totalCount: number;
  concurrency: number;
  autoSave: boolean;
  autoSaveDir: string;
  livePreview: boolean;
}

export interface BatchProcessResultLink {
  sourcePath: string;
  outputDir: string;
  outputNamePrefix: string;
}

export interface Workspace {
  id: string;
  name: string;
  // Fields whose value is workspace-scoped (different per tab).
  prompt: string;
  negativePrompt: string;
  mode: Mode;
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
  batchCount: number;
  selectedPresetId?: string | null;
  editSourceMode: EditSourceMode;
  editAutoAspectResolution?: AutoAspectResolutionPreset;
  batchProcess: BatchProcessConfig;
  loopGeneration: LoopGenerationConfig;
  sources: SourceImage[];
  // We store currentImageId rather than the full HistoryItem so we don't
  // duplicate large base64 blobs. The history list is shared across tabs.
  currentImageId: string | null;
  // IDs from the latest multi-request run for this workspace. These are history
  // IDs so the tab state stays light while the canvas can reopen the batch grid.
  batchResultIds: string[];
  resultGridOpen: boolean;
  runningJobIds: string[];
  jobsTotal: number;
  jobsCompleted: number;
  progress: ProgressInfo | null;
  streamPreview: StreamPreview | null;
  streamPreviews?: StreamPreviewMap;
  lastLogLine: string;
  errorMessage: string | null;
  errorCanRetry?: boolean;
  // 最近一次失败时上游原始响应文件的绝对路径(SSE / Images API JSON)。前端
  // 「查看日志」按钮调 OpenFile 直接打开。请求前期校验失败 / 早期 IO 错误时
  // 此字段为 null。跟 errorMessage 一对,workspace 隔离,切 tab 各自保持。
  errorRawPath?: string | null;
  lastPayload?: import("../platform/runtime/hostTypes").GenerateOptionsLike | null;
}

export interface Preset {
  id: string;
  name: string;
  size: SizeValue;
  quality: QualityValue;
  outputFormat?: OutputFormatValue;
  negativePrompt: string;
  background?: BackgroundValue;
  outputCompression?: number;
  inputFidelity?: InputFidelityValue;
  imageStyle?: ImageStyleValue;
  moderation?: ModerationValue;
  styleTag?: string;
  editAutoAspectResolution?: AutoAspectResolutionPreset;
  kernelRuntimeMode?: KernelRuntimeMode;
  batchCount: number;
}

export interface Toast {
  id: string;
  text: string;
  kind: "info" | "success" | "error" | "warn";
  // Unix ms when the toast was created — used for ordering and auto-dismiss.
  createdAt: number;
  // Auto-dismiss timeout in ms; 0 = sticky (manual close only).
  ttl: number;
  // 可选 CTA。点击触发 onClick 后 toast 自动关闭。
  action?: { label: string; onClick: () => void };
}

export interface AppUpdateInfo {
  currentVersion: string;
  latestVersion: string;
  releaseTag: string;
  releaseName?: string;
  releaseURL: string;
  publishedAt?: string;
  body?: string;
  hasUpdate: boolean;
}

export type AnnotationKind = "rect" | "arrow" | "text" | "freehand";

export interface Annotation {
  id: string;
  kind: AnnotationKind;
  x: number;
  y: number;
  width?: number;
  height?: number;
  text?: string;
  color: string;
  // For freehand: flat number[] of x,y,x,y,... in image-local coords.
  points?: number[];
}

export const ANNOTATION_COLORS = [
  "#ff4d4d", "#ff9c00", "#ffd400", "#7bd400",
  "#00c8ff", "#4d7cff", "#a060ff", "#ff60c8",
];
