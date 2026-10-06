import {
  DEFAULT_AUTO_RETRY_COUNT,
  MAX_AUTO_RETRY_COUNT,
} from "../../../../../../shared/kernel/requestModel.js";
import {
  Bell,
  ChevronRight,
  Database,
  Download,
  Folder,
  Github,
  Info,
  KeyRound,
  MessageSquare,
  Monitor,
  Moon,
  Network,
  PlugZap,
  Save,
  Shield,
  SlidersHorizontal,
  Sun,
  Trash2,
  Upload,
} from "lucide-react";
import type { CompletionSoundConfig, KernelRuntimeMode, ProxyMode, ThemeMode, UpstreamProfile } from "../../../types/domain";
import { androidSaveHint } from "../bridge";

export type AndroidSettingsSurface = "phone" | "pad";

export type AndroidSettingsPanelProps = {
  activeProfile: UpstreamProfile | undefined;
  activeProfileId: string;
  apiMode: "responses" | "images";
  clearAPIKey: () => void;
  clearHistory: () => void;
  completionSound: CompletionSoundConfig;
  exportHistory: () => void;
  fontScale: number;
  historyCount: number;
  importHistory: () => void;
  isTestingKey: boolean;
  autoRetryEnabled: boolean;
  autoRetryCount: number;
  protectStreamPreview: boolean;
  kernelRuntimeMode: KernelRuntimeMode;
  onOpenAbout: () => void;
  onOpenFeedback: () => void;
  onOpenRepo: () => void;
  onOpenUpstream: () => void;
  onPreviewCompletionSound: () => void;
  onResetCompletionSound: () => void;
  onSelectCompletionSound: () => void;
  onSetActiveProfile: (id: string) => void;
  onSetCompletionSoundEnabled: (value: boolean) => void;
  onSetCompletionSoundMode: (value: CompletionSoundConfig["mode"]) => void;
  onSetFontScale: (value: number) => void;
  onSetKernelRuntimeMode: (value: KernelRuntimeMode) => void;
  onSetAutoRetryEnabled: (value: boolean) => void;
  onSetAutoRetryCount: (value: number) => void;
  onSetProtectStreamPreview: (value: boolean) => void;
  onSetCleanupPreviewCacheOnExit: (value: boolean) => void;
  onSetProxyConfig: (mode: ProxyMode, url?: string) => void;
  onSetSavePromptSuppressed: (value: boolean) => void;
  onSetTheme: (value: ThemeMode) => void;
  openOutputLocation: () => void;
  outputLabel: string;
  profiles: UpstreamProfile[];
  proxyMode: ProxyMode;
  proxyURL: string;
  pruneHistory: (days: number) => void;
  cleanupPreviewCacheOnExit: boolean;
  savePromptSuppressed: boolean;
  surface: AndroidSettingsSurface;
  testAPIKey: () => void;
  theme: ThemeMode;
  upstreamReady: boolean;
};

const fontSizes = [
  { label: "小", value: 0.85 },
  { label: "中", value: 1 },
  { label: "大", value: 1.15 },
] as const;

function themeLabel(theme: ThemeMode) {
  if (theme === "dark") return "深色";
  if (theme === "light") return "浅色";
  return "跟随系统";
}

function runtimeLabel(mode: KernelRuntimeMode) {
  if (mode === "local") return "本地";
  if (mode === "remote") return "远程";
  return "自动";
}

function proxyLabel(mode: ProxyMode) {
  if (mode === "none") return "不使用";
  if (mode === "custom") return "自定义";
  return "系统配置";
}

export function AndroidSettingsPanel({
  activeProfile,
  activeProfileId,
  apiMode,
  clearAPIKey,
  clearHistory,
  completionSound,
  exportHistory,
  fontScale,
  historyCount,
  importHistory,
  isTestingKey,
  autoRetryEnabled,
  autoRetryCount,
  protectStreamPreview,
  kernelRuntimeMode,
  onOpenAbout,
  onOpenFeedback,
  onOpenRepo,
  onOpenUpstream,
  onPreviewCompletionSound,
  onResetCompletionSound,
  onSelectCompletionSound,
  onSetActiveProfile,
  onSetCompletionSoundEnabled,
  onSetCompletionSoundMode,
  onSetFontScale,
  onSetKernelRuntimeMode,
  onSetAutoRetryEnabled,
  onSetAutoRetryCount,
  onSetProtectStreamPreview,
  onSetCleanupPreviewCacheOnExit,
  onSetProxyConfig,
  onSetSavePromptSuppressed,
  onSetTheme,
  openOutputLocation,
  outputLabel,
  profiles,
  proxyMode,
  proxyURL,
  pruneHistory,
  cleanupPreviewCacheOnExit,
  savePromptSuppressed,
  surface,
  testAPIKey,
  theme,
  upstreamReady,
}: AndroidSettingsPanelProps) {
  const upstreamModeLabel = apiMode === "responses" ? "Responses API" : "Images API";
  const historyCountLabel = `${historyCount} 条`;
  const currentSummary = [
    upstreamReady ? "上游已配置" : "上游未配置",
    `代理 ${proxyLabel(proxyMode)}`,
    `主题 ${themeLabel(theme)}`,
    `字号 ${Math.round(fontScale * 100)}%`,
    savePromptSuppressed ? "保存提示 关" : "保存提示 开",
    cleanupPreviewCacheOnExit ? "预览缓存退出清理 开" : "预览缓存退出清理 关",
    autoRetryEnabled ? "自动重试 开" : "自动重试 关",
    `重试 ${autoRetryCount} 次`,
    protectStreamPreview ? "预览保护 开" : "预览保护 关",
    completionSound.enabled ? "提示音 开" : "提示音 关",
    `${historyCount} 条历史`,
  ];

  const heroSection = (
    <section className="android-settings-hero">
      <div className="android-settings-hero-orb">
        <SlidersHorizontal className="h-5 w-5" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="android-settings-kicker">Image Studio</div>
        <h2>偏好设置</h2>
        <p>{surface === "pad" ? "把运行、外观和本地数据分成左右两区，横屏触控不用来回滚动。" : "移动端常用控制集中在这里，上游配置仍保持独立入口。"}</p>
        <div className="android-settings-summary-strip" aria-label="设置概览">
          {currentSummary.map((item) => (
            <span key={item}>{item}</span>
          ))}
        </div>
      </div>
    </section>
  );

  const runtimeSection = (
    <section className="android-settings-card android-settings-card-runtime">
      <div className="android-settings-section-title">运行</div>
      <div className="android-settings-upstream-card">
        <div className="android-settings-upstream-head">
          <span className="android-settings-row-icon"><PlugZap className="h-4 w-4" /></span>
          <span className="min-w-0 flex-1">
            <span className="android-settings-field-title">上游配置</span>
            <span className="android-settings-field-subtitle">
              {activeProfile ? `${activeProfile.name} · ${upstreamModeLabel}` : "还没有可用上游配置"}
            </span>
          </span>
          <span className={`android-settings-status-pill ${upstreamReady ? "ready" : "missing"}`}>
            {upstreamReady ? "已配置" : "未配置"}
          </span>
        </div>
        {profiles.length > 0 ? (
          <select
            value={activeProfileId}
            onChange={(e) => onSetActiveProfile(e.target.value)}
            className="focus-ring android-settings-profile-select"
          >
            {profiles.map((profile) => (
              <option key={profile.id} value={profile.id}>
                {profile.name} · {profile.apiMode === "responses" ? "Responses" : "Images"}
              </option>
            ))}
          </select>
        ) : null}
        <div className="android-settings-action-grid android-settings-upstream-actions">
          <button type="button" onClick={onOpenUpstream}>管理配置</button>
          <button type="button" onClick={testAPIKey} disabled={!upstreamReady || isTestingKey}>
            {isTestingKey ? "检查中..." : "测试连通性"}
          </button>
        </div>
      </div>

      <div className="android-settings-field android-settings-field-stacked">
        <div>
          <span className="android-settings-field-title">内核执行</span>
          <span className="android-settings-field-subtitle">当前 {runtimeLabel(kernelRuntimeMode)}，默认自动选择。</span>
        </div>
        <div className="android-settings-segmented android-settings-runtime-segmented" role="group" aria-label="内核执行">
          {(["auto", "local", "remote"] as KernelRuntimeMode[]).map((value) => (
            <button
              key={value}
              type="button"
              className={kernelRuntimeMode === value ? "active" : ""}
              onClick={() => onSetKernelRuntimeMode(value)}
            >
              {runtimeLabel(value)}
            </button>
          ))}
        </div>
      </div>

      <div className="android-settings-field android-settings-field-stacked">
        <div>
          <span className="android-settings-field-title">代理服务器</span>
          <span className="android-settings-field-subtitle">当前 {proxyLabel(proxyMode)}，默认使用系统配置。</span>
        </div>
        <div className="android-settings-segmented" role="group" aria-label="代理服务器">
          {([
            ["none", "不用"],
            ["system", "系统"],
            ["custom", "自定义"],
          ] as Array<[ProxyMode, string]>).map(([value, label]) => (
            <button
              key={value}
              type="button"
              className={proxyMode === value ? "active" : ""}
              onClick={() => onSetProxyConfig(value)}
            >
              {value === "custom" ? <Network className="h-3.5 w-3.5" /> : null}
              {label}
            </button>
          ))}
        </div>
        {proxyMode === "custom" ? (
          <input
            value={proxyURL}
            onChange={(e) => onSetProxyConfig("custom", e.currentTarget.value)}
            className="focus-ring android-settings-profile-select"
            placeholder="http://127.0.0.1:7890"
            type="url"
          />
        ) : null}
      </div>

      <button type="button" className="android-settings-row-action" onClick={openOutputLocation}>
        <span className="android-settings-row-icon"><Folder className="h-4 w-4" /></span>
        <span className="min-w-0 flex-1">
          <span className="android-settings-field-title">保存位置</span>
          <span className="android-settings-field-subtitle truncate">{outputLabel}</span>
        </span>
        <ChevronRight className="h-4 w-4 text-zinc-400" />
      </button>
      <p className="android-settings-note">{androidSaveHint()}</p>

      <div className="android-settings-field android-settings-field-stacked">
        <div>
          <span className="android-settings-field-title">生成后保存提示</span>
          <span className="android-settings-field-subtitle">
            {savePromptSuppressed ? "当前不再弹出另存提醒。" : "生成完成后询问是否另存。"}
          </span>
        </div>
        <div className="android-settings-segmented" role="group" aria-label="生成后保存提示">
          <button
            type="button"
            className={!savePromptSuppressed ? "active" : ""}
            onClick={() => onSetSavePromptSuppressed(false)}
          >
            <Save className="h-3.5 w-3.5" /> 提示
          </button>
          <button
            type="button"
            className={savePromptSuppressed ? "active" : ""}
            onClick={() => onSetSavePromptSuppressed(true)}
          >
            不提示
          </button>
        </div>
      </div>

      <div className="android-settings-field android-settings-field-stacked">
        <div>
          <span className="android-settings-field-title">失败自动重试</span>
          <span className="android-settings-field-subtitle">
            {autoRetryEnabled ? "当前会对 403 / 502 / 503 / 504 / 524 和可重试网络错误自动再发请求。" : "当前不会自动重试，失败后只保留第一次结果。"}
          </span>
        </div>
        <div className="android-settings-segmented" role="group" aria-label="失败自动重试">
          <button
            type="button"
            className={autoRetryEnabled ? "active" : ""}
            onClick={() => onSetAutoRetryEnabled(true)}
          >
            开启
          </button>
          <button
            type="button"
            className={!autoRetryEnabled ? "active" : ""}
            onClick={() => onSetAutoRetryEnabled(false)}
          >
            关闭
          </button>
        </div>
        <div className="mt-3 flex items-center gap-3">
          <input
            type="range"
            min={1}
            max={MAX_AUTO_RETRY_COUNT}
            step={1}
            value={autoRetryCount}
            onChange={(e) => onSetAutoRetryCount(Number(e.currentTarget.value))}
            className="focus-ring flex-1"
          />
          <span className="android-settings-status-pill ready">{autoRetryCount} 次</span>
        </div>
        <p className="android-settings-note">默认 {DEFAULT_AUTO_RETRY_COUNT} 次，不含首次请求。</p>
      </div>

      <div className="android-settings-field android-settings-field-stacked">
        <div>
          <span className="android-settings-field-title">退出时清理预览缓存</span>
          <span className="android-settings-field-subtitle">
            {cleanupPreviewCacheOnExit
              ? "退出应用时会删除可重建的预览图和缩略图缓存。"
              : "默认关闭，避免每次重开后重新生成历史预览影响体验。"}
          </span>
        </div>
        <div className="android-settings-segmented" role="group" aria-label="退出时清理预览缓存">
          <button
            type="button"
            className={!cleanupPreviewCacheOnExit ? "active" : ""}
            onClick={() => onSetCleanupPreviewCacheOnExit(false)}
          >
            关闭
          </button>
          <button
            type="button"
            className={cleanupPreviewCacheOnExit ? "active" : ""}
            onClick={() => onSetCleanupPreviewCacheOnExit(true)}
          >
            开启
          </button>
        </div>
      </div>

      <div className="android-settings-field android-settings-field-stacked">
        <div>
          <span className="android-settings-field-title">流式预览保护</span>
          <span className="android-settings-field-subtitle">
            {protectStreamPreview ? "高并发或大尺寸任务时会自动关闭预览，优先保最终图。" : "不会自动代管流式预览，严格按当前预览帧数请求。"}
          </span>
        </div>
        <div className="android-settings-segmented" role="group" aria-label="流式预览保护">
          <button
            type="button"
            className={protectStreamPreview ? "active" : ""}
            onClick={() => onSetProtectStreamPreview(true)}
          >
            开启
          </button>
          <button
            type="button"
            className={!protectStreamPreview ? "active" : ""}
            onClick={() => onSetProtectStreamPreview(false)}
          >
            关闭
          </button>
        </div>
      </div>

      <div className="android-settings-field android-settings-field-stacked">
        <div>
          <span className="android-settings-field-title">任务提示音</span>
          <span className="android-settings-field-subtitle">
            {completionSound.enabled
              ? (completionSound.mode === "custom" && completionSound.customName
                ? `当前使用 ${completionSound.customName}`
                : "当前使用内置默认音")
              : "整批完成或生成失败时不播放提示音。"}
          </span>
        </div>
        <div className="android-settings-segmented" role="group" aria-label="任务提示音开关">
          <button
            type="button"
            className={completionSound.enabled ? "active" : ""}
            onClick={() => onSetCompletionSoundEnabled(true)}
          >
            <Bell className="h-3.5 w-3.5" /> 开启
          </button>
          <button
            type="button"
            className={!completionSound.enabled ? "active" : ""}
            onClick={() => onSetCompletionSoundEnabled(false)}
          >
            关闭
          </button>
        </div>
        <div className="android-settings-segmented" role="group" aria-label="完成提示音类型">
          <button
            type="button"
            className={completionSound.mode === "default" ? "active" : ""}
            onClick={() => onSetCompletionSoundMode("default")}
          >
            默认音
          </button>
          <button
            type="button"
            className={completionSound.mode === "custom" ? "active" : ""}
            onClick={() => {
              if (!completionSound.customDataURL) {
                onSelectCompletionSound();
                return;
              }
              onSetCompletionSoundMode("custom");
            }}
          >
            自定义
          </button>
        </div>
        <div className="android-settings-action-grid">
          <button type="button" onClick={onPreviewCompletionSound}>试听</button>
          <button type="button" onClick={onSelectCompletionSound}>导入音频</button>
          <button type="button" onClick={onResetCompletionSound}>恢复默认</button>
        </div>
      </div>
    </section>
  );

  const appearanceSection = (
    <section className="android-settings-card android-settings-card-appearance">
      <div className="android-settings-section-title">外观</div>
      <div className="android-settings-segmented" role="group" aria-label="主题">
        <button type="button" className={theme === "system" ? "active" : ""} onClick={() => onSetTheme("system")}>
          <Monitor className="h-3.5 w-3.5" /> 系统
        </button>
        <button type="button" className={theme === "light" ? "active" : ""} onClick={() => onSetTheme("light")}>
          <Sun className="h-3.5 w-3.5" /> 浅色
        </button>
        <button type="button" className={theme === "dark" ? "active" : ""} onClick={() => onSetTheme("dark")}>
          <Moon className="h-3.5 w-3.5" /> 深色
        </button>
      </div>
      <div className="android-settings-field">
        <div>
          <span className="android-settings-field-title">字号</span>
          <span className="android-settings-field-subtitle">当前 {Math.round(fontScale * 100)}%</span>
        </div>
        <div className="android-settings-size-pills">
          {fontSizes.map(({ label, value }) => (
            <button
              key={value}
              type="button"
              className={Math.abs(fontScale - value) < 0.01 ? "active" : ""}
              onClick={() => onSetFontScale(value)}
            >
              {label}
            </button>
          ))}
        </div>
      </div>
    </section>
  );

  const historySection = (
    <section className="android-settings-card android-settings-card-history">
      <div className="android-settings-section-title">历史数据</div>
      <div className="android-settings-history-meter">
        <span><Database className="h-4 w-4" /> 本地历史</span>
        <strong>{historyCountLabel}</strong>
      </div>
      <div className="android-settings-action-grid">
        <button type="button" onClick={exportHistory}><Upload className="h-4 w-4" /> 导出</button>
        <button type="button" onClick={importHistory}><Download className="h-4 w-4" /> 导入</button>
        <button type="button" onClick={() => pruneHistory(3)}>清理 3 天前</button>
        <button type="button" onClick={() => pruneHistory(7)}>清理 7 天前</button>
      </div>
    </section>
  );

  const dangerSection = (
    <section className="android-settings-card android-settings-danger-card">
      <div className="android-settings-section-title">安全与清理</div>
      <button type="button" className="android-settings-row-action danger" onClick={clearAPIKey}>
        <span className="android-settings-row-icon"><KeyRound className="h-4 w-4" /></span>
        <span className="min-w-0 flex-1">
          <span className="android-settings-field-title">清除 API Key</span>
          <span className="android-settings-field-subtitle">从系统凭据存储移除当前密钥。</span>
        </span>
        <Shield className="h-4 w-4 text-red-400" />
      </button>
      <button type="button" className="android-settings-row-action danger" onClick={clearHistory}>
        <span className="android-settings-row-icon"><Trash2 className="h-4 w-4" /></span>
        <span className="min-w-0 flex-1">
          <span className="android-settings-field-title">清空历史</span>
          <span className="android-settings-field-subtitle">删除本地数据库中的全部历史。</span>
        </span>
        <ChevronRight className="h-4 w-4 text-red-300" />
      </button>
    </section>
  );

  const supportSection = (
    <section className="android-settings-card android-settings-card-support">
      <div className="android-settings-section-title">支持</div>
      <div className="android-settings-action-grid">
        <button type="button" onClick={onOpenAbout}><Info className="h-4 w-4" /> 关于</button>
        <button type="button" onClick={onOpenRepo}><Github className="h-4 w-4" /> GitHub</button>
        <button type="button" onClick={onOpenFeedback}><MessageSquare className="h-4 w-4" /> 反馈</button>
      </div>
    </section>
  );

  if (surface === "pad") {
    return (
      <div className="android-settings-panel android-settings-panel-pad" data-android-settings-surface={surface}>
        <div className="android-settings-pad-column android-settings-pad-column-primary">
          {heroSection}
          {appearanceSection}
        </div>
        <div className="android-settings-pad-column android-settings-pad-column-secondary">
          {runtimeSection}
          <div className="android-settings-pad-secondary-grid">
            {historySection}
            {dangerSection}
          </div>
          {supportSection}
        </div>
      </div>
    );
  }

  return (
    <div className={`android-settings-panel android-settings-panel-${surface}`} data-android-settings-surface={surface}>
      {heroSection}
      {runtimeSection}
      {appearanceSection}
      {historySection}
      {dangerSection}
      {supportSection}
    </div>
  );
}
