import { useEffect, useMemo, useState } from "react";
import { FolderOpen, Save } from "lucide-react";
import { useStudioStore } from "../../state/studioStore";
import { BatchResultGrid } from "../canvas/BatchResultGrid";
import { historyPreviewSrc, useBlobURL } from "../../lib/images";
import {
  buildHistoryItemDragExport,
  shouldUseNativeFileDrag,
  writeImageFileDragData,
  writeInternalHistoryItemDragData,
} from "../../lib/dragExport.ts";
import { saveHistoryItemAs, saveHistoryItemsToDirectory } from "../../lib/saveResultImage";
import { androidSaveHint, androidTarget } from "../../platform/android/bridge";
import { BeginNativeFileDrag, getHostCapabilities, ChooseDirectory } from "../../platform/runtime/host";
import { usePlatform } from "../../platform/context";
import { Modal } from "./Modal";

export function SavePromptModal() {
  const request = useStudioStore((s) => s.savePromptRequest);
  const closeSavePrompt = useStudioStore((s) => s.closeSavePrompt);
  const suppressed = useStudioStore((s) => s.savePromptSuppressed);
  const setSuppressed = useStudioStore((s) => s.setSavePromptSuppressed);
  const pushToast = useStudioStore((s) => s.pushToast);
  const { usesFluentUI, isAndroidPhone, isAndroid, targetPlatform } = usePlatform();
  const [saving, setSaving] = useState(false);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

  useEffect(() => {
    if (request?.kind === "batch") {
      setSelectedIds(new Set(request.items.map((item) => item.id)));
      return;
    }
    setSelectedIds(new Set());
  }, [request]);

  const batchItems = request?.kind === "batch" ? request.items : [];
  const selectedBatchItems = useMemo(
    () => batchItems.filter((item) => selectedIds.has(item.id)),
    [batchItems, selectedIds],
  );

  if (!request) return null;
  const isBatch = request.kind === "batch";
  const singleItem = request.kind === "single" ? request.item : null;
  const batchRequest = request.kind === "batch" ? request : null;
  const singlePreviewURL = useBlobURL(singleItem?.previewBlob ?? singleItem?.imageBlob ?? null, singleItem?.imageB64 ?? null);

  async function saveSingleAs() {
    if (saving || !singleItem) return;
    setSaving(true);
    try {
      const saved = await saveHistoryItemAs(singleItem);
      if (saved) pushToast(`已保存:${saved.split(/[\\/]/).pop()}`, "success");
      closeSavePrompt();
    } catch (error: any) {
      pushToast(`保存失败:${error?.message ?? error}`, "error", 6000);
    } finally {
      setSaving(false);
    }
  }

  async function saveBatchAs() {
    if (saving || !batchRequest) return;
    if (selectedBatchItems.length === 0) {
      pushToast("请先勾选要另存的图片", "warn");
      return;
    }
    if (!(hostCapabilities.nativeOutputDirectoryPicker && !isAndroid)) {
      pushToast("当前平台暂不支持批量选择目录另存，请改用单图另存为。", "warn", 5000);
      return;
    }
    setSaving(true);
    try {
      const directory = await ChooseDirectory("选择批量另存为目录");
      if (!directory) {
        setSaving(false);
        return;
      }
      const saved = await saveHistoryItemsToDirectory(selectedBatchItems, directory);
      pushToast(`已另存 ${saved.length} 张到 ${directory.split(/[\\/]/).pop() || directory}`, "success", 6000);
      closeSavePrompt();
    } catch (error: any) {
      pushToast(`批量保存失败:${error?.message ?? error}`, "error", 6000);
    } finally {
      setSaving(false);
    }
  }

  if (!isBatch) {
    const imageSrc = historyPreviewSrc(singleItem, singlePreviewURL);
    const dragSpec = singleItem ? buildHistoryItemDragExport(singleItem) : null;

    function handleSinglePreviewDragStart(event: React.DragEvent<HTMLDivElement>) {
      if (!singleItem || !dragSpec) {
        event.preventDefault();
        return;
      }
      event.stopPropagation();
      if (shouldUseNativeFileDrag(targetPlatform, singleItem.savedPath)) {
        event.preventDefault();
        void BeginNativeFileDrag(singleItem.savedPath).catch((error) => {
          console.error("[drag-export] native-file-drag failed", error);
        });
        return;
      }
      event.dataTransfer.effectAllowed = "copy";
      writeInternalHistoryItemDragData(event.dataTransfer, singleItem);
      writeImageFileDragData(event.dataTransfer, dragSpec);
    }

    return (
      <Modal open onClose={closeSavePrompt} title="是否另存这张图片?" width={isAndroidPhone ? 420 : 520}>
        <div className="space-y-4">
          <div className="grid gap-3 sm:grid-cols-[132px_minmax(0,1fr)]">
            <div
              draggable={!!dragSpec}
              onDragStart={handleSinglePreviewDragStart}
              title={dragSpec ? "拖到文件夹复制原图" : undefined}
              className={`grid min-h-[132px] place-items-center overflow-hidden border border-black/[0.08] bg-[var(--surface)] p-2 dark:border-white/[0.06] ${usesFluentUI ? "rounded-[10px]" : "rounded-[16px]"}`}
            >
              {imageSrc ? (
                <img
                  src={imageSrc}
                  alt="生成结果预览"
                  decoding="async"
                  draggable={false}
                  className={`max-h-[120px] max-w-full object-contain ${usesFluentUI ? "rounded-[8px]" : "rounded-[12px]"}`}
                />
              ) : (
                <Save className="h-8 w-8 text-zinc-400" />
              )}
            </div>
            <div className="min-w-0 space-y-2">
              <p className="text-sm font-medium text-zinc-900 dark:text-zinc-100">
                图片已生成并保存在默认输出目录。
              </p>
              <p className="text-xs leading-relaxed text-zinc-600 dark:text-zinc-300">
                需要放到项目、相册或其他目录时，可以现在选择目标位置另存一份。
              </p>
              {singleItem?.savedPath ? (
                <p className={`font-mono-token break-all border border-black/[0.06] bg-[var(--surface)] px-2.5 py-2 text-[11px] text-zinc-500 dark:border-white/[0.04] ${usesFluentUI ? "rounded-[8px]" : "rounded-[12px]"}`}>
                  {singleItem.savedPath}
                </p>
              ) : null}
            </div>
          </div>

          {androidTarget.isAndroid ? (
            <p className="text-[11px] leading-relaxed text-zinc-500">{androidSaveHint()}</p>
          ) : null}

          <label className="flex cursor-pointer items-center gap-2 text-xs text-zinc-600 dark:text-zinc-300">
            <input
              type="checkbox"
              checked={suppressed}
              onChange={(event) => setSuppressed(event.currentTarget.checked)}
              className="h-4 w-4 accent-[var(--accent)]"
            />
            以后不再提示
          </label>

          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <button
              type="button"
              onClick={closeSavePrompt}
              className={`platform-action-btn border border-black/[0.08] px-4 py-2 text-sm text-zinc-700 transition-colors hover:bg-black/[0.04] dark:border-white/[0.08] dark:text-zinc-300 dark:hover:bg-white/[0.06] ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
            >
              稍后
            </button>
            <button
              type="button"
              onClick={saveSingleAs}
              disabled={saving}
              className={`liquid-primary-button inline-flex items-center justify-center gap-1.5 bg-[var(--accent)] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--accent-2)] disabled:cursor-not-allowed disabled:opacity-60 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
            >
              <FolderOpen className="h-4 w-4" />
              {saving ? "保存中..." : "保存到指定位置"}
            </button>
          </div>
        </div>
      </Modal>
    );
  }

  const hostCapabilities = getHostCapabilities();
  const canBatchSaveToDirectory = hostCapabilities.nativeOutputDirectoryPicker && !isAndroid;

  return (
    <Modal
      open
      onClose={closeSavePrompt}
      title={`本次结果 · ${batchItems.length} 张`}
      width={isAndroidPhone ? 420 : 860}
      cardClassName={!isAndroidPhone ? "max-w-[92vw]" : ""}
      bodyClassName="flex min-h-0 flex-col gap-4"
    >
      <div className="flex min-h-0 flex-1 flex-col gap-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="space-y-1">
            <p className="text-sm font-medium text-zinc-900 dark:text-zinc-100">
              单次任务只弹这一次。勾选需要的图片后，再统一另存为到目标目录。
            </p>
            <p className="text-xs leading-relaxed text-zinc-600 dark:text-zinc-300">
              当前已选 {selectedBatchItems.length} / {batchItems.length} 张。
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => setSelectedIds(new Set(batchItems.map((item) => item.id)))}
              className={`platform-action-btn border border-black/[0.08] px-3 py-1.5 text-xs text-zinc-700 transition-colors hover:bg-black/[0.04] dark:border-white/[0.08] dark:text-zinc-300 dark:hover:bg-white/[0.06] ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
            >
              全选
            </button>
            <button
              type="button"
              onClick={() => setSelectedIds(new Set())}
              className={`platform-action-btn border border-black/[0.08] px-3 py-1.5 text-xs text-zinc-700 transition-colors hover:bg-black/[0.04] dark:border-white/[0.08] dark:text-zinc-300 dark:hover:bg-white/[0.06] ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
            >
              清空
            </button>
          </div>
        </div>

        <div className="relative min-h-[320px] flex-1 overflow-hidden rounded-[18px] border border-black/[0.08] bg-[var(--canvas-bg)] dark:border-white/[0.06]">
          <BatchResultGrid
            items={batchItems}
            currentId={null}
            onSelect={() => undefined}
            onClose={closeSavePrompt}
            showClose={false}
            title={`勾选要另存的结果 · ${batchItems.length} 张`}
            selectionMode
            selectedIds={selectedIds}
            onToggleSelect={(item) => {
              setSelectedIds((prev) => {
                const next = new Set(prev);
                if (next.has(item.id)) next.delete(item.id);
                else next.add(item.id);
                return next;
              });
            }}
          />
        </div>

        {androidTarget.isAndroid ? (
          <p className="text-[11px] leading-relaxed text-zinc-500">
            {androidSaveHint()} 当前平台批量勾选后统一目录另存尚不可用，请改用单图另存为。
          </p>
        ) : !canBatchSaveToDirectory ? (
          <p className="text-[11px] leading-relaxed text-zinc-500">
            当前环境没有可用的目录选择器，暂时无法一次性批量另存。
          </p>
        ) : null}

        <label className="flex cursor-pointer items-center gap-2 text-xs text-zinc-600 dark:text-zinc-300">
          <input
            type="checkbox"
            checked={suppressed}
            onChange={(event) => setSuppressed(event.currentTarget.checked)}
            className="h-4 w-4 accent-[var(--accent)]"
          />
          以后不再提示
        </label>

        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button
            type="button"
            onClick={closeSavePrompt}
            className={`platform-action-btn border border-black/[0.08] px-4 py-2 text-sm text-zinc-700 transition-colors hover:bg-black/[0.04] dark:border-white/[0.08] dark:text-zinc-300 dark:hover:bg-white/[0.06] ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
          >
            稍后
          </button>
          <button
            type="button"
            onClick={saveBatchAs}
            disabled={saving || selectedBatchItems.length === 0 || !canBatchSaveToDirectory}
            className={`liquid-primary-button inline-flex items-center justify-center gap-1.5 bg-[var(--accent)] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--accent-2)] disabled:cursor-not-allowed disabled:opacity-60 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
          >
            <FolderOpen className="h-4 w-4" />
            {saving ? "保存中..." : `另存选中项 (${selectedBatchItems.length})`}
          </button>
        </div>
      </div>
    </Modal>
  );
}
