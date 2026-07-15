import { invokeAndroidNative } from "../../android/nativeInvoke.ts";
import type { NativeTextResponse } from "./types.ts";

type NativeProgressWindow = Window & {
  __imageStudioNativeProgress?: (requestId: string, payload: unknown) => void;
};

const nativeWSProgressHandlers = new Map<string, (payload: unknown) => void>();
let wsProgressHookInstalled = false;
let wsProgressHookWindow: NativeProgressWindow | null = null;

function ensureAndroidWSProgressHook() {
  if (typeof window === "undefined") return;
  const browserWindow = window as NativeProgressWindow;
  if (wsProgressHookInstalled && wsProgressHookWindow === browserWindow) return;
  const previous = browserWindow.__imageStudioNativeProgress;
  browserWindow.__imageStudioNativeProgress = (requestId, payload) => {
    const handler = nativeWSProgressHandlers.get(requestId);
    if (handler) {
      handler(payload);
      return;
    }
    previous?.(requestId, payload);
  };
  wsProgressHookInstalled = true;
  wsProgressHookWindow = browserWindow;
}

export async function nativeWebSocketResponsesRequest(
  requestKey: string,
  baseURL: string,
  apiKey: string,
  payload: string,
  signal?: AbortSignal,
  onStreamPayload?: (payload: unknown) => void,
  proxyConfig?: { proxyMode?: string; proxyURL?: string; allowInsecureConnection?: boolean },
): Promise<NativeTextResponse> {
  if (signal?.aborted) throw new DOMException("Aborted", "AbortError");
  ensureAndroidWSProgressHook();
  if (onStreamPayload) nativeWSProgressHandlers.set(requestKey, onStreamPayload);
  let aborted = false;
  const onAbort = () => {
    aborted = true;
    void invokeAndroidNative<void>("CancelWebSocketRequest", requestKey).catch(() => undefined);
  };
  signal?.addEventListener("abort", onAbort, { once: true });
  try {
    const response = await invokeAndroidNative<NativeTextResponse>("ResponsesWebSocketRequest", {
      requestKey,
      baseURL,
      apiKey,
      payload,
      proxyMode: proxyConfig?.proxyMode || "system",
      proxyURL: proxyConfig?.proxyURL || "",
      allowInsecureConnection: proxyConfig?.allowInsecureConnection === true,
    });
    if (aborted) throw new DOMException("Aborted", "AbortError");
    return response;
  } finally {
    nativeWSProgressHandlers.delete(requestKey);
    signal?.removeEventListener("abort", onAbort);
  }
}
