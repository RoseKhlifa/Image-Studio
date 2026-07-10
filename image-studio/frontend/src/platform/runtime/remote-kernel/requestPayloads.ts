import {
  detectImageMimeTypeFromBase64,
  imageExtensionForMimeType,
} from "../../../lib/images.ts";
import {
  buildResponsesPayload as buildSharedResponsesPayload,
  normalizeBackground,
  normalizeImageStyle,
  normalizeInputFidelity,
  normalizeOutputCompression,
  normalizePartialImages,
  normalizeModeration,
  normalizeReasoningEffort,
  normalizeUserIdentifier,
  googleInteractionsEndpoint,
  openAIAPIEndpoint,
  shouldSendExtendedImageParameters,
  supportsImageBackground,
  supportsImageStyle,
  supportsInputFidelity,
  supportsImageModeration,
  supportsOutputCompression,
  shouldUseImagesNewAPICompat,
  shouldUseGoogleNativeInteractions,
  supportsImagesResponseFormat,
} from "../../../../../../shared/kernel/requestModel.js";
import { normalizeBaseURL, normalizeImageModel } from "./common.ts";
import { RemoteKernelError, type RemoteGeneratePayload, type RemoteJobRequest } from "./types.ts";

export type ImagesRequestProtocol = "openai-images" | "google-interactions";

const MAX_MASK_IMAGE_BYTES = 50 * 1024 * 1024;
const GOOGLE_INTERACTION_ASPECT_RATIOS = [
  "1:8", "1:4", "2:3", "3:4", "4:5", "1:1", "5:4", "4:3", "3:2", "16:9", "21:9", "4:1", "8:1",
] as const;

function googleInteractionResponseFormat(size: string, outputFormat: string): Record<string, string> {
  const format: Record<string, string> = { type: "image", delivery: "inline" };
  if (outputFormat === "jpeg") format.mime_type = "image/jpeg";
  else if (outputFormat === "png" || !outputFormat) format.mime_type = "image/png";
  else throw new RemoteKernelError("Google Interactions 当前只支持 PNG/JPEG 输出，请调整输出格式后重试");

  const matched = /^(\d+)x(\d+)$/i.exec(size.trim());
  if (!matched) return format;
  const width = Number(matched[1]);
  const height = Number(matched[2]);
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) return format;

  const targetRatio = width / height;
  format.aspect_ratio = GOOGLE_INTERACTION_ASPECT_RATIOS.reduce((best, candidate) => {
    const [left, right] = candidate.split(":").map(Number);
    const [bestLeft, bestRight] = best.split(":").map(Number);
    return Math.abs(Math.log((left / right) / targetRatio)) < Math.abs(Math.log((bestLeft / bestRight) / targetRatio))
      ? candidate
      : best;
  }, "1:1" as (typeof GOOGLE_INTERACTION_ASPECT_RATIOS)[number]);
  const maxSide = Math.max(width, height);
  format.image_size = maxSide <= 768 ? "512" : maxSide >= 3072 ? "4K" : maxSide >= 1536 ? "2K" : "1K";
  return format;
}

function googleInteractionInput(prompt: string, sourceDataURLs: string[]): string | Array<Record<string, string>> {
  if (sourceDataURLs.length === 0) return prompt;
  const content: Array<Record<string, string>> = [{ type: "text", text: prompt }];
  for (const dataURL of sourceDataURLs) {
    const match = /^data:([^;,]+);base64,(.+)$/s.exec(dataURL);
    if (!match) throw new RemoteKernelError("Google Interactions 参考图不是有效的 base64 data URL");
    content.push({ type: "image", mime_type: match[1], data: match[2] });
  }
  return content;
}

export function buildResponsesPayload(
  payload: RemoteGeneratePayload,
  sourceDataURLs: string[],
): Record<string, unknown> {
  const maskMimeType = payload.maskB64
    ? (detectImageMimeTypeFromBase64(payload.maskB64) || "image/png")
    : "image/png";
  return buildSharedResponsesPayload({
    ...payload,
    reasoningEffort: normalizeReasoningEffort(payload.reasoningEffort || ""),
  }, sourceDataURLs, { maskMimeType });
}

export async function buildImagesRequestBody(
  request: RemoteJobRequest,
  sourceDataURLs: string[],
): Promise<{ url: string; headers?: Record<string, string>; body: BodyInit; protocol: ImagesRequestProtocol }> {
  const baseURL = normalizeBaseURL(request.payload.baseURL);
  const mode = request.payload.mode === "edit" ? "edit" : "generate";
  const imageModel = normalizeImageModel(request.payload.imageModelID);
  const size = request.payload.size || "1024x1024";
  const quality = request.payload.quality || "auto";
  const outputFormat = request.payload.outputFormat || "png";
  const background = normalizeBackground(request.payload.background);
  const imageStyle = normalizeImageStyle(request.payload.imageStyle);
  const inputFidelity = normalizeInputFidelity(request.payload.inputFidelity);
  const outputCompression = normalizeOutputCompression(request.payload.outputCompression);
  const moderation = normalizeModeration(request.payload.moderation);
  const userIdentifier = normalizeUserIdentifier(request.payload.userIdentifier || "");
  const includeExtended = shouldSendExtendedImageParameters(request.payload.requestPolicy);
  const partialImages = request.payload.disablePreview ? 0 : normalizePartialImages(request.payload.partialImages);
  const useNewAPICompat = shouldUseImagesNewAPICompat(request.payload);

  if (shouldUseGoogleNativeInteractions(baseURL, imageModel)) {
    if (mode === "edit" && sourceDataURLs.length === 0) {
      throw new RemoteKernelError("Google Interactions 图生图需要至少一张源图");
    }
    if (request.payload.maskB64) {
      throw new RemoteKernelError("Google Interactions 不支持 OpenAI mask 参数；请清除蒙版，或改用支持 /v1/images/edits 的中转站");
    }
    return {
      url: googleInteractionsEndpoint(baseURL),
      protocol: "google-interactions",
      headers: {
        "Content-Type": "application/json",
        "x-goog-api-key": request.payload.apiKey,
      },
      body: JSON.stringify({
        model: imageModel,
        input: googleInteractionInput(request.payload.prompt, sourceDataURLs),
        response_format: googleInteractionResponseFormat(size, outputFormat),
        store: false,
      }),
    };
  }

  if (mode === "edit") {
    if (sourceDataURLs.length === 0) {
      throw new RemoteKernelError("图生图模式需要至少一张源图(请先添加参考图)");
    }
    const form = new FormData();
    for (let i = 0; i < sourceDataURLs.length; i++) {
      const dataURL = sourceDataURLs[i];
      const payload = dataURL.slice(dataURL.indexOf(",") + 1);
      const mimeType = dataURL.slice(5, dataURL.indexOf(";")) || "image/png";
      const ext = imageExtensionForMimeType(mimeType);
      form.append(i === 0 ? "image" : "image[]", new Blob([Uint8Array.from(atob(payload), (ch) => ch.charCodeAt(0))], { type: mimeType }), `source-${i + 1}.${ext}`);
    }
    if (request.payload.maskB64) {
      const maskMime = detectImageMimeTypeFromBase64(request.payload.maskB64);
      if (!maskMime) throw new RemoteKernelError("蒙版图片不是支持的 PNG/JPEG/WebP 格式");
      const maskBytes = Uint8Array.from(atob(request.payload.maskB64), (ch) => ch.charCodeAt(0));
      if (maskBytes.byteLength > MAX_MASK_IMAGE_BYTES) {
        throw new RemoteKernelError(`蒙版图片过大(${maskBytes.byteLength}B > ${MAX_MASK_IMAGE_BYTES}B 上限)`);
      }
      const ext = imageExtensionForMimeType(maskMime);
      form.append("mask", new Blob([maskBytes], { type: maskMime }), `mask.${ext}`);
    }
    form.append("prompt", request.payload.prompt);
    form.append("model", imageModel);
    form.append("n", "1");
    form.append("size", size);
    form.append("quality", quality);
    form.append("output_format", outputFormat);
    if (supportsImageBackground(imageModel)) {
      form.append("background", background);
    }
    if (supportsOutputCompression(imageModel, outputFormat)) {
      form.append("output_compression", String(outputCompression));
    }
    if (supportsInputFidelity(imageModel) && inputFidelity !== "auto") {
      form.append("input_fidelity", inputFidelity);
    }
    if (supportsImageModeration(imageModel)) {
      form.append("moderation", moderation);
    }
    if (userIdentifier) {
      form.append("user", userIdentifier);
    }
    if (useNewAPICompat || supportsImagesResponseFormat(imageModel, mode)) {
      form.append("response_format", "b64_json");
    }
    if (!useNewAPICompat) {
      form.append("stream", "true");
      form.append("partial_images", String(partialImages));
    }
    if (includeExtended && request.payload.seed) form.append("seed", String(request.payload.seed));
    if (includeExtended && request.payload.negativePrompt.trim()) form.append("negative_prompt", request.payload.negativePrompt.trim());
    return { url: openAIAPIEndpoint(baseURL, "images/edits"), body: form, protocol: "openai-images" };
  }

  const payload: Record<string, unknown> = {
    model: imageModel,
    prompt: request.payload.prompt,
    n: 1,
    size,
    quality,
    output_format: outputFormat,
  };
  if (supportsImageBackground(imageModel)) {
    payload.background = background;
  }
  if (supportsOutputCompression(imageModel, outputFormat)) {
    payload.output_compression = outputCompression;
  }
  if (supportsImageStyle(imageModel) && imageStyle !== "default") {
    payload.style = imageStyle;
  }
  if (supportsImageModeration(imageModel)) {
    payload.moderation = moderation;
  }
  if (userIdentifier) {
    payload.user = userIdentifier;
  }
  if (useNewAPICompat || supportsImagesResponseFormat(imageModel, mode)) {
    payload.response_format = "b64_json";
  }
  if (!useNewAPICompat) {
    payload.stream = true;
    payload.partial_images = partialImages;
  }
  if (includeExtended && request.payload.seed) payload.seed = request.payload.seed;
  if (includeExtended && request.payload.negativePrompt.trim()) payload.negative_prompt = request.payload.negativePrompt.trim();
  return {
    url: openAIAPIEndpoint(baseURL, "images/generations"),
    protocol: "openai-images",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  };
}
