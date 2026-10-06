import { Github, Monitor, Moon, Plus, Settings, Star, Sun } from "lucide-react";
import { useStudioStore } from "../../state/studioStore";
import { useStudioFields } from "../../state/useStudioFields";
import { OpenExternalURL } from "../../platform/runtime/host";
import { usePlatform } from "../../platform/context";
import { openExternalURLForPlatform } from "../../platform/android/bridge";
import { AppHeaderBrand } from "./AppHeaderBrand";
import { HeaderIconBtn, HeaderToggleBtn } from "./headerPrimitives";

const REPO_URL = "https://github.com/RoseKhlifa/Image-Studio";

export function AppHeader({ onOpenSettings }: { onOpenSettings: () => void }) {
  const { fullscreen, theme, setTheme, pushToast, newWorkspace, openStarPrompt } = useStudioFields([
    "fullscreen", "theme", "setTheme", "pushToast",
    "newWorkspace", "openStarPrompt",
  ]);
  const workspaceCount = useStudioStore((state) => state.workspaces.length);
  const { isAndroid, isAndroidPhone, isAndroidPad, isMac, usesFluentUI, usesAndroidUI, usesAppleUI } = usePlatform();
  if (fullscreen) return null;

  return (
    <header
      className={`drag-region app-header sticky top-0 z-40 flex items-center gap-3 border-b border-[var(--border)] bg-[var(--toolbar)] backdrop-blur-2xl ${
        usesAppleUI ? "liquid-glass-bar" : ""
      } ${
        usesAndroidUI
          ? "android-app-header min-h-[46px] px-[calc(env(safe-area-inset-left,0px)+14px)] pr-[calc(env(safe-area-inset-right,0px)+14px)] pt-[calc(env(safe-area-inset-top,0px)+2px)] pb-1"
          :
        usesAppleUI
          ? `${isMac ? "mac-app-header" : ""} min-h-[64px] px-5 pb-2 pt-3`
          : usesFluentUI
            ? "min-h-[48px] px-3"
            : "min-h-12 px-4"
      }`}
    >
      <div className={`min-w-0 flex-1 ${usesAndroidUI ? "android-header-copy" : ""} ${isMac ? "mac-header-copy" : ""}`}>
        <AppHeaderBrand />
      </div>

      <div className={`no-drag ml-auto flex items-center shrink-0 ${usesAndroidUI ? "android-header-actions" : ""} ${isMac ? "mac-header-actions" : ""} ${usesFluentUI ? "gap-1" : isMac ? "gap-2" : "gap-1.5"}`}>
        {!isAndroid && <HeaderIconBtn
          onClick={() => newWorkspace()}
          title={workspaceCount > 1 ? `${workspaceCount} 个标签 · 新建` : "新建标签"}
        >
          <Plus className="h-4 w-4" />
          {workspaceCount > 1 && (
            <span className="absolute right-0 top-0 flex h-3.5 min-w-[14px] items-center justify-center rounded-full bg-[var(--accent)] px-[3px] text-[8px] font-semibold leading-none text-white shadow-sm">
              {workspaceCount}
            </span>
          )}
        </HeaderIconBtn>}
        {!isAndroid && <div className={`platform-seg flex items-center p-0.5 ring-1 ${
          usesFluentUI
            ? "bg-white/66 ring-black/[0.08] dark:bg-white/[0.04] dark:ring-white/[0.08]"
            : "rounded-full bg-black/[0.04] ring-black/[0.05] dark:bg-white/[0.06] dark:ring-white/[0.06]"
        }`}>
          <HeaderToggleBtn
            active={theme === "system"}
            onClick={() => setTheme("system")}
            title="跟随系统"
          >
            <Monitor className="h-3.5 w-3.5" />
          </HeaderToggleBtn>
          <HeaderToggleBtn
            active={theme === "light"}
            onClick={() => setTheme("light")}
            title="浅色外观"
          >
            <Sun className="h-3.5 w-3.5" />
          </HeaderToggleBtn>
          <HeaderToggleBtn
            active={theme === "dark"}
            onClick={() => setTheme("dark")}
            title="深色外观"
          >
            <Moon className="h-3.5 w-3.5" />
          </HeaderToggleBtn>
        </div>}
        {!isAndroid && !isMac && <HeaderIconBtn
          onClick={() => openExternalURLForPlatform(REPO_URL, OpenExternalURL).catch(() => pushToast("无法打开浏览器", "error"))}
          title="GitHub"
        >
          <Github className="h-4 w-4" />
        </HeaderIconBtn>}
        {!isAndroid && !isMac && <HeaderIconBtn
          onClick={openStarPrompt}
          title="给项目点个 Star"
        >
          <Star className="h-4 w-4 text-amber-500 dark:text-amber-400" fill="currentColor" strokeWidth={1.5} />
        </HeaderIconBtn>}
        <HeaderIconBtn
          onClick={onOpenSettings}
          title="设置"
        >
          <Settings className="h-4 w-4" />
        </HeaderIconBtn>
      </div>
    </header>
  );
}
