import { Folder, Github, MessageSquare } from "lucide-react";
import { useStudioStore } from "../../state/studioStore";
import { useStudioFields } from "../../state/useStudioFields";
import { OpenExternalURL, OpenOutputDir } from "../../platform/runtime/host";
import { androidTarget, openExternalURLForPlatform, openOutputLocationForPlatform } from "../../platform/android/bridge";
import { appVersion } from "../../lib/version";
import { usePlatform } from "../../platform/context";

const REPO_URL = "https://github.com/RoseKhlifa/Image-Studio";
const ISSUES_URL = "https://github.com/RoseKhlifa/Image-Studio/issues";

export function FooterBar() {
  const { fullscreen, history, runningJobs, isRunning, pushToast } = useStudioFields([
    "fullscreen", "history", "runningJobs", "isRunning",
    "pushToast",
  ]);
  const totalRunning = useStudioStore((state) => state.workspaces.reduce((sum, w) => sum + (w.runningJobIds?.length ?? 0), 0));
  const { isAndroid, isMac, isWindows, usesFluentUI, usesAppleUI } = usePlatform();
  if (fullscreen) return null;
  if (isAndroid) return null;
  if (isMac) return null;
  const activeRunning = isRunning;
  const anyRunning = activeRunning || totalRunning > 0;

  // 今日已生图 = 本地日历当天 00:00 起的条目数,不是「最近 24h」滚动窗口。
  const todayStart = new Date();
  todayStart.setHours(0, 0, 0, 0);
  const todayCount = history.filter((h) => h.createdAt >= todayStart.getTime()).length;

  function open(url: string) {
    openExternalURLForPlatform(url, OpenExternalURL).catch(() => pushToast("无法打开浏览器", "error"));
  }

  function openOutputLocation() {
    openOutputLocationForPlatform(OpenOutputDir).catch((e) => pushToast(e?.message ?? "无法打开保存位置", "warn"));
  }

  return (
    <footer className={`${isWindows ? "footer-bar" : ""} flex items-center justify-between border-t border-[var(--border)] bg-[var(--toolbar)] px-4 text-[11px] text-zinc-500 backdrop-blur-2xl dark:text-zinc-400 ${usesAppleUI ? "liquid-glass-bar" : ""} ${usesFluentUI ? "min-h-[36px]" : "min-h-10"}`}>
      <div className="flex items-center gap-1">
        <FooterBtn onClick={openOutputLocation}>
          <Folder className="h-3 w-3" /> {androidTarget.isAndroid ? "保存位置" : "输出目录"}
        </FooterBtn>
        {!isMac && (
          <>
            <FooterBtn onClick={() => open(REPO_URL)}>
              <Github className="h-3 w-3" /> GitHub
            </FooterBtn>
            <FooterBtn onClick={() => open(ISSUES_URL)}>
              <MessageSquare className="h-3 w-3" /> 反馈
            </FooterBtn>
          </>
        )}
      </div>
      <div className="flex items-center gap-3">
        <span className="flex items-baseline gap-1">
          <span className="opacity-70">今日已生图:</span>
          <span className="font-medium text-zinc-700 dark:text-zinc-300 tabular-nums">{todayCount}</span>
        </span>
        <span className="opacity-40">·</span>
        <span className="flex items-baseline gap-1">
          <span className="opacity-70">总生图:</span>
          <span className="font-medium text-zinc-700 dark:text-zinc-300 tabular-nums">{history.length}</span>
        </span>
        {anyRunning && (
          <>
            <span className="opacity-40">·</span>
            <span className="flex items-baseline gap-1">
              <span className="opacity-70">{activeRunning ? "当前标签" : "后台运行"}</span>
              <span className="font-medium text-[var(--accent)] tabular-nums">{activeRunning ? runningJobs.length : totalRunning}</span>
            </span>
          </>
        )}
      </div>
      <div className="flex items-center gap-2">
        <span>{activeRunning ? "运行中" : anyRunning ? "后台运行中" : "就绪"}</span>
        <span
          className={`h-1.5 w-1.5 rounded-full ${
            anyRunning
              ? "bg-[var(--accent)] shadow-[0_0_6px_rgb(0_122_255_/_0.6)] animate-pulse"
              : "bg-zinc-400 dark:bg-zinc-600"
          }`}
        />
        <span className="font-mono-token text-zinc-400 dark:text-zinc-600">v{appVersion}</span>
      </div>
    </footer>
  );
}

function FooterBtn({ children, onClick }: { children: React.ReactNode; onClick: () => void }) {
  const { usesFluentUI } = usePlatform();
  return (
    <button
      type="button"
      onClick={onClick}
      className={`platform-pill inline-flex items-center gap-1 px-2.5 py-1 transition-colors hover:bg-black/[0.04] hover:text-zinc-900 dark:hover:bg-white/[0.06] dark:hover:text-zinc-200 ${usesFluentUI ? "rounded-[6px]" : "rounded-full"}`}
    >
      {children}
    </button>
  );
}
