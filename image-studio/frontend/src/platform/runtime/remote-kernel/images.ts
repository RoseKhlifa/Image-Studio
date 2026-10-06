import { detectImageMimeTypeFromBase64 } from "../../../lib/images.ts";
import { buildImagesRequestBody, type ImagesRequestProtocol } from "./requestPayloads.ts";
import {
  nowSeconds,
  registerRawText,
  resolveSourceDataURLs,
  shouldUseAndroidNativeHTTP,
} from "./common.ts";
import { nativeHttpRequestText } from "./nativeHttp.ts";
import {
  RemoteKernelError,
  STATUS_INTERVAL_MS,
  type ExtractedImageResult,
  type RemoteJobCallbacks,
  type RemoteJobRequest,
  type RemoteJobResult,
} from "./types.ts";

const MAX_IMAGE_URL_BYTES = 50 * 1024 * 1024;

function parseStreamEvent(line: string, allowRawJSON = false): any | null {
  const stripped = line.trim();
  const payload = stripped.startsWith("data:")
    ? stripped.slice("data:".length).trim()
    : allowRawJSON ? stripped : "";
  if (!payload || payload === "[DONE]") return null;
  try {
    return JSON.parse(payload);
  } catch {
    return null;
  }
}

function parseNativeProgressPayload(payload: unknown, allowRawJSON = false): { line: string; event: any | null } {
  if (typeof payload === "string") {
    return { line: payload, event: parseStreamEvent(payload, allowRawJSON) };
  }
  if (!payload || typeof payload !== "object") {
    return { line: "", event: null };
  }
  const line = typeof (payload as { line?: unknown }).line === "string"
    ? (payload as { line: string }).line
    : "";
  const structured = (payload as { event?: unknown }).event;
  const event = structured && typeof structured === "object"
    ? structured
    : parseStreamEvent(line, allowRawJSON);
  return { line, event };
}

function parseImagesStreamEvent(
  event: any,
  callbacks: RemoteJobCallbacks,
): ExtractedImageResult | null {
  const type = event?.type;
  if (type === "image_generation.partial_image" || type === "image_edit.partial_image") {
    if (event.b64_json) {
      callbacks.onPartialImage?.({
        imageB64: event.b64_json,
        partialImageIndex: typeof event.partial_image_index === "number" ? event.partial_image_index : undefined,
        sourceEvent: "images_partial",
      });
    }
    return null;
  }
  if (type === "image_generation.completed" || type === "image_edit.completed") {
    if (event.b64_json) {
      return {
        imageB64: event.b64_json,
        revisedPrompt: "",
        sourceEvent: "images_api",
      };
    }
  }
  if (event?.object === "image.generation.result" || event?.object === "image.edit.result") {
    return parseImagesResponseSync(JSON.stringify(event), 200);
  }
  const first = Array.isArray(event?.data) ? event.data[0] : null;
  if (first?.b64_json) {
    return {
      imageB64: first.b64_json,
      revisedPrompt: first.revised_prompt || "",
      sourceEvent: "images_api",
    };
  }
  return null;
}

function parseImagesResponseSync(raw: string, status: number): ExtractedImageResult {
  let parsed: any;
  try {
    parsed = JSON.parse(raw);
  } catch (error) {
    if (status >= 400) {
      throw new RemoteKernelError(`上游返回 HTTP ${status}: ${raw.slice(0, 400)}`);
    }
    throw new RemoteKernelError(`解析 Images API 响应失败:${(error as any)?.message || error}`);
  }
  if (status >= 400) {
    if (parsed?.error?.message) {
      throw new RemoteKernelError(`上游返回 ${status}:${parsed.error.message}`);
    }
    throw new RemoteKernelError(`上游返回 HTTP ${status}`);
  }
  if (parsed?.error?.message) {
    throw new RemoteKernelError(`上游返回错误:${parsed.error.message}`);
  }
  const first = Array.isArray(parsed?.data) ? parsed.data[0] : null;
  if (!first?.b64_json) {
    if (first?.url) {
      throw new RemoteKernelError("上游返回 URL 而非 b64_json，当前路径需要下载 URL 图片");
    }
    throw new RemoteKernelError("上游没有返回可用图片");
  }
  return {
    imageB64: first.b64_json,
    revisedPrompt: first.revised_prompt || "",
    sourceEvent: "images_api",
  };
}

function ensureSupportedImageBase64(imageB64: string, source: string): string {
  if (!imageB64 || !detectImageMimeTypeFromBase64(imageB64)) {
    throw new RemoteKernelError(`${source}没有返回支持的 PNG/JPEG/WebP 图片`);
  }
  return imageB64;
}

async function imageURLToBase64(
  rawURL: string,
  signal: AbortSignal,
  proxyMode = "system",
  proxyURL = "",
  allowInsecureConnection = false,
): Promise<string> {
  let parsed: URL;
  try {
    parsed = new URL(rawURL);
  } catch {
    throw new RemoteKernelError(`上游返回的图片 URL 无效:${rawURL}`);
  }
  if (parsed.protocol !== "https:" && parsed.protocol !== "http:") {
    throw new RemoteKernelError(`上游返回的图片 URL 协议不支持:${parsed.protocol.replace(":", "")}`);
  }
  if (shouldUseAndroidNativeHTTP()) {
    const native = await nativeHttpRequestText(
      parsed.toString(),
      "GET",
      { Accept: "image/png, image/jpeg, image/webp, */*" },
      null,
      signal,
      undefined,
      {
        proxyMode,
        proxyURL,
        responseBase64: true,
        maxResponseBytes: MAX_IMAGE_URL_BYTES,
        allowInsecureConnection,
      },
    );
    if (native.status < 200 || native.status >= 300) {
      throw new RemoteKernelError(`下载上游 URL 图片返回 HTTP ${native.status}`);
    }
    return ensureSupportedImageBase64(native.resultImageB64 || "", "上游 URL");
  }
  const response = await fetch(parsed.toString(), {
    method: "GET",
    headers: { Accept: "image/png, image/jpeg, image/webp, */*" },
    signal,
  });
  if (!response.ok) {
    throw new RemoteKernelError(`下载上游 URL 图片返回 HTTP ${response.status}`);
  }
  const blob = await response.blob();
  if (blob.size > MAX_IMAGE_URL_BYTES) {
    throw new RemoteKernelError(`上游 URL 图片过大(${blob.size}B > ${MAX_IMAGE_URL_BYTES}B 上限)`);
  }
  if (blob.type && !["image/png", "image/jpeg", "image/webp"].includes(blob.type.toLowerCase())) {
    throw new RemoteKernelError(`上游 URL 图片类型不支持:${blob.type}`);
  }
  const buffer = await blob.arrayBuffer();
  const bytes = new Uint8Array(buffer);
  let binary = "";
  const chunk = 0x8000;
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk));
  }
  return ensureSupportedImageBase64(btoa(binary), "上游 URL");
}

async function parseGoogleInteractionResponse(
  raw: string,
  status: number,
  signal: AbortSignal,
  proxyMode: string,
  proxyURL: string,
  allowInsecureConnection: boolean,
): Promise<ExtractedImageResult> {
  let parsed: any;
  try {
    parsed = JSON.parse(raw);
  } catch (error) {
    if (status >= 400) throw new RemoteKernelError(`Google Interactions 返回 HTTP ${status}: ${raw.slice(0, 400)}`);
    throw new RemoteKernelError(`解析 Google Interactions 响应失败:${(error as any)?.message || error}`);
  }
  if (status >= 400 || parsed?.error) {
    const message = parsed?.error?.message || parsed?.message || `HTTP ${status}`;
    throw new RemoteKernelError(`Google Interactions 返回错误:${message}`);
  }

  const candidates: any[] = [];
  if (parsed?.output_image) candidates.push(parsed.output_image);
  if (Array.isArray(parsed?.steps)) {
    for (let stepIndex = parsed.steps.length - 1; stepIndex >= 0; stepIndex--) {
      const content = parsed.steps[stepIndex]?.content;
      if (!Array.isArray(content)) continue;
      for (let contentIndex = content.length - 1; contentIndex >= 0; contentIndex--) {
        if (content[contentIndex]?.type === "image") candidates.push(content[contentIndex]);
      }
    }
  }
  for (const image of candidates) {
    if (typeof image?.data === "string" && image.data.trim()) {
      return {
        imageB64: ensureSupportedImageBase64(image.data.trim(), "Google Interactions"),
        revisedPrompt: "",
        sourceEvent: "google_interactions",
      };
    }
    if (typeof image?.uri === "string" && image.uri.trim()) {
      return {
        imageB64: await imageURLToBase64(image.uri, signal, proxyMode, proxyURL, allowInsecureConnection),
        revisedPrompt: "",
        sourceEvent: "google_interactions_url",
      };
    }
  }
  const text = Array.isArray(parsed?.steps)
    ? parsed.steps.flatMap((step: any) => Array.isArray(step?.content) ? step.content : [])
      .find((content: any) => content?.type === "text" && typeof content?.text === "string")?.text
    : "";
  const suffix = text ? `:${String(text).slice(0, 240)}` : "";
  throw new RemoteKernelError(`Google Interactions 响应未包含 image data/uri${suffix}`);
}

async function parseImagesResponse(
  raw: string,
  status: number,
  signal: AbortSignal,
  protocol: ImagesRequestProtocol,
  proxyMode: string,
  proxyURL: string,
  allowInsecureConnection: boolean,
  allowEmptyKeepAlive = false,
): Promise<ExtractedImageResult> {
  if (protocol === "google-interactions") {
    return parseGoogleInteractionResponse(raw, status, signal, proxyMode, proxyURL, allowInsecureConnection);
  }
  let parsedValues: any[];
  try {
    parsedValues = [JSON.parse(raw)];
  } catch (error) {
    if (!allowEmptyKeepAlive) return parseImagesResponseSync(raw, status);
    parsedValues = raw
      .split(/\r?\n/)
      .map((line) => parseStreamEvent(line, true))
      .filter((value) => value !== null);
    if (parsedValues.length === 0 && raw.trim()) {
      throw new RemoteKernelError(`解析 Images API 响应失败:${(error as any)?.message || error}`);
    }
  }
  for (const parsed of parsedValues) {
    if (status >= 400) {
      if (parsed?.error?.message) {
        throw new RemoteKernelError(`上游返回 ${status}:${parsed.error.message}`);
      }
      continue;
    }
    if (parsed?.error?.message) {
      throw new RemoteKernelError(`上游返回错误:${parsed.error.message}`);
    }
    const first = Array.isArray(parsed?.data) ? parsed.data[0] : null;
    if (first?.b64_json) {
      return {
        imageB64: first.b64_json,
        revisedPrompt: first.revised_prompt || "",
        sourceEvent: "images_api",
      };
    }
    if (first?.url) {
      return {
        imageB64: await imageURLToBase64(first.url, signal, proxyMode, proxyURL, allowInsecureConnection),
        revisedPrompt: first.revised_prompt || "",
        sourceEvent: "images_api_url",
      };
    }
  }
  if (status >= 400) throw new RemoteKernelError(`上游返回 HTTP ${status}`);
  throw new RemoteKernelError("上游没有返回可用图片");
}

function parseImagesStreamRaw(
  raw: string,
  callbacks: RemoteJobCallbacks,
  emitPartials = false,
  allowRawJSON = false,
): ExtractedImageResult | null {
  const partialCallbacks = emitPartials ? callbacks : { signal: callbacks.signal };
  for (const line of raw.split(/\r?\n/)) {
    const event = parseStreamEvent(line, allowRawJSON);
    if (!event) continue;
    const result = parseImagesStreamEvent(event, partialCallbacks);
    if (result) return result;
  }
  return null;
}

export async function requestImagesOnce(
  request: RemoteJobRequest,
  attempt: number,
  maxAttempts: number,
  callbacks: RemoteJobCallbacks,
): Promise<RemoteJobResult> {
  const sourceDataURLs = await resolveSourceDataURLs(request.sourceImages, request.payload);
  const built = await buildImagesRequestBody(request, sourceDataURLs);
  const allowEmptyKeepAlive = request.payload.imagesNewAPICompat === true;
  const startedAt = Date.now();
  const protocolLabel = built.protocol === "google-interactions" ? "Google Interactions" : built.protocol === "grok-images" ? "Grok Imagine" : "Images API";
  callbacks.onLog?.(`[${protocolLabel}] 第 ${attempt}/${maxAttempts} 次请求...`);
  callbacks.onProgress?.(`等待 ${protocolLabel} 返回(无 SSE 保活)`, 0, 0);
  const ticker = globalThis.setInterval(() => {
    callbacks.onProgress?.(`等待 ${protocolLabel} 返回(无 SSE 保活)`, nowSeconds(startedAt), 0);
  }, STATUS_INTERVAL_MS);
  try {
    const proxyMode = request.payload.proxyMode === "none" || request.payload.proxyMode === "custom" ? request.payload.proxyMode : "system";
    if (shouldUseAndroidNativeHTTP()) {
      let rawFromLines = "";
      let nativeStreamResult: ExtractedImageResult | null = null;
      let nativeBytesReceived = 0;
      const consumeNativePayload = (payload: unknown) => {
        const parsedPayload = parseNativeProgressPayload(payload, allowEmptyKeepAlive);
        if (parsedPayload.line) {
          rawFromLines += `${parsedPayload.line}\n`;
          nativeBytesReceived += parsedPayload.line.length + 1;
        }
        const parsed = parsedPayload.event ? parseImagesStreamEvent(parsedPayload.event, callbacks) : null;
        if (parsed) nativeStreamResult = parsed;
        callbacks.onProgress?.("已收到 Images API 流式事件", nowSeconds(startedAt), nativeBytesReceived);
      };
      const response = await nativeHttpRequestText(
        built.url,
        "POST",
        {
          ...(built.protocol !== "google-interactions" ? { Authorization: `Bearer ${request.payload.apiKey}` } : {}),
          Accept: "text/event-stream, application/json",
          ...(built.headers ?? {}),
        },
        built.body,
        callbacks.signal,
        built.protocol === "openai-images" ? consumeNativePayload : undefined,
        {
          proxyMode,
          proxyURL: request.payload.proxyURL || "",
          keepAlive: true,
          allowInsecureConnection: request.payload.allowInsecureConnection === true,
        },
      );
      const rawBody = response.body || rawFromLines;
      const rawPath = response.rawPath || registerRawText("images", attempt, rawBody);
      const isStream = String(response.contentType || "").toLowerCase().includes("text/event-stream");
      if (response.resultImageB64) {
        return {
          imageB64: response.resultImageB64,
          revisedPrompt: response.revisedPrompt || "",
          sourceEvent: response.sourceEvent || "images_api",
          rawPath,
          prompt: request.payload.prompt,
          mode: request.payload.mode,
        };
      }
      let result = isStream
        ? nativeStreamResult ?? parseImagesStreamRaw(rawBody, callbacks, false, allowEmptyKeepAlive)
        : await parseImagesResponse(
            rawBody,
            response.status,
            callbacks.signal,
            built.protocol,
            proxyMode,
            request.payload.proxyURL || "",
            request.payload.allowInsecureConnection === true,
            allowEmptyKeepAlive,
          );
      if (!result && isStream && allowEmptyKeepAlive) {
        result = await parseImagesResponse(
          rawBody,
          response.status,
          callbacks.signal,
          built.protocol,
          proxyMode,
          request.payload.proxyURL || "",
          request.payload.allowInsecureConnection === true,
          true,
        );
      }
      if (!result) throw new RemoteKernelError("上游没有返回可用图片", rawPath);
      return { ...result, rawPath, prompt: request.payload.prompt, mode: request.payload.mode };
    }
    if (proxyMode !== "system") {
      throw new RemoteKernelError("当前远程内核不能控制代理,请切回本地内核或使用 Android 原生运行");
    }
    const response = await fetch(built.url, {
      method: "POST",
      headers: {
        ...(built.protocol !== "google-interactions" ? { Authorization: `Bearer ${request.payload.apiKey}` } : {}),
        Accept: "text/event-stream, application/json",
        ...(built.headers ?? {}),
      },
      body: built.body,
      signal: callbacks.signal,
    });
    const contentType = response.headers.get("content-type")?.toLowerCase() || "";
    if (response.body && contentType.includes("text/event-stream")) {
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let raw = "";
      let pending = "";
      let result: ExtractedImageResult | null = null;
      let bytesReceived = 0;
      try {
        while (true) {
          const { value, done } = await reader.read();
          if (done) break;
          bytesReceived += value.byteLength;
          const chunk = decoder.decode(value, { stream: true });
          raw += chunk;
          pending += chunk;
          let newline = pending.indexOf("\n");
          while (newline >= 0) {
            const line = pending.slice(0, newline).replace(/\r$/, "");
            pending = pending.slice(newline + 1);
            const event = parseStreamEvent(line, allowEmptyKeepAlive);
            const parsed = event ? parseImagesStreamEvent(event, callbacks) : null;
            if (parsed) result = parsed;
            callbacks.onProgress?.("已收到 Images API 流式事件", nowSeconds(startedAt), bytesReceived);
            newline = pending.indexOf("\n");
          }
        }
        raw += decoder.decode();
        if (pending.trim()) {
          const event = parseStreamEvent(pending, allowEmptyKeepAlive);
          const parsed = event ? parseImagesStreamEvent(event, callbacks) : null;
          if (parsed) result = parsed;
        }
      } catch (error) {
        const fallback = parseImagesStreamRaw(raw, callbacks, false, allowEmptyKeepAlive);
        if (fallback?.imageB64) {
          const rawPath = registerRawText("images", attempt, raw);
          return { ...fallback, rawPath, prompt: request.payload.prompt, mode: request.payload.mode };
        }
        throw error;
      }
      const rawPath = registerRawText("images", attempt, raw);
      if (!response.ok) {
        throw new RemoteKernelError(`上游返回 HTTP ${response.status}`, rawPath);
      }
      result ??= parseImagesStreamRaw(raw, callbacks, false, allowEmptyKeepAlive);
      if (!result && allowEmptyKeepAlive) {
        result = await parseImagesResponse(
          raw,
          response.status,
          callbacks.signal,
          built.protocol,
          proxyMode,
          request.payload.proxyURL || "",
          request.payload.allowInsecureConnection === true,
          true,
        );
      }
      if (!result) throw new RemoteKernelError("上游没有返回可用图片", rawPath);
      return { ...result, rawPath, prompt: request.payload.prompt, mode: request.payload.mode };
    }
    const raw = await response.text();
    const rawPath = registerRawText("images", attempt, raw);
    const result = await parseImagesResponse(
      raw,
      response.status,
      callbacks.signal,
      built.protocol,
      proxyMode,
      request.payload.proxyURL || "",
      request.payload.allowInsecureConnection === true,
      allowEmptyKeepAlive,
    );
    return { ...result, rawPath, prompt: request.payload.prompt, mode: request.payload.mode };
  } catch (error) {
    if (error instanceof RemoteKernelError) throw error;
    throw new RemoteKernelError(String((error as any)?.message || error));
  } finally {
    globalThis.clearInterval(ticker);
  }
}
