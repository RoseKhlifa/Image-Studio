import type { RequestPolicy, UpstreamProvider } from "../../../types/domain";
import type { PreparedMaskComposite } from "../../../lib/maskComposite.ts";

export type KernelImageSource = {
  path?: string;
  name?: string;
  mimeType?: string | null;
  imageB64?: string | null;
  imageBlob?: Blob | null;
};

export type RemoteGeneratePayload = {
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
  provider?: UpstreamProvider;
  proxyMode?: string;
  proxyURL?: string;
  apiMode: string;
  responsesTransport?: string;
  requestPolicy: RequestPolicy;
  imagesNewAPICompat?: boolean;
  allowInsecureConnection?: boolean;
  noPromptRevision: boolean;
  concurrencyLimit?: number;
  partialImages?: number;
  disablePreview?: boolean;
  fallbackProfile?: {
    baseURL: string;
    apiKey: string;
    textModelID: string;
    imageModelID: string;
    reasoningEffort?: string;
    provider?: UpstreamProvider;
    apiMode: string;
    responsesTransport?: string;
    requestPolicy: RequestPolicy;
    imagesNewAPICompat?: boolean;
    allowInsecureConnection?: boolean;
  };
  autoRetryEnabled?: boolean;
  autoRetryCount?: number;
};

export type ProgressCallback = (stage: string, elapsedSeconds: number, bytesReceived: number) => void;
export type PartialImageCallback = (partial: {
  imageB64: string;
  revisedPrompt?: string;
  partialImageIndex?: number;
  sourceEvent?: "responses_partial" | "images_partial";
}) => void;

export type RemoteJobRequest = {
  payload: RemoteGeneratePayload;
  sourceImages?: KernelImageSource[];
  preparedMask?: PreparedMaskComposite | null;
};

export type RemoteJobCallbacks = {
  signal: AbortSignal;
  onLog?: (line: string) => void;
  onProgress?: ProgressCallback;
  onPartialImage?: PartialImageCallback;
};

export type RemoteJobResult = {
  imageB64: string;
  revisedPrompt: string;
  sourceEvent: string;
  rawPath: string | null;
  prompt: string;
  mode: string;
};

export type RemotePromptOptimizeInput = {
  apiKey: string;
  prompt: string;
  mode: string;
  baseURL: string;
  textModelID: string;
  proxyMode?: string;
  proxyURL?: string;
  allowInsecureConnection?: boolean;
  imagePaths?: string[];
  imagePath?: string;
  sourceImages?: KernelImageSource[];
};

export const RETRY_BACKOFF_MS = 15_000;
export const STATUS_INTERVAL_MS = 10_000;

export class RemoteKernelError extends Error {
  rawPath: string | null;

  constructor(message: string, rawPath: string | null = null) {
    super(message);
    this.name = "RemoteKernelError";
    this.rawPath = rawPath;
  }
}

export type NativeTextResponse = {
  status: number;
  body: string;
  contentType?: string;
  rawPath?: string | null;
  resultImageB64?: string;
  revisedPrompt?: string;
  sourceEvent?: string;
};

export type ExtractedImageResult = {
  imageB64: string;
  revisedPrompt: string;
  sourceEvent: string;
};
