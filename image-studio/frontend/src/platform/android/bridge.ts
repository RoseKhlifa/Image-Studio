import { readRuntimePlatformState } from "..";
import { invokeAndroidNative } from "./nativeInvoke";
import { guessImageMimeTypeFromName } from "../../lib/images";

export type AndroidBridge = {
  invoke?: (requestId: string, method: string, payloadJson: string) => void;
  saveImage?: (imageB64: string, suggestedName: string) => string | Promise<string>;
  saveImagePathAs?: (path: string, suggestedName: string) => string | Promise<string>;
  shareImage?: (imageB64: string, suggestedName: string) => string | Promise<string>;
  openOutputDir?: () => string | Promise<string | void>;
  pickImage?: () => string | Promise<string | AndroidPickedImage | null>;
  exportHistory?: (jsonContent: string, suggestedName: string) => string | Promise<string>;
  importHistory?: () => string | Promise<string | null>;
  vibrate?: (ms: number) => void | Promise<void>;
};

type AndroidPickedImage = {
  path?: string;
  name?: string;
  size?: number;
  imageB64?: string;
  mimeType?: string;
};

declare global {
  interface Window {
    AndroidImageStudio?: AndroidBridge;
    Android?: AndroidBridge;
  }
}

function bridge(): AndroidBridge | null {
  if (typeof window === "undefined") return null;
  return window.AndroidImageStudio ?? window.Android ?? null;
}

function byteStringToBlobURL(imageB64: string, type = "image/png"): string {
  const bin = atob(imageB64);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i += 1) bytes[i] = bin.charCodeAt(i);
  return URL.createObjectURL(new Blob([bytes], { type }));
}

function triggerDownload(imageB64: string, suggestedName: string): string {
  const objectURL = byteStringToBlobURL(imageB64, guessImageMimeTypeFromName(suggestedName) || "image/png");
  const anchor = document.createElement("a");
  anchor.href = objectURL;
  anchor.download = suggestedName || "image-studio.png";
  anchor.rel = "noopener";
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  window.setTimeout(() => URL.revokeObjectURL(objectURL), 1000);
  return suggestedName;
}

function ensureImageName(name: string): string {
  const trimmed = name.trim() || "image-studio.png";
  return /\.[a-z0-9]+$/i.test(trimmed) ? trimmed : `${trimmed}.png`;
}

export const androidTarget = {
  get isAndroid() {
    return readRuntimePlatformState().isAndroid;
  },
  get isPad() {
    return readRuntimePlatformState().isAndroidPad;
  },
  get isPhone() {
    return readRuntimePlatformState().isAndroidPhone;
  },
};

export function hasAndroidBridge(): boolean {
  return !!bridge();
}

export async function saveImageForPlatform(
  imageB64: string,
  suggestedName: string,
  desktopSave: (imageB64: string, suggestedName: string) => Promise<string>,
): Promise<string> {
  const filename = ensureImageName(suggestedName);
  const platform = readRuntimePlatformState();
  if (!platform.isAndroid) return desktopSave(imageB64, filename);

  const nativeBridge = bridge();
  if (nativeBridge?.saveImage) {
    const saved = await nativeBridge.saveImage(imageB64, filename);
    return String(saved || filename);
  }

  if (navigator.share && typeof File !== "undefined") {
    try {
      const mimeType = guessImageMimeTypeFromName(filename) || "image/png";
      const blob = await (await fetch(`data:${mimeType};base64,${imageB64}`)).blob();
      const file = new File([blob], filename, { type: mimeType });
      const canShare = !navigator.canShare || navigator.canShare({ files: [file] });
      if (canShare) {
        await navigator.share({ files: [file], title: filename });
        return filename;
      }
    } catch {
      // Fall back to direct download below.
    }
  }

  return triggerDownload(imageB64, filename);
}

export async function openOutputLocationForPlatform(desktopOpen: () => Promise<void>): Promise<void> {
  const platform = readRuntimePlatformState();
  if (!platform.isAndroid) {
    await desktopOpen();
    return;
  }

  const nativeBridge = bridge();
  if (nativeBridge?.openOutputDir) {
    await nativeBridge.openOutputDir();
    return;
  }

  throw new Error(platform.isAndroidPad ? "Android Pad 壳层未提供打开图片目录接口" : "手机版请从系统下载或分享记录里查看保存图片");
}

export async function exportHistoryForPlatform(
  jsonContent: string,
  desktopExport: (jsonContent: string) => Promise<string>,
): Promise<string> {
  if (!readRuntimePlatformState().isAndroid) return desktopExport(jsonContent);
  const suggested = `image-studio-history-${new Date().toISOString().replace(/[:.]/g, "-")}.json`;
  const nativeBridge = bridge();
  if (nativeBridge?.exportHistory) {
    const exported = await nativeBridge.exportHistory(jsonContent, suggested);
    return String(exported || suggested);
  }

  const objectURL = URL.createObjectURL(new Blob([jsonContent], { type: "application/json" }));
  const anchor = document.createElement("a");
  anchor.href = objectURL;
  anchor.download = suggested;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  window.setTimeout(() => URL.revokeObjectURL(objectURL), 1000);
  return suggested;
}

export async function openExternalURLForPlatform(url: string, desktopOpen: (url: string) => Promise<void>): Promise<void> {
  if (!readRuntimePlatformState().isAndroid) {
    await desktopOpen(url);
    return;
  }

  const opened = window.open(url, "_blank", "noopener,noreferrer");
  if (!opened) window.location.href = url;
}

export async function importHistoryForPlatform(desktopImport: () => Promise<string>): Promise<string> {
  if (!readRuntimePlatformState().isAndroid) return desktopImport();
  const nativeBridge = bridge();
  if (nativeBridge?.importHistory) return String((await nativeBridge.importHistory()) || "");
  return "";
}

export function vibrateForPlatform(ms = 50) {
  if (!readRuntimePlatformState().isAndroid) return;
  const nativeBridge = bridge();
  if (nativeBridge?.vibrate) {
    try {
      nativeBridge.vibrate(ms);
    } catch {
      invokeAndroidNative("Vibrate", ms).catch(() => {});
    }
  } else {
    invokeAndroidNative("Vibrate", ms).catch(() => {});
  }
}

export function androidSaveHint(): string {
  const platform = readRuntimePlatformState();
  if (platform.isAndroidPad) return "Pad 版默认保存到相册 ImageStudio 目录;无壳层时会下载或调系统分享面板。";
  if (platform.isAndroidPhone) return "手机版保存后会出现在系统相册 ImageStudio 目录、下载或分享记录中。";
  return "";
}
