import { useEffect, useMemo, useState } from "react";

export function base64ToBlob(b64: string, mimeType = "image/png"): Blob {
  const bin = atob(b64);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  return new Blob([bytes], { type: mimeType });
}

export async function blobToBase64(blob: Blob): Promise<string> {
  const buf = await blob.arrayBuffer();
  let binary = "";
  const bytes = new Uint8Array(buf);
  const chunk = 0x8000;
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk));
  }
  return btoa(binary);
}

export function blobToObjectURL(blob: Blob): string {
  return URL.createObjectURL(blob);
}

const objectURLCache = new WeakMap<Blob, { url: string; refs: number }>();

export function acquireBlobObjectURL(blob: Blob): string {
  const cached = objectURLCache.get(blob);
  if (cached) {
    cached.refs += 1;
    return cached.url;
  }
  const url = URL.createObjectURL(blob);
  objectURLCache.set(blob, { url, refs: 1 });
  return url;
}

export function releaseBlobObjectURL(blob: Blob): void {
  const cached = objectURLCache.get(blob);
  if (!cached) return;
  cached.refs -= 1;
  if (cached.refs > 0) return;
  URL.revokeObjectURL(cached.url);
  objectURLCache.delete(blob);
}

export function detectImageMimeTypeFromBase64(b64: string): string | null {
  try {
    const bin = atob(b64.slice(0, 64));
    const bytes = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
    if (
      bytes.length >= 8 &&
      bytes[0] === 0x89 &&
      bytes[1] === 0x50 &&
      bytes[2] === 0x4e &&
      bytes[3] === 0x47 &&
      bytes[4] === 0x0d &&
      bytes[5] === 0x0a &&
      bytes[6] === 0x1a &&
      bytes[7] === 0x0a
    ) {
      return "image/png";
    }
    if (bytes.length >= 3 && bytes[0] === 0xff && bytes[1] === 0xd8 && bytes[2] === 0xff) {
      return "image/jpeg";
    }
    if (
      bytes.length >= 12 &&
      bytes[0] === 0x52 &&
      bytes[1] === 0x49 &&
      bytes[2] === 0x46 &&
      bytes[3] === 0x46 &&
      bytes[8] === 0x57 &&
      bytes[9] === 0x45 &&
      bytes[10] === 0x42 &&
      bytes[11] === 0x50
    ) {
      return "image/webp";
    }
  } catch {
    // ignore
  }
  return null;
}

export function guessImageMimeTypeFromName(name: string | null | undefined): string | null {
  const lower = (name ?? "").trim().toLowerCase();
  if (lower.endsWith(".png")) return "image/png";
  if (lower.endsWith(".jpg") || lower.endsWith(".jpeg")) return "image/jpeg";
  if (lower.endsWith(".webp")) return "image/webp";
  return null;
}

export function imageExtensionForMimeType(mimeType: string | null | undefined): string {
  switch ((mimeType ?? "").trim().toLowerCase()) {
    case "image/jpeg":
      return "jpg";
    case "image/webp":
      return "webp";
    default:
      return "png";
  }
}

export function dataURLFromBase64(
  b64: string,
  mimeType?: string | null,
): string {
  const detected = mimeType || detectImageMimeTypeFromBase64(b64) || "image/png";
  return `data:${detected};base64,${b64}`;
}

export function useBlobURL(blob?: Blob | null, fallbackB64?: string | null): string | null {
  const stableFallbackBlob = useMemo(() => {
    if (blob || !fallbackB64) return null;
    try {
      return base64ToBlob(fallbackB64);
    } catch {
      return null;
    }
  }, [blob, fallbackB64]);
  const [url, setUrl] = useState<string | null>(null);

  useEffect(() => {
    if (blob) {
      const objectURL = acquireBlobObjectURL(blob);
      setUrl(objectURL);
      return () => releaseBlobObjectURL(blob);
    }

    if (stableFallbackBlob) {
      const objectURL = acquireBlobObjectURL(stableFallbackBlob);
      setUrl(objectURL);
      return () => releaseBlobObjectURL(stableFallbackBlob);
    }

    setUrl(null);
  }, [blob, stableFallbackBlob]);

  return url;
}

export function historyPreviewSrc(
  item: {
    id?: string | null;
    previewUrl?: string | null;
    imageBlob?: Blob | null;
    previewBlob?: Blob | null;
    imageB64?: string | null;
  } | null | undefined,
  objectURL: string | null,
): string {
  if (!item) return "";
  return item.previewUrl || objectURL || (item.imageB64 ? dataURLFromBase64(item.imageB64) : "");
}

export function useImageLoadState(src: string | null | undefined): "idle" | "loading" | "ready" | "error" {
  const [state, setState] = useState<"idle" | "loading" | "ready" | "error">(() => src ? "loading" : "idle");

  useEffect(() => {
    if (!src) {
      setState("idle");
      return;
    }
    setState("loading");
    const image = new Image();
    image.onload = () => setState("ready");
    image.onerror = () => setState("error");
    image.src = src;
    return () => {
      image.onload = null;
      image.onerror = null;
    };
  }, [src]);

  return state;
}

export function mediaFullUrlFromImageId(imageId?: string | null): string {
  return imageId ? `/media/full/${imageId}` : "";
}

export function sortHistoryItemsByCreatedAtAsc<T extends { createdAt: number; id: string }>(items: T[]): T[] {
  return [...items].sort((a, b) => a.createdAt - b.createdAt || a.id.localeCompare(b.id));
}

export function orderedNavigationItemsForCurrent<T extends { createdAt: number; id: string }>(
  currentId: string | null | undefined,
  historyItems: T[],
  batchItems: T[],
): T[] {
  const orderedBatchItems = sortHistoryItemsByCreatedAtAsc(batchItems);
  if (currentId && orderedBatchItems.some((item) => item.id === currentId)) return orderedBatchItems;

  const orderedHistoryItems = sortHistoryItemsByCreatedAtAsc(historyItems);
  if (currentId && orderedHistoryItems.some((item) => item.id === currentId)) return orderedHistoryItems;

  return orderedBatchItems.length > 1 ? orderedBatchItems : orderedHistoryItems;
}

export function isTransientPreviewItem(
  item: {
    id?: string | null;
    previewOnly?: boolean | null;
  } | null | undefined,
): boolean {
  return !!item?.previewOnly && typeof item.id === "string" && item.id.startsWith("preview-");
}

export function historyFullSrc(
  item: {
    id?: string | null;
    imageId?: string | null;
    fullUrl?: string | null;
    previewUrl?: string | null;
    imageB64?: string | null;
    previewOnly?: boolean | null;
  } | null | undefined,
  objectURL: string | null,
): string {
  if (!item) return "";
  const mediaFullUrl = isTransientPreviewItem(item) ? "" : mediaFullUrlFromImageId(item.imageId);
  return item.fullUrl || mediaFullUrl || objectURL || item.previewUrl || (item.imageB64 ? dataURLFromBase64(item.imageB64) : "");
}

export function useImageElement(source?: Blob | string | null): HTMLImageElement | null {
  const url = useBlobURL(source instanceof Blob ? source : null, typeof source === "string" ? source : null);
  const [img, setImg] = useState<HTMLImageElement | null>(null);

  useEffect(() => {
    if (!url) { setImg(null); return; }
    setImg(null);
    const el = new Image();
    el.onload = () => setImg(el);
    el.onerror = () => setImg(null);
    el.src = url;
    return () => {
      el.onload = null;
      el.onerror = null;
    };
  }, [url]);

  return img;
}

export async function getImageDimensionsFromBlob(blob: Blob): Promise<{ w: number; h: number } | null> {
  try {
    const buf = await blob.arrayBuffer();
    const view = new DataView(buf);
    if (buf.byteLength >= 24) {
      const w = view.getUint32(16, false);
      const h = view.getUint32(20, false);
      if (w > 0 && h > 0 && w < 20000 && h < 20000) return { w, h };
    }
  } catch {
    // ignore
  }
  return null;
}

export function getImageDimensionsFromBase64(b64: string): { w: number; h: number } | null {
  try {
    const bin = atob(b64.slice(0, 64));
    const view = new DataView(new ArrayBuffer(bin.length));
    for (let i = 0; i < bin.length; i++) view.setUint8(i, bin.charCodeAt(i));
    const w = view.getUint32(16, false);
    const h = view.getUint32(20, false);
    if (w > 0 && h > 0 && w < 20000 && h < 20000) return { w, h };
  } catch {
    // ignore
  }
  return null;
}

export async function getImageDimensions(
  source: { imageBlob?: Blob | null; imageB64?: string | null } | Blob | string | null | undefined,
): Promise<{ w: number; h: number } | null> {
  if (!source) return null;
  if (typeof source === "string") return getImageDimensionsFromBase64(source);
  if (source instanceof Blob) return getImageDimensionsFromBlob(source);
  if (source.imageBlob) return getImageDimensionsFromBlob(source.imageBlob);
  if (source.imageB64) return getImageDimensionsFromBase64(source.imageB64);
  return null;
}

export async function ensureBase64FromSource(
  source: { imageB64?: string | null; imageBlob?: Blob | null } | Blob | string | null | undefined,
): Promise<string> {
  if (!source) return "";
  if (typeof source === "string") return source;
  if (source instanceof Blob) return blobToBase64(source);
  if (source.imageB64) return source.imageB64;
  if (source.imageBlob) return blobToBase64(source.imageBlob);
  return "";
}

export function blobSourceToURL(source: { imageBlob?: Blob | null; imageB64?: string | null } | null | undefined): string | null {
  if (!source) return null;
  if (source.imageBlob) return URL.createObjectURL(source.imageBlob);
  if (source.imageB64) return URL.createObjectURL(base64ToBlob(source.imageB64));
  return null;
}

export function prioritizeImageSource<T extends { path: string }>(sources: T[], target: T): T[] {
  const targetPath = target.path.trim();
  if (!targetPath) return sources;
  return [target, ...sources.filter((source) => source.path.trim() !== targetPath)];
}
