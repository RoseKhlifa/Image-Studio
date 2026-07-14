import { useState, useEffect, useMemo } from "react";
import { ClipboardPaste, Eye, EyeOff, HelpCircle, Info, Plug, Plus, RefreshCw, Sparkles } from "lucide-react";
import { Modal } from "../common/Modal";
import { useStudioStore } from "../../state/studioStore";
import {
  ExportUpstreamConfigToFile,
  GetStoredAPIKey,
  ImportUpstreamConfigFromFile,
  LoadCodexAPIConfig,
  SetStoredAPIKey,
  canLoadCodexAPIConfig,
  probeCurrentUpstream,
} from "../../platform/runtime/host";
import { genProfileId, keyringUserFor } from "../../lib/profiles";
import type { APIMode, RequestPolicy, UpstreamProfile } from "../../types/domain";
import { FAQModal } from "./FAQModal";
import { UpstreamProfileEditor } from "./UpstreamProfileEditor";
import { UpstreamProfileList } from "./UpstreamProfileList";
import { usePlatform } from "../../platform/context";
import { buildUpstreamModelCatalog, type UpstreamModelCatalog } from "../../lib/upstreamModels";
import {
  applyParsedUpstreamConfigImport,
  buildUpstreamConfigExportFile,
  parseUpstreamConfigImportFile,
} from "../../lib/upstreamConfigTransfer";

// v0.1.6 多 profile 配置 modal。左侧 profile 列表 + 右侧编辑表单。
// 列表点击 = 切 active(立即生效);右侧改字段 = 编辑当前选中,点保存才落盘。
export function UpstreamConfigModal({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const { isAndroidPhone, usesFluentUI, usesAppleUI } = usePlatform();
  const {
    profiles, activeProfileId, aiProfileId,
    createProfile, updateProfile, deleteProfile, duplicateProfile, setActiveProfile, setAIProfile,
    testAPIKey, isTestingKey, pushToast,
  } = useStudioStore();
  const canSyncCodexConfig = canLoadCodexAPIConfig();

  // selected = 当前编辑的 profile id(可以跟 active 不同 —— 用户在浏览/编辑
  // 别的 profile,但还没把它设为 active)。打开 modal 默认 selected = active。
  const [selectedId, setSelectedId] = useState<string>(activeProfileId);
  // 当前 selected 的草稿副本,改完字段后调 updateProfile 才生效
  const [draft, setDraft] = useState<UpstreamProfile | null>(null);
  const [draftKey, setDraftKey] = useState("");
  const [showKey, setShowKey] = useState(false);
  const [savedKeyLoaded, setSavedKeyLoaded] = useState(false);
  const [faqOpen, setFaqOpen] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [quickImportOpen, setQuickImportOpen] = useState(false);
  const [quickImportText, setQuickImportText] = useState("");
  const [syncingCodexConfig, setSyncingCodexConfig] = useState(false);
  const [loadingModels, setLoadingModels] = useState(false);
  const [modelCatalog, setModelCatalog] = useState<UpstreamModelCatalog | null>(null);
  const [modelCatalogError, setModelCatalogError] = useState<string | null>(null);

  // 打开 modal / 切 selected → 重新加载草稿与 keyring 里的 apiKey
  useEffect(() => {
    if (!open) return;
    const sid = selectedId && profiles.some((p) => p.id === selectedId)
      ? selectedId
      : (activeProfileId || profiles[0]?.id || "");
    setSelectedId(sid);
    const p = profiles.find((x) => x.id === sid) ?? null;
    setDraft(p);
    setDraftKey("");
    setSavedKeyLoaded(false);
    setLoadingModels(false);
    setModelCatalog(null);
    setModelCatalogError(null);
    if (p) {
      GetStoredAPIKey(keyringUserFor(p.id))
        .then((k) => { setDraftKey(k ?? ""); setSavedKeyLoaded(true); })
        .catch(() => setSavedKeyLoaded(true));
    } else {
      setSavedKeyLoaded(true);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, selectedId, profiles.length]);

  useEffect(() => {
    if (open) return;
    setFaqOpen(false);
    setDeleteConfirmOpen(false);
    setQuickImportOpen(false);
    setQuickImportText("");
  }, [open]);

  // 列表切换 selected
  function selectProfile(id: string) {
    if (id === selectedId) return;
    setSelectedId(id);
  }

  const baseURLError = useMemo(() => null, [draft?.baseURL]);

  const canSave = !!draft && !!draft.baseURL.trim() && !!draftKey.trim();

  function patchDraft(patch: Partial<UpstreamProfile>) {
    if (!draft) return;
    setDraft({ ...draft, ...patch });
  }

  function codexProfileName(provider: string) {
    const trimmed = provider.trim();
    return trimmed ? `Codex · ${trimmed}` : "Codex";
  }

  async function handleSyncCodex() {
    if (syncingCodexConfig) return;
    setSyncingCodexConfig(true);
    try {
      const imported = await LoadCodexAPIConfig();
      const name = codexProfileName(imported.provider);
      const existing = useStudioStore.getState().profiles.find((profile) => profile.name === name) ?? null;
      let syncedId = existing?.id ?? "";

      if (existing) {
        await updateProfile(existing.id, {
          name,
          apiMode: "responses",
          requestPolicy: "openai",
          imagesNewAPICompat: false,
          baseURL: imported.baseURL,
          textModelID: existing.textModelID,
          imageModelID: existing.imageModelID,
          reasoningEffort: existing.reasoningEffort,
          concurrencyLimit: existing.concurrencyLimit,
          apiKey: imported.apiKey,
        });
        await setActiveProfile(existing.id);
        syncedId = existing.id;
      } else {
        syncedId = await createProfile({
          name,
          apiMode: "responses",
          requestPolicy: "openai",
          baseURL: imported.baseURL,
          apiKey: imported.apiKey,
          setActive: true,
        });
      }

      const syncedProfile = useStudioStore.getState().profiles.find((profile) => profile.id === syncedId) ?? null;
      setSelectedId(syncedId);
      setDraft(syncedProfile);
      setDraftKey(imported.apiKey);
      setSavedKeyLoaded(true);
      setShowKey(false);
      pushToast(`已同步 ${name}`, "success");
    } catch (error: any) {
      pushToast(`同步 Codex 配置失败:${error?.message ?? error}`, "error", 6000);
    } finally {
      setSyncingCodexConfig(false);
    }
  }

  async function handleExportConfigs() {
    try {
      const currentProfiles = useStudioStore.getState().profiles;
      if (currentProfiles.length === 0) {
        pushToast("当前没有可导出的上游配置", "warn");
        return;
      }
      const apiKeysById = Object.fromEntries(await Promise.all(
        currentProfiles.map(async (profile) => [profile.id, await GetStoredAPIKey(keyringUserFor(profile.id)).catch(() => "")] as const),
      ));
      const currentState = useStudioStore.getState();
      const payload = buildUpstreamConfigExportFile(currentProfiles, currentState.activeProfileId, apiKeysById, currentState.aiProfileId);
      const dst = await ExportUpstreamConfigToFile(JSON.stringify(payload, null, 2));
      if (dst) pushToast(`已导出上游配置 → ${dst.split(/[\\/]/).pop()}`, "success");
    } catch (error: any) {
      pushToast(`导出上游配置失败:${error?.message ?? error}`, "error", 6000);
    }
  }

  async function handleImportConfigs() {
    try {
      const raw = await ImportUpstreamConfigFromFile();
      if (!raw) return;
      await handleImportFromRawJSON(raw, "已导入");
    } catch (error: any) {
      pushToast(`导入上游配置失败:${error?.message ?? error}`, "error", 6000);
    }
  }

  async function handleImportFromRawJSON(raw: string, successPrefix = "已导入") {
    const parsed = parseUpstreamConfigImportFile(raw);
    const result = await applyParsedUpstreamConfigImport(parsed, {
      getProfiles: () => useStudioStore.getState().profiles,
      createProfile,
      updateProfile,
      setActiveProfile,
      setAIProfile,
    });
    const targetId = result.activeProfileId || result.importedProfileIds[0] || selectedId;
    const selectedProfile = useStudioStore.getState().profiles.find((profile) => profile.id === targetId)
      ?? useStudioStore.getState().profiles[0]
      ?? null;
    if (selectedProfile) {
      setSelectedId(selectedProfile.id);
      setDraft(selectedProfile);
      setDraftKey(await GetStoredAPIKey(keyringUserFor(selectedProfile.id)).catch(() => ""));
      setSavedKeyLoaded(true);
    }
    pushToast(`${successPrefix} ${result.importedCount} 组上游配置`, "success");
  }

  async function handleQuickImport() {
    const raw = quickImportText.trim();
    if (!raw) {
      pushToast("先粘贴 JSON 模板", "warn");
      return;
    }
    try {
      await handleImportFromRawJSON(raw, "已快捷导入");
      setQuickImportOpen(false);
      setQuickImportText("");
    } catch (error: any) {
      pushToast(`快捷导入失败:${error?.message ?? error}`, "error", 6000);
    }
  }

  async function handleNew(apiMode: APIMode = "responses") {
    const id = await createProfile({
      apiMode,
      requestPolicy: "openai",
      setActive: profiles.length === 0, // 第一个自动 active,后续手动切
    });
    setSelectedId(id);
  }

  async function handleDuplicate() {
    if (!selectedId) return;
    const newId = await duplicateProfile(selectedId);
    if (newId) setSelectedId(newId);
  }

  async function handleDelete() {
    if (!draft) return;
    const deletingId = draft.id;
    await deleteProfile(deletingId);
    // 删完 selected:切到第一个剩余(action 内部已经更新 active);UI 跟着
    const remaining = useStudioStore.getState().profiles;
    setSelectedId(remaining[0]?.id ?? "");
    setDeleteConfirmOpen(false);
  }

  async function handleSave() {
    if (!draft) return;
        await updateProfile(draft.id, {
          name: draft.name,
          apiMode: draft.apiMode,
          responsesTransport: draft.responsesTransport ?? "sse",
          requestPolicy: draft.requestPolicy,
          imagesNewAPICompat: draft.imagesNewAPICompat === true,
          baseURL: draft.baseURL,
          textModelID: draft.textModelID,
          imageModelID: draft.imageModelID,
          reasoningEffort: draft.reasoningEffort,
          concurrencyLimit: draft.concurrencyLimit,
          fallbackProfileId: draft.fallbackProfileId,
          apiKey: draftKey,
        });
    // 如果当前 selected 不是 active,问要不要切;不弹了,直接什么都不做
  }

  async function handleSetActive() {
    if (!draft) return;
    await setActiveProfile(draft.id);
  }

  async function handleSetAI() {
    if (!draft) return;
    if (draft.apiMode !== "responses") {
      pushToast("AI 优化与图片反推需要 Responses API 配置", "warn", 5000);
      return;
    }
    if (!canSave) {
      pushToast("先补全并保存这条配置的 BASE_URL 与 API Key", "warn", 5000);
      return;
    }
    await handleSave();
    if (await setAIProfile(draft.id)) {
      pushToast(`已将「${draft.name}」设为 AI 渠道`, "success");
    }
  }

  async function handleTest() {
    if (!draft || !canSave) return;
    // 先保存,再测;testAPIKey 读 active profile 的字段,所以要让它先切到 selected
    await handleSave();
    if (draft.id !== activeProfileId) {
      await setActiveProfile(draft.id);
    }
    onClose();
    setTimeout(() => { void testAPIKey(); }, 0);
  }

  async function handleLoadModels() {
    if (!draft) return;
    const apiKey = draftKey.trim();
    const baseURL = draft.baseURL.trim();
    if (!apiKey) {
      pushToast("先填入 API Key", "warn");
      return;
    }
    if (!baseURL) {
      pushToast("先填入上游 BASE_URL", "warn");
      return;
    }
    setLoadingModels(true);
    setModelCatalogError(null);
    try {
      const runtimeState = useStudioStore.getState();
      const result = await probeCurrentUpstream(
        baseURL,
        apiKey,
        runtimeState.proxyMode,
        runtimeState.proxyURL,
        draft.apiMode,
        draft.responsesTransport ?? "sse",
      );
      const catalog = buildUpstreamModelCatalog(result.models ?? []);
      setModelCatalog(catalog);
      if (result.responsesTransport === "websocket" && result.responsesTransportOK === false) {
        pushToast(
          `已拉取模型，但 Responses WebSocket 不可用:${result.responsesTransportError || "未返回具体原因"}`,
          "warn",
          7000,
        );
      } else {
        const message = result.responsesTransport === "websocket"
          ? `已加载 ${catalog.all.length} 个模型，Responses WebSocket 可用`
          : catalog.all.length > 0
            ? `已加载 ${catalog.all.length} 个模型`
            : `已连接上游，共返回 ${result.modelCount} 个条目，但没有可识别的模型 ID`;
        pushToast(message, catalog.all.length > 0 ? "success" : "warn");
      }
    } catch (error: any) {
      const message = `加载模型失败:${error?.message ?? error}`;
      setModelCatalogError(message);
      pushToast(message, "error", 6000);
    } finally {
      setLoadingModels(false);
    }
  }

  if (profiles.length === 0) {
    return (
      <Modal
        open={open}
        onClose={onClose}
        title="上游配置"
        width={760}
        cardClassName="upstream-config-modal"
        bodyClassName="upstream-config-modal-body"
      >
        <section className={`flex flex-col ${isAndroidPhone ? "gap-4" : "gap-5"}`}>
          <div className={`border border-black/[0.06] bg-[var(--surface)]/70 dark:border-white/[0.06] dark:bg-white/[0.03] ${isAndroidPhone ? "rounded-[20px] px-4 py-4" : "rounded-[22px] px-5 py-5"}`}>
            <div className="flex items-start gap-3">
              <div className={`flex shrink-0 items-center justify-center border border-[color:var(--accent)]/18 bg-[var(--accent-soft)] ${isAndroidPhone ? "h-11 w-11 rounded-[14px]" : "h-12 w-12 rounded-[16px]"}`}>
                <Sparkles className="h-5 w-5 text-[var(--accent)]" />
              </div>
              <div className="min-w-0">
                <h4 className={`text-zinc-900 dark:text-zinc-100 ${isAndroidPhone ? "text-[17px] font-semibold" : "text-[18px] font-semibold"}`}>先连上一个可用上游</h4>
                <p className={`mt-1 text-zinc-500 dark:text-zinc-400 ${isAndroidPhone ? "text-[13px] leading-6" : "text-sm leading-6"}`}>
                  先保存一条可用配置。生图与 AI 操作可以分别指定渠道，图片反推与提示词优化使用 Responses API。
                </p>
              </div>
            </div>
          </div>

          <div className={`grid gap-2 ${isAndroidPhone ? "grid-cols-1" : "grid-cols-2"}`}>
            {canSyncCodexConfig ? (
              <button
                type="button"
                onClick={() => void handleSyncCodex()}
                disabled={syncingCodexConfig}
                className={`platform-card col-span-full flex items-center justify-between gap-3 border border-[color:var(--accent)]/25 bg-[var(--accent-soft)] px-4 py-3 text-left text-[13px] text-[var(--accent)] transition-colors hover:bg-[color:var(--accent)]/15 disabled:cursor-not-allowed disabled:opacity-60 ${usesFluentUI ? "rounded-[10px]" : "rounded-[18px]"}`}
              >
                <span className="min-w-0">
                <span className="block font-semibold">同步 Codex 配置</span>
                  <span className="mt-1 block text-[11px] text-[var(--accent)]/80">自动读取当前电脑里的 Codex `base_url` 和 `OPENAI_API_KEY`。</span>
                </span>
                <RefreshCw className={`h-4 w-4 shrink-0 ${syncingCodexConfig ? "animate-spin" : ""}`} />
              </button>
            ) : null}
            <button
              type="button"
              onClick={() => setQuickImportOpen(true)}
              className={`platform-card col-span-full flex items-center justify-between gap-3 border border-black/[0.08] bg-white/70 px-4 py-3 text-left text-[13px] text-zinc-700 transition-colors hover:border-[color:var(--accent)]/35 hover:bg-[var(--accent-soft)]/60 dark:border-white/[0.06] dark:bg-white/[0.03] dark:text-zinc-200 ${usesFluentUI ? "rounded-[10px]" : "rounded-[18px]"}`}
            >
              <span className="min-w-0">
                <span className="block font-semibold">粘贴 JSON 快捷导入</span>
                <span className="mt-1 block text-[11px] text-zinc-500 dark:text-zinc-400">支持本应用导出文件、`newapi_channel_conn`、OpenCode `provider` 模板。</span>
              </span>
              <ClipboardPaste className="h-4 w-4 shrink-0 text-[var(--accent)]" />
            </button>
            {([
              {
                id: "responses" as APIMode,
                title: "Responses API",
                sub: "首选。支持 SSE 保活，长任务更稳。",
                note: "适合 GPT 图像链路和提示词优化。",
              },
              {
                id: "images" as APIMode,
                title: "Images API",
                sub: "兼容性更广，接标准 generations / edits。",
                note: "适合只想尽快接上常规生图接口。",
              },
            ]).map((item) => (
              <button
                key={item.id}
                type="button"
                onClick={() => handleNew(item.id)}
                className={`platform-card flex flex-col items-start gap-2 border border-black/[0.08] bg-white/70 p-4 text-left transition-colors hover:border-[color:var(--accent)]/35 hover:bg-[var(--accent-soft)]/60 dark:border-white/[0.06] dark:bg-white/[0.03] ${usesFluentUI ? "rounded-[10px]" : "rounded-[18px]"}`}
              >
                <div className="flex items-center gap-2">
                  <span className="inline-flex h-8 min-w-[32px] items-center justify-center rounded-full bg-[var(--accent-soft)] px-2 text-[11px] font-semibold text-[var(--accent)]">
                    {item.id === "responses" ? "R" : "I"}
                  </span>
                  <span className="text-[14px] font-semibold text-zinc-900 dark:text-zinc-100">{item.title}</span>
                </div>
                <p className="text-[12px] leading-5 text-zinc-600 dark:text-zinc-300">{item.sub}</p>
                <p className="text-[11px] leading-5 text-zinc-500 dark:text-zinc-400">{item.note}</p>
                <span className={`mt-1 inline-flex items-center gap-1 text-[11px] font-medium text-[var(--accent)] ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}>
                  <Plus className="h-3 w-3" /> 新建这类配置
                </span>
              </button>
            ))}
          </div>

          <div className={`flex items-start gap-2 border border-[color:var(--accent)]/18 bg-[var(--accent-soft)] px-3 py-2 text-[11px] text-[var(--accent)] ${usesFluentUI ? "rounded-[10px]" : "rounded-[14px]"}`}>
            <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            <span>保存后会写入系统凭据存储。之后你可以在这里继续新增多个上游配置，再按场景切换。</span>
          </div>
        </section>
      </Modal>
    );
  }

  return (
    <>
    <Modal
      open={open}
      onClose={onClose}
      title="上游配置"
      width={760}
      cardClassName="upstream-config-modal"
      bodyClassName="upstream-config-modal-body"
    >
      <div className={`flex min-w-0 gap-4 ${isAndroidPhone ? "flex-col" : ""}`}>
        <UpstreamProfileList
          profiles={profiles}
          selectedId={selectedId}
          activeProfileId={activeProfileId}
          aiProfileId={aiProfileId}
          draftId={draft?.id}
          draftAPIMode={draft?.apiMode}
          isAndroidPhone={isAndroidPhone}
          canSyncCodexConfig={canSyncCodexConfig}
          isSyncingCodexConfig={syncingCodexConfig}
          onSelectProfile={selectProfile}
          onHandleNew={() => handleNew()}
          onHandleDuplicate={handleDuplicate}
          onHandleDelete={() => setDeleteConfirmOpen(true)}
          onHandleExport={handleExportConfigs}
          onHandleImport={handleImportConfigs}
          onHandleQuickImport={() => setQuickImportOpen(true)}
          onHandleSetActive={handleSetActive}
          onHandleSetAI={handleSetAI}
          onHandleSyncCodex={handleSyncCodex}
        />

        {/* ---------------- 右侧编辑表单 ---------------- */}
        <section className="flex-1 min-w-0">
          {!draft ? (
            <div className="grid h-full place-items-center py-10 text-sm text-zinc-500">
              在左侧选一个配置,或新建一个。
            </div>
          ) : (
            <UpstreamProfileEditor
              draft={draft}
              draftKey={draftKey}
              showKey={showKey}
              savedKeyLoaded={savedKeyLoaded}
              baseURLError={baseURLError}
              canSave={canSave}
              isTestingKey={isTestingKey}
              loadingModels={loadingModels}
              modelCatalog={modelCatalog}
              modelCatalogError={modelCatalogError}
              profiles={profiles}
              usesAppleUI={usesAppleUI}
              onOpenFAQ={() => setFaqOpen(true)}
              onPatchDraft={patchDraft}
              onChangeDraftKey={setDraftKey}
              onToggleShowKey={() => setShowKey((v) => !v)}
              onLoadModels={handleLoadModels}
              onTest={handleTest}
              onClose={onClose}
              onSaveAndClose={async () => { await handleSave(); onClose(); }}
            />
          )}
        </section>
      </div>
    </Modal>
    <FAQModal open={faqOpen} onClose={() => setFaqOpen(false)} />
    <Modal
      open={quickImportOpen}
      onClose={() => setQuickImportOpen(false)}
      title="快捷导入上游配置"
      width={620}
    >
      <section className="flex flex-col gap-3">
        <p className="m-0 text-[13px] leading-6 text-zinc-600 dark:text-zinc-300">
          直接粘贴对方提供的 JSON 模板。当前支持本应用导出文件、<code className="font-mono-token">newapi_channel_conn</code>、OpenCode <code className="font-mono-token">provider</code> 配置。
        </p>
        <textarea
          value={quickImportText}
          onChange={(event) => setQuickImportText(event.target.value)}
          placeholder={"在这里粘贴 JSON...\n例如 {\"_type\":\"newapi_channel_conn\",...}"}
          spellCheck={false}
          className={`focus-ring min-h-[280px] w-full resize-y border border-black/[0.08] bg-[var(--surface)] px-3 py-2 text-sm text-zinc-900 placeholder:text-zinc-400 dark:border-white/[0.08] dark:text-zinc-100 dark:placeholder:text-zinc-500 font-mono-token ${usesFluentUI ? "rounded-[10px]" : "rounded-[14px]"}`}
        />
        <div className="flex items-start gap-2 border border-[color:var(--accent)]/18 bg-[var(--accent-soft)] px-3 py-2 text-[11px] leading-5 text-[var(--accent)] rounded-[14px]">
          <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          <span>导入后会直接生成可用 profile，并把 API Key 写入系统凭据存储。若模板里自带 `/v1`，会自动适配成站点根地址。</span>
        </div>
        <div className="flex justify-end gap-2">
          <button
            type="button"
            onClick={() => setQuickImportOpen(false)}
            className={`platform-action-btn border border-black/[0.08] px-4 py-2 text-sm text-zinc-700 transition-colors hover:bg-black/[0.04] dark:border-white/[0.08] dark:text-zinc-300 dark:hover:bg-white/[0.06] ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
          >
            取消
          </button>
          <button
            type="button"
            onClick={() => void handleQuickImport()}
            className={`liquid-primary-button bg-[var(--accent)] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--accent-2)] ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
          >
            立即导入
          </button>
        </div>
      </section>
    </Modal>
    <Modal
      open={deleteConfirmOpen}
      onClose={() => setDeleteConfirmOpen(false)}
      title="删除上游配置"
      width={448}
      cardClassName="upstream-delete-confirm-modal"
      bodyClassName="upstream-delete-confirm-modal-body"
    >
      <div className="upstream-delete-confirm-shell">
        <div className="upstream-delete-confirm-copy rounded-[18px] border border-red-500/16 bg-red-500/[0.06] px-4 py-3 text-[13px] leading-6 text-zinc-700 dark:border-red-400/20 dark:bg-red-400/[0.08] dark:text-zinc-200">
          <p className="m-0">
            确认删除「{draft?.name || "当前配置"}」?
          </p>
          <p className="mt-1.5 mb-0 text-[12px] leading-5 text-zinc-500 dark:text-zinc-400">
            这会同时清除这条配置对应的 API Key 凭据。删除后不可恢复。
          </p>
        </div>
        <div className="upstream-delete-confirm-actions">
          <button
            type="button"
            onClick={() => setDeleteConfirmOpen(false)}
            className={`platform-action-btn upstream-delete-confirm-btn upstream-delete-confirm-btn-secondary inline-flex items-center justify-center ${usesAppleUI ? "rounded-[14px]" : usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
          >
            取消
          </button>
          <button
            type="button"
            onClick={() => void handleDelete()}
            className={`platform-action-btn upstream-delete-confirm-btn upstream-delete-confirm-btn-danger inline-flex items-center justify-center ${usesAppleUI ? "rounded-[14px]" : usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
          >
            确认删除
          </button>
        </div>
      </div>
    </Modal>
    </>
  );
}
