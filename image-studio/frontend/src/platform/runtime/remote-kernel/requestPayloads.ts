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
  openAIAPIEndpoint,
  shouldSendExtendedImageParameters,
  supportsImageBackground,
  supportsImageStyle,
  supportsInputFidelity,
  supportsImageModeration,
  supportsOutputCompression,
  shouldUseImagesNewAPICompat,
  supportsImagesResponseFormat,
} from "../../../../../../shared/kernel/requestModel.js";
import { normalizeBaseURL, normalizeImageModel } from "./common.ts";
import { RemoteKernelError, type RemoteGeneratePayload, type RemoteJobRequest } from "./types.ts";

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
): Promise<{ url: string; headers?: Record<string, string>; body: BodyInit }> {
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
      const maskMime = detectImageMimeTypeFromBase64(request.payload.maskB64) || "image/png";
      const ext = imageExtensionForMimeType(maskMime);
      form.append("mask", new Blob([Uint8Array.from(atob(request.payload.maskB64), (ch) => ch.charCodeAt(0))], { type: maskMime }), `mask.${ext}`);
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
    return { url: openAIAPIEndpoint(baseURL, "images/edits"), body: form };
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
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  };
}
