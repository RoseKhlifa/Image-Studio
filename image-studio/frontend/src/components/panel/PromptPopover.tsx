import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";
import { useStudioStore } from "../../state/studioStore";
import { usePlatform } from "../../platform/context";
import { buildPromptHistoryEntries } from "../../lib/promptTemplates";

const PROMPT_TEMPLATES: { label: string; text: string }[] = [
  { label: "写实摄影", text: "photorealistic, professional photography, 35mm, natural lighting, sharp focus, high detail" },
  { label: "电影感", text: "cinematic, dramatic lighting, shallow depth of field, film grain, anamorphic, 2.39:1" },
  { label: "二次元", text: "anime style, vibrant colors, cel shading, detailed illustration" },
  { label: "油画", text: "oil painting, thick brush strokes, classical art style, warm tones" },
  { label: "水彩", text: "watercolor painting, soft edges, pastel colors, paper texture" },
  { label: "扁平插画", text: "flat illustration, minimalist, geometric shapes, vector style" },
  { label: "3D 渲染", text: "3D render, octane render, ray tracing, glossy, studio lighting" },
  { label: "像素风", text: "pixel art, 16-bit, retro game style, limited palette" },
];

export function PromptPopover({
  anchorRef,
  onClose,
  onManageTemplates,
  onPick,
}: {
  anchorRef: React.RefObject<HTMLElement | null>;
  onClose: () => void;
  onManageTemplates: () => void;
  onPick: (text: string) => void;
}) {
  const history = useStudioStore((s) => s.promptHistory);
  const historyEntries = buildPromptHistoryEntries(history);
  const promptTemplates = useStudioStore((s) => s.promptTemplates);
  const [tab, setTab] = useState<"templates" | "history">("templates");
  const { isMac, usesFluentUI, usesAppleUI } = usePlatform();
  const popoverRef = useRef<HTMLDivElement | null>(null);
  const [pos, setPos] = useState<{ top: number; left: number; width: number } | null>(null);

  useLayoutEffect(() => {
    const compute = () => {
      const anchor = anchorRef.current;
      if (!anchor) return;
      const rect = anchor.getBoundingClientRect();
      const viewportWidth = document.documentElement.clientWidth || window.innerWidth;
      const width = Math.min(isMac ? 400 : 360, Math.max(280, viewportWidth - 24));
      const left = Math.min(Math.max(12, rect.left), Math.max(12, viewportWidth - width - 12));
      setPos({ top: rect.bottom + 12, left, width });
    };
    compute();
    window.addEventListener("resize", compute);
    window.addEventListener("scroll", compute, true);
    return () => {
      window.removeEventListener("resize", compute);
      window.removeEventListener("scroll", compute, true);
    };
  }, [anchorRef, isMac]);

  useEffect(() => {
    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as Node;
      if (popoverRef.current?.contains(target)) return;
      if (anchorRef.current?.contains(target)) return;
      onClose();
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("pointerdown", handlePointerDown, true);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown, true);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [anchorRef, onClose]);

  if (!pos) return null;

  return createPortal(
    <div
      ref={popoverRef}
      style={{ position: "fixed", top: pos.top, left: pos.left, width: pos.width, zIndex: 9050 }}
      className={`flex max-h-[360px] flex-col overflow-hidden border border-black/[0.08] bg-white/96 shadow-[0_28px_70px_rgb(15_23_42_/_0.22)] backdrop-blur-2xl dark:border-white/[0.08] dark:bg-[rgb(24_27_34_/_0.96)] ${usesAppleUI ? "liquid-glass-panel" : ""} ${usesFluentUI ? "rounded-[12px]" : "rounded-[22px]"}`}
    >
      <div className="flex items-center border-b border-black/[0.06] px-2 py-1.5 dark:border-white/[0.05]">
        <button
          onClick={() => setTab("templates")}
          className={`flex-1 rounded-full ${isMac ? "px-3.5 py-2.5 text-[12px]" : "px-3 py-2 text-[11px]"} font-semibold transition-colors ${
            tab === "templates"
              ? "bg-[var(--accent-soft)] text-[var(--accent)]"
              : "text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100"
          }`}
        >
          模板
        </button>
        <button
          onClick={() => setTab("history")}
          className={`flex-1 rounded-full ${isMac ? "px-3.5 py-2.5 text-[12px]" : "px-3 py-2 text-[11px]"} font-semibold transition-colors ${
            tab === "history"
              ? "bg-[var(--accent-soft)] text-[var(--accent)]"
              : "text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100"
          }`}
        >
          历史 ({history.length})
        </button>
        <button
          onClick={onClose}
          title="关闭"
          className={`px-2 py-2 text-zinc-500 transition-colors hover:bg-black/[0.05] hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-white/[0.06] dark:hover:text-zinc-100 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
        >
          <X className="w-3.5 h-3.5" />
        </button>
      </div>
      <div className={`flex-1 overflow-y-auto ${isMac ? "p-3" : "p-2.5"}`}>
        {tab === "templates" && (
          <div className="flex flex-col gap-3">
            <div className="flex items-center justify-between gap-2">
              <div className="text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-500 dark:text-zinc-400">自定义模板</div>
              <button
                type="button"
                onClick={() => { onClose(); onManageTemplates(); }}
                className={`text-[11px] text-[var(--accent)] ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
              >
                管理
              </button>
            </div>
            {promptTemplates.length === 0 ? (
              <div className={`border border-dashed border-black/[0.08] px-4 py-5 text-center text-[12px] text-zinc-500 dark:border-white/[0.08] dark:text-zinc-300 ${usesFluentUI ? "rounded-[12px]" : "rounded-[18px]"}`}>
                还没有自定义模板
              </div>
            ) : (
              promptTemplates.map((item) => (
                <button
                  key={item.id}
                  onClick={() => { onPick(item.text); onClose(); }}
                  className={`w-full text-left transition-colors hover:bg-[var(--accent-soft)] ${isMac ? "px-3.5 py-3.5" : "px-3 py-3"} ${usesFluentUI ? "rounded-[10px]" : "rounded-[16px]"}`}
                >
                  <div className={`${isMac ? "mb-1.5 text-[13px]" : "mb-1 text-[12px]"} font-semibold text-zinc-900 dark:text-zinc-100`}>{item.label}</div>
                  <div className={`${isMac ? "text-[12px] leading-6" : "text-[11px] leading-relaxed"} text-zinc-500 dark:text-zinc-300`}>{item.text}</div>
                </button>
              ))
            )}

            <div className="text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-500 dark:text-zinc-400">内置模板</div>
            {PROMPT_TEMPLATES.map((t) => (
              <button
                key={t.label}
                onClick={() => { onPick(t.text); onClose(); }}
                className={`w-full text-left transition-colors hover:bg-[var(--accent-soft)] ${isMac ? "px-3.5 py-3.5" : "px-3 py-3"} ${usesFluentUI ? "rounded-[10px]" : "rounded-[16px]"}`}
              >
                <div className={`${isMac ? "mb-1.5 text-[13px]" : "mb-1 text-[12px]"} font-semibold text-zinc-900 dark:text-zinc-100`}>{t.label}</div>
                <div className={`${isMac ? "text-[12px] leading-6" : "text-[11px] leading-relaxed"} text-zinc-500 dark:text-zinc-300`}>{t.text}</div>
              </button>
            ))}
          </div>
        )}
        {tab === "history" && (
          historyEntries.length === 0 ? (
            <div className={`border border-dashed border-black/[0.08] px-4 py-8 text-center text-[12px] text-zinc-500 dark:border-white/[0.08] dark:text-zinc-300 ${usesFluentUI ? "rounded-[12px]" : "rounded-[18px]"}`}>
              还没有提交过 prompt
            </div>
          ) : (
            <ol className="list-none border-y border-black/[0.07] dark:border-white/[0.07]">
              {historyEntries.map((entry) => (
                <li
                  key={`${entry.position}:${entry.text}`}
                  className="border-b border-black/[0.07] last:border-b-0 dark:border-white/[0.07]"
                >
                  <button
                    onClick={() => { onPick(entry.text); onClose(); }}
                    title={`使用第 ${entry.position} 条历史 prompt`}
                    className={`prompt-history-entry flex w-full items-start gap-2.5 text-left transition-colors hover:bg-[var(--accent-soft)] ${isMac ? "px-3.5 py-3.5" : "px-3 py-3"}`}
                  >
                    <span
                      aria-hidden="true"
                      className={`mt-0.5 w-7 shrink-0 border-r border-black/[0.08] pr-2 text-right font-mono text-[10px] font-semibold tabular-nums text-zinc-400 dark:border-white/[0.08] dark:text-zinc-500 ${isMac ? "leading-6" : "leading-relaxed"}`}
                    >
                      {String(entry.position).padStart(2, "0")}
                    </span>
                    <span className={`min-w-0 flex-1 ${isMac ? "text-[13px] leading-6" : "text-[12px] leading-relaxed"} text-zinc-700 dark:text-zinc-200`}>
                      {entry.text}
                    </span>
                  </button>
                </li>
              ))}
            </ol>
          )
        )}
      </div>
    </div>,
    document.body,
  );
}
