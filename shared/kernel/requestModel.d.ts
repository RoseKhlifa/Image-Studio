export const DEFAULT_TEXT_MODEL: string;
export const DEFAULT_IMAGE_MODEL: string;
export const DEFAULT_SIZE: string;
export const DEFAULT_QUALITY: string;
export const DEFAULT_OUTPUT_FORMAT: string;
export const DEFAULT_BACKGROUND: string;
export const DEFAULT_OUTPUT_COMPRESSION: number;
export const DEFAULT_INPUT_FIDELITY: string;
export const DEFAULT_IMAGE_STYLE: string;
export const DEFAULT_MODERATION: string;
export const DEFAULT_REASONING_EFFORT: string;
export const DEFAULT_REQUEST_POLICY: string;
export const DEFAULT_PARTIAL_IMAGES: number;
export const DEFAULT_AUTO_RETRY_COUNT: number;
export const MAX_AUTO_RETRY_COUNT: number;
export const MAX_ATTEMPTS: number;
export const RETRY_BACKOFF_MS: number;
export const STATUS_INTERVAL_MS: number;
export const OPENAI_IMAGE_SIZE_ALIGNMENT: number;
export const MIN_OPENAI_IMAGE_SIDE: number;
export const MAX_OPENAI_IMAGE_SIDE: number;
export const MAX_OPENAI_IMAGE_PIXELS: number;
export const MAX_OPENAI_IMAGE_ASPECT_RATIO: number;

export type RequestPolicy = "openai" | "compat";
export type ReasoningEffort = "low" | "medium" | "high" | "xhigh";

export type SharedImageRequestPayload = {
  size?: string;
  quality?: string;
  outputFormat?: string;
  background?: string;
  outputCompression?: number;
  inputFidelity?: string;
  imageStyle?: string;
  userIdentifier?: string;
  prompt?: string;
  imageModelID?: string;
  textModelID?: string;
  negativePrompt?: string;
  moderation?: string;
  reasoningEffort?: ReasoningEffort;
  maskB64?: string;
  seed?: number;
  requestPolicy?: RequestPolicy;
  imagesNewAPICompat?: boolean;
  noPromptRevision?: boolean;
  mode?: string;
  partialImages?: number;
};

export function normalizeBaseURL(raw: string): string;
export function isVersionedOpenAICompatibilityBaseURL(raw: string): boolean;
export function openAIAPIEndpoint(baseURL: string, endpointPath: string): string;
export function normalizeAPIMode(apiMode: string): "responses" | "images";
export function normalizeRequestPolicy(requestPolicy: string): RequestPolicy;
export function normalizeTextModel(modelID: string): string;
export function normalizeImageModel(modelID: string): string;
export function normalizePromptText(prompt: string): string;
export function normalizeNegativePrompt(negativePrompt: string): string;
export function normalizeUserIdentifier(value: string): string;
export function normalizeBackground(value: string): "auto" | "opaque" | "transparent";
export function normalizeOutputCompression(value: unknown): number;
export function normalizeInputFidelity(value: string): "auto" | "low" | "high";
export function normalizeImageStyle(value: string): "default" | "vivid" | "natural";
export function normalizeModeration(value: string): "auto" | "low";
export function normalizeReasoningEffort(value: string): ReasoningEffort;
export function normalizePartialImages(value: unknown): number;
export function normalizeAutoRetryCount(value: unknown): number;
export function parseSizeValue(size: string): { width: number; height: number } | null;
export function formatSizeValue(width: number, height: number): string;
export function normalizeOpenAIImageSize(size: string | { width: number; height: number }): { width: number; height: number } | null;
export function repairSizeForOpenAI(payload: Record<string, unknown>): Record<string, unknown> | null;
export function extractInvalidSize(raw: string): { original: string; reason: "divisible_by_16" } | null;
export function isCompatRequestPolicy(requestPolicy: string): boolean;
export function classifyImageModel(modelID: string): "gpt-image" | "dalle2" | "dalle3" | "other";
export function supportsImagesResponseFormat(imageModelID: string, mode?: string): boolean;
export function supportsImageModeration(imageModelID: string): boolean;
export function supportsImageBackground(imageModelID: string): boolean;
export function supportsOutputCompression(imageModelID: string, outputFormat: string): boolean;
export function supportsInputFidelity(imageModelID: string): boolean;
export function supportsImageStyle(imageModelID: string): boolean;
export function isGoogleImageModel(imageModelID: string): boolean;
export function shouldSendExtendedImageParameters(requestPolicy: string): boolean;
export function shouldUseImagesNewAPICompat(payload: SharedImageRequestPayload | Record<string, unknown>): boolean;
export function fileNameFromPath(path?: string): string;
export function dataURLFromBase64Image(b64: string, mimeType?: string): string;
export function buildResponsesInputContent(prompt: string, sourceDataURLs: string[]): Array<Record<string, unknown>>;
export function buildResponsesImageTool(
  payload: SharedImageRequestPayload,
  sourceDataURLs: string[],
  options?: { maskMimeType?: string },
): Record<string, unknown>;
export function buildResponsesPayload(
  payload: SharedImageRequestPayload,
  sourceDataURLs: string[],
  options?: { maskMimeType?: string },
): Record<string, unknown>;
export function buildPromptOptimizePayload(
  input: {
    prompt?: string;
    mode?: string;
    textModelID?: string;
  },
  sourceDataURLs: string[],
): Record<string, unknown>;
export function retryableMarkers(): string[];
export function isRetryableRaw(raw: string): boolean;
export function describeAPIError(error: Record<string, unknown>): string;
export function describeProblem(raw: string): string;
