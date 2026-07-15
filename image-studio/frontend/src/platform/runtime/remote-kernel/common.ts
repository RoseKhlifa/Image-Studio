import {
  readVirtualText,
  registerVirtualText,
  sourceToDataURL,
} from "../../../lib/virtualHostStore.ts";
import { hasAndroidInvokeBridge } from "../../android/nativeInvoke.ts";
import {
  describeProblem as describeSharedProblem,
  isRetryableRaw as isRetryableRawShared,
  normalizeAPIMode as normalizeSharedAPIMode,
  normalizeBaseURL as normalizeSharedBaseURL,
  normalizeImageModel as normalizeSharedImageModel,
  normalizeTextModel as normalizeSharedTextModel,
} from "../../../../../../shared/kernel/requestModel.js";
import type { KernelImageSource, RemoteGeneratePayload } from "./types.ts";

export function nowSeconds(startedAt: number): number {
  return Math.max(0, Math.floor((Date.now() - startedAt) / 1000));
}

export function fileNameFromPath(path: string | undefined): string {
  if (!path) return "image.png";
  return path.split(/[\\/]/).pop() || "image.png";
}

export async function resolveSourceDataURLs(
  sourceImages: KernelImageSource[] | undefined,
  payload: RemoteGeneratePayload,
): Promise<string[]> {
  const ordered = sourceImages?.length
    ? sourceImages
    : payload.imagePaths.map((path) => ({ path, name: fileNameFromPath(path) }));
  const out: string[] = [];
  for (const source of ordered) {
    const dataURL = await sourceToDataURL(source);
    if (dataURL) out.push(dataURL);
  }
  return out;
}

export function normalizeBaseURL(raw: string): string {
  return normalizeSharedBaseURL(raw);
}

export function validateRemoteBaseURL(raw: string, allowInsecureConnection: boolean): string {
  const normalized = normalizeBaseURL(raw);
  let parsed: URL;
  try {
    parsed = new URL(normalized);
  } catch {
    throw new Error("BASE_URL 必须包含协议和主机，例如 https://example.com");
  }
  if (parsed.protocol === "https:") return normalized;
  if (parsed.protocol !== "http:") {
    throw new Error("BASE_URL 仅支持 http:// 或 https://");
  }
  const host = parsed.hostname.toLowerCase().replace(/^\[|\]$/g, "");
  const loopback = host === "localhost"
    || host.endsWith(".localhost")
    || host === "127.0.0.1"
    || host === "::1"
    || host === "0:0:0:0:0:0:0:1";
  if (loopback || allowInsecureConnection) return normalized;
  throw new Error(`拒绝使用非 TLS 上游: ${normalized}。请在渠道中开启“允许不安全连接”`);
}

export function normalizeAPIMode(apiMode: string): "responses" | "images" {
  return normalizeSharedAPIMode(apiMode);
}

export function normalizeTextModel(modelID: string): string {
  return normalizeSharedTextModel(modelID);
}

export function normalizeImageModel(modelID: string): string {
  return normalizeSharedImageModel(modelID);
}

export function shouldUseAndroidNativeHTTP(): boolean {
  return typeof window !== "undefined" && hasAndroidInvokeBridge();
}

export const describeProblem = describeSharedProblem;
export const isRetryableRaw = isRetryableRawShared;

export function isTransportishError(error: unknown): boolean {
  const message = String((error as any)?.message || error || "").toLowerCase();
  return [
    "timeout",
    "networkerror",
    "network error",
    "failed to fetch",
    "load failed",
    "i/o timeout",
    "connection reset",
    "econnreset",
    "econnrefused",
    "gateway",
    "只返回了流式预览帧",
  ].some((marker) => message.includes(marker));
}

export async function sleepWithSignal(signal: AbortSignal, ms: number): Promise<void> {
  if (signal.aborted) throw new DOMException("Aborted", "AbortError");
  await new Promise<void>((resolve, reject) => {
    const timer = globalThis.setTimeout(() => {
      cleanup();
      resolve();
    }, ms);
    const onAbort = () => {
      cleanup();
      reject(new DOMException("Aborted", "AbortError"));
    };
    const cleanup = () => {
      globalThis.clearTimeout(timer);
      signal.removeEventListener("abort", onAbort);
    };
    signal.addEventListener("abort", onAbort, { once: true });
  });
}

export function registerRawText(kind: "responses" | "images" | "optimize", attempt: number, raw: string): string | null {
  if (!raw.trim()) return null;
  const ext = kind === "responses" ? "txt" : "json";
  return registerVirtualText(raw, `${kind}-response-attempt${attempt}.${ext}`);
}

export function readRegisteredText(path: string): string {
  return readVirtualText(path);
}

export function extractResponseText(raw: string): string {
  try {
    const parsed: any = JSON.parse(raw);
    if (typeof parsed?.output_text === "string" && parsed.output_text.trim()) {
      return parsed.output_text.trim();
    }
    if (Array.isArray(parsed?.output)) {
      for (const output of parsed.output) {
        if (!Array.isArray(output?.content)) continue;
        for (const content of output.content) {
          if (content?.type === "output_text" && typeof content?.text === "string" && content.text.trim()) {
            return content.text.trim();
          }
        }
      }
    }
  } catch {
    // ignore
  }
  return "";
}

export function extractResponseErrorMessage(raw: string): string {
  try {
    const parsed: any = JSON.parse(raw);
    if (typeof parsed?.error?.message === "string" && parsed.error.message.trim()) {
      return parsed.error.message.trim();
    }
    if (typeof parsed?.message === "string" && parsed.message.trim()) {
      return parsed.message.trim();
    }
  } catch {
    // ignore
  }
  return raw.trim();
}

export { sourceToDataURL };
