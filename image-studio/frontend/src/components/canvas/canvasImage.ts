import { useEffect, useState } from "react";
import { base64ToBlob } from "../../lib/images";

export async function copyImageB64ToClipboard(b64: string): Promise<boolean> {
  try {
    const blob = await (await fetch(`data:image/png;base64,${b64}`)).blob();
    if (typeof ClipboardItem === "undefined" || !navigator.clipboard?.write) return false;
    await navigator.clipboard.write([new ClipboardItem({ "image/png": blob })]);
    return true;
  } catch {
    return false;
  }
}

export async function copyImageURLToClipboard(url: string): Promise<boolean> {
  try {
    if (!url || typeof ClipboardItem === "undefined" || !navigator.clipboard?.write) return false;
    const blob = await (await fetch(url)).blob();
    await navigator.clipboard.write([new ClipboardItem({ [blob.type || "image/png"]: blob })]);
    return true;
  } catch {
    return false;
  }
}

// Load the original image with its MIME type preserved, including animated GIFs.
// Clears the previous image synchronously when the source changes so the rest
// of the canvas never renders with a stale-image / new-view mismatch.
export function useImageFromSource(
  blob: Blob | null | undefined,
  b64: string | undefined,
  url?: string | null,
): HTMLImageElement | null {
  const [img, setImg] = useState<HTMLImageElement | null>(null);

  useEffect(() => {
    if (!blob && !b64 && !url) {
      setImg(null);
      return;
    }
    setImg(null);
    const el = new Image();
    const objectURL = blob ? URL.createObjectURL(blob) : b64 ? b64ToObjectURL(b64) : null;
    const src = objectURL || url || null;
    if (!src) return;
    el.onload = () => setImg(el);
    el.onerror = () => setImg(null);
    el.src = src;
    return () => {
      el.onload = null;
      el.onerror = null;
      if (objectURL) URL.revokeObjectURL(objectURL);
    };
  }, [blob, b64, url]);

  return img;
}

function b64ToObjectURL(b64: string): string | null {
  try {
    return URL.createObjectURL(base64ToBlob(b64));
  } catch {
    return null;
  }
}
