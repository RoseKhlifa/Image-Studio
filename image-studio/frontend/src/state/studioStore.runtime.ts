import type { backend } from "../../wailsjs/go/models";
import {
  ImportImageFromB64,
  RegisterMediaAsset,
  RegisterImportedImageAsset,
  ReadImageAsBase64,
} from "../platform/runtime/host";
import {
  base64ToBlob,
} from "../lib/images";
import {
  loadHistoryFullImage,
  persistHistoryItem,
} from "../lib/storage";
import { suggestedImportNameForHistory } from "../lib/security";
import type {
  BatchProcessConfig,
  HistoryItem,
  Workspace,
} from "../types/domain";
import type { StudioState } from "./studioStore.types";
import { currentImageIdForWorkspaceSnapshot } from "./studioStore.streamPreview";
import { normalizeBatchProcessConfig, normalizeLoopGenerationConfig } from "./workspaceRuntime";

export function historyItemsByIds(history: HistoryItem[], ids: string[]): HistoryItem[] {
  if (ids.length === 0) return [];
  const byID = new Map(history.map((item) => [item.id, item]));
  return ids.map((id) => byID.get(id)).filter((item): item is HistoryItem => !!item);
}

export function withMediaAssetRef(
  item: HistoryItem,
  ref: {
    imageId?: string;
    savedPath?: string;
    thumbPath?: string;
    previewUrl?: string;
    fullUrl?: string;
    previewWidth?: number;
    previewHeight?: number;
  },
): HistoryItem {
  return {
    ...item,
    imageId: ref.imageId || item.imageId,
    savedPath: ref.savedPath || item.savedPath,
    thumbPath: ref.thumbPath || item.thumbPath,
    previewUrl: ref.previewUrl || item.previewUrl,
    fullUrl: ref.fullUrl || item.fullUrl,
    previewWidth: ref.previewWidth || item.previewWidth,
    previewHeight: ref.previewHeight || item.previewHeight,
  };
}

export function toPreviewOnlyHistoryItem(item: HistoryItem): HistoryItem {
  const canUseCompactPreview = !!item.previewUrl || !!item.previewBlob;
  return {
    ...item,
    imageBlob: null,
    imageB64: canUseCompactPreview ? undefined : item.imageB64,
    previewOnly: true,
  };
}

function fullUrlFromImageID(imageId?: string | null): string {
  return imageId ? `/media/full/${imageId}` : "";
}

export async function materializeHistoryItem(
  item: HistoryItem,
  deps: {
    setState: (fn: (state: StudioState) => Partial<StudioState>) => void;
  },
): Promise<HistoryItem> {
  if (item.savedPath) {
    if (!item.savedPath.startsWith("memory://")) return item;
    const readable = await ReadImageAsBase64(item.savedPath).then(() => true).catch(() => false);
    if (readable) return item;
  }
  if (!item.imageB64) return item;
  const imported = await ImportImageFromB64(item.imageB64, suggestedImportNameForHistory(item));
  const next: HistoryItem = {
    ...item,
    savedPath: imported.path,
    imageB64: imported.previewUrl ? undefined : item.imageB64,
    imageId: imported.imageId || item.imageId,
    previewUrl: imported.previewUrl || item.previewUrl,
    previewWidth: imported.previewWidth || item.previewWidth,
    previewHeight: imported.previewHeight || item.previewHeight,
  };
  deps.setState((state) => ({
    currentImage: state.currentImage?.id === item.id ? next : state.currentImage,
    history: state.history.map((h) => (h.id === item.id ? next : h)),
  }));
  await persistHistoryItem(next).catch(() => undefined);
  return next;
}

export async function ensureFullHistoryItem(
  item: HistoryItem | null,
  deps: {
    setState: (fn: (state: StudioState) => Partial<StudioState>) => void;
  },
): Promise<HistoryItem | null> {
  if (!item) return null;
  if (item.savedPath && !item.savedPath.startsWith("memory://") && !item.imageB64 && !item.imageBlob) {
    try {
      const ref = item.thumbPath
        ? await RegisterMediaAsset(item.savedPath, item.thumbPath)
        : await RegisterImportedImageAsset(item.savedPath);
      const fullUrl = ref.fullUrl || (ref.imageId ? fullUrlFromImageID(ref.imageId) : item.fullUrl);
      if (fullUrl) {
        return { ...withMediaAssetRef(item, ref), fullUrl, previewOnly: false };
      }
      const fullB64 = await ReadImageAsBase64(item.savedPath).catch(() => "");
      if (fullB64) {
        return { ...withMediaAssetRef(item, ref), imageB64: fullB64, imageBlob: base64ToBlob(fullB64), previewOnly: false };
      }
    } catch {
      // Fall through to legacy full-image materialization / stale URLs.
    }
  }
  if ((item.fullUrl || item.imageId) && !item.imageB64 && !item.imageBlob) {
    return { ...item, fullUrl: item.fullUrl || fullUrlFromImageID(item.imageId), previewOnly: false };
  }
  if (item.savedPath && item.thumbPath && !item.imageB64 && !item.imageBlob) {
    try {
      const ref = await RegisterMediaAsset(item.savedPath, item.thumbPath);
      return { ...withMediaAssetRef(item, ref), fullUrl: ref.fullUrl, previewOnly: false };
    } catch {
      return item;
    }
  }
  if (!item.previewOnly) return item;
  try {
    let fullB64 = item.savedPath
      ? await ReadImageAsBase64(item.savedPath).catch(() => "")
      : "";
    if (!fullB64) {
      fullB64 = await loadHistoryFullImage(item.id).catch(() => "");
    }
    if (!fullB64) return item;
    const next: HistoryItem = { ...item, imageB64: fullB64, imageBlob: base64ToBlob(fullB64), previewOnly: false };
    deps.setState((state) => ({
      currentImage: state.currentImage?.id === item.id ? next : state.currentImage,
      compareB: state.compareB?.id === item.id ? next : state.compareB,
    }));
    return next;
  } catch {
    return item;
  }
}

export async function ensureFullBatchItem(
  item: HistoryItem,
  deps: {
    setState: (fn: (state: StudioState) => Partial<StudioState>) => void;
  },
): Promise<HistoryItem> {
  return (await ensureFullHistoryItem(item, deps)) ?? item;
}

export function cryptoIDFallback(): string {
  try {
    if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") return crypto.randomUUID();
  } catch {}
  return "id-" + Date.now().toString(36) + "-" + Math.random().toString(36).slice(2, 10);
}

export function saveActiveWorkspaceSnapshot(s: StudioState): Workspace[] {
  if (!s.activeWorkspaceId) return s.workspaces;
  return s.workspaces.map((w) => {
    if (w.id !== s.activeWorkspaceId) return w;
    let name = w.name;
    const hasDefaultName = /^图片 \d+$/.test(w.name);
    if (hasDefaultName && s.prompt.trim()) {
      const concise = s.prompt.trim().replace(/\s+/g, " ").slice(0, 18);
      name = concise || w.name;
    }
    return {
      ...w,
      name,
      prompt: s.prompt,
      negativePrompt: s.negativePrompt,
      mode: s.mode,
      size: s.size,
      quality: s.quality,
      outputFormat: s.outputFormat,
      seed: s.seed,
      background: s.background,
      outputCompression: s.outputCompression,
      inputFidelity: s.inputFidelity,
      imageStyle: s.imageStyle,
      moderation: s.moderation,
      userIdentifier: s.userIdentifier,
      partialImages: s.partialImages,
      batchCount: s.batchCount,
      selectedPresetId: s.selectedPresetId,
      editSourceMode: s.editSourceMode,
      editAutoAspectResolution: s.editAutoAspectResolution,
      batchProcess: normalizeBatchProcessConfig(s.batchProcess),
      loopGeneration: normalizeLoopGenerationConfig(s.loopGeneration),
      sources: s.sources,
      currentImageId: currentImageIdForWorkspaceSnapshot(s.currentImage, s.streamPreview, s.streamPreviews, w.currentImageId),
      batchResultIds: s.batchResults.map((item) => item.id),
      resultGridOpen: s.resultGridOpen,
      runningJobIds: s.runningJobs,
      jobsTotal: s.jobsTotal,
      jobsCompleted: s.jobsCompleted,
      progress: s.progress,
      streamPreview: s.streamPreview,
      streamPreviews: s.streamPreviews,
      lastLogLine: s.lastLogLine,
      errorMessage: s.errorMessage,
      errorCanRetry: s.errorCanRetry,
      lastPayload: s.lastPayload,
    };
  });
}

export function tryNotify(title: string, body: string, onClick?: () => void) {
  try {
    if (typeof Notification === "undefined") return;
    const fire = () => {
      const n = new Notification(title, { body });
      if (onClick) {
        n.onclick = () => {
          try { window.focus(); } catch {}
          onClick();
          n.close();
        };
      }
    };
    if (Notification.permission === "granted") {
      fire();
    } else if (Notification.permission === "default") {
      Notification.requestPermission().then((p) => {
        if (p === "granted") fire();
      });
    }
  } catch {}
}

export const STYLE_SUFFIXES: Record<string, string> = {
  cyberpunk: "cyberpunk style, neon lights, glowing reflections, futuristic",
  anime: "anime style, cel shading, vibrant colors, detailed illustration",
  illust: "modern illustration, flat colors, clean lines",
  "3d": "3D render, octane render, ray tracing, glossy surfaces, studio lighting",
  chinese: "traditional Chinese painting style, ink wash, misty landscape",
};

export async function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      const dataURL = reader.result as string;
      const idx = dataURL.indexOf(",");
      resolve(idx >= 0 ? dataURL.slice(idx + 1) : dataURL);
    };
    reader.onerror = () => reject(reader.error);
    reader.readAsDataURL(file);
  });
}
