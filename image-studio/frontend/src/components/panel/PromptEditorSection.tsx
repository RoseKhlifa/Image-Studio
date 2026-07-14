import { lazy, Suspense, useRef, useState } from "react";
import { ListPlus, ScanSearch, Sparkles } from "lucide-react";
import { submitShortcutLabel } from "../../platform";
import { usePlatform } from "../../platform/context";
import { useStudioStore } from "../../state/studioStore";
import { ParameterPresetsSection } from "./ParameterPresetsSection";
import { appendPromptTemplateText } from "../../lib/promptTemplateInsert";

const PromptPopover = lazy(() => import("./PromptPopover").then((m) => ({ default: m.PromptPopover })));
const PromptTemplateManagerModal = lazy(() => import("./PromptTemplateManagerModal").then((m) => ({ default: m.PromptTemplateManagerModal })));

export function PromptEditorSection({
  mode,
  prompt,
  promptLen,
  promptPopover,
  setPromptPopover,
  optimizeReady,
  inferReady,
  isOptimizingPrompt,
  isInferringPrompt,
  onSetPrompt,
  onOptimizePrompt,
  onInferPrompt,
}: {
  mode: "generate" | "edit";
  prompt: string;
  promptLen: number;
  promptPopover: boolean;
  setPromptPopover: (open: boolean | ((v: boolean) => boolean)) => void;
  optimizeReady: boolean;
  inferReady: boolean;
  isOptimizingPrompt: boolean;
  isInferringPrompt: boolean;
  onSetPrompt: (value: string) => void;
  onOptimizePrompt: () => void;
  onInferPrompt: () => void;
}) {
  const { isMac, usesFluentUI } = usePlatform();
  const promptTemplates = useStudioStore((s) => s.promptTemplates);
  const promptPopoverAnchorRef = useRef<HTMLButtonElement | null>(null);
  const [templateManagerOpen, setTemplateManagerOpen] = useState(false);

  function appendTemplate(text: string) {
    const current = useStudioStore.getState().prompt;
    onSetPrompt(appendPromptTemplateText(current, text));
  }

  return (
    <section className={`platform-card relative overflow-visible ${promptPopover ? "z-30" : "z-0"} ${isMac ? "p-5" : "p-4"}`}>
      <div className="mb-3">
        <ParameterPresetsSection />
      </div>
      <div className="mb-1 flex items-center justify-between gap-3">
        <label className="windows-prompt-label text-[10px] uppercase tracking-[0.12em] text-zinc-400 dark:text-zinc-500">
          {mode === "edit" ? "修改要求" : "提示词"}
        </label>
        <span className={`font-mono-token tabular-nums ${isMac ? "rounded-full bg-black/[0.04] px-2 py-0.5 text-[11px] dark:bg-white/[0.06]" : ""} text-zinc-400 dark:text-zinc-500`}>{promptLen}</span>
      </div>
      {isMac && (
        <p className="mb-3 text-[12px] leading-6 text-zinc-500 dark:text-zinc-400">
          建议把主体、场景、镜头、材质和光照拆成短句，模板会追加到当前内容末尾。
        </p>
      )}
      <textarea
        value={prompt}
        placeholder={mode === "edit"
          ? "主体保持不变\n把背景换成夜空，补一圈冷色边缘光，保留原有构图"
          : "主体 / 场景 / 光照 / 镜头 / 风格\n例如：一只橘猫坐在雨夜窗边，电影级侧逆光，50mm，浅景深，写实摄影"}
        onChange={(e) => onSetPrompt(e.target.value)}
        className={`focus-ring w-full resize-y border border-black/[0.08] bg-[var(--surface)] text-zinc-900 placeholder:text-zinc-400 dark:border-white/[0.08] dark:text-zinc-100 dark:placeholder:text-zinc-500 ${usesFluentUI ? "min-h-[124px] rounded-[10px] px-3.5 py-3 text-[14px] leading-[1.65]" : isMac ? "min-h-[176px] rounded-[18px] px-4 py-3.5 text-[15px] leading-[1.72]" : "min-h-[124px] rounded-[14px] px-3.5 py-3 text-[14px] leading-[1.65]"}`}
      />
      <div className="windows-prompt-template-list mt-3 flex flex-wrap gap-2">
        <button
          type="button"
          onClick={() => setTemplateManagerOpen(true)}
          className={`platform-pill inline-flex items-center justify-center gap-1.5 px-3 py-1.5 text-[11px] text-zinc-500 transition-colors hover:bg-[var(--accent-soft)] hover:text-[var(--accent)] ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
        >
          <ListPlus className="w-3 h-3" /> 管理模板
        </button>
        {promptTemplates.map((item) => (
          <button
            key={item.id}
            type="button"
            onClick={() => appendTemplate(item.text)}
            title={item.text}
            className={`platform-pill inline-flex max-w-full items-center justify-center px-3 py-1.5 text-[11px] text-zinc-600 transition-colors hover:bg-[var(--accent-soft)] hover:text-[var(--accent)] dark:text-zinc-300 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
          >
            {item.label}
          </button>
        ))}
      </div>
      <div className={`windows-prompt-toolbar mt-3 ${isMac ? "flex flex-col gap-3" : "flex gap-2.5 items-center justify-between"}`}>
        <div className={`windows-prompt-actions ${isMac ? "grid grid-cols-3 gap-2.5" : "flex gap-2.5 items-center"}`}>
          <div className={`relative ${isMac ? "min-w-0" : "shrink-0"}`}>
            <button
              ref={promptPopoverAnchorRef}
              type="button"
              onClick={() => setPromptPopover((v) => !v)}
              title="prompt 模板与历史"
              className={`platform-pill inline-flex items-center justify-center gap-1.5 ${isMac ? "min-h-[38px] w-full px-3 py-2 text-[11px] font-medium" : "px-3 py-1.5 text-[10px]"} transition-colors ${
                promptPopover
                  ? "bg-[var(--accent-soft)] text-[var(--accent)] ring-1 ring-[color:var(--accent)]/20"
                  : "text-zinc-500 hover:bg-[var(--accent-soft)] hover:text-[var(--accent)]"
              } ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
            >
              <ListPlus className="w-3 h-3" /> 模板 / 历史
            </button>
            {promptPopover && (
              <Suspense fallback={null}>
                <PromptPopover
                  anchorRef={promptPopoverAnchorRef}
                  onClose={() => setPromptPopover(false)}
                  onManageTemplates={() => setTemplateManagerOpen(true)}
                  onPick={appendTemplate}
                />
              </Suspense>
            )}
          </div>
          <button
            type="button"
            onClick={onOptimizePrompt}
            disabled={!optimizeReady || isOptimizingPrompt || isInferringPrompt}
            className={`platform-pill inline-flex items-center justify-center gap-1.5 ${isMac ? "min-h-[38px] w-full px-3 py-2 text-[11px] font-medium" : "px-3 py-1.5 text-[10px]"} transition-colors ${
              isOptimizingPrompt
                ? "bg-[var(--accent-soft)] text-[var(--accent)]"
                : "text-zinc-500 hover:bg-[var(--accent-soft)] hover:text-[var(--accent)]"
            } disabled:cursor-not-allowed disabled:opacity-50 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
            title="调用 Responses API 优化当前提示词"
          >
            <Sparkles className={`w-3 h-3 ${isOptimizingPrompt ? "animate-pulse" : ""}`} />
            {isOptimizingPrompt ? "优化中..." : "AI 优化"}
          </button>
          <button
            type="button"
            onClick={onInferPrompt}
            disabled={!inferReady || isOptimizingPrompt || isInferringPrompt}
            className={`platform-pill inline-flex items-center justify-center gap-1.5 ${isMac ? "min-h-[38px] w-full px-3 py-2 text-[11px] font-medium" : "px-3 py-1.5 text-[10px]"} transition-colors ${
              isInferringPrompt
                ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-300"
                : "text-zinc-500 hover:bg-emerald-500/10 hover:text-emerald-600 dark:hover:text-emerald-300"
            } disabled:cursor-not-allowed disabled:opacity-50 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
            title="使用 AI 分析画布中的图片并反推提示词"
          >
            <ScanSearch className={`w-3 h-3 ${isInferringPrompt ? "animate-pulse" : ""}`} />
            {isInferringPrompt ? "反推中..." : "图片反推"}
          </button>
        </div>
        <div className={`windows-prompt-shortcut flex ${isMac ? "items-center justify-between gap-2.5" : "ml-auto items-center gap-2.5"}`}>
          <span className={`${isMac ? "ml-auto rounded-full bg-black/[0.03] px-2.5 py-1.5 text-[11px] dark:bg-white/[0.04]" : "text-[10px]"} text-zinc-400 dark:text-zinc-500`}>{submitShortcutLabel}</span>
        </div>
      </div>
      {templateManagerOpen ? (
        <Suspense fallback={null}>
          <PromptTemplateManagerModal open={templateManagerOpen} onClose={() => setTemplateManagerOpen(false)} />
        </Suspense>
      ) : null}
    </section>
  );
}
