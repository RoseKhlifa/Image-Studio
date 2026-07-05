export const DEFAULT_TEXT_MODEL = "gpt-5.5";
export const DEFAULT_IMAGE_MODEL = "gpt-image-2";
export const DEFAULT_SIZE = "1024x1024";
export const DEFAULT_QUALITY = "auto";
export const DEFAULT_OUTPUT_FORMAT = "png";
export const DEFAULT_BACKGROUND = "auto";
export const DEFAULT_OUTPUT_COMPRESSION = 100;
export const DEFAULT_INPUT_FIDELITY = "auto";
export const DEFAULT_IMAGE_STYLE = "default";
export const DEFAULT_MODERATION = "low";
export const DEFAULT_REASONING_EFFORT = "xhigh";
export const DEFAULT_REQUEST_POLICY = "openai";
export const DEFAULT_PARTIAL_IMAGES = 1;
export const DEFAULT_AUTO_RETRY_COUNT = 5;
export const MAX_AUTO_RETRY_COUNT = 10;
export const MAX_ATTEMPTS = DEFAULT_AUTO_RETRY_COUNT + 1;
export const RETRY_BACKOFF_MS = 15_000;
export const STATUS_INTERVAL_MS = 10_000;
export const OPENAI_IMAGE_SIZE_ALIGNMENT = 16;
export const MIN_OPENAI_IMAGE_SIDE = 64;
export const MAX_OPENAI_IMAGE_SIDE = 3840;
export const MAX_OPENAI_IMAGE_PIXELS = 3840 * 2160;
export const MAX_OPENAI_IMAGE_ASPECT_RATIO = 3;

const NO_PROMPT_REVISION_INSTRUCTIONS = "You are a tool runner. Pass the user prompt to image_generation VERBATIM. DO NOT rewrite, expand, polish, or revise it in any way. Use the exact text the user gave.";

export function normalizeBaseURL(raw) {
  const trimmed = String(raw || "").trim().replace(/\/+$/, "");
  return trimmed.replace(/\/v1$/i, "");
}

export function isVersionedOpenAICompatibilityBaseURL(raw) {
  try {
    const parsed = new URL(normalizeBaseURL(raw));
    return parsed.pathname.replace(/\/+$/, "").toLowerCase().endsWith("/openai");
  } catch {
    return false;
  }
}

export function openAIAPIEndpoint(baseURL, endpointPath) {
  const normalized = normalizeBaseURL(baseURL);
  const path = String(endpointPath || "").trim().replace(/^\/+|\/+$/g, "");
  if (!path) return normalized;
  if (isVersionedOpenAICompatibilityBaseURL(normalized)) {
    return `${normalized}/${path}`;
  }
  return `${normalized}/v1/${path}`;
}

export function normalizeAPIMode(apiMode) {
  return apiMode === "images" ? "images" : "responses";
}

export function normalizeRequestPolicy(requestPolicy) {
  return requestPolicy === "compat" ? "compat" : DEFAULT_REQUEST_POLICY;
}

export function normalizeTextModel(modelID) {
  return String(modelID || "").trim() || DEFAULT_TEXT_MODEL;
}

export function normalizeImageModel(modelID) {
  return String(modelID || "").trim() || DEFAULT_IMAGE_MODEL;
}

export function normalizePromptText(prompt) {
  return String(prompt || "").trim();
}

export function normalizeNegativePrompt(negativePrompt) {
  return String(negativePrompt || "").trim();
}

export function normalizeUserIdentifier(value) {
  const trimmed = String(value || "").trim();
  if (!trimmed) return "";
  return Array.from(trimmed).slice(0, 64).join("");
}

export function normalizeBackground(value) {
  if (value === "opaque" || value === "transparent") return value;
  return DEFAULT_BACKGROUND;
}

export function normalizeOutputCompression(value) {
  if (value === null || value === undefined || value === "") return DEFAULT_OUTPUT_COMPRESSION;
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) return DEFAULT_OUTPUT_COMPRESSION;
  return Math.max(0, Math.min(100, Math.round(numeric)));
}

export function normalizeInputFidelity(value) {
  if (value === "low" || value === "high") return value;
  return DEFAULT_INPUT_FIDELITY;
}

export function normalizeImageStyle(value) {
  if (value === "vivid" || value === "natural") return value;
  return DEFAULT_IMAGE_STYLE;
}

export function normalizeModeration(value) {
  return value === "auto" ? "auto" : DEFAULT_MODERATION;
}

export function normalizeReasoningEffort(value) {
  return value === "low" || value === "medium" || value === "high" || value === "xhigh"
    ? value
    : DEFAULT_REASONING_EFFORT;
}

export function normalizePartialImages(value) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric < 0) return DEFAULT_PARTIAL_IMAGES;
  return Math.max(0, Math.min(3, Math.floor(numeric)));
}

export function normalizeAutoRetryCount(value) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric < 0) return DEFAULT_AUTO_RETRY_COUNT;
  return Math.max(1, Math.min(MAX_AUTO_RETRY_COUNT, Math.floor(numeric)));
}

export function parseSizeValue(size) {
  const match = /^(\d+)x(\d+)$/i.exec(String(size || "").trim());
  if (!match) return null;
  const width = Number(match[1]);
  const height = Number(match[2]);
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) return null;
  return { width, height };
}

export function formatSizeValue(width, height) {
  return `${Math.floor(width)}x${Math.floor(height)}`;
}

function roundAligned(value, mode = "nearest", alignment = OPENAI_IMAGE_SIZE_ALIGNMENT) {
  const scaled = Number(value) / alignment;
  if (!Number.isFinite(scaled)) return 0;
  if (mode === "down") return Math.floor(scaled) * alignment;
  if (mode === "up") return Math.ceil(scaled) * alignment;
  return Math.round(scaled) * alignment;
}

function sizeCandidateSet(value, min, max, alignment = OPENAI_IMAGE_SIZE_ALIGNMENT) {
  const clamped = Math.max(min, Math.min(max, Number(value)));
  return Array.from(new Set([
    roundAligned(clamped, "nearest", alignment),
    roundAligned(clamped, "down", alignment),
    roundAligned(clamped, "up", alignment),
  ].map((candidate) => Math.max(min, Math.min(max, candidate)))))
    .filter((candidate) => candidate >= min && candidate <= max)
    .sort((left, right) => Math.abs(left - clamped) - Math.abs(right - clamped) || right - left);
}

function sizeDistance(width, height, targetWidth, targetHeight) {
  return Math.abs(width - targetWidth) / Math.max(targetWidth, 1)
    + Math.abs(height - targetHeight) / Math.max(targetHeight, 1);
}

function sizeWithinLimits(width, height) {
  if (width < MIN_OPENAI_IMAGE_SIDE || height < MIN_OPENAI_IMAGE_SIDE) return false;
  if (width > MAX_OPENAI_IMAGE_SIDE || height > MAX_OPENAI_IMAGE_SIDE) return false;
  if (width * height > MAX_OPENAI_IMAGE_PIXELS) return false;
  const aspect = width / height;
  return aspect <= MAX_OPENAI_IMAGE_ASPECT_RATIO && aspect >= (1 / MAX_OPENAI_IMAGE_ASPECT_RATIO);
}

export function normalizeOpenAIImageSize(size) {
  const parsed = typeof size === "string" ? parseSizeValue(size) : size;
  if (!parsed) return null;
  let targetWidth = parsed.width;
  let targetHeight = parsed.height;
  const aspect = Math.max(1 / MAX_OPENAI_IMAGE_ASPECT_RATIO, Math.min(MAX_OPENAI_IMAGE_ASPECT_RATIO, targetWidth / targetHeight));

  if (targetWidth / targetHeight !== aspect) {
    if (targetWidth >= targetHeight) targetWidth = targetHeight * aspect;
    else targetHeight = targetWidth / aspect;
  }

  const maxSide = Math.max(targetWidth, targetHeight);
  if (maxSide > MAX_OPENAI_IMAGE_SIDE) {
    const scale = MAX_OPENAI_IMAGE_SIDE / maxSide;
    targetWidth *= scale;
    targetHeight *= scale;
  }

  const pixelCount = targetWidth * targetHeight;
  if (pixelCount > MAX_OPENAI_IMAGE_PIXELS) {
    const scale = Math.sqrt(MAX_OPENAI_IMAGE_PIXELS / pixelCount);
    targetWidth *= scale;
    targetHeight *= scale;
  }

  const widthCandidates = sizeCandidateSet(targetWidth, MIN_OPENAI_IMAGE_SIDE, MAX_OPENAI_IMAGE_SIDE);
  const heightCandidates = sizeCandidateSet(targetHeight, MIN_OPENAI_IMAGE_SIDE, MAX_OPENAI_IMAGE_SIDE);
  let best = null;
  let bestDistance = Number.POSITIVE_INFINITY;
  let bestAspectDistance = Number.POSITIVE_INFINITY;
  let bestAreaDistance = Number.POSITIVE_INFINITY;

  for (const width of widthCandidates) {
    for (const height of heightCandidates) {
      if (!sizeWithinLimits(width, height)) continue;
      const distance = sizeDistance(width, height, targetWidth, targetHeight);
      const aspectDistance = Math.abs((width / height) - (targetWidth / targetHeight));
      const areaDistance = Math.abs((width * height) - (targetWidth * targetHeight)) / Math.max(targetWidth * targetHeight, 1);
      if (
        distance < bestDistance
        || (distance === bestDistance && aspectDistance < bestAspectDistance)
        || (distance === bestDistance && aspectDistance === bestAspectDistance && areaDistance < bestAreaDistance)
      ) {
        best = { width, height };
        bestDistance = distance;
        bestAspectDistance = aspectDistance;
        bestAreaDistance = areaDistance;
      }
    }
  }

  return best;
}

export function repairSizeForOpenAI(payload) {
  const currentSize = String(payload?.size || "").trim();
  const parsed = parseSizeValue(currentSize);
  if (!parsed) return null;
  const normalized = normalizeOpenAIImageSize(parsed);
  if (!normalized) return null;
  const nextSize = formatSizeValue(normalized.width, normalized.height);
  if (nextSize === currentSize) return null;
  return {
    ...payload,
    size: nextSize,
  };
}

export function extractInvalidSize(raw) {
  const text = String(raw || "").trim();
  if (!text) return null;
  const pattern = /Invalid size '(\d+x\d+)'\. Width and height must both be divisible by 16/i;
  const direct = text.match(pattern);
  if (direct) {
    return { original: direct[1], reason: "divisible_by_16" };
  }
  try {
    const data = JSON.parse(text);
    const message = String(data?.error?.message || data?.message || "");
    const nested = message.match(pattern);
    if (!nested) return null;
    return { original: nested[1], reason: "divisible_by_16" };
  } catch {
    return null;
  }
}

export function isCompatRequestPolicy(requestPolicy) {
  return normalizeRequestPolicy(requestPolicy) === "compat";
}

export function classifyImageModel(modelID) {
  const normalized = normalizeImageModel(modelID).toLowerCase();
  if (normalized.startsWith("dall-e-2")) return "dalle2";
  if (normalized.startsWith("dall-e-3")) return "dalle3";
  if (normalized.startsWith("gpt-image") || normalized.startsWith("chatgpt-image")) return "gpt-image";
  return "other";
}

export function supportsImagesResponseFormat(imageModelID, mode = "generate") {
  const family = classifyImageModel(imageModelID);
  if (mode === "edit") return family === "dalle2";
  return family === "dalle2" || family === "dalle3";
}

export function supportsImageModeration(imageModelID) {
  return classifyImageModel(imageModelID) === "gpt-image";
}

export function supportsImageBackground(imageModelID) {
  return classifyImageModel(imageModelID) === "gpt-image";
}

export function supportsOutputCompression(imageModelID, outputFormat) {
  return supportsImageBackground(imageModelID) && (outputFormat === "jpeg" || outputFormat === "webp");
}

export function supportsInputFidelity(imageModelID) {
  const normalized = normalizeImageModel(imageModelID).toLowerCase();
  if (normalized.startsWith("gpt-image-2")) return false;
  if (normalized.startsWith("gpt-image-1.5")) return true;
  if (normalized.startsWith("gpt-image-1-mini")) return true;
  if (normalized.startsWith("gpt-image-1")) return true;
  if (normalized.startsWith("chatgpt-image-latest")) return true;
  return false;
}

export function supportsImageStyle(imageModelID) {
  return classifyImageModel(imageModelID) === "dalle3";
}

export function isGoogleImageModel(imageModelID) {
  const normalized = normalizeImageModel(imageModelID).toLowerCase();
  return normalized.startsWith("gemini-") ||
    normalized.startsWith("imagen-") ||
    normalized.includes("nano-banana");
}

export function shouldSendExtendedImageParameters(requestPolicy) {
  return isCompatRequestPolicy(requestPolicy);
}

export function shouldUseImagesNewAPICompat(payload) {
  return payload?.imagesNewAPICompat === true || isGoogleImageModel(payload?.imageModelID);
}

export function fileNameFromPath(path) {
  if (!path) return "image.png";
  return String(path).split(/[\\/]/).pop() || "image.png";
}

export function dataURLFromBase64Image(b64, mimeType = "image/png") {
  const encoded = String(b64 || "").trim();
  if (!encoded) return "";
  return `data:${mimeType};base64,${encoded}`;
}

export function buildResponsesInputContent(prompt, sourceDataURLs) {
  const content = [{ type: "input_text", text: normalizePromptText(prompt) }];
  for (const dataURL of sourceDataURLs) {
    content.push({ type: "input_image", image_url: dataURL });
  }
  return content;
}

export function buildResponsesImageTool(payload, sourceDataURLs, options = {}) {
  const size = payload.size || DEFAULT_SIZE;
  const quality = payload.quality || DEFAULT_QUALITY;
  const outputFormat = payload.outputFormat || DEFAULT_OUTPUT_FORMAT;
  const background = normalizeBackground(payload.background);
  const outputCompression = normalizeOutputCompression(payload.outputCompression);
  const inputFidelity = normalizeInputFidelity(payload.inputFidelity);
  const negativePrompt = normalizeNegativePrompt(payload.negativePrompt);
  const moderation = normalizeModeration(payload.moderation);
  const compatExtensions = shouldSendExtendedImageParameters(payload.requestPolicy);
  const partialImages = payload.disablePreview ? 0 : normalizePartialImages(payload.partialImages);
  const tool = {
    type: "image_generation",
    model: normalizeImageModel(payload.imageModelID),
    action: sourceDataURLs.length > 0 ? "edit" : "generate",
    size,
    quality,
    output_format: outputFormat,
    partial_images: partialImages,
  };
  if (supportsImageBackground(payload.imageModelID)) tool.background = background;
  if (supportsOutputCompression(payload.imageModelID, outputFormat)) tool.output_compression = outputCompression;
  if (supportsInputFidelity(payload.imageModelID) && sourceDataURLs.length > 0 && inputFidelity !== DEFAULT_INPUT_FIDELITY) {
    tool.input_fidelity = inputFidelity;
  }
  if (supportsImageModeration(payload.imageModelID)) tool.moderation = moderation;
  if (compatExtensions && payload.seed) tool.seed = payload.seed;
  if (compatExtensions && negativePrompt) tool.negative_prompt = negativePrompt;

  const maskMimeType = String(options.maskMimeType || "image/png").trim() || "image/png";
  if (payload.maskB64) {
    tool.input_image_mask = {
      image_url: dataURLFromBase64Image(payload.maskB64, maskMimeType),
    };
  }
  return tool;
}

export function buildResponsesPayload(payload, sourceDataURLs, options = {}) {
  const content = buildResponsesInputContent(payload.prompt, sourceDataURLs);
  const userIdentifier = normalizeUserIdentifier(payload.userIdentifier);
  const tool = {
    ...buildResponsesImageTool(payload, sourceDataURLs, options),
  };

  const request = {
    model: normalizeTextModel(payload.textModelID),
    input: [{ role: "user", content }],
    tools: [tool],
    tool_choice: { type: "image_generation" },
    reasoning: { effort: normalizeReasoningEffort(payload.reasoningEffort) },
    store: false,
    stream: true,
  };
  request.instructions = NO_PROMPT_REVISION_INSTRUCTIONS;
  if (userIdentifier) request.safety_identifier = userIdentifier;
  return request;
}

export function buildPromptOptimizePayload(input, sourceDataURLs) {
  let instruction = "Rewrite the user's image prompt into a clearer, more detailed prompt for image generation. Keep the meaning, preserve the requested subject, and only return the improved prompt text. Do not add explanations, labels, markdown, or quotes.";
  if (String(input.mode || "").trim() === "edit") {
    instruction += " Treat any attached images as reference context and preserve edit intent.";
  }
  const content = [{ type: "input_text", text: `Original prompt:\n${normalizePromptText(input.prompt)}` }];
  for (const dataURL of sourceDataURLs) {
    content.push({ type: "input_image", image_url: dataURL });
  }
  return {
    model: normalizeTextModel(input.textModelID),
    instructions: instruction,
    input: [{ role: "user", content }],
    reasoning: { effort: "low" },
    store: false,
  };
}

export function retryableMarkers() {
  return [
    "upstream request failed",
    "error code 524",
    "524: a timeout occurred",
    "error code 503",
    "service unavailable",
    "error code 504",
    "gateway time-out",
    "service temporarily unavailable",
    "origin_gateway_timeout",
  ];
}

function hasPartialImageWithoutFinal(raw) {
  const text = String(raw || "").trim();
  if (!text) return false;
  let partialSeen = false;
  let finalSeen = false;
  for (const line of text.split(/\r?\n/)) {
    if (!line.startsWith("data: ")) continue;
    const payload = line.slice(6).trim();
    if (!payload || payload === "[DONE]") continue;
    let event;
    try {
      event = JSON.parse(payload);
    } catch {
      continue;
    }
    switch (event?.type) {
      case "response.image_generation_call.partial_image":
        if (event.partial_image_b64) partialSeen = true;
        break;
      case "response.output_item.done":
        if (event?.item?.type === "image_generation_call" && event?.item?.result) {
          finalSeen = true;
        }
        break;
      case "image_generation.partial_image":
      case "image_edit.partial_image":
        if (event.b64_json) partialSeen = true;
        break;
      case "image_generation.completed":
      case "image_edit.completed":
        if (event.b64_json) finalSeen = true;
        break;
      default:
        break;
    }
  }
  return partialSeen && !finalSeen;
}

export function isRetryableRaw(raw) {
  const text = String(raw || "").trim();
  const lower = text.toLowerCase();
  if (retryableMarkers().some((marker) => lower.includes(marker))) return true;
  if (hasPartialImageWithoutFinal(text)) return true;
  try {
    const data = JSON.parse(text);
    if (data?.retryable === true) return true;
    if ([403, 502, 503, 504, 524].includes(Number(data?.status))) return true;
    const err = data?.error;
    if (err && typeof err === "object") {
      const message = String(err.message || "").toLowerCase();
      const type = String(err.type || "").toLowerCase();
      const upstreamStatus = Number(err.upstreamStatus ?? err.upstream_status ?? err.status);
      if ([403, 502, 503, 504, 524].includes(upstreamStatus)) return true;
      if (message.includes("temporarily unavailable")) return true;
      if (message.includes("upstream request failed")) return true;
      if (type === "api_error" || type === "server_error" || type === "upstream_error") return true;
    }
  } catch {
    // ignore
  }
  return false;
}

export function describeAPIError(error) {
  const code = String(error?.code || "");
  const message = String(error?.message || "");
  const type = String(error?.type || "");

  switch (code.toLowerCase()) {
    case "moderation_blocked":
      return "🚫 上游内容审核拦截 · 生成被拒";
    case "content_policy_violation":
      return "🚫 上游内容政策拦截 (content_policy_violation)";
    case "rate_limit_exceeded":
      return `⏱ 上游限速 (rate_limit_exceeded)\n\n${message}`;
    case "insufficient_quota":
    case "billing_hard_limit_reached":
      return `💳 上游账户额度不足\n\n${message}`;
    case "model_not_found":
      return `🤷 上游找不到指定模型\n\n${message}`;
    default:
      break;
  }

  const parts = [];
  if (message) parts.push(message);
  const tail = [];
  if (code) tail.push(`code: ${code}`);
  if (type) tail.push(`type: ${type}`);
  if (tail.length > 0) parts.push(`(${tail.join(", ")})`);
  return parts.length > 0 ? `接口返回错误:${parts.join(" ")}` : "接口返回错误";
}

function extractOutputTextFromContent(content) {
  if (!Array.isArray(content)) return "";
  for (const part of content) {
    if (part?.type === "output_text" && typeof part?.text === "string" && part.text.trim()) {
      return part.text.trim();
    }
  }
  return "";
}

function extractStructuredMessage(value) {
  if (!value || typeof value !== "object") return "";

  if (Array.isArray(value)) {
    for (const child of value) {
      const text = extractStructuredMessage(child);
      if (text) return text;
    }
    return "";
  }

  if ((value.type === "output_text" || value.type === "response.output_text.done") && typeof value.text === "string" && value.text.trim()) {
    return value.text.trim();
  }

  const directContentText = extractOutputTextFromContent(value.content);
  if (directContentText) return directContentText;

  for (const key of ["part", "item", "response", "output"]) {
    const text = extractStructuredMessage(value[key]);
    if (text) return text;
  }

  return "";
}

export function describeProblem(raw) {
  const text = String(raw || "").trim();
  if (!text) return "接口返回为空。";
  const lower = text.toLowerCase();
  if (lower.includes("error code 524") || lower.includes("524: a timeout occurred")) {
    return "Cloudflare 524:源站在超时时间内没有返回有效响应。";
  }
  if (lower.includes("error code 504") || lower.includes("gateway time-out")) {
    return "Cloudflare 504:源站网关超时。";
  }

  try {
    const data = JSON.parse(text);
    if (data?.error && typeof data.error === "object") return describeAPIError(data.error);
    if (typeof data?.message === "string" && data.message.trim()) return `接口返回消息:${data.message.trim()}`;
    if (data?.status && [502, 503, 504, 524].includes(Number(data.status))) {
      return `接口返回 ${data.status}:上游服务超时。`;
    }
    const structuredMessage = extractStructuredMessage(data);
    if (structuredMessage) return structuredMessage;
  } catch {
    // ignore
  }

  for (const line of text.split(/\r?\n/)) {
    if (!line.startsWith("data: ")) continue;
    const payload = line.slice(6).trim();
    if (!payload || payload === "[DONE]") continue;
    try {
      const event = JSON.parse(payload);
      if (event?.error && typeof event.error === "object") return describeAPIError(event.error);
      if (event?.response?.error && typeof event.response.error === "object") return describeAPIError(event.response.error);
      const structuredMessage = extractStructuredMessage(event);
      if (structuredMessage) return structuredMessage;
    } catch {
      // ignore
    }
  }
  if (hasPartialImageWithoutFinal(text)) {
    return "接口只返回了流式预览帧,没有返回最终图片。";
  }
  return "接口已返回内容,但没有发现 image_generation_call.result。";
}
