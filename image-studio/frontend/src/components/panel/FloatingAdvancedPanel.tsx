import { ChevronDown, ChevronRight, GripHorizontal, SlidersHorizontal, X } from "lucide-react";
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { usePlatform } from "../../platform/context";
import { useStudioStore } from "../../state/studioStore";
import type {
  BackgroundValue,
  ImageStyleValue,
  InputFidelityValue,
  ModerationValue,
  OutputFormatValue,
} from "../../types/domain";
import {
  AdvancedBackgroundField,
  AdvancedCard,
  AdvancedImageStyleField,
  AdvancedInputFidelityField,
  AdvancedModerationField,
  AdvancedNegativePromptField,
  AdvancedPartialImagesField,
  AdvancedOutputCompressionField,
  AdvancedOutputFormatField,
  AdvancedSeedField,
  AdvancedUserIdentifierField,
} from "./AdvancedParameterBlocks";
import { Seg, SegItem } from "./panelChrome";
import { GifAnimationControls } from "./GifAnimationControls";

type AdvancedGroupKey = "core" | "output" | "strategy" | "stream";

type FloatingAdvancedPanelProps = {
  open: boolean;
  onClose: () => void;
  advancedSummary: string;
  background: BackgroundValue;
  imageStyle: ImageStyleValue;
  inputFidelity: InputFidelityValue;
  moderation: ModerationValue;
  negativePrompt: string;
  outputCompression: number;
  outputFormat: OutputFormatValue;
  partialImages: number;
  seed: number;
  userIdentifier: string;
  setField: (key: string, value: any) => void;
  variant: "mac" | "desktop";
};

type PanelPrefs = {
  x?: number;
  y?: number;
  groups?: Partial<Record<AdvancedGroupKey, boolean>>;
};

const DEFAULT_GROUP_OPEN: Record<AdvancedGroupKey, boolean> = {
  core: true,
  output: false,
  strategy: false,
  stream: false,
};

const PANEL_MARGIN = 16;

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}

function readPrefs(storageKey: string): PanelPrefs {
  if (typeof window === "undefined") return {};
  try {
    const raw = window.localStorage.getItem(storageKey);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as PanelPrefs;
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch {
    return {};
  }
}

function writePrefs(storageKey: string, prefs: PanelPrefs): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(storageKey, JSON.stringify(prefs));
  } catch {}
}

function summaryOrFallback(value: string, fallback: string): string {
  return value.trim() ? value.trim() : fallback;
}

function buildDefaultPosition(panelWidth: number, isMac: boolean): { x: number; y: number } {
  if (typeof window === "undefined") {
    return { x: PANEL_MARGIN, y: 96 };
  }
  return {
    x: Math.max(PANEL_MARGIN, window.innerWidth - panelWidth - 28),
    y: isMac ? 104 : 88,
  };
}

function clampPanelPosition(x: number, y: number, width: number, height: number) {
  if (typeof window === "undefined") return { x, y };
  const maxX = Math.max(PANEL_MARGIN, window.innerWidth - width - PANEL_MARGIN);
  const maxY = Math.max(PANEL_MARGIN, window.innerHeight - height - PANEL_MARGIN);
  return {
    x: clamp(x, PANEL_MARGIN, maxX),
    y: clamp(y, PANEL_MARGIN, maxY),
  };
}

function GroupSection({
  title,
  summary,
  activeCount,
  open,
  onToggle,
  children,
}: {
  title: string;
  summary: string;
  activeCount: number;
  open: boolean;
  onToggle: () => void;
  children: React.ReactNode;
}) {
  return (
    <section className="rounded-[18px] border border-black/[0.05] bg-black/[0.025] p-3.5 dark:border-white/[0.06] dark:bg-white/[0.03]">
      <button
        type="button"
        onClick={onToggle}
        className="flex w-full items-start justify-between gap-3 text-left"
      >
        <div className="min-w-0">
          <div className="text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-500 dark:text-zinc-300">
            {title}
          </div>
          <div className="mt-1 text-[12px] leading-5 text-zinc-500 dark:text-zinc-400">
            {summary}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2 pl-2">
          {activeCount > 0 ? (
            <span className="rounded-full border border-[color:var(--accent)]/18 bg-[var(--accent-soft)] px-2 py-1 text-[10px] font-medium text-[var(--accent)]">
              已启用 {activeCount}
            </span>
          ) : null}
          <span className="text-zinc-400 dark:text-zinc-500">
            {open ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
          </span>
        </div>
      </button>
      {open ? <div className="mt-3 grid gap-3">{children}</div> : null}
    </section>
  );
}

export function FloatingAdvancedPanel({
  open,
  onClose,
  advancedSummary,
  background,
  imageStyle,
  inputFidelity,
  moderation,
  negativePrompt,
  outputCompression,
  outputFormat,
  partialImages,
  seed,
  userIdentifier,
  setField,
  variant,
}: FloatingAdvancedPanelProps) {
  const { targetPlatform, usesAppleUI, usesFluentUI, isMac } = usePlatform();
  const frameCount = useStudioStore((state) => state.frameCount);
  const frameDurationMs = useStudioStore((state) => state.frameDurationMs);
  const activeWorkspaceName = useStudioStore((state) => {
    const workspace = state.workspaces.find((item) => item.id === state.activeWorkspaceId);
    return workspace?.name ?? "当前工作区";
  });
  const panelWidth = variant === "mac" ? 356 : 336;
  const storageKey = useMemo(() => `gptcodex.advancedFloatingPanel.${targetPlatform}`, [targetPlatform]);
  const initialPrefs = useMemo(() => readPrefs(storageKey), [storageKey]);
  const defaultPosition = useMemo(() => buildDefaultPosition(panelWidth, isMac), [panelWidth, isMac]);
  const [groupOpen, setGroupOpen] = useState<Record<AdvancedGroupKey, boolean>>(() => ({
    ...DEFAULT_GROUP_OPEN,
    ...initialPrefs.groups,
  }));
  const [position, setPosition] = useState(() => ({
    x: typeof initialPrefs.x === "number" ? initialPrefs.x : defaultPosition.x,
    y: typeof initialPrefs.y === "number" ? initialPrefs.y : defaultPosition.y,
  }));
  const panelRef = useRef<HTMLDivElement | null>(null);
  const dragStateRef = useRef<{
    pointerId: number;
    offsetX: number;
    offsetY: number;
  } | null>(null);

  useEffect(() => {
    if (outputFormat === "gif") setGroupOpen((current) => ({ ...current, output: true }));
  }, [outputFormat]);

  useEffect(() => {
    writePrefs(storageKey, {
      x: position.x,
      y: position.y,
      groups: groupOpen,
    });
  }, [groupOpen, position.x, position.y, storageKey]);

  useLayoutEffect(() => {
    if (!open || typeof window === "undefined") return;
    const syncPosition = () => {
      const rect = panelRef.current?.getBoundingClientRect();
      const height = rect?.height ?? Math.min(window.innerHeight - PANEL_MARGIN * 2, 720);
      setPosition((current) => clampPanelPosition(current.x, current.y, panelWidth, height));
    };
    syncPosition();
    window.addEventListener("resize", syncPosition);
    return () => window.removeEventListener("resize", syncPosition);
  }, [open, panelWidth]);

  useEffect(() => {
    if (!open) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [open, onClose]);

  useEffect(() => {
    return () => {
      if (typeof document !== "undefined") {
        document.body.style.userSelect = "";
      }
    };
  }, []);

  if (!open) return null;

  const titleSummary = summaryOrFallback(advancedSummary, "按分组展开常用与进阶参数");
  const negativeSummary = negativePrompt.trim() ? "已设置负向提示词" : "未设置负向提示词";
  const seedSummary = seed > 0 ? `Seed ${seed}` : "随机 Seed";
  const outputSummary = outputFormat === "gif"
    ? `GIF · ${frameCount} 帧 · 每帧 ${frameDurationMs} ms`
    : `${outputFormat.toUpperCase()} · 背景 ${background}${outputFormat === "png" || outputFormat === "svg" ? "" : ` · 压缩 ${outputCompression}`}`;
  const strategySummary = `保真 ${inputFidelity} · 风格 ${imageStyle} · 审核 ${moderation}`;
  const streamSummary = `${outputFormat === "gif" || outputFormat === "svg" || partialImages === 0 ? "仅最终结果" : `${partialImages} 帧预览`} · ${userIdentifier.trim() ? "已填用户标识" : "未填用户标识"}`;

  const outputActiveCount = Number(outputFormat !== "png") + Number(background !== "auto") + Number(outputCompression !== 100 && outputFormat !== "gif" && outputFormat !== "svg");
  const strategyActiveCount = Number(inputFidelity !== "auto") + Number(imageStyle !== "default") + Number(moderation !== "low");
  const streamActiveCount = Number(partialImages !== 1) + Number(userIdentifier.trim().length > 0);
  const coreActiveCount = Number(negativePrompt.trim().length > 0) + Number(seed > 0);

  const updateGroupOpen = (key: AdvancedGroupKey) => {
    setGroupOpen((current) => ({
      ...current,
      [key]: !current[key],
    }));
  };

  const onPointerDown = (event: React.PointerEvent<HTMLDivElement>) => {
    if ((event.target as HTMLElement).closest("[data-no-drag='true']")) return;
    const rect = panelRef.current?.getBoundingClientRect();
    dragStateRef.current = {
      pointerId: event.pointerId,
      offsetX: event.clientX - (rect?.left ?? position.x),
      offsetY: event.clientY - (rect?.top ?? position.y),
    };
    event.currentTarget.setPointerCapture(event.pointerId);
    document.body.style.userSelect = "none";
  };

  const onPointerMove = (event: React.PointerEvent<HTMLDivElement>) => {
    const dragState = dragStateRef.current;
    if (!dragState || dragState.pointerId !== event.pointerId) return;
    const rect = panelRef.current?.getBoundingClientRect();
    const height = rect?.height ?? Math.min(window.innerHeight - PANEL_MARGIN * 2, 720);
    const next = clampPanelPosition(
      event.clientX - dragState.offsetX,
      event.clientY - dragState.offsetY,
      panelWidth,
      height,
    );
    setPosition(next);
  };

  const stopDragging = (event: React.PointerEvent<HTMLDivElement>) => {
    if (!dragStateRef.current || dragStateRef.current.pointerId !== event.pointerId) return;
    dragStateRef.current = null;
    document.body.style.userSelect = "";
    try {
      event.currentTarget.releasePointerCapture(event.pointerId);
    } catch {}
  };

  const floatingPanel = (
    <div className="pointer-events-none fixed inset-0 z-[9050]">
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="false"
        aria-label="高级参数浮动窗口"
        className={`pointer-events-auto fixed flex max-h-[calc(100vh-32px)] flex-col overflow-hidden border shadow-[0_28px_72px_rgba(15,23,42,0.18)] ${
          usesAppleUI
            ? "border-white/50 bg-[color:var(--lg-bg-strong)] backdrop-blur-[20px] supports-[backdrop-filter]:bg-[color:var(--lg-bg)]"
            : "border-black/[0.08] bg-[color:var(--panel)]/96 backdrop-blur-[18px] dark:border-white/[0.08] dark:bg-[color:var(--panel)]/92"
        } ${usesFluentUI ? "rounded-[14px]" : "rounded-[24px]"}`}
        style={{
          width: panelWidth,
          left: position.x,
          top: position.y,
        }}
      >
        <div
          className={`flex cursor-grab items-start gap-3 border-b px-4 py-3 active:cursor-grabbing ${
            usesFluentUI
              ? "border-black/[0.07] bg-white/55 dark:border-white/[0.06] dark:bg-white/[0.04]"
              : "border-black/[0.06] bg-white/34 dark:border-white/[0.05] dark:bg-white/[0.03]"
          }`}
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={stopDragging}
          onPointerCancel={stopDragging}
        >
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[12px] bg-[var(--accent-soft)] text-[var(--accent)]">
            <SlidersHorizontal className="h-4 w-4" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-1 text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-500 dark:text-zinc-300">
              <GripHorizontal className="h-3.5 w-3.5" />
              高级参数
            </div>
            <div className="mt-1 truncate text-[12px] font-medium text-zinc-700 dark:text-zinc-200">
              {activeWorkspaceName}
            </div>
            <div className="mt-1 text-[11px] leading-5 text-zinc-500 dark:text-zinc-400">
              {titleSummary}
            </div>
          </div>
          <button
            type="button"
            data-no-drag="true"
            onClick={onClose}
            className={`platform-icon-btn mt-0.5 inline-flex h-8 w-8 shrink-0 items-center justify-center border border-transparent text-zinc-500 transition-colors hover:text-zinc-900 dark:text-zinc-300 dark:hover:text-zinc-100 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
            title="关闭"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="advanced-floating-panel-body flex-1 overflow-y-auto px-3.5 py-3.5">
          <div className="grid gap-3">
            <GroupSection
              title="基础增强"
              summary={`${negativeSummary} · ${seedSummary}`}
              activeCount={coreActiveCount}
              open={groupOpen.core}
              onToggle={() => updateGroupOpen("core")}
            >
              <AdvancedCard
                title="负向提示词"
                hint="描述不希望出现的物体、色彩或构图倾向。"
                variant={variant}
                className="rounded-[16px]"
              >
                <AdvancedNegativePromptField
                  negativePrompt={negativePrompt}
                  onChange={(value) => setField("negativePrompt", value)}
                  variant={variant}
                />
              </AdvancedCard>
              <AdvancedCard
                title="随机种子"
                hint={seed > 0 ? `当前固定为 ${seed}` : "留空即随机，每次生成都会变化。"}
                variant={variant}
                className="rounded-[16px]"
              >
                <AdvancedSeedField
                  seed={seed}
                  onChange={(value) => setField("seed", value)}
                  onRandomize={() => setField("seed", Math.floor(Math.random() * 2_000_000_000))}
                  onClear={() => setField("seed", 0)}
                  variant={variant}
                />
              </AdvancedCard>
            </GroupSection>

            <GroupSection
              title="输出控制"
              summary={outputSummary}
              activeCount={outputActiveCount}
              open={groupOpen.output}
              onToggle={() => updateGroupOpen("output")}
            >
              <AdvancedCard title="输出格式" hint="PNG 保留细节最多；JPEG / WebP 更省空间。" variant={variant} className="rounded-[16px]">
                <AdvancedOutputFormatField
                  outputFormat={outputFormat}
                  onChange={(value) => setField("outputFormat", value)}
                  Seg={Seg}
                  SegItem={SegItem}
                  noteClassName="text-[11px] leading-5 text-zinc-500 dark:text-zinc-400"
                />
              </AdvancedCard>
              {outputFormat === "gif" ? (
                <AdvancedCard title="GIF 动画" hint="设置动画帧数与播放速度。" variant={variant} className="rounded-[16px]">
                  <GifAnimationControls variant={variant} />
                </AdvancedCard>
              ) : null}
              {outputFormat !== "gif" && outputFormat !== "svg" ? <>
                <AdvancedCard title="背景" hint="透明背景需要 PNG/WebP；GIF/SVG 会省略栅格背景设置。" variant={variant} className="rounded-[16px]">
                  <AdvancedBackgroundField
                    background={background}
                    onChange={(value) => setField("background", value)}
                    Seg={Seg}
                    SegItem={SegItem}
                    noteClassName="text-[11px] leading-5 text-zinc-500 dark:text-zinc-400"
                  />
                </AdvancedCard>
                <AdvancedCard title="输出压缩" hint="仅 JPEG/WebP 生效，范围 `0-100`。" variant={variant} className="rounded-[16px]">
                  <AdvancedOutputCompressionField
                    outputCompression={outputCompression}
                    onChange={(value) => setField("outputCompression", value)}
                    variant={variant}
                    noteClassName="text-[11px] leading-5 text-zinc-500 dark:text-zinc-400"
                  />
                </AdvancedCard>
              </> : null}
            </GroupSection>

            <GroupSection
              title="生成策略"
              summary={strategySummary}
              activeCount={strategyActiveCount}
              open={groupOpen.strategy}
              onToggle={() => updateGroupOpen("strategy")}
            >
              <AdvancedCard title="输入保真" hint="用于图生图/参考图流程。" variant={variant} className="rounded-[16px]">
                <AdvancedInputFidelityField
                  inputFidelity={inputFidelity}
                  onChange={(value) => setField("inputFidelity", value)}
                  Seg={Seg}
                  SegItem={SegItem}
                  noteClassName="text-[11px] leading-5 text-zinc-500 dark:text-zinc-400"
                />
              </AdvancedCard>
              <AdvancedCard title="图像风格" hint="仅 `dall-e-3` 文生图支持；默认值会省略该字段。" variant={variant} className="rounded-[16px]">
                <AdvancedImageStyleField
                  imageStyle={imageStyle}
                  onChange={(value) => setField("imageStyle", value)}
                  Seg={Seg}
                  SegItem={SegItem}
                  noteClassName="text-[11px] leading-5 text-zinc-500 dark:text-zinc-400"
                />
              </AdvancedCard>
              <AdvancedCard title="内容审核" hint="`low` 更宽松；`auto` 使用官方默认审核强度。" variant={variant} className="rounded-[16px]">
                <AdvancedModerationField
                  moderation={moderation}
                  onChange={(value) => setField("moderation", value)}
                  Seg={Seg}
                  SegItem={SegItem}
                  noteClassName="text-[11px] leading-5 text-zinc-500 dark:text-zinc-400"
                />
              </AdvancedCard>
            </GroupSection>

            <GroupSection
              title="流式与标识"
              summary={streamSummary}
              activeCount={streamActiveCount}
              open={groupOpen.stream}
              onToggle={() => updateGroupOpen("stream")}
            >
              {outputFormat !== "gif" && outputFormat !== "svg" ? <AdvancedCard title="流式预览帧数" hint="`0` 只返回最终图，`1-3` 会流式返回预览帧。高并发或大尺寸任务时，应用可能自动关闭预览。" variant={variant} className="rounded-[16px]">
                <AdvancedPartialImagesField
                  partialImages={partialImages}
                  onChange={(value) => setField("partialImages", value)}
                  Seg={Seg}
                  SegItem={SegItem}
                  noteClassName="text-[11px] leading-5 text-zinc-500 dark:text-zinc-400"
                />
              </AdvancedCard> : <p className="text-[12px] text-zinc-500">{outputFormat.toUpperCase()} 会在生成完成后返回完整结果。</p>}
              <AdvancedCard title="稳定用户标识" hint="建议传哈希后的用户名或邮箱。" variant={variant} className="rounded-[16px]">
                <AdvancedUserIdentifierField
                  userIdentifier={userIdentifier}
                  onChange={(value) => setField("userIdentifier", value)}
                  variant={variant}
                  noteClassName="text-[11px] leading-5 text-zinc-500 dark:text-zinc-400"
                />
              </AdvancedCard>
            </GroupSection>

            <div className="rounded-[18px] border border-black/[0.05] bg-black/[0.025] px-3.5 py-3 text-[11px] leading-[1.7] text-zinc-500 dark:border-white/[0.06] dark:bg-white/[0.025] dark:text-zinc-400">
              `background` / `output_compression` / `input_fidelity` / `style` / `moderation` / `partial_images` / `user`(`safety_identifier`) 都是官方字段；`seed` / `negative prompt` 仍只在兼容中转扩展策略下发送。
            </div>
          </div>
        </div>
      </div>
    </div>
  );

  if (typeof document === "undefined") return floatingPanel;
  return createPortal(floatingPanel, document.body);
}
