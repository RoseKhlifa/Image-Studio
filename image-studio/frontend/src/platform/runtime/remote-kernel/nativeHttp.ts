import { invokeAndroidNative } from "../../android/nativeInvoke.ts";
import type { NativeTextResponse } from "./types.ts";

type NativeProgressWindow = Window & {
  __imageStudioNativeProgress?: (requestId: string, payload: unknown) => void;
};

export type NativeHTTPProxyConfig = {
  proxyMode?: string;
  proxyURL?: string;
  keepAlive?: boolean;
  responseBase64?: boolean;
  maxResponseBytes?: number;
  allowInsecureConnection?: boolean;
};

const nativeHttpProgressHandlers = new Map<string, (payload: unknown) => void>();
let progressHookInstalled = false;
let progressHookWindow: NativeProgressWindow | null = null;

function bytesToBase64(bytes: Uint8Array): string {
  let binary = "";
  const chunkSize = 0x8000;
  for (let i = 0; i < bytes.length; i += chunkSize) {
    const chunk = bytes.subarray(i, i + chunkSize);
    binary += String.fromCharCode(...chunk);
  }
  return btoa(binary);
}

async function encodeRequestBody(
  body: BodyInit | null | undefined,
  headers?: Record<string, string>,
): Promise<{ bodyBase64: string; contentType: string }> {
  if (!body) {
    return { bodyBase64: "", contentType: headers?.["Content-Type"] || headers?.["content-type"] || "" };
  }
  if (typeof body === "string") {
    const bytes = new TextEncoder().encode(body);
    return {
      bodyBase64: bytesToBase64(bytes),
      contentType: headers?.["Content-Type"] || headers?.["content-type"] || "",
    };
  }
  const request = new Request("https://native-request.invalid", {
    method: "POST",
    headers,
    body,
  });
  const buffer = await request.arrayBuffer();
  return {
    bodyBase64: bytesToBase64(new Uint8Array(buffer)),
    contentType: request.headers.get("content-type") || headers?.["Content-Type"] || headers?.["content-type"] || "",
  };
}

function ensureAndroidProgressHook() {
  if (typeof window === "undefined") return;
  const browserWindow = window as NativeProgressWindow;
  if (progressHookInstalled && progressHookWindow === browserWindow) return;
  const previous = browserWindow.__imageStudioNativeProgress;
  browserWindow.__imageStudioNativeProgress = (requestId, payload) => {
    const handler = nativeHttpProgressHandlers.get(requestId);
    if (handler) {
      handler(payload);
      return;
    }
    previous?.(requestId, payload);
  };
  progressHookInstalled = true;
  progressHookWindow = browserWindow;
}

export async function nativeHttpRequestText(
  url: string,
  method: string,
  headers: Record<string, string>,
  body: BodyInit | null | undefined,
  signal?: AbortSignal,
  onStreamPayload?: (payload: unknown) => void,
  proxyConfig?: NativeHTTPProxyConfig,
): Promise<NativeTextResponse> {
  const requestKey = `native-http-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
  const encoded = await encodeRequestBody(body, headers);
  if (signal?.aborted) throw new DOMException("Aborted", "AbortError");
  ensureAndroidProgressHook();
  if (onStreamPayload) {
    nativeHttpProgressHandlers.set(requestKey, onStreamPayload);
  }
  let aborted = false;
  const onAbort = () => {
    aborted = true;
    void invokeAndroidNative<void>("CancelHttpRequest", requestKey).catch(() => undefined);
  };
  signal?.addEventListener("abort", onAbort, { once: true });
  try {
    const response = await invokeAndroidNative<NativeTextResponse>("HttpRequestText", {
      requestKey,
      url,
      method,
      headers,
      bodyBase64: encoded.bodyBase64,
      contentType: encoded.contentType,
      streamLines: Boolean(onStreamPayload),
      proxyMode: proxyConfig?.proxyMode || "system",
      proxyURL: proxyConfig?.proxyURL || "",
      keepAlive: Boolean(proxyConfig?.keepAlive),
      responseBase64: Boolean(proxyConfig?.responseBase64),
      maxResponseBytes: proxyConfig?.maxResponseBytes || 0,
      allowInsecureConnection: proxyConfig?.allowInsecureConnection === true,
    });
    if (aborted) throw new DOMException("Aborted", "AbortError");
    return response;
  } finally {
    nativeHttpProgressHandlers.delete(requestKey);
    signal?.removeEventListener("abort", onAbort);
  }
}
