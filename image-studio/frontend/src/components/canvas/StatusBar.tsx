import { CheckCircle2, Loader2 } from "lucide-react";
import { useEffect, useState } from "react";
import { useStudioStore } from "../../state/studioStore";
import { useStudioFields } from "../../state/useStudioFields";
import { usePlatform } from "../../platform/context";
import { HistoryMetaBadges } from "../history/HistoryMetaBadges";
import { qualityLabel, sizeLabel } from "../history/historyLabels";
import { StreamPreviewBadge } from "./StreamPreviewBadge";

function fmtBytes(b: number): string {
  if (b < 1024) return `${b} B`;
  if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`;
  return `${(b / 1024 / 1024).toFixed(1)} MB`;
}

export function StatusBar() {
  const { isRunning, progress, currentImage, streamPreview, lastLogLine, viewZoom, recentDurations, jobsTotal, jobsCompleted, runningJobs } = useStudioFields([
    "isRunning", "progress", "currentImage", "streamPreview",
    "lastLogLine", "viewZoom", "recentDurations", "jobsTotal",
    "jobsCompleted", "runningJobs",
  ]);
  const { isAndroidPhone, isMac, isWindows, usesFluentUI, usesAppleUI } = usePlatform();
  const [clockNow, setClockNow] = useState(() => Date.now());
  const [progressAnchor, setProgressAnchor] = useState(() => ({ elapsed: 0, at: Date.now() }));
  const zoomLabel = currentImage ? `${Math.round(viewZoom * 100)}%` : "";
  const avg = recentDurations.length > 0
    ? recentDurations.reduce((a, b) => a + b, 0) / recentDurations.length
    : 0;
  const eta = isRunning && progress && avg > 0
    ? Math.max(0, Math.round(avg - progress.elapsed))
    : null;
  const liveElapsedSeconds = progress
    ? Math.max(progress.elapsed, progressAnchor.elapsed + (clockNow - progressAnchor.at) / 1000)
    : 0;

  useEffect(() => {
    if (!isRunning) return undefined;
    const timer = window.setInterval(() => setClockNow(Date.now()), 100);
    return () => window.clearInterval(timer);
  }, [isRunning]);

  useEffect(() => {
    if (!progress) return;
    setProgressAnchor({ elapsed: progress.elapsed, at: Date.now() });
  }, [progress?.elapsed, progress?.stage]);

  function formatElapsed(seconds: number): string {
    return `${seconds.toFixed(1)}s`;
  }

  if (isRunning) {
    return (
      <div className={`${isWindows ? "statusbar" : ""} relative flex items-center gap-3 overflow-hidden border-t border-[var(--border)] bg-[var(--toolbar)] px-3 py-2 text-[11px] text-zinc-700 backdrop-blur-2xl dark:text-zinc-300 ${usesAppleUI ? "liquid-glass-bar" : ""} ${usesFluentUI ? "min-h-[34px]" : ""} ${isAndroidPhone ? "min-h-[30px]" : ""} ${isMac ? "min-h-[28px]" : ""}`}>
        <Loader2 className="h-3 w-3 shrink-0 animate-spin text-[var(--accent)]" />
        <span className="font-medium">
          {progress
            ? isMac
              ? `${progress.stage} · ${formatElapsed(liveElapsedSeconds)}`
              : `${progress.stage} · ${formatElapsed(liveElapsedSeconds)} · ${fmtBytes(progress.bytes)}`
            : "正在请求..."}
        </span>
        {jobsTotal > 1 && (
          <span className="font-medium text-[var(--accent)]">
            并发 {runningJobs.length} · {jobsCompleted}/{jobsTotal}
          </span>
        )}
        {streamPreview ? <StreamPreviewBadge compact /> : null}
        {eta !== null && <span className="text-zinc-500">≈ 剩余 {eta}s</span>}
        <div className="absolute bottom-0 left-0 right-0 h-px animate-pulse bg-[color:var(--accent)]/35" />
        {!isAndroidPhone && !isMac && (
          <span className="text-zinc-500 truncate max-w-[30%] ml-auto" title={lastLogLine}>
            {lastLogLine}
          </span>
        )}
      </div>
    );
  }
  if (currentImage) {
    const headline = currentImage.mode === "edit" ? "编辑结果" : "生成结果";
    const metaBadges = [sizeLabel(currentImage.size), qualityLabel(currentImage.quality)];
    if (currentImage.elapsedSec) metaBadges.push(`${currentImage.elapsedSec}s`);
    if (!isMac && currentImage.seed) metaBadges.push(`seed ${currentImage.seed}`);
    if (!isMac && currentImage.styleTag) metaBadges.push(`#${currentImage.styleTag}`);
    return (
      <div className={`${isWindows ? "statusbar" : ""} flex items-center gap-3 overflow-hidden border-t border-[var(--border)] bg-[var(--toolbar)] px-3 py-2 text-[11px] text-zinc-600 backdrop-blur-2xl dark:text-zinc-400 ${usesAppleUI ? "liquid-glass-bar" : ""} ${usesFluentUI ? "min-h-[34px]" : ""} ${isAndroidPhone ? "min-h-[30px]" : ""} ${isMac ? "min-h-[28px]" : ""}`}>
        <span className="inline-flex shrink-0 items-center gap-1.5 text-[var(--accent)]">
          <CheckCircle2 className="w-3 h-3" />
          <span className="font-medium">{headline}</span>
        </span>
        <HistoryMetaBadges items={metaBadges} compact className="opacity-90" />
        {!isAndroidPhone && !isMac && <span className="text-zinc-500 font-mono-token">{new Date(currentImage.createdAt).toLocaleTimeString()}</span>}
        {!isAndroidPhone && !isMac && currentImage.revisedPrompt && (
          <span className="text-zinc-500 truncate flex-1 italic" title={currentImage.revisedPrompt}>
            ✨ {currentImage.revisedPrompt}
          </span>
        )}
        <span className="text-zinc-500 font-mono-token ml-auto shrink-0">{zoomLabel}</span>
      </div>
    );
  }
  if (isMac) return null;
  return (
    <div className={`${isWindows ? "statusbar" : ""} border-t border-[var(--border)] bg-[var(--toolbar)] px-3 py-2 text-[11px] text-zinc-500 backdrop-blur-2xl ${usesAppleUI ? "liquid-glass-bar" : ""} ${usesFluentUI ? "min-h-[34px]" : ""} ${isAndroidPhone ? "min-h-[30px]" : ""}`}>
      准备就绪
    </div>
  );
}
