import { useEffect, useMemo, useState } from "react";
import { GetStoredAPIKey, probeCurrentUpstream } from "../../runtime/host";
import { keyringUserFor } from "../../../lib/profiles";
import { useStudioStore } from "../../../state/studioStore";
import type { APIMode, ReasoningEffortValue, RequestPolicy, UpstreamProfile, UpstreamProvider } from "../../../types/domain";
import { buildUpstreamModelCatalog, type UpstreamModelCatalog } from "../../../lib/upstreamModels";
import {
  applyParsedUpstreamConfigImport,
  parseUpstreamConfigImportFile,
} from "../../../lib/upstreamConfigTransfer";

export const ANDROID_API_MODE_OPTIONS: Array<{
  id: APIMode;
  title: string;
  meta: string;
}> = [
  { id: "responses", title: "Responses", meta: "SSE 长任务" },
  { id: "images", title: "Images", meta: "标准图像接口" },
];

export const ANDROID_PROVIDER_OPTIONS: Array<{
  id: UpstreamProvider;
  title: string;
  meta: string;
}> = [
  { id: "openai", title: "OpenAI / 兼容", meta: "Responses 或 Images" },
  { id: "google", title: "Google Gemini", meta: "Interactions 原生协议" },
  { id: "grok", title: "Grok / xAI", meta: "Imagine 原生协议" },
];

export const ANDROID_REQUEST_POLICY_OPTIONS: Array<{
  id: RequestPolicy;
  title: string;
  meta: string;
}> = [
  { id: "openai", title: "OpenAI 标准", meta: "只发送公开字段" },
  { id: "compat", title: "兼容中转", meta: "允许 relay 扩展字段" },
];

export const ANDROID_REASONING_EFFORT_OPTIONS: Array<{
  id: ReasoningEffortValue;
  title: string;
  meta: string;
}> = [
  { id: "xhigh", title: "xhigh", meta: "默认，最稳" },
  { id: "high", title: "high", meta: "高强度" },
  { id: "medium", title: "medium", meta: "中等" },
  { id: "low", title: "low", meta: "低强度，可能掉工具调用" },
];

export function useAndroidUpstreamConfig(open: boolean) {
  const {
    profiles,
    activeProfileId,
    createProfile,
    updateProfile,
    deleteProfile,
    duplicateProfile,
    setActiveProfile,
    testAPIKey,
    isTestingKey,
    pushToast,
  } = useStudioStore();

  const [selectedId, setSelectedId] = useState(activeProfileId);
  const [draft, setDraft] = useState<UpstreamProfile | null>(null);
  const [draftKey, setDraftKey] = useState("");
  const [showKey, setShowKey] = useState(false);
  const [savedKeyLoaded, setSavedKeyLoaded] = useState(false);
  const [saving, setSaving] = useState(false);
  const [quickImportOpen, setQuickImportOpen] = useState(false);
  const [quickImportText, setQuickImportText] = useState("");
  const [loadingModels, setLoadingModels] = useState(false);
  const [modelCatalog, setModelCatalog] = useState<UpstreamModelCatalog | null>(null);
  const [modelCatalogError, setModelCatalogError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    const nextSelectedId = selectedId && profiles.some((profile) => profile.id === selectedId)
      ? selectedId
      : activeProfileId || profiles[0]?.id || "";

    if (nextSelectedId !== selectedId) {
      setSelectedId(nextSelectedId);
      return;
    }

    const selected = profiles.find((profile) => profile.id === nextSelectedId) ?? null;
    setDraft(selected ? { ...selected } : null);
    setDraftKey("");
    setShowKey(false);
    setQuickImportOpen(false);
    setQuickImportText("");
    setSavedKeyLoaded(false);
    setLoadingModels(false);
    setModelCatalog(null);
    setModelCatalogError(null);

    if (!selected) {
      setSavedKeyLoaded(true);
      return;
    }

    let cancelled = false;
    GetStoredAPIKey(keyringUserFor(selected.id))
      .then((key) => {
        if (cancelled) return;
        setDraftKey(key ?? "");
        setSavedKeyLoaded(true);
      })
      .catch(() => {
        if (!cancelled) setSavedKeyLoaded(true);
      });

    return () => {
      cancelled = true;
    };
  }, [activeProfileId, open, profiles, selectedId]);

  const activeProfile = useMemo(
    () => profiles.find((profile) => profile.id === activeProfileId) ?? null,
    [activeProfileId, profiles],
  );

  const baseURLError = useMemo(() => null, [draft]);

  const canSave = !!draft
    && !!draft.name.trim()
    && !!draft.baseURL.trim()
    && !!draftKey.trim()
    && savedKeyLoaded
    && !saving;

  function patchDraft(patch: Partial<UpstreamProfile>) {
    setDraft((current) => {
      if (!current) return current;
      const provider = patch.provider ?? current.provider ?? "openai";
      return {
        ...current,
        ...patch,
        provider,
        apiMode: provider === "openai" ? patch.apiMode ?? current.apiMode : "images",
      };
    });
  }

  async function handleNew(provider: UpstreamProvider = "openai") {
    const id = await createProfile({
      provider,
      apiMode: provider === "openai" ? "responses" : "images",
      requestPolicy: "openai",
      setActive: profiles.length === 0,
    });
    setSelectedId(id);
  }

  async function handleDuplicate() {
    if (!selectedId) return;
    const id = await duplicateProfile(selectedId);
    if (id) {
      setSelectedId(id);
      pushToast("已复制上游配置", "success");
    }
  }

  async function handleDelete() {
    if (!draft) return;
    if (!window.confirm(`确认删除「${draft.name}」配置? 对应的 API Key 也会从系统凭据存储清除。`)) return;
    await deleteProfile(draft.id);
    const remaining = useStudioStore.getState().profiles;
    setSelectedId(remaining[0]?.id ?? "");
    pushToast("已删除上游配置", "success");
  }

  async function handleSave() {
    if (!draft || !canSave) return false;
    setSaving(true);
    try {
      const ok = await updateProfile(draft.id, {
        name: draft.name,
        provider: draft.provider,
        apiMode: draft.apiMode,
        responsesTransport: draft.responsesTransport ?? "sse",
        requestPolicy: draft.requestPolicy,
        imagesNewAPICompat: draft.imagesNewAPICompat === true,
        allowInsecureConnection: draft.allowInsecureConnection === true,
        baseURL: draft.baseURL,
        textModelID: draft.textModelID,
        imageModelID: draft.imageModelID,
        reasoningEffort: draft.reasoningEffort,
        concurrencyLimit: draft.concurrencyLimit,
        apiKey: draftKey.trim(),
      });
      if (ok) pushToast("已保存上游配置", "success");
      return ok;
    } finally {
      setSaving(false);
    }
  }

  async function handleSetActive() {
    if (!draft) return;
    await setActiveProfile(draft.id);
    pushToast("已切换当前上游", "success");
  }

  async function handleSaveAndSetActive(onSaved?: () => void) {
    if (!draft) return;
    const draftId = draft.id;
    const saved = await handleSave();
    if (saved && draftId !== activeProfileId) {
      await setActiveProfile(draftId);
    }
    if (saved) onSaved?.();
  }

  async function handleSaveAndTest(onSaved?: () => void) {
    const saved = await handleSave();
    if (!saved || !draft) return;
    if (draft.id !== useStudioStore.getState().activeProfileId) {
      await setActiveProfile(draft.id);
    }
    onSaved?.();
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
      const state = useStudioStore.getState();
      const result = await probeCurrentUpstream(
        baseURL,
        apiKey,
        state.proxyMode,
        state.proxyURL,
        draft.apiMode,
        draft.responsesTransport ?? "sse",
        draft.allowInsecureConnection === true,
        undefined,
        draft.provider,
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
        pushToast(
          result.responsesTransport === "websocket"
            ? `已加载 ${catalog.all.length} 个模型，Responses WebSocket 可用`
            : catalog.all.length > 0
              ? `已加载 ${catalog.all.length} 个模型`
              : `已连接上游，共返回 ${result.modelCount} 个条目，但没有可识别的模型 ID`,
          catalog.all.length > 0 ? "success" : "warn",
        );
      }
    } catch (error: any) {
      const message = `加载模型失败:${error?.message ?? error}`;
      setModelCatalogError(message);
      pushToast(message, "error", 6000);
    } finally {
      setLoadingModels(false);
    }
  }

  async function handleImportFromRawJSON(raw: string, successPrefix = "已导入") {
    const parsed = parseUpstreamConfigImportFile(raw);
    const result = await applyParsedUpstreamConfigImport(parsed, {
      getProfiles: () => useStudioStore.getState().profiles,
      createProfile,
      updateProfile,
      setActiveProfile,
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

  return {
    activeProfile,
    activeProfileId,
    baseURLError,
    canSave,
    draft,
    draftKey,
    handleDelete,
    handleDuplicate,
    handleQuickImport,
    handleNew,
    handleSave,
    handleSaveAndSetActive,
    handleSaveAndTest,
    handleSetActive,
    isTestingKey,
    loadingModels,
    modelCatalog,
    modelCatalogError,
    patchDraft,
    profiles,
    quickImportOpen,
    quickImportText,
    savedKeyLoaded,
    saving,
    selectedId,
    setDraftKey,
    handleLoadModels,
    setQuickImportOpen,
    setQuickImportText,
    setSelectedId,
    setShowKey,
    showKey,
  };
}
