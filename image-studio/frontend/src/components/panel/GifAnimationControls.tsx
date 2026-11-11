import { useEffect, useState } from "react";
import { useStudioStore } from "../../state/studioStore";
import {
  DEFAULT_GIF_FRAME_COUNT,
  DEFAULT_GIF_FRAME_DURATION_MS,
  MAX_GIF_FRAME_COUNT,
  MAX_GIF_FRAME_DURATION_MS,
  MIN_GIF_FRAME_COUNT,
  MIN_GIF_FRAME_DURATION_MS,
  normalizeGIFFrameCount,
  normalizeGIFFrameDurationMs,
} from "../../../../../shared/kernel/requestModel.js";

export function GifAnimationControls({ variant = "desktop" }: { variant?: "mac" | "desktop" | "android" }) {
  const frameCount = useStudioStore((state) => state.frameCount);
  const frameDurationMs = useStudioStore((state) => state.frameDurationMs);
  const setField = useStudioStore((state) => state.setField);
  const [countInput, setCountInput] = useState(String(frameCount));
  const [durationInput, setDurationInput] = useState(String(frameDurationMs));

  useEffect(() => setCountInput(String(frameCount)), [frameCount]);
  useEffect(() => setDurationInput(String(frameDurationMs)), [frameDurationMs]);

  function commitCount() {
    const value = normalizeGIFFrameCount(countInput);
    setCountInput(String(value));
    setField("frameCount", value);
  }

  function commitDuration() {
    const value = normalizeGIFFrameDurationMs(durationInput);
    setDurationInput(String(value));
    setField("frameDurationMs", value);
  }

  function reset() {
    setCountInput(String(DEFAULT_GIF_FRAME_COUNT));
    setDurationInput(String(DEFAULT_GIF_FRAME_DURATION_MS));
    setField("frameCount", DEFAULT_GIF_FRAME_COUNT);
    setField("frameDurationMs", DEFAULT_GIF_FRAME_DURATION_MS);
  }

  const inputClassName = variant === "android"
    ? "android-phone-seed-input w-full min-w-0"
    : `w-full border border-black/[0.08] bg-[var(--surface)] px-3 py-2.5 text-[13px] text-zinc-900 dark:border-white/[0.08] dark:text-zinc-100 ${variant === "mac" ? "rounded-[14px]" : "rounded-[10px]"}`;
  const seconds = Number((frameCount * frameDurationMs / 1000).toFixed(2));
  const fps = Number((1000 / frameDurationMs).toFixed(2));

  return (
    <div className="grid gap-3" data-testid="gif-animation-controls">
      <div className="grid grid-cols-2 gap-3">
        <label className="grid min-w-0 gap-1.5 text-[12px] text-zinc-600 dark:text-zinc-300">
          <span>动画帧数</span>
          <input
            aria-label="GIF 动画帧数"
            type="number"
            min={MIN_GIF_FRAME_COUNT}
            max={MAX_GIF_FRAME_COUNT}
            step={1}
            value={countInput}
            onChange={(event) => setCountInput(event.target.value)}
            onBlur={commitCount}
            onKeyDown={(event) => { if (event.key === "Enter") event.currentTarget.blur(); }}
            className={`focus-ring font-mono-token ${inputClassName}`}
          />
          <span className="text-[11px] text-zinc-500">2–16 帧</span>
        </label>
        <label className="grid min-w-0 gap-1.5 text-[12px] text-zinc-600 dark:text-zinc-300">
          <span>每帧时长（ms）</span>
          <input
            aria-label="GIF 每帧时长（毫秒）"
            type="number"
            min={MIN_GIF_FRAME_DURATION_MS}
            max={MAX_GIF_FRAME_DURATION_MS}
            step={10}
            value={durationInput}
            onChange={(event) => setDurationInput(event.target.value)}
            onBlur={commitDuration}
            onKeyDown={(event) => { if (event.key === "Enter") event.currentTarget.blur(); }}
            className={`focus-ring font-mono-token ${inputClassName}`}
          />
          <span className="text-[11px] text-zinc-500">20–2000 ms，步长 10 ms</span>
        </label>
      </div>
      <div className="rounded-xl bg-[var(--accent-soft)] px-3 py-2 text-[12px] text-[var(--accent)]" aria-live="polite">
        单次循环 {seconds} 秒 · {fps} 帧/秒
      </div>
      <div className="flex items-start justify-between gap-3 text-[11px] leading-5 text-zinc-500 dark:text-zinc-400">
        <p>在提示词中描述动作和循环方式。动画帧数与生成过程中的预览帧数分别设置。</p>
        <button type="button" onClick={reset} className="shrink-0 text-[var(--accent)] hover:underline">恢复默认</button>
      </div>
    </div>
  );
}
