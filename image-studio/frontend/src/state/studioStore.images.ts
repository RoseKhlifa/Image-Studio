import {
  ChooseBatchInputDir,
  ListBatchInputImages,
  OpenImagesDialog,
  OpenImageDialog,
  ImportImageFromB64,
  RegisterImportedImageAsset,
  SaveImageAs,
  SaveImagePathAs,
} from "../platform/runtime/host";
import { saveImageForPlatform } from "../platform/android/bridge";
import { base64ToBlob } from "../lib/images";
import { clearHistoryStorage, removeHistoryItem } from "../lib/storage";
import type { BatchProcessSourceImage, HistoryItem, SourceImage } from "../types/domain";
import type { StudioState } from "./studioStore.types";
import {
  ensureFullHistoryItem,
  fileToBase64,
  materializeHistoryItem,
  toPreviewOnlyHistoryItem,
  withMediaAssetRef,
} from "./studioStore.runtime";
import { patchWorkspaceRuntime } from "./workspaceRuntime";
import { genId } from "./studioStore.shared";
import { buildHistoryCleanupPatch, waitForActiveHistoryLoad } from "./historyCleanup";

type StateAdapter = {
  getState: () => StudioState;
  setState: (patch: Partial<StudioState> | ((state: StudioState) => Partial<StudioState>)) => void;
};

function buildSourceCanvasItem(
  source: SourceImage,
  ref?: {
    imageId?: string;
    savedPath?: string;
    thumbPath?: string;
    previewUrl?: string;
    fullUrl?: string;
    previewWidth?: number;
    previewHeight?: number;
  } | null,
): HistoryItem {
  const baseItem: HistoryItem = {
    id: `source-preview:${source.path}`,
    prompt: `(参考图)${source.name}`,
    mode: "edit",
    size: "auto",
    quality: "medium",
    createdAt: Date.now(),
    savedPath: source.path,
    previewUrl: source.previewUrl,
    imageB64: source.previewUrl ? undefined : source.imageB64,
    imageBlob: source.previewUrl ? null : (source.imageBlob ?? null),
    previewBlob: source.previewUrl ? null : (source.imageBlob ?? null),
    previewOnly: true,
  };
  if (!ref) return baseItem;
  return {
    ...withMediaAssetRef(baseItem, ref),
    imageB64: ref.fullUrl || ref.imageId ? undefined : baseItem.imageB64,
    imageBlob: ref.fullUrl || ref.imageId ? null : baseItem.imageBlob,
    previewBlob: ref.fullUrl || ref.imageId ? null : baseItem.previewBlob,
    previewOnly: !(ref.fullUrl || ref.imageId),
  };
}

export function createImageActions(store: StateAdapter) {
  function mapBatchSource(source: {
    path: string;
    name: string;
    size: number;
    width?: number;
    height?: number;
    previewUrl?: string;
    previewWidth?: number;
    previewHeight?: number;
  }): BatchProcessSourceImage {
    return {
      path: source.path,
      name: source.name,
      size: source.size,
      width: source.width,
      height: source.height,
      previewUrl: source.previewUrl,
      previewWidth: source.previewWidth,
      previewHeight: source.previewHeight,
    };
  }

  return {
    async selectSourceImage() {
      try {
        const res = await OpenImageDialog();
        if (!res || !res.path) return;
        const baseName = res.path.split(/[\\/]/).pop() ?? res.path;
        const existing = store.getState().sources;
        if (existing.some((source) => source.path === res.path)) {
          store.setState({ mode: "edit", errorMessage: null, errorCanRetry: false, errorRawPath: null });
          return;
        }
        store.setState({
          sources: [...existing, {
            path: res.path,
            name: baseName,
            size: res.size,
            imageB64: res.imageB64 || undefined,
            imageBlob: res.imageB64 ? base64ToBlob(res.imageB64) : null,
            previewUrl: res.previewUrl,
            previewWidth: res.previewWidth,
            previewHeight: res.previewHeight,
          }],
          mode: "edit",
          editSourceMode: "manual",
          size: existing.length === 0 ? "auto" : store.getState().size,
          errorMessage: null,
          errorCanRetry: false,
          errorRawPath: null,
        });
      } catch (error: any) {
        store.setState({ errorMessage: `选择图片失败:${error?.message ?? error}`, errorCanRetry: false, errorRawPath: null });
      }
    },

    removeSource(index: number) {
      const next = store.getState().sources.filter((_, i) => i !== index);
      store.setState({ sources: next, mode: next.length > 0 ? "edit" : "generate", editSourceMode: "manual" });
    },

    clearSources() {
      store.setState({ sources: [], mode: "generate", editSourceMode: "manual" });
    },

    reorderSources(from: number, to: number) {
      const list = [...store.getState().sources];
      if (from < 0 || from >= list.length || to < 0 || to >= list.length) return;
      const [moved] = list.splice(from, 1);
      list.splice(to, 0, moved);
      store.setState({ sources: list });
    },

    async viewSourceOnCanvas(index: number) {
      const source = store.getState().sources[index];
      if (!source) return;
      const ref = await RegisterImportedImageAsset(source.path).catch(() => null);
      const item = buildSourceCanvasItem(source, ref);
      store.setState({ mode: "edit", errorMessage: null, errorCanRetry: false, errorRawPath: null });
      store.getState().setField("currentImage", item);
    },

    async compareSourceOnCanvas(index: number) {
      const state = store.getState();
      const source = state.sources[index];
      if (!source) return;
      if (!state.currentImage) {
        state.pushToast("先在画布显示结果图后再对比参考图", "warn");
        return;
      }
      if (state.compareB?.savedPath === source.path) {
        state.setCompareB(null);
        return;
      }
      const ref = await RegisterImportedImageAsset(source.path).catch(() => null);
      const item = buildSourceCanvasItem(source, ref);
      store.setState({ mode: "edit", errorMessage: null, errorCanRetry: false, errorRawPath: null });
      await state.setCompareB(item);
    },

    async reuseAsSource(item: HistoryItem) {
      let localItem = await materializeHistoryItem(item, {
        setState: (fn) => store.setState((state) => fn(state)),
      }).catch((e: any) => {
        store.setState({ errorMessage: `源图准备失败:${e?.message ?? e}`, errorCanRetry: false, errorRawPath: null });
        return null;
      });
      if (!localItem?.savedPath) return;
      const savedPath = localItem.savedPath;
      if (!localItem.previewUrl && !localItem.previewBlob && !localItem.imageB64) {
        const ref = await RegisterImportedImageAsset(savedPath).catch(() => null);
        if (ref) localItem = withMediaAssetRef(localItem, ref);
      }
      const baseName = savedPath.split(/[\\/]/).pop() ?? "source.png";
      const existing = store.getState().sources;
      const alreadyIn = existing.some((source) => source.path === savedPath);
      store.setState({
        mode: "edit",
        editSourceMode: "manual",
        currentImage: toPreviewOnlyHistoryItem(localItem),
        resultGridOpen: false,
        sources: alreadyIn
          ? existing
          : [...existing, {
              path: savedPath,
              name: baseName,
              size: 0,
              imageBlob: localItem.previewUrl ? null : (localItem.previewBlob ?? localItem.imageBlob ?? null),
              imageB64: localItem.previewUrl ? undefined : localItem.imageB64,
              previewUrl: localItem.previewUrl,
              previewWidth: localItem.previewWidth,
              previewHeight: localItem.previewHeight,
            }],
        size: alreadyIn || existing.length > 0 ? store.getState().size : "auto",
      });
    },

    applyHistoryParams(item: HistoryItem) {
      const patch: Partial<StudioState> = {
        prompt: item.prompt ?? "",
        mode: item.mode,
        size: item.size,
        quality: item.quality,
        editAutoAspectResolution: "",
      };
      if (item.seed !== undefined) patch.seed = item.seed;
      if (item.negativePrompt !== undefined) patch.negativePrompt = item.negativePrompt;
      if (item.background !== undefined) patch.background = item.background;
      if (item.outputCompression !== undefined) patch.outputCompression = item.outputCompression;
      if (item.inputFidelity !== undefined) patch.inputFidelity = item.inputFidelity;
      if (item.imageStyle !== undefined) patch.imageStyle = item.imageStyle;
      if (item.moderation !== undefined) patch.moderation = item.moderation;
      if (item.styleTag !== undefined) patch.styleTag = item.styleTag;
      if (item.outputFormat) patch.outputFormat = item.outputFormat;
      store.setState(patch);
      store.getState().pushToast("已应用此图的参数到控制台", "success");
    },

    async regenerateFromHistory(item: HistoryItem) {
      this.applyHistoryParams(item);
      await Promise.resolve();
      await store.getState().submit();
    },

    async deleteHistoryItem(id: string) {
      await removeHistoryItem(id);
      const currentBefore = store.getState().currentImage;
      const wasCurrent = currentBefore?.id === id;
      const nextBatch = store.getState().batchResults.filter((entry) => entry.id !== id);
      const patch: Partial<StudioState> = { batchResults: nextBatch };
      if (wasCurrent) patch.currentImage = null;
      if (nextBatch.length <= 1) patch.resultGridOpen = false;
      store.setState((state) => ({
        history: state.history.filter((entry) => entry.id !== id),
        ...(patch as any),
        workspaces: patchWorkspaceRuntime(state.workspaces, state.activeWorkspaceId, {
          currentImageId: wasCurrent ? null : currentBefore?.id ?? null,
          batchResultIds: nextBatch.map((entry) => entry.id),
          resultGridOpen: nextBatch.length > 1 && (patch.resultGridOpen ?? state.resultGridOpen),
        }),
      }));
    },

    async clearHistory() {
      await waitForActiveHistoryLoad(store.getState);
      const before = store.getState();
      const loadedIDs = before.history.map((item) => item.id);
      store.setState({ historyHasMore: false, historyLoading: true });
      try {
        const storedIDs = await clearHistoryStorage();
        const clearedIDs = new Set([...loadedIDs, ...storedIDs]);
        store.setState((state) => buildHistoryCleanupPatch(state, clearedIDs));
        return clearedIDs.size;
      } catch (error) {
        store.setState({
          historyHasMore: before.historyHasMore,
          historyLoading: before.historyLoading,
          historyCursorBeforeDayStart: before.historyCursorBeforeDayStart,
        });
        throw error;
      }
    },

    async saveCurrentImageAs() {
      const current = store.getState().currentImage;
      if (!current) return;
      const suggested = `image-${current.mode}-${current.id.slice(0, 8)}.png`;
      try {
        const saved = current.savedPath
          ? await SaveImagePathAs(current.savedPath, suggested)
          : await saveImageForPlatform((await ensureFullHistoryItem(current, {
              setState: (fn) => store.setState((state) => fn(state)),
            }))?.imageB64 ?? "", suggested, SaveImageAs);
        if (saved) store.getState().pushToast(`已保存:${saved.split(/[\\/]/).pop()}`, "success");
      } catch (e: any) {
        const msg = `保存失败:${e?.message ?? e}`;
        store.setState({ errorMessage: msg, errorCanRetry: false, errorRawPath: null });
        store.getState().pushToast(msg, "error");
      }
    },

    async importImageFile(file: File) {
      try {
        if (!/^image\/(png|jpe?g|webp)$/.test(file.type)) {
          store.setState({ errorMessage: `不支持的图片类型:${file.type || "(未知)"},请用 PNG/JPG/WebP`, errorCanRetry: false, errorRawPath: null });
          return;
        }
        const b64 = await fileToBase64(file);
        const result = await ImportImageFromB64(b64, file.name);
        const ref = await RegisterImportedImageAsset(result.path).catch(() => null);
        const legacyB64 = result.previewUrl || ref?.previewUrl ? "" : (result.imageB64 || b64);
        const legacyBlob = legacyB64 ? base64ToBlob(legacyB64) : null;
        const transientItem: HistoryItem = {
          id: genId(),
          imageB64: legacyB64 || undefined,
          imageBlob: null,
          previewBlob: legacyBlob,
          prompt: `(导入)${file.name}`,
          mode: "edit",
          size: "1024x1024",
          quality: "medium",
          createdAt: Date.now(),
          savedPath: result.path,
        };
        const importedItem = ref ? withMediaAssetRef(transientItem, ref) : transientItem;
        const existingSources = store.getState().sources;
        const alreadyIn = existingSources.some((source) => source.path === result.path);
        store.setState({
          currentImage: ref ? { ...importedItem, previewOnly: true } : importedItem,
          batchResults: [],
          resultGridOpen: false,
          mode: "edit",
          editSourceMode: "manual",
          size: alreadyIn || existingSources.length > 0 ? store.getState().size : "auto",
          sources: alreadyIn
            ? existingSources
            : [...existingSources, {
                path: result.path,
                name: file.name,
                size: file.size,
                imageBlob: legacyBlob,
                imageB64: legacyB64 || undefined,
                previewUrl: importedItem.previewUrl,
                previewWidth: importedItem.previewWidth,
                previewHeight: importedItem.previewHeight,
          }],
          errorMessage: null,
          errorCanRetry: false,
          errorRawPath: null,
        });
      } catch (e: any) {
        store.setState({ errorMessage: `导入失败:${e?.message ?? e}`, errorCanRetry: false, errorRawPath: null });
      }
    },

    async chooseBatchInputDir() {
      try {
        const result = await ChooseBatchInputDir();
        if (!result?.directory) return;
        store.setState((state) => ({
          mode: "edit",
          editSourceMode: "batch",
          batchProcess: {
            ...state.batchProcess,
            enabled: true,
            inputDir: result.directory,
            discoveredSources: result.images.map(mapBatchSource),
          },
          errorMessage: null,
          errorCanRetry: false,
          errorRawPath: null,
        }));
      } catch (error: any) {
        store.setState({ errorMessage: `选择批处理目录失败:${error?.message ?? error}`, errorCanRetry: false, errorRawPath: null });
      }
    },

    async chooseBatchInputFiles() {
      try {
        const result = await OpenImagesDialog();
        if (!result?.files?.length) return;
        store.setState((state) => {
          const existing = new Map(state.batchProcess.discoveredSources.map((item) => [item.path, item]));
          for (const file of result.files) {
            existing.set(file.path, mapBatchSource(file));
          }
          return {
            mode: "edit",
            editSourceMode: "batch",
            batchProcess: {
              ...state.batchProcess,
              enabled: true,
              discoveredSources: Array.from(existing.values()),
            },
            errorMessage: null,
            errorCanRetry: false,
            errorRawPath: null,
          };
        });
      } catch (error: any) {
        store.setState({ errorMessage: `选择批处理图片失败:${error?.message ?? error}`, errorCanRetry: false, errorRawPath: null });
      }
    },

    async refreshBatchInputDir() {
      const { batchProcess } = store.getState();
      if (!batchProcess.inputDir.trim()) return;
      try {
        const result = await ListBatchInputImages(batchProcess.inputDir);
        store.setState((state) => ({
          batchProcess: {
            ...state.batchProcess,
            inputDir: result.directory || state.batchProcess.inputDir,
            discoveredSources: result.images.map(mapBatchSource),
          },
          errorMessage: null,
          errorCanRetry: false,
          errorRawPath: null,
        }));
      } catch (error: any) {
        store.setState({ errorMessage: `刷新批处理目录失败:${error?.message ?? error}`, errorCanRetry: false, errorRawPath: null });
      }
    },
  };
}
