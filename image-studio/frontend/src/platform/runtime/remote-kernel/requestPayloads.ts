import {
  detectImageMimeTypeFromBase64,
  imageExtensionForMimeType,
} from "../../../lib/images.ts";
import {
  assertBase64WithinLimit,
  assertImageDataURLWithinLimit,
  normalizeMaskForSource,
} from "../../../lib/maskComposite.ts";
import {
  buildResponsesPayload as buildSharedResponsesPayload,
  normalizeBackground,
  normalizeImageStyle,
  normalizeInputFidelity,
  normalizeUpstreamProvider,
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

export type ImagesRequestProtocol = "openai-images" | "google-interactions" | "grok-images";

const GOOGLE_INTERACTION_ASPECT_RATIOS = [
  "1:8", "1:4", "2:3", "3:4", "4:5", "1:1", "5:4", "4:3", "3:2", "16:9", "21:9", "4:1", "8:1",
] as const;
const GROK_ASPECT_RATIOS = [
  "1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3", "2:1", "1:2", "19.5:9", "9:19.5", "20:9", "9:20",
] as const;

function grokImageDimensions(size: string): { aspect_ratio: string; resolution: string } | null {
  const matched = /^(\d+)x(\d+)$/i.exec(size.trim());
  if (!matched) return null;
  const width = Number(matched[1]);
  const height = Number(matched[2]);
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) return null;
  const targetRatio = width / height;
  const aspectRatio = GROK_ASPECT_RATIOS.reduce((best, candidate) => {
    const [left, right] = candidate.split(":").map(Number);
    const [bestLeft, bestRight] = best.split(":").map(Number);
    return Math.abs(Math.log((left / right) / targetRatio)) < Math.abs(Math.log((bestLeft / bestRight) / targetRatio))
      ? candidate
      : best;
  }, "1:1" as (typeof GROK_ASPECT_RATIOS)[number]);
  return { aspect_ratio: aspectRatio, resolution: Math.max(width, height) >= 1536 ? "2k" : "1k" };
}

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
  const provider = normalizeUpstreamProvider(request.payload.provider || "openai");
  const maskB64 = String(request.payload.maskB64 || "").trim();

  if (maskB64 && mode !== "edit") {
    throw new RemoteKernelError("蒙版仅支持图生图模式");
  }

  if (shouldUseGoogleNativeInteractions(baseURL, imageModel, provider)) {
    if (mode === "edit" && sourceDataURLs.length === 0) {
      throw new RemoteKernelError("Google Interactions 图生图需要至少一张源图");
    }
    if (maskB64) {
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

  if (provider === "grok") {
    if (mode === "edit" && sourceDataURLs.length === 0) {
      throw new RemoteKernelError("Grok Imagine 图生图需要至少一张源图");
    }
    if (maskB64) {
      throw new RemoteKernelError("Grok Imagine 不支持 OpenAI mask 参数；请清除蒙版后重试");
    }
    const dimensions = grokImageDimensions(size);
    const payload: Record<string, unknown> = {
      model: imageModel,
      prompt: request.payload.prompt,
      response_format: "b64_json",
      ...(dimensions ?? {}),
    };
    if (mode === "edit") {
      const images = sourceDataURLs.map((url) => ({ type: "image_url", url }));
      payload.image = images.length === 1 ? images[0] : images;
    }
    return {
      url: openAIAPIEndpoint(baseURL, mode === "edit" ? "images/edits" : "images/generations"),
      protocol: "grok-images",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    };
  }

  if (mode === "edit") {
    if (sourceDataURLs.length === 0) {
      throw new RemoteKernelError("图生图模式需要至少一张源图(请先添加参考图)");
    }
    let uploadSourceDataURLs = sourceDataURLs;
    let uploadMaskB64 = maskB64;
    if (maskB64) {
      let prepared = request.preparedMask;
      if (!prepared || prepared.maskB64.trim() !== maskB64) {
        try {
          prepared = await normalizeMaskForSource(sourceDataURLs[0], maskB64);
        } catch (error) {
          throw new RemoteKernelError(`准备蒙版 PNG 失败: ${String((error as any)?.message || error)}`);
        }
      }
      const sourceUpload = assertPreparedPNGDataURL(prepared.sourceUploadDataURL, "蒙版首张源图");
      uploadMaskB64 = assertPreparedPNGBase64(prepared.maskB64, "蒙版图片");
      uploadSourceDataURLs = [sourceUpload, ...sourceDataURLs.slice(1)];
    }

    const form = new FormData();
    for (let i = 0; i < uploadSourceDataURLs.length; i++) {
      const dataURL = uploadSourceDataURLs[i];
      const payload = dataURL.slice(dataURL.indexOf(",") + 1);
      const mimeType = dataURL.slice(5, dataURL.indexOf(";")) || "image/png";
      const ext = imageExtensionForMimeType(mimeType);
      const fieldName = uploadSourceDataURLs.length > 1 ? "image[]" : "image";
      form.append(fieldName, new Blob([Uint8Array.from(atob(payload), (ch) => ch.charCodeAt(0))], { type: mimeType }), `source-${i + 1}.${ext}`);
    }
    if (uploadMaskB64) {
      const maskBytes = decodeCheckedBase64(uploadMaskB64, "蒙版图片");
      form.append("mask", new Blob([maskBytes], { type: "image/png" }), "mask.png");
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

function assertPreparedPNGDataURL(value: string, label: string): string {
  let parsed: { mimeType: string; b64: string };
  try {
    parsed = assertImageDataURLWithinLimit(value, label);
  } catch (error) {
    throw new RemoteKernelError(String((error as any)?.message || error));
  }
  if (parsed.mimeType !== "image/png" || detectImageMimeTypeFromBase64(parsed.b64) !== "image/png") {
    throw new RemoteKernelError(`${label}必须是 PNG`);
  }
  return `data:image/png;base64,${parsed.b64}`;
}

function assertPreparedPNGBase64(value: string, label: string): string {
  let encoded: string;
  try {
    encoded = assertBase64WithinLimit(value, label);
  } catch (error) {
    throw new RemoteKernelError(String((error as any)?.message || error));
  }
  if (detectImageMimeTypeFromBase64(encoded) !== "image/png") {
    throw new RemoteKernelError(`${label}必须是 PNG`);
  }
  return encoded;
}

function decodeCheckedBase64(value: string, label: string): ArrayBuffer {
  const encoded = assertPreparedPNGBase64(value, label);
  try {
    const binary = atob(encoded);
    const bytes = new Uint8Array(binary.length);
    for (let index = 0; index < binary.length; index++) {
      bytes[index] = binary.charCodeAt(index);
    }
    return bytes.buffer;
  } catch {
    throw new RemoteKernelError(`${label} base64 无效`);
  }
}
