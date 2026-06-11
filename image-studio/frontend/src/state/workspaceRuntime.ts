import type {
  AutoAspectResolutionPreset,
  BatchProcessAutoAspectResolution,
  BatchProcessSourceImage,
  BatchProcessConfig,
  LoopGenerationConfig,
  ProgressInfo,
  StreamPreview,
  StreamPreviewMap,
  Workspace,
} from "../types/domain";
import type { GenerateOptionsLike } from "../platform/runtime/hostTypes";

export type APIModeValue = "responses" | "images";

export interface RunningJobMeta {
  workspaceId: string;
  apiMode: APIModeValue;
}

export interface WorkspacePatch extends Partial<Workspace> {
  runningJobs?: string[];
  runningJobIds?: string[];
}

export interface WorkspaceRuntimeState {
  activeWorkspaceId: string;
  runningJobs: string[];
  jobsTotal: number;
  jobsCompleted: number;
  progress: ProgressInfo | null;
  streamPreview: StreamPreview | null;
  streamPreviews?: StreamPreviewMap;
  lastLogLine: string;
  errorMessage: string | null;
  errorCanRetry: boolean;
  errorRawPath: string | null;
  lastPayload: GenerateOptionsLike | null;
  workspaces: Workspace[];
}

export interface WorkspaceRuntimeMirror {
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
  lastPayload: GenerateOptionsLike | null;
  isRunning: boolean;
}

export function normalizeAPIMode(mode: string): APIModeValue {
  return String(mode).trim() === "images" ? "images" : "responses";
}

export function apiModeLabel(mode: string): string {
  return normalizeAPIMode(mode) === "images" ? "Images API" : "Responses API";
}

export function normalizeConcurrencyLimit(value: unknown): number {
  const n = Number(value);
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : 0;
}

export function normalizeBatchCount(value: unknown): number {
  const n = Number(value);
  if (!Number.isFinite(n)) return 1;
  return Math.max(1, Math.min(9, Math.floor(n)));
}

export const DEFAULT_LOOP_GENERATION_COUNT = 10;
export const DEFAULT_LOOP_GENERATION_CONCURRENCY = 2;
export const MAX_LOOP_GENERATION_COUNT = 99;
export const MAX_LOOP_GENERATION_CONCURRENCY = 9;
export const DEFAULT_BATCH_PROCESS_CONCURRENCY = 2;
export const MAX_BATCH_PROCESS_CONCURRENCY = 9;

export function defaultLoopGenerationConfig(): LoopGenerationConfig {
  return {
    enabled: false,
    totalCount: DEFAULT_LOOP_GENERATION_COUNT,
    concurrency: DEFAULT_LOOP_GENERATION_CONCURRENCY,
    autoSave: false,
    autoSaveDir: "",
    livePreview: true,
  };
}

export function defaultBatchProcessConfig(): BatchProcessConfig {
  return {
    enabled: false,
    inputDir: "",
    outputMode: "source_dir",
    outputDir: "",
    concurrency: DEFAULT_BATCH_PROCESS_CONCURRENCY,
    retryOnFailure: false,
    fileNamePrefix: "processed-",
    autoAspectResolution: "",
    discoveredSources: [],
  };
}

export function normalizeAutoAspectResolutionPreset(value: unknown): AutoAspectResolutionPreset {
  return value === "256" || value === "512" || value === "1k" || value === "2k" || value === "4k"
    ? value
    : "";
}

export function normalizeBatchProcessAutoAspectResolution(value: unknown): BatchProcessAutoAspectResolution {
  return normalizeAutoAspectResolutionPreset(value);
}

export function normalizeBatchProcessConcurrency(value: unknown): number {
  const n = Number(value);
  if (!Number.isFinite(n)) return DEFAULT_BATCH_PROCESS_CONCURRENCY;
  return Math.max(1, Math.min(MAX_BATCH_PROCESS_CONCURRENCY, Math.floor(n)));
}

export function normalizeBatchProcessConfig(value: unknown): BatchProcessConfig {
  const source = value && typeof value === "object"
    ? value as Partial<BatchProcessConfig>
    : {};
  const discoveredSources: BatchProcessSourceImage[] = [];
  if (Array.isArray(source.discoveredSources)) {
    for (const item of source.discoveredSources) {
      if (!item || typeof item !== "object") continue;
      const candidate = item as {
        path?: unknown;
        name?: unknown;
        size?: unknown;
        width?: unknown;
        height?: unknown;
        previewUrl?: unknown;
        previewWidth?: unknown;
        previewHeight?: unknown;
      };
      const path = typeof candidate.path === "string" ? candidate.path.trim() : "";
      const name = typeof candidate.name === "string" ? candidate.name.trim() : "";
      if (!path || !name) continue;
      discoveredSources.push({
        path,
        name,
        size: Number.isFinite(Number(candidate.size)) ? Math.max(0, Math.floor(Number(candidate.size))) : 0,
        width: Number.isFinite(Number(candidate.width)) ? Math.floor(Number(candidate.width)) : undefined,
        height: Number.isFinite(Number(candidate.height)) ? Math.floor(Number(candidate.height)) : undefined,
        previewUrl: typeof candidate.previewUrl === "string" ? candidate.previewUrl : undefined,
        previewWidth: Number.isFinite(Number(candidate.previewWidth)) ? Math.floor(Number(candidate.previewWidth)) : undefined,
        previewHeight: Number.isFinite(Number(candidate.previewHeight)) ? Math.floor(Number(candidate.previewHeight)) : undefined,
      });
    }
  }
  return {
    enabled: source.enabled === true,
    inputDir: typeof source.inputDir === "string" ? source.inputDir.trim() : "",
    outputMode: source.outputMode === "custom_dir" ? "custom_dir" : "source_dir",
    outputDir: typeof source.outputDir === "string" ? source.outputDir.trim() : "",
    concurrency: normalizeBatchProcessConcurrency(source.concurrency),
    retryOnFailure: source.retryOnFailure === true,
    fileNamePrefix: typeof source.fileNamePrefix === "string" && source.fileNamePrefix.trim()
      ? source.fileNamePrefix.trim()
      : "processed-",
    autoAspectResolution: normalizeBatchProcessAutoAspectResolution(source.autoAspectResolution),
    discoveredSources,
  };
}

export function normalizeLoopGenerationCount(value: unknown): number {
  const n = Number(value);
  if (!Number.isFinite(n)) return DEFAULT_LOOP_GENERATION_COUNT;
  return Math.max(1, Math.min(MAX_LOOP_GENERATION_COUNT, Math.floor(n)));
}

export function normalizeLoopGenerationConcurrency(value: unknown): number {
  const n = Number(value);
  if (!Number.isFinite(n)) return DEFAULT_LOOP_GENERATION_CONCURRENCY;
  return Math.max(1, Math.min(MAX_LOOP_GENERATION_CONCURRENCY, Math.floor(n)));
}

export function normalizeLoopGenerationConfig(value: unknown): LoopGenerationConfig {
  const source = value && typeof value === "object"
    ? value as Partial<LoopGenerationConfig>
    : {};
  return {
    enabled: source.enabled === true,
    totalCount: normalizeLoopGenerationCount(source.totalCount),
    concurrency: normalizeLoopGenerationConcurrency(source.concurrency),
    autoSave: source.autoSave === true,
    autoSaveDir: typeof source.autoSaveDir === "string" ? source.autoSaveDir.trim() : "",
    livePreview: source.livePreview !== false,
  };
}

export function patchWorkspaceRuntime(workspaces: Workspace[], workspaceId: string, patch: WorkspacePatch): Workspace[] {
  return workspaces.map((w) => {
    if (w.id !== workspaceId) return w;
    const next: Workspace = { ...w };
    if (patch.name !== undefined) next.name = patch.name;
    if (patch.loopGeneration !== undefined) next.loopGeneration = normalizeLoopGenerationConfig(patch.loopGeneration);
    if (patch.editAutoAspectResolution !== undefined) next.editAutoAspectResolution = normalizeAutoAspectResolutionPreset(patch.editAutoAspectResolution);
    if (patch.batchProcess !== undefined) next.batchProcess = normalizeBatchProcessConfig(patch.batchProcess);
    if (patch.currentImageId !== undefined) next.currentImageId = patch.currentImageId;
    if (patch.batchResultIds !== undefined) next.batchResultIds = patch.batchResultIds;
    if (patch.resultGridOpen !== undefined) next.resultGridOpen = patch.resultGridOpen;
    if (patch.runningJobs !== undefined) next.runningJobIds = patch.runningJobs;
    if (patch.runningJobIds !== undefined) next.runningJobIds = patch.runningJobIds;
    if (patch.jobsTotal !== undefined) next.jobsTotal = patch.jobsTotal;
    if (patch.jobsCompleted !== undefined) next.jobsCompleted = patch.jobsCompleted;
    if (patch.progress !== undefined) next.progress = patch.progress;
    if (patch.streamPreview !== undefined) next.streamPreview = patch.streamPreview;
    if (patch.streamPreviews !== undefined) next.streamPreviews = patch.streamPreviews;
    if (patch.lastLogLine !== undefined) next.lastLogLine = patch.lastLogLine;
    if (patch.errorMessage !== undefined) next.errorMessage = patch.errorMessage;
    if (patch.errorCanRetry !== undefined) next.errorCanRetry = patch.errorCanRetry;
    if (patch.errorRawPath !== undefined) next.errorRawPath = patch.errorRawPath;
    if (patch.lastPayload !== undefined) next.lastPayload = patch.lastPayload;
    return next;
  });
}

export function workspaceRuntimeFromState(
  s: WorkspaceRuntimeState,
  workspaceId: string,
): WorkspaceRuntimeMirror {
  if (s.activeWorkspaceId === workspaceId) {
    return {
      runningJobs: s.runningJobs,
      jobsTotal: s.jobsTotal,
      jobsCompleted: s.jobsCompleted,
      progress: s.progress,
      streamPreview: s.streamPreview,
      streamPreviews: s.streamPreviews ?? {},
      lastLogLine: s.lastLogLine,
      errorMessage: s.errorMessage,
      errorCanRetry: s.errorCanRetry,
      errorRawPath: s.errorRawPath,
      lastPayload: s.lastPayload,
      isRunning: s.runningJobs.length > 0,
    };
  }
  const w = s.workspaces.find((item) => item.id === workspaceId);
  const runningJobs = w?.runningJobIds ?? [];
  return {
    runningJobs,
    jobsTotal: w?.jobsTotal ?? 0,
    jobsCompleted: w?.jobsCompleted ?? 0,
    progress: w?.progress ?? null,
    streamPreview: w?.streamPreview ?? null,
    streamPreviews: w?.streamPreviews ?? {},
    lastLogLine: w?.lastLogLine ?? "",
    errorMessage: w?.errorMessage ?? null,
    errorCanRetry: w?.errorCanRetry ?? false,
    errorRawPath: w?.errorRawPath ?? null,
    lastPayload: w?.lastPayload ?? null,
    isRunning: runningJobs.length > 0,
  };
}

export function activeRuntimePatch(patch: WorkspacePatch): Partial<WorkspaceRuntimeMirror> {
  const out: Partial<WorkspaceRuntimeMirror> = {};
  if (patch.runningJobs !== undefined) {
    out.runningJobs = patch.runningJobs;
    out.isRunning = patch.runningJobs.length > 0;
  }
  if (patch.runningJobIds !== undefined) {
    out.runningJobs = patch.runningJobIds;
    out.isRunning = patch.runningJobIds.length > 0;
  }
  if (patch.jobsTotal !== undefined) out.jobsTotal = patch.jobsTotal;
  if (patch.jobsCompleted !== undefined) out.jobsCompleted = patch.jobsCompleted;
  if (patch.progress !== undefined) out.progress = patch.progress;
  if (patch.streamPreview !== undefined) out.streamPreview = patch.streamPreview;
  if (patch.streamPreviews !== undefined) out.streamPreviews = patch.streamPreviews;
  if (patch.lastLogLine !== undefined) out.lastLogLine = patch.lastLogLine;
  if (patch.errorMessage !== undefined) out.errorMessage = patch.errorMessage;
  if (patch.errorCanRetry !== undefined) out.errorCanRetry = patch.errorCanRetry;
  if (patch.errorRawPath !== undefined) out.errorRawPath = patch.errorRawPath;
  if (patch.lastPayload !== undefined) out.lastPayload = patch.lastPayload;
  return out;
}

export function workspaceRunningCount(s: { runningJobMeta: Record<string, RunningJobMeta> }, apiMode: APIModeValue): number {
  return Object.values(s.runningJobMeta).filter((job) => job.apiMode === apiMode).length;
}
