import {
  CropImage,
  ExportHistoryToFile,
  FlipImage,
  ImportHistoryFromFile,
  RegisterImportedImageAsset,
  RegisterMediaAsset,
  ReadImageAsBase64,
  RotateImage,
} from "../platform/runtime/host";
import { exportHistoryForPlatform } from "../platform/android/bridge";
import {
  sanitizeHistoryForExport,
  sanitizeImportedHistoryItem,
} from "../lib/security";
import {
  buildPresetFromSnapshot,
  buildPresetPatch,
  pickPresetStateSnapshot,
} from "../lib/presets";
import { base64ToBlob, orderedNavigationItemsForCurrent } from "../lib/images";
import { loadAllHistory, persistHistoryItems, removeHistoryOlderThan } from "../lib/storage";
import { mergeHistoryItems } from "../lib/history";
import { buildHistoryCleanupPatch, waitForActiveHistoryLoad } from "./historyCleanup";
import type { HistoryItem, Preset, Toast } from "../types/domain";
import type { StudioState } from "./studioStore.types";
import {
  genId,
} from "./studioStore.shared";
import {
  ensureFullBatchItem,
  ensureFullHistoryItem,
  materializeHistoryItem,
  toPreviewOnlyHistoryItem,
  withMediaAssetRef,
} from "./studioStore.runtime";
import { patchWorkspaceRuntime } from "./workspaceRuntime";

type StateAdapter = {
  getState: () => StudioState;
  setState: (patch: Partial<StudioState> | ((state: StudioState) => Partial<StudioState>)) => void;
};

export function createMediaActions(store: StateAdapter) {
  function persistPresets(next: Preset[]) {
    store.setState({ presets: next });
    try { localStorage.setItem("gptcodex.presets", JSON.stringify(next)); } catch {}
  }

  return {
    async setCompareB(item: HistoryItem | null) {
      if (!item) {
        store.setState({ compareB: null, compareSplit: 0.5 });
        return;
      }
      const full = await ensureFullHistoryItem(item, {
        setState: (fn) => store.setState((state) => fn(state)),
      });
      if (full) store.setState({ compareB: full, compareSplit: 0.5 });
    },

    setCompareSplit(v: number) {
      store.setState({ compareSplit: Math.max(0, Math.min(1, v)) });
    },

    openResultGrid() {
      const state = store.getState();
      if (state.batchResults.length <= 1) return;
      store.setState({
        resultGridOpen: true,
        compareB: null,
        currentImage: state.currentImage ? toPreviewOnlyHistoryItem(state.currentImage) : null,
        workspaces: patchWorkspaceRuntime(state.workspaces, state.activeWorkspaceId, { resultGridOpen: true }),
      });
    },

    closeResultGrid() {
      const state = store.getState();
      store.setState({
        resultGridOpen: false,
        currentImage: state.currentImage ? toPreviewOnlyHistoryItem(state.currentImage) : null,
        workspaces: patchWorkspaceRuntime(state.workspaces, state.activeWorkspaceId, { resultGridOpen: false }),
      });
    },

    async selectBatchResult(item: HistoryItem) {
      const full = await ensureFullBatchItem(item, {
        setState: (fn) => store.setState((state) => fn(state)),
      });
      const state = store.getState();
      store.setState({
        currentImage: { ...full, previewOnly: false },
        resultGridOpen: false,
        compareB: null,
        maskDataURL: null,
        maskTargetPath: null,
        strokes: [],
        annotations: [],
        undoStack: [],
        redoStack: [],
        tool: "pan",
        workspaces: patchWorkspaceRuntime(state.workspaces, state.activeWorkspaceId, {
          currentImageId: full.id,
          resultGridOpen: false,
        }),
      });
    },

    async stepBatchResult(delta: -1 | 1) {
      const state = store.getState();
      if (!state.currentImage) return;
      const navigationItems = orderedNavigationItemsForCurrent(
        state.currentImage.id,
        state.history,
        state.batchResults,
      );
      if (navigationItems.length <= 1) return;
      const currentIndex = navigationItems.findIndex((item) => item.id === state.currentImage?.id);
      if (currentIndex < 0) return;
      const nextIndex = (currentIndex + delta + navigationItems.length) % navigationItems.length;
      const nextItem = navigationItems[nextIndex];
      if (!nextItem) return;
      await store.getState().selectBatchResult(nextItem);
    },

    pushToast(text: string, kind: Toast["kind"] = "info", ttl = 3500, action?: Toast["action"]) {
      const id = genId();
      const toast: Toast = { id, text, kind, createdAt: Date.now(), ttl, action };
      store.setState((state) => ({ toasts: [...state.toasts, toast] }));
      if (ttl > 0) {
        setTimeout(() => {
          store.setState((state) => ({ toasts: state.toasts.filter((t) => t.id !== id) }));
        }, ttl);
      }
    },

    dismissToast(id: string) {
      store.setState((state) => ({ toasts: state.toasts.filter((t) => t.id !== id) }));
    },

    async openResultDetail(item: HistoryItem) {
      store.setState({ resultDetail: toPreviewOnlyHistoryItem(item) });
    },

    closeResultDetail() {
      store.setState({ resultDetail: null });
    },

    async materializeCurrentImage(item: HistoryItem) {
      const full = await ensureFullHistoryItem(item, {
        setState: (fn) => store.setState((state) => fn(state)),
      });
      return full ?? item;
    },

    setHistoryRailCollapsed(collapsed: boolean) {
      store.setState({ historyRailCollapsed: collapsed });
    },

    openHistoryTimeline() {
      store.setState({ historyTimelineOpen: true });
    },

    closeHistoryTimeline() {
      store.setState({ historyTimelineOpen: false });
    },

    async pruneHistoryOlderThanDays(days: number) {
      if (!Number.isFinite(days) || days <= 0) return 0;
      await waitForActiveHistoryLoad(store.getState);
      const cutoff = Date.now() - days * 24 * 3600 * 1000;
      const removedIDs = await removeHistoryOlderThan(cutoff);
      const state = store.getState();
      store.setState({
        ...buildHistoryCleanupPatch(state, removedIDs),
        historyHasMore: state.historyHasMore,
        historyLoading: state.historyLoading,
        historyCursor: state.historyCursor,
      });
      return removedIDs.length;
    },

    async rotateCurrent(degrees: number) {
      let current = store.getState().currentImage;
      if (!current) {
        store.getState().pushToast("当前没有图片", "warn");
        return;
      }
      current = await materializeHistoryItem(current, {
        setState: (fn) => store.setState((state) => fn(state)),
      }).catch((e: any) => {
        store.getState().pushToast(`当前图无法落盘:${e?.message ?? e}`, "error");
        return null;
      });
      if (!current?.savedPath) return;
      try {
        const result = await RotateImage(current.savedPath, degrees);
        await loadTransformedAsCurrent(store, result.path);
        store.getState().pushToast(`已旋转 ${degrees}° · ${result.acceleration ?? "native"}`, "success");
      } catch (e: any) {
        store.getState().pushToast(`旋转失败:${e?.message ?? e}`, "error");
      }
    },

    async flipCurrent(horizontal: boolean) {
      let current = store.getState().currentImage;
      if (!current) {
        store.getState().pushToast("当前没有图片", "warn");
        return;
      }
      current = await materializeHistoryItem(current, {
        setState: (fn) => store.setState((state) => fn(state)),
      }).catch((e: any) => {
        store.getState().pushToast(`当前图无法落盘:${e?.message ?? e}`, "error");
        return null;
      });
      if (!current?.savedPath) return;
      try {
        const result = await FlipImage(current.savedPath, horizontal);
        await loadTransformedAsCurrent(store, result.path);
        store.getState().pushToast(`${horizontal ? "已水平翻转" : "已竖直翻转"} · ${result.acceleration ?? "native"}`, "success");
      } catch (e: any) {
        store.getState().pushToast(`翻转失败:${e?.message ?? e}`, "error");
      }
    },

    async cropToRect(x: number, y: number, w: number, h: number) {
      let current = store.getState().currentImage;
      if (!current) {
        store.getState().pushToast("当前没有图片", "warn");
        return;
      }
      current = await materializeHistoryItem(current, {
        setState: (fn) => store.setState((state) => fn(state)),
      }).catch((e: any) => {
        store.getState().pushToast(`当前图无法落盘:${e?.message ?? e}`, "error");
        return null;
      });
      if (!current?.savedPath) return;
      try {
        const result = await CropImage(current.savedPath, Math.round(x), Math.round(y), Math.round(w), Math.round(h));
        await loadTransformedAsCurrent(store, result.path);
        store.getState().pushToast(`已裁出 ${Math.round(w)}×${Math.round(h)} · ${result.acceleration ?? "native"}`, "success");
      } catch (e: any) {
        store.getState().pushToast(`裁剪失败:${e?.message ?? e}`, "error");
      }
    },

    savePreset(name: string) {
      const state = store.getState();
      const trimmed = name.trim();
      if (!trimmed) return null;
      const preset: Preset = buildPresetFromSnapshot(trimmed, genId(), pickPresetStateSnapshot(state));
      const next = [...state.presets, preset];
      persistPresets(next);
      store.getState().pushToast(`已保存预设「${trimmed}」`, "success");
      return preset.id;
    },

    overwritePreset(id: string) {
      const state = store.getState();
      const preset = state.presets.find((item) => item.id === id);
      if (!preset) return false;
      const next = state.presets.map((item) => (
        item.id === id
          ? {
              ...item,
              ...pickPresetStateSnapshot(state),
              name: item.name,
            }
          : item
      ));
      persistPresets(next);
      store.getState().pushToast(`已更新预设「${preset.name}」`, "success");
      return true;
    },

    updatePreset(id: string, patch: Partial<Omit<Preset, "id">>) {
      const state = store.getState();
      const index = state.presets.findIndex((item) => item.id === id);
      if (index < 0) return false;
      const current = state.presets[index];
      const nextPreset: Preset = {
        ...current,
        ...patch,
        name: patch.name !== undefined ? patch.name.trim() : current.name,
      };
      if (!nextPreset.name) return false;
      const next = state.presets.map((item, itemIndex) => itemIndex === index ? nextPreset : item);
      persistPresets(next);
      store.getState().pushToast(`已保存预设「${nextPreset.name}」`, "success");
      return true;
    },

    applyPreset(id: string) {
      const preset = store.getState().presets.find((x) => x.id === id);
      if (!preset) return;
      store.setState(buildPresetPatch(preset, pickPresetStateSnapshot(store.getState())));
      store.getState().pushToast(`已应用预设「${preset.name}」`, "success");
    },

    deletePreset(id: string) {
      const next = store.getState().presets.filter((preset) => preset.id !== id);
      if (next.length === store.getState().presets.length) return;
      persistPresets(next);
      store.getState().pushToast("已删除预设", "success");
    },

    async exportHistory() {
      const state = store.getState();
      try {
        const items = mergeHistoryItems([...state.history, ...await loadAllHistory()]);
        if (items.length === 0) {
          state.pushToast("没有可导出的历史记录", "warn");
          return;
        }
        const payload = {
          version: 1,
          exportedAt: new Date().toISOString(),
          count: items.length,
          items: items.map(sanitizeHistoryForExport),
        };
        const dst = await exportHistoryForPlatform(JSON.stringify(payload, null, 2), ExportHistoryToFile);
        if (dst) state.pushToast(`已导出 ${items.length} 条 → ${dst.split(/[\\/]/).pop()}`, "success");
      } catch (e: any) {
        state.pushToast(`导出失败:${e?.message ?? e}`, "error");
      }
    },

    async importHistory() {
      const state = store.getState();
      try {
        const json = await ImportHistoryFromFile();
        if (!json) return;
        const parsed = JSON.parse(json);
        const incoming: HistoryItem[] = Array.isArray(parsed?.items) ? parsed.items : [];
        if (incoming.length === 0) {
          state.pushToast("文件里没有历史记录", "warn");
          return;
        }
        const existing = new Set((await loadAllHistory()).map((h) => h.id));
        const toPersist: HistoryItem[] = [];
        let added = 0;
        for (const item of incoming) {
          if (!item.id || existing.has(item.id)) continue;
          if (!item.createdAt) continue;
          const hasRenderableImage = !!(item.previewUrl || item.previewBlob || item.imageB64 || (item.savedPath && item.thumbPath));
          if (!hasRenderableImage) continue;
          let safeItem = sanitizeImportedHistoryItem(item);
          if (safeItem.savedPath && !safeItem.savedPath.startsWith("memory://") && !safeItem.imageB64 && !safeItem.previewBlob) {
            try {
              const ref = safeItem.thumbPath
                ? await RegisterMediaAsset(safeItem.savedPath, safeItem.thumbPath)
                : await RegisterImportedImageAsset(safeItem.savedPath);
              safeItem = withMediaAssetRef(safeItem, ref);
            } catch {
              // Keep the metadata/legacy preview if the file is unavailable in this environment.
            }
          }
          existing.add(safeItem.id);
          toPersist.push(safeItem);
          added++;
        }
        await persistHistoryItems(toPersist);
        store.setState({ history: mergeHistoryItems([...store.getState().history, ...toPersist]) });
        state.pushToast(`已导入 ${added} 条(跳过 ${incoming.length - added} 条重复/无效)`, "success");
      } catch (e: any) {
        state.pushToast(`导入失败:${e?.message ?? e}`, "error");
      }
    },
  };
}

async function loadTransformedAsCurrent(store: StateAdapter, path: string) {
  const snapshot = store.getState();
  try {
    const baseName = path.split(/[\\/]/).pop() ?? "transformed.png";
    const ref = await RegisterImportedImageAsset(path).catch(() => null);
    const fallbackB64 = ref ? "" : await ReadImageAsBase64(path).catch(() => "");
    const current = snapshot.currentImage;
    const updated: HistoryItem = current
      ? withMediaAssetRef({
          ...current,
          imageB64: fallbackB64 || undefined,
          imageBlob: null,
          previewBlob: null,
          savedPath: path,
          previewOnly: !fallbackB64,
        }, ref ?? {})
      : {
          id: genId(),
          ...(ref ? {
            imageId: ref.imageId,
            previewUrl: ref.previewUrl,
            previewWidth: ref.previewWidth,
            previewHeight: ref.previewHeight,
            previewOnly: true,
          } : {
            imageB64: fallbackB64,
            previewOnly: false,
          }),
          imageBlob: null,
          previewBlob: null,
          prompt: `(变换)${baseName}`,
          mode: "edit",
          size: snapshot.size,
          quality: snapshot.quality,
          createdAt: Date.now(),
          savedPath: path,
        };
    const nextSources = snapshot.sources.length > 0
      ? [{
          path,
          name: baseName,
          size: 0,
          previewUrl: updated.previewUrl,
          imageB64: updated.previewUrl ? undefined : updated.imageB64,
          imageBlob: null,
        }, ...snapshot.sources.slice(1)]
      : [{
          path,
          name: baseName,
          size: 0,
          previewUrl: updated.previewUrl,
          imageB64: updated.previewUrl ? undefined : updated.imageB64,
          imageBlob: null,
        }];
    store.setState({
      currentImage: updated,
      sources: nextSources,
      mode: "edit",
      maskDataURL: null,
      strokes: [],
      annotations: [],
      canvasViewResetTick: snapshot.canvasViewResetTick + 1,
    });
  } catch (e: any) {
    store.getState().pushToast(`加载变换结果失败:${e?.message ?? e}`, "error");
  }
}
