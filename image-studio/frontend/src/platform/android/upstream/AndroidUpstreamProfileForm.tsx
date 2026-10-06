import { Check, Eye, EyeOff, Minus, Plug, Plus, RefreshCw, Save } from "lucide-react";
import type { ReactNode } from "react";
import type { UpstreamProfile } from "../../../types/domain";
import {
  ANDROID_API_MODE_OPTIONS,
  ANDROID_PROVIDER_OPTIONS,
  ANDROID_REASONING_EFFORT_OPTIONS,
  ANDROID_REQUEST_POLICY_OPTIONS,
} from "./useAndroidUpstreamConfig";
import {
  formatUpstreamModelLabel,
  preferredModelsForAPIMode,
  type UpstreamModelCatalog,
  type UpstreamModelDescriptor,
} from "../../../lib/upstreamModels";

export function AndroidUpstreamProfileForm({
  activeProfileId,
  baseURLError,
  canSave,
  draft,
  draftKey,
  isTestingKey,
  loadingModels,
  modelCatalog,
  modelCatalogError,
  onChangeDraftKey,
  onLoadModels,
  onPatchDraft,
  onSave,
  onSaveAndSetActive,
  onSaveAndTest,
  onSetActive,
  savedKeyLoaded,
  saving,
  showKey,
  onToggleShowKey,
}: {
  activeProfileId: string;
  baseURLError: string | null;
  canSave: boolean;
  draft: UpstreamProfile;
  draftKey: string;
  isTestingKey: boolean;
  loadingModels: boolean;
  modelCatalog: UpstreamModelCatalog | null;
  modelCatalogError: string | null;
  onChangeDraftKey: (value: string) => void;
  onLoadModels: () => void | Promise<void>;
  onPatchDraft: (patch: Partial<UpstreamProfile>) => void;
  onSave: () => void | Promise<void>;
  onSaveAndSetActive: () => void | Promise<void>;
  onSaveAndTest: () => void | Promise<void>;
  onSetActive: () => void | Promise<void>;
  savedKeyLoaded: boolean;
  saving: boolean;
  showKey: boolean;
  onToggleShowKey: () => void;
}) {
  const isActive = draft.id === activeProfileId;
  const busy = saving || isTestingKey;
  const preferredModels = modelCatalog ? preferredModelsForAPIMode(modelCatalog, draft.apiMode) : null;
  const requestModels = modelCatalog
    ? (draft.apiMode === "responses" ? modelCatalog.all : preferredModels?.image ?? [])
    : [];

  return (
    <section className="android-upstream-form" aria-label="编辑上游配置">
      <div className="android-upstream-section-head">
        <span>编辑</span>
        {isActive ? <strong>当前启用</strong> : <button type="button" onClick={onSetActive}>设为当前</button>}
      </div>

      <AndroidField label="名称" required>
        <input
          type="text"
          value={draft.name}
          onChange={(event) => onPatchDraft({ name: event.target.value })}
          className="focus-ring android-upstream-input"
          spellCheck={false}
        />
      </AndroidField>

      <AndroidField label="服务商请求方式">
        <div className="android-upstream-option-grid two">
          {ANDROID_PROVIDER_OPTIONS.map((option) => (
            <button
              key={option.id}
              type="button"
              className={draft.provider === option.id ? "active" : ""}
              onClick={() => onPatchDraft({
                provider: option.id,
                apiMode: option.id === "openai" ? draft.apiMode : "images",
                imageModelID: draft.imageModelID || (option.id === "google" ? "gemini-3.1-flash-image" : option.id === "grok" ? "grok-imagine-image" : ""),
              })}
            >
              <strong>{option.title}</strong>
              <small>{option.meta}</small>
            </button>
          ))}
        </div>
      </AndroidField>

      {draft.provider === "openai" ? <AndroidField label="API 形态">
        <div className="android-upstream-option-grid two">
          {ANDROID_API_MODE_OPTIONS.map((option) => (
            <button
              key={option.id}
              type="button"
              className={draft.apiMode === option.id ? "active" : ""}
              onClick={() => onPatchDraft({ apiMode: option.id })}
            >
              <strong>{option.title}</strong>
              <small>{option.meta}</small>
            </button>
          ))}
        </div>
      </AndroidField> : null}

      {draft.provider === "openai" ? <AndroidField label="参数策略">
        <div className="android-upstream-option-grid two">
          {ANDROID_REQUEST_POLICY_OPTIONS.map((option) => (
            <button
              key={option.id}
              type="button"
              className={draft.requestPolicy === option.id ? "active" : ""}
              onClick={() => onPatchDraft({ requestPolicy: option.id })}
            >
              <strong>{option.title}</strong>
              <small>{option.meta}</small>
            </button>
          ))}
        </div>
      </AndroidField> : null}

      <AndroidField label="上游 BASE_URL" required hint={draft.provider === "google" ? "Google 官方填写 https://generativelanguage.googleapis.com。" : draft.provider === "grok" ? "xAI 官方填写 https://api.x.ai。" : "填写站点根地址，应用会按 API 形态自动拼接 /v1 路径。"}>
        <input
          type="text"
          value={draft.baseURL}
          onChange={(event) => onPatchDraft({ baseURL: event.target.value })}
          placeholder="https://your-relay.example.com"
          className="focus-ring android-upstream-input font-mono-token"
          spellCheck={false}
        />
        {baseURLError ? <p className="android-upstream-error">{baseURLError}</p> : null}
      </AndroidField>

      <AndroidField label="连接安全" hint="仅在可信网络中使用；API Key、提示词和图片可能被窃听或篡改。">
        <button
          type="button"
          role="switch"
          aria-checked={draft.allowInsecureConnection === true}
          className={`android-upstream-compat-toggle ${draft.allowInsecureConnection ? "active" : ""}`}
          onClick={() => onPatchDraft({ allowInsecureConnection: !(draft.allowInsecureConnection === true) })}
        >
          <span>
            <strong>允许不安全连接</strong>
            <small>允许远程 HTTP，并忽略 HTTPS / WSS 证书错误。</small>
          </span>
          <em>{draft.allowInsecureConnection ? "已开启" : "已关闭"}</em>
        </button>
      </AndroidField>

      <AndroidField label="API Key" required hint="密钥写入系统凭据存储，不进入 localStorage。">
        <div className="android-upstream-secret">
          <input
            type={showKey ? "text" : "password"}
            value={draftKey}
            onChange={(event) => onChangeDraftKey(event.target.value)}
            placeholder={savedKeyLoaded ? "sk-..." : "加载中..."}
            autoComplete="off"
            className="focus-ring android-upstream-input font-mono-token"
            spellCheck={false}
          />
          <button type="button" onClick={onToggleShowKey} title={showKey ? "隐藏密钥" : "显示密钥"}>
            {showKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
          </button>
        </div>
      </AndroidField>

      <AndroidField
        label="上游模型列表"
        hint="通过宿主侧请求 /v1/models 获取模型列表，避免 WebView 跨域差异。"
      >
        <button type="button" className="android-upstream-load-models" onClick={() => void onLoadModels()} disabled={loadingModels}>
          <RefreshCw className={`h-4 w-4 ${loadingModels ? "animate-spin" : ""}`} />
          <span>{loadingModels ? "拉取中..." : "拉取并解析上游模型"}</span>
        </button>
        {modelCatalog ? <p className="android-upstream-hint">已识别 {modelCatalog.all.length} 个模型。</p> : null}
        {modelCatalogError ? <p className="android-upstream-error">{modelCatalogError}</p> : null}
      </AndroidField>

      {draft.provider === "openai" && draft.apiMode === "responses" ? (
        <>
          <AndroidField label="Responses 传输" hint="这是 Responses API 的传输方式，不是 Realtime API。">
            <div className="android-upstream-option-grid two">
              {[
                { id: "sse", title: "HTTP SSE", meta: "默认，兼容性更稳" },
                { id: "websocket", title: "WebSocket", meta: "需要上游支持" },
              ].map((option) => (
                <button
                  key={option.id}
                  type="button"
                  className={(draft.responsesTransport ?? "sse") === option.id ? "active" : ""}
                  onClick={() => onPatchDraft({ responsesTransport: option.id as UpstreamProfile["responsesTransport"] })}
                >
                  <strong>{option.title}</strong>
                  <small>{option.meta}</small>
                </button>
              ))}
            </div>
          </AndroidField>

          <AndroidField label="请求模型 ID">
            <input
              type="text"
              value={draft.textModelID}
              onChange={(event) => onPatchDraft({ textModelID: event.target.value })}
              placeholder="留空 = 默认 gpt-5.5"
              className="focus-ring android-upstream-input font-mono-token"
              spellCheck={false}
            />
            {requestModels.length > 0 ? (
              <AndroidModelSuggestions
                models={requestModels}
                selectedID={draft.textModelID}
                onSelect={(id) => onPatchDraft({ textModelID: id })}
              />
            ) : null}
          </AndroidField>

          <AndroidField label="推理强度" hint="默认 xhigh。低强度在部分模型或中转上可能导致工具调用失败。">
            <div className="android-upstream-option-grid two">
              {ANDROID_REASONING_EFFORT_OPTIONS.map((option) => (
                <button
                  key={option.id}
                  type="button"
                  className={draft.reasoningEffort === option.id ? "active" : ""}
                  onClick={() => onPatchDraft({ reasoningEffort: option.id })}
                >
                  <strong>{option.title}</strong>
                  <small>{option.meta}</small>
                </button>
              ))}
            </div>
          </AndroidField>
        </>
      ) : null}

      {draft.apiMode === "images" ? (
        <AndroidField label="请求模型 ID">
          <input
            type="text"
            value={draft.imageModelID}
            onChange={(event) => onPatchDraft({ imageModelID: event.target.value })}
            placeholder={draft.provider === "google" ? "例如 gemini-3.1-flash-image" : draft.provider === "grok" ? "例如 grok-imagine-image" : "留空 = 默认 gpt-image-2"}
            className="focus-ring android-upstream-input font-mono-token"
            spellCheck={false}
          />
          {requestModels.length > 0 ? (
            <AndroidModelSuggestions
              models={requestModels}
              selectedID={draft.imageModelID}
              onSelect={(id) => onPatchDraft({ imageModelID: id })}
            />
          ) : null}
        </AndroidField>
      ) : null}

      <AndroidField label="并发数量限制" hint="0 表示不限制；正整数会限制同一配置跨标签页的并发任务。">
        <div className="android-upstream-stepper">
          <button
            type="button"
            onClick={() => onPatchDraft({ concurrencyLimit: Math.max(0, draft.concurrencyLimit - 1) })}
            title="减少"
          >
            <Minus className="h-4 w-4" />
          </button>
          <input
            type="number"
            value={draft.concurrencyLimit || ""}
            min={0}
            step={1}
            placeholder="不限"
            onChange={(event) => onPatchDraft({ concurrencyLimit: Math.max(0, Math.floor(Number(event.target.value) || 0)) })}
            className="focus-ring android-upstream-input font-mono-token"
          />
          <button
            type="button"
            onClick={() => onPatchDraft({ concurrencyLimit: Math.max(0, draft.concurrencyLimit) + 1 })}
            title="增加"
          >
            <Plus className="h-4 w-4" />
          </button>
        </div>
      </AndroidField>

      {draft.provider === "openai" && draft.apiMode === "images" ? (
        <AndroidField
          label="Images API 中转兼容"
          hint="默认关闭，只有默认标准参数无法生图时，再尝试开启。"
        >
          <button
            type="button"
            className={`android-upstream-compat-toggle ${draft.imagesNewAPICompat ? "active" : ""}`}
            onClick={() => onPatchDraft({ imagesNewAPICompat: !(draft.imagesNewAPICompat === true) })}
          >
            <span>
              <strong>开启此开关可能可以解决newapi生图问题</strong>
              <small>开启后会强制使用 b64_json，并关闭 stream / partial_images。</small>
            </span>
            <em>{draft.imagesNewAPICompat ? "已开启" : "已关闭"}</em>
          </button>
        </AndroidField>
      ) : null}

      <div className="android-upstream-actions">
        <button type="button" onClick={() => void onSave()} disabled={!canSave || busy}>
          <Save className="h-4 w-4" />
          {saving ? "保存中" : "保存"}
        </button>
        <button type="button" onClick={() => void onSaveAndSetActive()} disabled={!canSave || busy}>
          <Check className="h-4 w-4" />
          保存并启用
        </button>
        <button type="button" className="primary" onClick={() => void onSaveAndTest()} disabled={!canSave || busy}>
          <Plug className={`h-4 w-4 ${isTestingKey ? "animate-spin" : ""}`} />
          {isTestingKey ? "测试中" : "保存并测试"}
        </button>
      </div>

      {!canSave ? <p className="android-upstream-save-hint">名称、BASE_URL 和 API Key 填齐后才能保存。</p> : null}
    </section>
  );
}

function AndroidField({
  children,
  hint,
  label,
  required,
}: {
  children: ReactNode;
  hint?: string;
  label: string;
  required?: boolean;
}) {
  return (
    <div className="android-upstream-field">
      <span className="android-upstream-label">
        {label}
        {required ? <em>*</em> : null}
      </span>
      {children}
      {hint ? <span className="android-upstream-hint">{hint}</span> : null}
    </div>
  );
}

function AndroidModelSuggestions({
  models,
  selectedID,
  onSelect,
}: {
  models: UpstreamModelDescriptor[];
  selectedID: string;
  onSelect: (id: string) => void;
}) {
  return (
    <div className="android-upstream-model-suggestions">
      {models.slice(0, 10).map((model) => {
        const active = model.id === selectedID.trim();
        return (
          <button
            key={model.id}
            type="button"
            className={active ? "active" : ""}
            onClick={() => onSelect(model.id)}
          >
            <strong>{formatUpstreamModelLabel(model)}</strong>
          </button>
        );
      })}
    </div>
  );
}
