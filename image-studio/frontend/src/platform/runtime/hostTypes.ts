import type { RequestPolicy } from "../../types/domain";

export type GenerateOptionsLike = {
  apiKey: string;
  mode: string;
  prompt: string;
  size: string;
  quality: string;
  outputFormat: string;
  imagePaths: string[];
  imagePath: string;
  maskB64: string;
  seed: number;
  negativePrompt: string;
  background: string;
  outputCompression: number;
  inputFidelity: string;
  imageStyle: string;
  moderation: string;
  userIdentifier?: string;
  baseURL: string;
  textModelID: string;
  imageModelID: string;
  reasoningEffort?: string;
  proxyMode?: string;
  proxyURL?: string;
  apiMode: string;
  responsesTransport?: string;
  requestPolicy: string;
  imagesNewAPICompat?: boolean;
  noPromptRevision: boolean;
  concurrencyLimit?: number;
  partialImages?: number;
  fallbackProfile?: {
    baseURL: string;
    apiKey: string;
    textModelID: string;
    imageModelID: string;
    reasoningEffort?: string;
    apiMode: string;
    responsesTransport?: string;
    requestPolicy: RequestPolicy;
    imagesNewAPICompat?: boolean;
  };
  autoRetryEnabled?: boolean;
  autoRetryCount?: number;
  disablePreview?: boolean;
  requestedJobId?: string;
  sourceImages?: Array<{
    path?: string;
    name?: string;
    mimeType?: string | null;
    imageB64?: string | null;
    imageBlob?: Blob | null;
  }>;
};

export type PromptOptimizeOptionsLike = {
  apiKey: string;
  prompt: string;
  mode: string;
  baseURL: string;
  textModelID: string;
  proxyMode?: string;
  proxyURL?: string;
  imagePaths: string[];
  imagePath: string;
  sourceImages?: Array<{
    path?: string;
    name?: string;
    mimeType?: string | null;
    imageB64?: string | null;
    imageBlob?: Blob | null;
  }>;
};

export type ProbeUpstreamOptionsLike = {
  apiKey: string;
  baseURL: string;
  proxyMode?: string;
  proxyURL?: string;
  apiMode?: string;
  responsesTransport?: string;
};

export type ProbeUpstreamResultLike = {
  modelCount: number;
  models?: UpstreamModelDescriptorLike[];
  responsesTransport?: string;
  responsesTransportOK?: boolean;
  responsesTransportError?: string;
};

export type UpstreamModelDescriptorLike = {
  id: string;
  object?: string;
  ownedBy?: string;
  displayName?: string;
};

export type CodexAPIConfigLike = {
  provider: string;
  baseURL: string;
  apiKey: string;
  wireAPI: string;
};

export type JobStartedLike = { jobId: string };
export type ImportedImageLike = {
  path: string;
  imageB64?: string;
  imageId?: string;
  previewUrl?: string;
  previewWidth?: number;
  previewHeight?: number;
};
export type BatchInputImageLike = {
  path: string;
  name: string;
  size: number;
  width?: number;
  height?: number;
  previewUrl?: string;
  previewWidth?: number;
  previewHeight?: number;
};
export type BatchInputDirectoryLike = {
  directory: string;
  images: BatchInputImageLike[];
};
export type SelectFilesResponseLike = {
  files: BatchInputImageLike[];
};
export type ImageTransformResultLike = { path: string; acceleration?: string };
export type SelectFileResponseLike = {
  path: string;
  size: number;
  imageB64?: string;
  imageId?: string;
  previewUrl?: string;
  previewWidth?: number;
  previewHeight?: number;
};
export type MediaAssetRefLike = {
  imageId?: string;
  savedPath?: string;
  thumbPath?: string;
  previewUrl?: string;
  fullUrl?: string;
  previewWidth?: number;
  previewHeight?: number;
};
export type CompatibilityStateLike = Record<string, unknown>;
export type AppUpdateInfoLike = {
  currentVersion: string;
  latestVersion: string;
  releaseTag: string;
  releaseName?: string;
  releaseURL: string;
  publishedAt?: string;
  body?: string;
  hasUpdate: boolean;
};

export type AppUpdateProbeResultLike = {
  appVersion?: string;
  currentVersion?: string;
  latestVersion?: string;
  releaseTag?: string;
  releaseURL?: string;
  ignoredReleaseTag?: string;
  updateInfoAvailable: boolean;
  hasUpdate: boolean;
  shouldShowUpdate: boolean;
  appUpdateModalOpen: boolean;
};

export type PromptImportBilingualTextLike = {
  zh?: string;
  en?: string;
};

export type PromptImportPayloadLike = {
  prompt: PromptImportBilingualTextLike;
  negative_prompt?: PromptImportBilingualTextLike;
  aspect_ratio?: string;
  resolvedSize?: string;
};

export type PromptImportActivationLike = {
  tokens?: string[];
  invalidCount?: number;
};
export type HostKind = "wails-desktop" | "android-shell" | "browser";

export type HostCapabilities = {
  localGeneration: boolean;
  promptOptimization: boolean;
  nativeFileDialogs: boolean;
  nativeImageTransforms: boolean;
  imageTransformAcceleration: "gpu-metal" | "gpu-webgl" | "cpu-canvas" | "native" | "none";
  nativeHistoryFileIO: boolean;
  nativeOutputDirectoryPicker: boolean;
  secureCredentialStore: boolean;
};

export type KernelRuntimeMode = "auto" | "local" | "remote";
