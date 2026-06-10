import { useEffect, useRef, useState } from "react";
import {
  Bell, Download, Folder, FolderEdit, Github, Info, KeyRound,
  MessageSquare, Monitor, Moon, Network, RotateCw, Save, Sun, Trash2, Upload,
} from "lucide-react";
import { useStudioStore } from "../../state/studioStore";
import {
  GetOutputDir, OpenOutputDir, OpenExternalURL, ChooseOutputDir, SetOutputDir,
} from "../../platform/runtime/host";
import type { KernelRuntimeMode, ProxyMode, SystemNotificationPermissionState } from "../../types/domain";
import { DEFAULT_AUTO_RETRY_COUNT, MAX_AUTO_RETRY_COUNT } from "../../../../../shared/kernel/requestModel.js";
import { Modal } from "../common/Modal";
import { rememberTrustedOutputRoot } from "../../lib/storage";
import { scheduleCompatibilityExport } from "../../lib/compatState";
import { platformOutputRootLabel } from "../../platform";
import { androidTarget, openExternalURLForPlatform, openOutputLocationForPlatform } from "../../platform/android/bridge";
import { AndroidSettingsPanel } from "../../platform/android/settings/AndroidSettingsPanel";
import { usePlatform } from "../../platform/context";
import { AboutImageStudioModal } from "./AboutImageStudioModal";
import {
  SettingsAnchorNav,
  SettingsRow,
  SettingsSection,
  SettingsSegButton,
} from "./settingsPrimitives";
import { importCompletionSoundFile } from "../../lib/completionSound";

const REPO_URL = "https://github.com/RoseKhlifa/Image-Studio";
const RELEASES_URL = "https://github.com/RoseKhlifa/Image-Studio/releases";
const ISSUES_URL = "https://github.com/RoseKhlifa/Image-Studio/issues";
const LICENSE_URL = "https://www.gnu.org/licenses/agpl-3.0.html";

type DesktopSettingsSectionId =
  | "runtime"
  | "files"
  | "alerts"
  | "appearance"
  | "data"
  | "about";

const DESKTOP_SETTINGS_SECTIONS: ReadonlyArray<{
  id: DesktopSettingsSectionId;
  title: string;
  description: string;
}> = [
  {
    id: "runtime",
    title: "运行与网络",
    description: "内核、代理、自动重试与流式预览策略",
  },
  {
    id: "files",
    title: "输出与缓存",
    description: "保存位置、保存提示、日志与退出清理",
  },
  {
    id: "alerts",
    title: "通知与提示",
    description: "完成提示音与系统通知权限",
  },
  {
    id: "appearance",
    title: "外观与字号",
    description: "主题切换与界面字体缩放",
  },
  {
    id: "data",
    title: "数据与重置",
    description: "历史导入导出、本地清理与安全重置",
  },
  {
    id: "about",
    title: "关于与反馈",
    description: "版本入口、仓库反馈与许可证信息",
  },
];

export function SettingsPanel({ open, onClose }: { open: boolean; onClose: () => void }) {
  const {
    kernelRuntimeMode,
    proxyMode, proxyURL,
    autoRetryEnabled,
    autoRetryCount,
    protectStreamPreview,
    theme, fontScale,
    setField, setAPIKey, setProxyConfig,
    history,
    loadMoreHistory,
    exportHistory, importHistory,
    pruneHistoryOlderThanDays,
    setTheme, setFontScale,
    pushToast,
    apiKey, baseURL, apiMode,
    profiles, activeProfileId, setActiveProfile,
    openUpstreamConfig, testAPIKey, isTestingKey,
    savePromptSuppressed, setSavePromptSuppressed,
    keepLogs, setKeepLogs,
    cleanupPreviewCacheOnExit, setCleanupPreviewCacheOnExit,
    completionSound,
    completionNotification,
    completionNotificationPermission,
    setCompletionSoundEnabled,
    setCompletionSoundMode,
    setCompletionSoundCustom,
    resetCompletionSoundCustom,
    previewCompletionSound,
    setCompletionNotificationEnabled,
    requestCompletionNotificationPermission,
  } = useStudioStore();

  const [outputDir, setOutputDir] = useState("");
  const [aboutOpen, setAboutOpen] = useState(false);
  const [activeDesktopSection, setActiveDesktopSection] = useState<DesktopSettingsSectionId>(DESKTOP_SETTINGS_SECTIONS[0].id);
  const desktopScrollBodyRef = useRef<HTMLDivElement | null>(null);
  const desktopSectionRefs = useRef<Record<DesktopSettingsSectionId, HTMLElement | null>>({
    runtime: null,
    files: null,
    alerts: null,
    appearance: null,
    data: null,
    about: null,
  });
  const { isMac, usesFluentUI, isAndroid, isAndroidPad } = usePlatform();

  useEffect(() => {
    if (!open) return;
    GetOutputDir().then(setOutputDir).catch(() => undefined);
  }, [open]);

  useEffect(() => {
    if (!open || isAndroid) return;
    setActiveDesktopSection(DESKTOP_SETTINGS_SECTIONS[0].id);
    if (desktopScrollBodyRef.current) desktopScrollBodyRef.current.scrollTop = 0;
  }, [isAndroid, open]);

  useEffect(() => {
    if (!open || isAndroid) return;
    const container = desktopScrollBodyRef.current;
    if (!container) return;

    const syncActiveSection = () => {
      const scrollTop = container.scrollTop + 96;
      let currentSection = DESKTOP_SETTINGS_SECTIONS[0].id;
      for (const section of DESKTOP_SETTINGS_SECTIONS) {
        const sectionNode = desktopSectionRefs.current[section.id];
        if (sectionNode && sectionNode.offsetTop <= scrollTop) {
          currentSection = section.id;
        }
      }
      setActiveDesktopSection((previous) => (previous === currentSection ? previous : currentSection));
    };

    syncActiveSection();
    container.addEventListener("scroll", syncActiveSection, { passive: true });
    window.addEventListener("resize", syncActiveSection);
    return () => {
      container.removeEventListener("scroll", syncActiveSection);
      window.removeEventListener("resize", syncActiveSection);
    };
  }, [isAndroid, open]);

  async function clearAPIKey() {
    if (!confirm("确定清除已保存的 API Key 吗?")) return;
    try {
      await setAPIKey("");
      pushToast("已清除安全存储中的 API Key", "success");
    } catch (e: any) {
      pushToast(`清除失败:${e?.message ?? e}`, "error", 5000);
    }
  }

  async function clearHistory() {
    await loadMoreHistory();
    const loadedHistory = useStudioStore.getState().history;
    if (!confirm(`确定清除 ${loadedHistory.length} 条历史记录吗?(本地数据库也会删除)`)) return;
    for (const h of loadedHistory) {
      await useStudioStore.getState().deleteHistoryItem(h.id);
    }
  }

  async function pruneHistory(days: number) {
    const removed = await pruneHistoryOlderThanDays(days);
    if (removed > 0) pushToast(`已清理 ${removed} 条 ${days} 天前的历史`, "success");
    else pushToast(`没有 ${days} 天前的历史需要清理`, "info");
  }

  function openOutputLocation() {
    openOutputLocationForPlatform(OpenOutputDir).catch((e) => pushToast(e?.message ?? "无法打开保存位置", "warn"));
  }

  function openExternal(url: string) {
    openExternalURLForPlatform(url, OpenExternalURL).catch(() => undefined);
  }

  function updateSavePromptSuppressed(value: boolean) {
    setSavePromptSuppressed(value);
    scheduleCompatibilityExport(useStudioStore.getState());
    pushToast(value ? "已关闭生成后保存提示" : "已开启生成后保存提示", "success");
  }

  async function updateKeepLogs(value: boolean) {
    await setKeepLogs(value);
    pushToast(value ? "已开启日志保留" : "已关闭日志保留，退出应用后会自动清理 log", "success");
  }

  async function updateCleanupPreviewCacheOnExit(value: boolean) {
    await setCleanupPreviewCacheOnExit(value);
    pushToast(
      value ? "已开启退出时清理预览缓存" : "已关闭退出时清理预览缓存",
      "success",
    );
  }

  async function chooseCompletionSoundFile() {
    if (typeof document === "undefined") return;
    const input = document.createElement("input");
    input.type = "file";
    input.accept = "audio/*,.mp3,.wav,.ogg,.m4a,.aac,.webm";
    input.style.position = "fixed";
    input.style.left = "-9999px";
    document.body.appendChild(input);
    input.addEventListener("change", () => {
      const file = input.files?.[0];
      input.remove();
      if (!file) return;
      void (async () => {
        try {
          const imported = await importCompletionSoundFile(file);
          setCompletionSoundCustom(imported);
          pushToast(`已设置自定义提示音: ${imported.name}`, "success");
        } catch (e: any) {
          pushToast(e?.message ?? "导入提示音失败", "error", 5000);
        }
      })();
    }, { once: true });
    input.click();
  }

  function systemNotificationPermissionLabel(permission: SystemNotificationPermissionState): string {
    if (permission === "granted") return "已允许";
    if (permission === "denied") return "已拒绝";
    if (permission === "unsupported") return "当前平台不支持";
    return "尚未授权";
  }

  async function updateCompletionNotificationEnabled(value: boolean) {
    const permission = await setCompletionNotificationEnabled(value);
    if (!value) {
      pushToast("已关闭系统通知", "success");
      return;
    }
    if (permission === "granted") {
      pushToast("已开启系统通知", "success");
      return;
    }
    if (permission === "unsupported") {
      pushToast("当前平台不支持系统通知", "warn", 5000);
      return;
    }
    if (permission === "denied") {
      pushToast("系统通知权限已被拒绝，请在系统设置中允许后重试", "warn", 6000);
      return;
    }
    pushToast("请先允许系统通知权限", "warn");
  }

  async function askCompletionNotificationPermission() {
    const permission = await requestCompletionNotificationPermission();
    if (permission === "granted") {
      pushToast("系统通知权限已授权", "success");
    } else if (permission === "denied") {
      pushToast("系统通知权限已被拒绝，请在系统设置中允许后重试", "warn", 6000);
    } else if (permission === "unsupported") {
      pushToast("当前平台不支持系统通知", "warn", 5000);
    } else {
      pushToast("系统通知权限仍未授权", "warn");
    }
  }

  function closeSettings() {
    setAboutOpen(false);
    onClose();
  }

  function setDesktopSectionRef(id: DesktopSettingsSectionId) {
    return (node: HTMLElement | null) => {
      desktopSectionRefs.current[id] = node;
    };
  }

  function scrollToDesktopSection(id: DesktopSettingsSectionId) {
    const container = desktopScrollBodyRef.current;
    const sectionNode = desktopSectionRefs.current[id];
    if (!container || !sectionNode) return;
    sectionNode.scrollIntoView({ block: "start", behavior: "smooth" });
    container.scrollTop = Math.max(sectionNode.offsetTop - 8, 0);
    setActiveDesktopSection(id);
  }

  const outputLabel = androidTarget.isAndroid ? platformOutputRootLabel() : (outputDir || "...");
  const activeProfile = profiles.find((profile) => profile.id === activeProfileId);
  const upstreamReady = !!apiKey.trim() && !!baseURL.trim();
  const segmentedControlClassName = `platform-seg flex flex-wrap gap-1 bg-black/[0.04] p-0.5 ring-1 ring-black/[0.05] dark:bg-white/[0.06] dark:ring-white/[0.06] ${usesFluentUI ? "rounded-[10px]" : "rounded-[18px]"}`;
  const actionButtonBaseClassName = `inline-flex min-h-[34px] items-center justify-center gap-1.5 border border-black/[0.08] px-3 ${isMac ? "py-2.5 text-[13px]" : "py-2 text-[12px]"} font-medium transition-colors dark:border-white/[0.08] ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`;
  const actionButtonPrimaryClassName = `${actionButtonBaseClassName} text-zinc-700 hover:border-[color:var(--accent)]/35 hover:text-[var(--accent)] dark:text-zinc-300`;
  const actionButtonSecondaryClassName = `${actionButtonBaseClassName} text-zinc-500 hover:border-[color:var(--accent)]/35 hover:text-[var(--accent)] dark:text-zinc-300`;
  const actionButtonDangerClassName = `${actionButtonBaseClassName} text-zinc-500 hover:border-red-400/40 hover:text-red-400 dark:text-zinc-300`;

  const androidSettings = isAndroid ? (
    <AndroidSettingsPanel
      activeProfile={activeProfile}
      activeProfileId={activeProfileId}
      apiMode={apiMode}
      clearAPIKey={() => void clearAPIKey()}
      clearHistory={() => void clearHistory()}
      exportHistory={() => void exportHistory()}
      fontScale={fontScale}
      historyCount={history.length}
      importHistory={() => void importHistory()}
      isTestingKey={isTestingKey}
      autoRetryEnabled={autoRetryEnabled}
      autoRetryCount={autoRetryCount}
      protectStreamPreview={protectStreamPreview}
      kernelRuntimeMode={kernelRuntimeMode}
      onOpenAbout={() => setAboutOpen(true)}
      onOpenFeedback={() => openExternal(ISSUES_URL)}
      onOpenRepo={() => openExternal(REPO_URL)}
      onOpenUpstream={() => openUpstreamConfig("settings")}
      onPreviewCompletionSound={() => void previewCompletionSound()}
      onResetCompletionSound={() => {
        resetCompletionSoundCustom();
        pushToast("已恢复默认提示音", "success");
      }}
      onSelectCompletionSound={() => void chooseCompletionSoundFile()}
      onSetActiveProfile={(id) => {
        if (id) void setActiveProfile(id);
      }}
      onSetCompletionSoundEnabled={(value) => {
        setCompletionSoundEnabled(value);
        pushToast(value ? "已开启完成提示音" : "已关闭完成提示音", "success");
      }}
      onSetCompletionSoundMode={(value) => {
        setCompletionSoundMode(value);
        pushToast(value === "custom" ? "已切换到自定义提示音" : "已切换到默认提示音", "success");
      }}
      onSetFontScale={setFontScale}
      onSetKernelRuntimeMode={(value) => setField("kernelRuntimeMode", value)}
      onSetAutoRetryEnabled={(value) => {
        setField("autoRetryEnabled", value);
        scheduleCompatibilityExport(useStudioStore.getState());
        pushToast(value ? "已开启自动重试" : "已关闭自动重试", "success");
      }}
      onSetAutoRetryCount={(value) => setField("autoRetryCount", value)}
      onSetProtectStreamPreview={(value) => {
        setField("protectStreamPreview", value);
        scheduleCompatibilityExport(useStudioStore.getState());
        pushToast(value ? "已开启流式预览保护" : "已关闭流式预览保护", "success");
      }}
      onSetCleanupPreviewCacheOnExit={(value) => void updateCleanupPreviewCacheOnExit(value)}
      onSetProxyConfig={setProxyConfig}
      onSetSavePromptSuppressed={updateSavePromptSuppressed}
      onSetTheme={setTheme}
      completionSound={completionSound}
      openOutputLocation={openOutputLocation}
      outputLabel={outputLabel}
      profiles={profiles}
      proxyMode={proxyMode}
      proxyURL={proxyURL}
      pruneHistory={(days) => void pruneHistory(days)}
      cleanupPreviewCacheOnExit={cleanupPreviewCacheOnExit}
      savePromptSuppressed={savePromptSuppressed}
      surface={isAndroidPad ? "pad" : "phone"}
      testAPIKey={() => void testAPIKey()}
      theme={theme}
      upstreamReady={upstreamReady}
    />
  ) : null;

  const desktopSettings = (
    <div className="grid items-start gap-4 lg:grid-cols-[220px_minmax(0,1fr)]">
      <SettingsAnchorNav
        sections={DESKTOP_SETTINGS_SECTIONS}
        activeId={activeDesktopSection}
        onSelect={(id) => scrollToDesktopSection(id as DesktopSettingsSectionId)}
      />

      <div className="min-w-0 flex flex-col gap-6">
        <SettingsSection
          id="settings-runtime"
          ref={setDesktopSectionRef("runtime")}
          title="运行与网络"
          description="把请求链路相关的设置放在一起，方便快速切换内核、代理和自动重试策略。"
        >
          <SettingsRow label="内核执行">
            <select
              value={kernelRuntimeMode}
              onChange={(e) => setField("kernelRuntimeMode", e.target.value as KernelRuntimeMode)}
              className={`focus-ring w-full border border-black/[0.08] bg-[var(--surface)] px-3 ${isMac ? "min-h-[44px] py-3 text-[14px]" : "py-2.5 text-[12px]"} text-zinc-900 dark:border-white/[0.08] dark:text-zinc-100 ${usesFluentUI ? "rounded-[10px]" : "rounded-[16px]"}`}
            >
              <option value="auto">auto(按宿主自动选择)</option>
              <option value="local">local(桌面 Go/Wails)</option>
              <option value="remote">remote(共享远程内核)</option>
            </select>
            <p className="mt-1 text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-300">
              桌面可切到 remote 验证与 Android / Worker 是否走同一套共享请求内核。
            </p>
          </SettingsRow>

          <SettingsRow label="代理服务器">
            <div className={segmentedControlClassName}>
              {([
                ["none", "不使用"],
                ["system", "系统配置"],
                ["custom", "自定义"],
              ] as Array<[ProxyMode, string]>).map(([value, label]) => (
                <SettingsSegButton key={value} active={proxyMode === value} onClick={() => setProxyConfig(value)}>
                  {value === "custom" ? <Network className="w-3 h-3" /> : null}
                  {label}
                </SettingsSegButton>
              ))}
            </div>
            {proxyMode === "custom" ? (
              <input
                value={proxyURL}
                onChange={(e) => setProxyConfig("custom", e.target.value)}
                placeholder="http://127.0.0.1:7890"
                className={`focus-ring mt-2 w-full border border-black/[0.08] bg-[var(--surface)] px-3 ${isMac ? "min-h-[42px] py-2.5 text-[13px]" : "py-2.5 text-[12px]"} font-mono-token text-zinc-900 placeholder:text-zinc-400 dark:border-white/[0.08] dark:text-zinc-100 ${usesFluentUI ? "rounded-[8px]" : "rounded-[14px]"}`}
              />
            ) : null}
            <p className="mt-1 text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-300">
              默认使用系统配置；自定义地址支持 http:// 和 https://。
            </p>
          </SettingsRow>

          <SettingsRow label="失败自动重试">
            <div className={segmentedControlClassName}>
              <SettingsSegButton active={autoRetryEnabled} onClick={() => {
                setField("autoRetryEnabled", true);
                scheduleCompatibilityExport(useStudioStore.getState());
                pushToast("已开启自动重试", "success");
              }}>
                开启
              </SettingsSegButton>
              <SettingsSegButton active={!autoRetryEnabled} onClick={() => {
                setField("autoRetryEnabled", false);
                scheduleCompatibilityExport(useStudioStore.getState());
                pushToast("已关闭自动重试", "success");
              }}>
                关闭
              </SettingsSegButton>
            </div>
            <p className="mt-1 text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-300">
              默认开启。当前会对 403 / 502 / 503 / 504 / 524 以及可重试网络抖动自动重发请求；关闭后只保留第一次结果。
            </p>
            <div className="mt-2 flex items-center gap-3">
              <input
                type="range"
                min={1}
                max={MAX_AUTO_RETRY_COUNT}
                step={1}
                value={autoRetryCount}
                onChange={(e) => setField("autoRetryCount", Number(e.target.value))}
                onMouseUp={() => {
                  const value = useStudioStore.getState().autoRetryCount;
                  scheduleCompatibilityExport(useStudioStore.getState());
                  pushToast(`自动重试次数已设为 ${value} 次`, "success");
                }}
                onTouchEnd={() => {
                  const value = useStudioStore.getState().autoRetryCount;
                  scheduleCompatibilityExport(useStudioStore.getState());
                  pushToast(`自动重试次数已设为 ${value} 次`, "success");
                }}
                className="focus-ring flex-1"
              />
              <div className="min-w-[88px] rounded-[14px] border border-black/[0.08] bg-[var(--surface)] px-3 py-2 text-center text-[12px] font-medium text-zinc-800 dark:border-white/[0.08] dark:text-zinc-100">
                {autoRetryCount} 次
              </div>
            </div>
            <p className="mt-1 text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-300">
              默认 {DEFAULT_AUTO_RETRY_COUNT} 次，不含首次请求。
            </p>
          </SettingsRow>

          <SettingsRow label="流式预览保护">
            <div className={segmentedControlClassName}>
              <SettingsSegButton active={protectStreamPreview} onClick={() => {
                setField("protectStreamPreview", true);
                scheduleCompatibilityExport(useStudioStore.getState());
                pushToast("已开启流式预览保护", "success");
              }}>
                开启
              </SettingsSegButton>
              <SettingsSegButton active={!protectStreamPreview} onClick={() => {
                setField("protectStreamPreview", false);
                scheduleCompatibilityExport(useStudioStore.getState());
                pushToast("已关闭流式预览保护", "success");
              }}>
                关闭
              </SettingsSegButton>
            </div>
            <p className="mt-1 text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-300">
              默认开启。高并发或 Android 大尺寸任务时，会自动关闭流式预览，优先保证最终图完整；关闭后严格按你设置的预览帧数请求。
            </p>
          </SettingsRow>
        </SettingsSection>

        <SettingsSection
          id="settings-files"
          ref={setDesktopSectionRef("files")}
          title="输出与缓存"
          description="统一管理结果保存路径、保存确认和退出时的清理行为。"
        >
          <SettingsRow label="输出目录">
            <div className={`flex items-center gap-1 border border-black/[0.08] bg-[var(--surface)] px-3 ${isMac ? "py-3" : "py-2.5"} dark:border-white/[0.08] ${usesFluentUI ? "rounded-[10px]" : "rounded-[16px]"}`}>
              <span title={outputDir} className={`flex-1 truncate font-mono-token text-zinc-700 dark:text-zinc-200 ${isMac ? "text-[13px]" : "text-[12px]"}`}>
                {outputDir || "..."}
              </span>
              <button
                onClick={openOutputLocation}
                title="在系统文件管理器中打开"
                className={`p-1 text-zinc-500 hover:bg-[var(--accent-soft)] hover:text-[var(--accent)] ${usesFluentUI ? "rounded-[6px]" : "rounded-full"}`}
              >
                <Folder className="w-3.5 h-3.5" />
              </button>
            </div>
            <div className="mt-1.5 flex gap-1.5">
              <button
                onClick={async () => {
                  try {
                    const chosen = await ChooseOutputDir();
                    if (chosen) {
                      try { localStorage.setItem("gptcodex.outputDir", chosen); } catch {}
                      rememberTrustedOutputRoot(chosen);
                      setOutputDir(chosen);
                      scheduleCompatibilityExport(useStudioStore.getState());
                      pushToast(`输出目录已切换:${chosen}`, "success");
                    }
                  } catch (e: any) {
                    pushToast(`切换失败:${e?.message ?? e}`, "error", 5000);
                  }
                }}
                className={`flex-1 ${actionButtonPrimaryClassName}`}
              >
                <FolderEdit className="w-3 h-3" /> 修改
              </button>
              <button
                onClick={async () => {
                  try {
                    await SetOutputDir("");
                    try { localStorage.removeItem("gptcodex.outputDir"); } catch {}
                    const def = await GetOutputDir();
                    rememberTrustedOutputRoot(def);
                    setOutputDir(def);
                    scheduleCompatibilityExport(useStudioStore.getState());
                    pushToast("已恢复默认输出目录", "success");
                  } catch (e: any) {
                    pushToast(`重置失败:${e?.message ?? e}`, "error", 5000);
                  }
                }}
                title={`清除自定义路径,回到 ${platformOutputRootLabel()}/images`}
                className={actionButtonSecondaryClassName}
              >
                <RotateCw className="w-3 h-3" /> 默认
              </button>
            </div>
            <p className="mt-1 text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-300">
              当前目录下会保存原图、结果图以及必要的会话输出。
            </p>
          </SettingsRow>

          <SettingsRow label="生成后保存提示">
            <div className={segmentedControlClassName}>
              <SettingsSegButton active={!savePromptSuppressed} onClick={() => updateSavePromptSuppressed(false)}>
                <Save className="w-3 h-3" /> 提示
              </SettingsSegButton>
              <SettingsSegButton active={savePromptSuppressed} onClick={() => updateSavePromptSuppressed(true)}>
                不提示
              </SettingsSegButton>
            </div>
            <p className="mt-1 text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-300">
              生成完成后询问是否另存到指定位置。
            </p>
          </SettingsRow>

          <SettingsRow label="日志保留">
            <div className={segmentedControlClassName}>
              <SettingsSegButton active={!keepLogs} onClick={() => void updateKeepLogs(false)}>
                关闭
              </SettingsSegButton>
              <SettingsSegButton active={keepLogs} onClick={() => void updateKeepLogs(true)}>
                开启
              </SettingsSegButton>
            </div>
            <p className="mt-1 text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-300">
              默认关闭。关闭时当前会话仍可查看原始响应，退出应用后会自动清理输出目录中的 log。
            </p>
          </SettingsRow>

          <SettingsRow label="退出时清理预览缓存">
            <div className={segmentedControlClassName}>
              <SettingsSegButton active={!cleanupPreviewCacheOnExit} onClick={() => void updateCleanupPreviewCacheOnExit(false)}>
                关闭
              </SettingsSegButton>
              <SettingsSegButton active={cleanupPreviewCacheOnExit} onClick={() => void updateCleanupPreviewCacheOnExit(true)}>
                开启
              </SettingsSegButton>
            </div>
            <p className="mt-1 text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-300">
              默认关闭。开启后，退出应用时会删除可重建的预览图和缩略图缓存；源图、结果图和历史记录不会删除。
            </p>
          </SettingsRow>
        </SettingsSection>

        <SettingsSection
          id="settings-alerts"
          ref={setDesktopSectionRef("alerts")}
          title="通知与提示"
          description="完成整批生成时的声音和系统级通知统一放在这里。"
        >
          <SettingsRow label="完成提示音">
            <div className={segmentedControlClassName}>
              <SettingsSegButton active={completionSound.enabled} onClick={() => {
                setCompletionSoundEnabled(true);
                pushToast("已开启完成提示音", "success");
              }}>
                <Bell className="w-3 h-3" /> 开启
              </SettingsSegButton>
              <SettingsSegButton active={!completionSound.enabled} onClick={() => {
                setCompletionSoundEnabled(false);
                pushToast("已关闭完成提示音", "success");
              }}>
                关闭
              </SettingsSegButton>
            </div>
            <div className={`mt-2 ${segmentedControlClassName}`}>
              <SettingsSegButton active={completionSound.mode === "default"} onClick={() => {
                setCompletionSoundMode("default");
                pushToast("已切换到默认提示音", "success");
              }}>
                默认音
              </SettingsSegButton>
              <SettingsSegButton active={completionSound.mode === "custom"} onClick={() => {
                if (!completionSound.customDataURL) {
                  void chooseCompletionSoundFile();
                  return;
                }
                setCompletionSoundMode("custom");
                pushToast("已切换到自定义提示音", "success");
              }}>
                自定义
              </SettingsSegButton>
            </div>
            <div className="mt-2 flex gap-1.5">
              <button onClick={() => void previewCompletionSound()} className={`flex-1 ${actionButtonPrimaryClassName}`}>
                试听
              </button>
              <button onClick={() => void chooseCompletionSoundFile()} className={`flex-1 ${actionButtonPrimaryClassName}`}>
                导入音频
              </button>
              <button
                onClick={() => {
                  resetCompletionSoundCustom();
                  pushToast("已恢复默认提示音", "success");
                }}
                className={actionButtonSecondaryClassName}
              >
                默认
              </button>
            </div>
            <p className="mt-1 text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-300">
              整批生成全部完成后只播放一次。当前 {completionSound.mode === "custom" && completionSound.customName ? `使用 ${completionSound.customName}` : "使用内置默认音"}。
            </p>
          </SettingsRow>

          <SettingsRow label="系统通知">
            <div className={segmentedControlClassName}>
              <SettingsSegButton active={completionNotification.enabled} onClick={() => void updateCompletionNotificationEnabled(true)}>
                <Bell className="w-3 h-3" /> 开启
              </SettingsSegButton>
              <SettingsSegButton active={!completionNotification.enabled} onClick={() => void updateCompletionNotificationEnabled(false)}>
                关闭
              </SettingsSegButton>
            </div>
            <div className="mt-2 flex gap-1.5">
              <button onClick={() => void askCompletionNotificationPermission()} className={`flex-1 ${actionButtonPrimaryClassName}`}>
                {completionNotificationPermission === "granted" ? "重新检查权限" : "申请权限"}
              </button>
            </div>
            <p className="mt-1 text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-300">
              整批任务全部完成后，仅在窗口不在前台时发送系统通知。当前权限：{systemNotificationPermissionLabel(completionNotificationPermission)}。
            </p>
          </SettingsRow>
        </SettingsSection>

        <SettingsSection
          id="settings-appearance"
          ref={setDesktopSectionRef("appearance")}
          title="外观与字号"
          description="界面主题和显示密度放在一起，切换后更容易感知整体变化。"
        >
          <SettingsRow label="主题">
            <div className={segmentedControlClassName}>
              <SettingsSegButton active={theme === "system"} onClick={() => setTheme("system")}>
                <Monitor className="w-3 h-3" /> 系统
              </SettingsSegButton>
              <SettingsSegButton active={theme === "dark"} onClick={() => setTheme("dark")}>
                <Moon className="w-3 h-3" /> 深色
              </SettingsSegButton>
              <SettingsSegButton active={theme === "light"} onClick={() => setTheme("light")}>
                <Sun className="w-3 h-3" /> 浅色
              </SettingsSegButton>
            </div>
          </SettingsRow>

          <SettingsRow label={`字号 ${Math.round(fontScale * 100)}%`}>
            <div className={segmentedControlClassName}>
              {[0.85, 1, 1.15].map((v) => (
                <SettingsSegButton key={v} active={Math.abs(fontScale - v) < 0.01} onClick={() => setFontScale(v)}>
                  {v === 0.85 ? "小" : v === 1 ? "中" : "大"}
                </SettingsSegButton>
              ))}
            </div>
          </SettingsRow>
        </SettingsSection>

        <SettingsSection
          id="settings-data"
          ref={setDesktopSectionRef("data")}
          title="数据与重置"
          description="导入导出历史、清理旧数据和重置本地凭据都集中在这一组。"
        >
          <SettingsRow label="历史记录">
            <div className="flex gap-1.5">
              <button onClick={exportHistory} title="导出全部历史为 JSON" className={`flex-1 ${actionButtonPrimaryClassName}`}>
                <Upload className="w-3 h-3" /> 导出历史
              </button>
              <button onClick={importHistory} title="从 JSON 文件导入" className={`flex-1 ${actionButtonPrimaryClassName}`}>
                <Download className="w-3 h-3" /> 导入历史
              </button>
            </div>
            <p className="mt-1 text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-300">
              可导出本地历史做备份，或从 JSON 文件恢复到当前设备。
            </p>
          </SettingsRow>

          <SettingsRow label="清理与重置">
            <div className="flex gap-1.5">
              <button onClick={clearAPIKey} className={`flex-1 ${actionButtonDangerClassName}`}>
                <KeyRound className="w-3 h-3" /> 清除 API Key
              </button>
              <button onClick={clearHistory} className={`flex-1 ${actionButtonDangerClassName}`}>
                <Trash2 className="w-3 h-3" /> 清空历史
              </button>
            </div>
            <div className="mt-1.5 flex gap-1.5">
              <button onClick={() => pruneHistory(3)} className={`flex-1 ${actionButtonSecondaryClassName}`}>
                清理 3 天前
              </button>
              <button onClick={() => pruneHistory(7)} className={`flex-1 ${actionButtonSecondaryClassName}`}>
                清理 7 天前
              </button>
            </div>
            <p className="mt-1 text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-300">
              清除 API Key 只影响本机安全存储；历史清理会同步删除本地数据库记录。
            </p>
          </SettingsRow>
        </SettingsSection>

        <SettingsSection
          id="settings-about"
          ref={setDesktopSectionRef("about")}
          title="关于与反馈"
          description="项目说明、更新入口和问题反馈分开陈列，避免和日常设置混在一起。"
        >
          <SettingsRow label="关于 Image Studio">
            <button onClick={() => setAboutOpen(true)} className={actionButtonSecondaryClassName}>
              <Info className="w-3 h-3" /> 查看关于信息
            </button>
            <p className="mt-1 text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-300">
              查看版本信息、许可证说明以及项目简介。
            </p>
          </SettingsRow>

          <SettingsRow label="支持与反馈">
            <div className="flex gap-1.5">
              <button onClick={() => openExternal(RELEASES_URL)} className={`flex-1 ${actionButtonPrimaryClassName}`}>
                <Github className="w-3 h-3" /> 更新
              </button>
              <button onClick={() => openExternal(ISSUES_URL)} className={`flex-1 ${actionButtonPrimaryClassName}`}>
                <MessageSquare className="w-3 h-3" /> 反馈
              </button>
            </div>
          </SettingsRow>
        </SettingsSection>
      </div>
    </div>
  );

  return (
    <>
      <Modal
        open={open}
        onClose={closeSettings}
        title="设置"
        width={isAndroidPad ? 1040 : (isAndroid ? 540 : 920)}
        backdropClassName={isAndroid ? "android-settings-modal-backdrop" : ""}
        cardClassName={isAndroid ? "android-settings-modal-card" : "max-w-[calc(100vw-40px)]"}
        headerClassName={isAndroid ? "android-settings-modal-header" : ""}
        bodyClassName={isAndroid ? "android-settings-modal-body" : ""}
        bodyRef={isAndroid ? undefined : desktopScrollBodyRef}
      >
        {androidSettings ?? desktopSettings}
      </Modal>

      <AboutImageStudioModal
        open={aboutOpen}
        onClose={() => setAboutOpen(false)}
        onOpenFeedback={() => openExternal(REPO_URL + "/issues")}
        onOpenLicense={() => openExternal(LICENSE_URL)}
        onOpenRepo={() => openExternal(REPO_URL)}
        licenseLabel="GNU AGPL v3.0"
      />
    </>
  );
}
