import { CheckCircle2, Eye, EyeOff, HelpCircle, Info, Plug, RefreshCw } from "lucide-react";
import {
  REASONING_EFFORT_OPTIONS,
  type APIMode,
  type RequestPolicy,
  type ResponsesTransport,
  type UpstreamProfile,
} from "../../types/domain";
import { requestPolicyLabel } from "../../lib/profiles";
import { usePlatform } from "../../platform/context";
import {
  formatUpstreamModelLabel,
  preferredModelsForAPIMode,
  type UpstreamModelCatalog,
  type UpstreamModelDescriptor,
} from "../../lib/upstreamModels";

export function UpstreamProfileEditor({
  draft,
  draftKey,
  showKey,
  savedKeyLoaded,
  baseURLError,
  canSave,
  isTestingKey,
  loadingModels,
  modelCatalog,
  modelCatalogError,
  profiles,
  usesAppleUI,
  onOpenFAQ,
  onPatchDraft,
  onChangeDraftKey,
  onToggleShowKey,
  onLoadModels,
  onTest,
  onClose,
  onSaveAndClose,
}: {
  draft: UpstreamProfile;
  draftKey: string;
  showKey: boolean;
  savedKeyLoaded: boolean;
  baseURLError: string | null;
  canSave: boolean;
  isTestingKey: boolean;
  loadingModels: boolean;
  modelCatalog: UpstreamModelCatalog | null;
  modelCatalogError: string | null;
  profiles: UpstreamProfile[];
  usesAppleUI: boolean;
  onOpenFAQ: () => void;
  onPatchDraft: (patch: Partial<UpstreamProfile>) => void;
  onChangeDraftKey: (value: string) => void;
  onToggleShowKey: () => void;
  onLoadModels: () => void | Promise<void>;
  onTest: () => void | Promise<void>;
  onClose: () => void;
  onSaveAndClose: () => void | Promise<void>;
}) {
  const { isAndroidPhone, usesFluentUI } = usePlatform();
  const apiModeOptions = [
    { id: "responses" as APIMode, title: "Responses API", sub: "SSE 保活(CF 超时推荐)" },
    { id: "images" as APIMode, title: "Images API", sub: "标准 generations / edits" },
  ];
  const requestPolicyOptions = [
    { id: "openai" as RequestPolicy, title: requestPolicyLabel("openai"), sub: "默认。只发送 OpenAI 官方公开字段。" },
    { id: "compat" as RequestPolicy, title: requestPolicyLabel("compat"), sub: "兼容部分 relay 扩展字段，例如 seed / negative_prompt。" },
  ];
  const responsesTransportOptions = [
    { id: "sse" as ResponsesTransport, title: "HTTP SSE", sub: "默认，兼容性更稳" },
    { id: "websocket" as ResponsesTransport, title: "WebSocket mode", sub: "需要上游支持" },
  ];
  const selectedAPIMode = apiModeOptions.find((option) => option.id === draft.apiMode) ?? apiModeOptions[0];
  const selectedRequestPolicy = requestPolicyOptions.find((option) => option.id === draft.requestPolicy) ?? requestPolicyOptions[0];
  const selectedReasoningEffort = REASONING_EFFORT_OPTIONS.find((option) => option.value === draft.reasoningEffort) ?? REASONING_EFFORT_OPTIONS[0];
  const preferredModels = modelCatalog ? preferredModelsForAPIMode(modelCatalog, draft.apiMode) : null;
  const fallbackCandidates = profiles.filter((profile) => profile.id !== draft.id && profile.baseURL.trim());

  return (
    <div className={`upstream-profile-editor flex min-w-0 flex-col ${isAndroidPhone ? "gap-3" : "gap-3.5"}`}>
      <div className="flex items-center justify-end">
        <button
          type="button"
          onClick={onOpenFAQ}
          className={`inline-flex items-center gap-1 text-[11px] text-zinc-500 transition-colors hover:text-[var(--accent)] ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
        >
          <HelpCircle className="h-3.5 w-3.5" /> 接口说明
        </button>
      </div>

      <Field label="名称">
        <input
          type="text"
          value={draft.name}
          onChange={(e) => onPatchDraft({ name: e.target.value })}
          spellCheck={false}
          className={`focus-ring w-full min-w-0 border border-black/[0.08] bg-[var(--surface)] px-3 py-2 text-sm text-zinc-900 placeholder:text-zinc-400 dark:border-white/[0.08] dark:text-zinc-100 dark:placeholder:text-zinc-500 ${usesFluentUI ? "rounded-[10px]" : "rounded-[14px]"}`}
        />
      </Field>

      <Field
        label={(
          <span className="flex items-center justify-between gap-3">
            <span>API 形态</span>
            <span className="shrink-0 text-[11px] font-medium text-[var(--accent)]">已选 {selectedAPIMode.title}</span>
          </span>
        )}
      >
        <div className={`grid gap-2 ${isAndroidPhone ? "grid-cols-1" : "grid-cols-2"}`}>
          {apiModeOptions.map((option) => {
            const active = draft.apiMode === option.id;
            return (
              <OptionCard
                key={option.id}
                active={active}
                usesFluentUI={usesFluentUI}
                title={option.title}
                sub={option.sub}
                onClick={() => onPatchDraft({ apiMode: option.id })}
              />
            );
          })}
        </div>
        <Hint>
          {draft.apiMode === "responses"
            ? "需要 key 绑定到「拥有 gpt-5.5 模型的分组」。SSE 保活可防 Cloudflare 524。"
            : "使用标准 Images API,key 用 image-2 / image API 分组,兼容性最广。"}
        </Hint>
      </Field>

      <Field
        label={(
          <span className="flex items-center justify-between gap-3">
            <span>参数策略</span>
            <span className="shrink-0 text-[11px] font-medium text-[var(--accent)]">已选 {selectedRequestPolicy.title}</span>
          </span>
        )}
      >
        <div className="grid gap-2">
          {requestPolicyOptions.map((option) => {
            const active = draft.requestPolicy === option.id;
            return (
              <OptionCard
                key={option.id}
                active={active}
                usesFluentUI={usesFluentUI}
                title={option.title}
                sub={option.sub}
                onClick={() => onPatchDraft({ requestPolicy: option.id })}
              />
            );
          })}
        </div>
        <Hint>
          `OpenAI 标准` 更适合直连 OpenAI 或严格兼容实现。`兼容中转扩展` 会额外发送一些 relay 常见扩展字段。
        </Hint>
      </Field>

      <Field label={<>上游 BASE_URL <Req /></>}>
        <input
          type="text"
          value={draft.baseURL}
          placeholder="https://your-relay.example.com"
          onChange={(e) => onPatchDraft({ baseURL: e.target.value })}
          spellCheck={false}
          className={`focus-ring w-full min-w-0 border border-black/[0.08] bg-[var(--surface)] px-3 py-2 text-sm text-zinc-900 placeholder:text-zinc-400 dark:border-white/[0.08] dark:text-zinc-100 dark:placeholder:text-zinc-500 font-mono-token ${usesFluentUI ? "rounded-[10px]" : "rounded-[14px]"}`}
        />
        {baseURLError ? <Hint>{baseURLError}</Hint> : null}
        <Hint>
          中转站只填站点根地址，应用会自动拼接当前 API 路径；Google 官方 Images 配置填写 <code className="font-mono-token">https://generativelanguage.googleapis.com/v1beta/openai</code>。除该官方兼容地址外，<strong>不要</strong>手动贴入具体接口路径。
        </Hint>
      </Field>

      <Field label="连接安全">
        <button
          type="button"
          role="switch"
          aria-checked={draft.allowInsecureConnection === true}
          onClick={() => onPatchDraft({ allowInsecureConnection: !(draft.allowInsecureConnection === true) })}
          className={`flex w-full items-center justify-between gap-3 border px-3 py-2.5 text-left text-sm transition-colors ${
            draft.allowInsecureConnection
              ? "border-red-500/30 bg-red-500/[0.06] text-red-700 dark:text-red-300"
              : "border-black/[0.08] bg-[var(--surface)] text-zinc-900 hover:border-[color:var(--accent)]/35 dark:border-white/[0.08] dark:text-zinc-100"
          } ${usesFluentUI ? "rounded-[10px]" : "rounded-[14px]"}`}
        >
          <span className="min-w-0">
            <span className="block font-medium">允许不安全连接</span>
            <span className="mt-1 block break-words text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-400">
              允许远程 <code className="font-mono-token">http://</code>，并忽略 HTTPS / WSS 的自签名、过期或域名不匹配证书。
            </span>
          </span>
          <span
            className={`inline-flex min-h-[26px] min-w-[58px] shrink-0 items-center justify-center border px-2.5 text-[11px] font-semibold ${
              draft.allowInsecureConnection
                ? "border-red-500/25 bg-red-500/10 text-red-700 dark:text-red-300"
                : "border-black/[0.08] bg-black/[0.04] text-zinc-500 dark:border-white/[0.08] dark:bg-white/[0.05] dark:text-zinc-300"
            } ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
          >
            {draft.allowInsecureConnection ? "已开启" : "已关闭"}
          </span>
        </button>
        <Hint>仅在可信网络中使用。开启后 API Key、提示词和图片可能被窃听或篡改。</Hint>
      </Field>

      <Field label={<>API Key <Req /></>}>
        <div className="relative min-w-0">
          <input
            type={showKey ? "text" : "password"}
            value={draftKey}
            placeholder={savedKeyLoaded ? "sk-..." : "(加载中...)"}
            onChange={(e) => onChangeDraftKey(e.target.value)}
            spellCheck={false}
            autoComplete="off"
            className={`focus-ring w-full min-w-0 border border-black/[0.08] bg-[var(--surface)] py-2 pl-3 pr-10 text-sm text-zinc-900 placeholder:text-zinc-400 dark:border-white/[0.08] dark:text-zinc-100 dark:placeholder:text-zinc-500 font-mono-token ${usesFluentUI ? "rounded-[10px]" : "rounded-[14px]"}`}
          />
          <button
            type="button"
            onClick={onToggleShowKey}
            title={showKey ? "隐藏" : "显示"}
            className={`absolute right-2 top-1/2 -translate-y-1/2 p-1 text-zinc-500 hover:bg-[var(--accent-soft)] hover:text-[var(--accent)] ${usesFluentUI ? "rounded-[6px]" : "rounded-full"}`}
          >
            {showKey ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
          </button>
        </div>
        <Hint>API Key 保存到系统凭据存储(Keychain / Credential Manager / Secret Service),不在 localStorage 中明文存放。</Hint>
      </Field>

      <Field
        label={(
          <span className="flex items-center justify-between gap-3">
            <span>上游模型列表</span>
            {modelCatalog ? (
              <span className="shrink-0 text-[11px] font-medium text-[var(--accent)]">已识别 {modelCatalog.all.length} 个模型</span>
            ) : null}
          </span>
        )}
      >
        <button
          type="button"
          onClick={() => void onLoadModels()}
          disabled={loadingModels}
          className={`platform-action-btn inline-flex w-full items-center justify-center gap-2 border border-black/[0.08] px-3 py-2 text-sm text-zinc-700 transition-colors hover:border-[color:var(--accent)]/35 hover:text-[var(--accent)] disabled:cursor-not-allowed disabled:opacity-50 dark:border-white/[0.08] dark:text-zinc-300 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
        >
          <RefreshCw className={`h-3.5 w-3.5 ${loadingModels ? "animate-spin" : ""}`} />
          {loadingModels ? "拉取中..." : "拉取并解析上游模型"}
        </button>
        <Hint>
          通过宿主侧请求 <code className="font-mono-token">/v1/models</code> 获取模型列表，避免浏览器跨域或 WebView 差异影响结果。
        </Hint>
        {modelCatalogError ? <Hint>{modelCatalogError}</Hint> : null}
      </Field>

      {draft.apiMode === "responses" ? (
        <>
          <Field
            label={(
              <span className="flex items-center justify-between gap-3">
                <span>Responses 传输</span>
                <span className="shrink-0 text-[11px] font-medium text-[var(--accent)]">
                  已选 {(draft.responsesTransport ?? "sse") === "websocket" ? "WebSocket mode" : "HTTP SSE"}
                </span>
              </span>
            )}
          >
            <div className={`grid gap-2 ${isAndroidPhone ? "grid-cols-1" : "grid-cols-2"}`}>
              {responsesTransportOptions.map((option) => {
                const active = (draft.responsesTransport ?? "sse") === option.id;
                return (
                  <OptionCard
                    key={option.id}
                    active={active}
                    usesFluentUI={usesFluentUI}
                    title={option.title}
                    sub={option.sub}
                    onClick={() => onPatchDraft({ responsesTransport: option.id })}
                  />
                );
              })}
            </div>
            <Hint>
              这是 Responses API 的传输方式，不是 Realtime API。当前仅桌面本地内核与 Android 壳层支持 WebSocket mode。
            </Hint>
          </Field>

          <Field label="文本模型 ID">
            <input
              type="text"
              value={draft.textModelID}
              placeholder="留空=默认 gpt-5.5"
              onChange={(e) => onPatchDraft({ textModelID: e.target.value })}
              spellCheck={false}
              className={`focus-ring w-full min-w-0 border border-black/[0.08] bg-[var(--surface)] px-3 py-2 text-sm text-zinc-900 placeholder:text-zinc-400 dark:border-white/[0.08] dark:text-zinc-100 dark:placeholder:text-zinc-500 font-mono-token ${usesFluentUI ? "rounded-[10px]" : "rounded-[14px]"}`}
            />
            {preferredModels && preferredModels.text.length > 0 ? (
              <ModelSuggestions
                title="推荐文本模型"
                models={preferredModels.text}
                selectedID={draft.textModelID}
                usesFluentUI={usesFluentUI}
                onSelect={(id) => onPatchDraft({ textModelID: id })}
              />
            ) : null}
          </Field>

          <Field
            label={(
              <span className="flex items-center justify-between gap-3">
                <span>推理强度</span>
                <span className="shrink-0 text-[11px] font-medium text-[var(--accent)]">已选 {selectedReasoningEffort.label}</span>
              </span>
            )}
          >
            <div className={`grid gap-2 ${isAndroidPhone ? "grid-cols-2" : "grid-cols-4"}`}>
              {REASONING_EFFORT_OPTIONS.map((option) => {
                const active = draft.reasoningEffort === option.value;
                return (
                  <OptionCard
                    key={option.value}
                    active={active}
                    usesFluentUI={usesFluentUI}
                    title={option.label}
                    onClick={() => onPatchDraft({ reasoningEffort: option.value })}
                  />
                );
              })}
            </div>
            <Hint>
              默认 <code className="font-mono-token">xhigh</code>。低强度在部分模型或中转上可能不触发 <code className="font-mono-token">image_generation</code> 工具调用，优先保持 <code className="font-mono-token">xhigh</code> 或 <code className="font-mono-token">high</code>。
            </Hint>
          </Field>
        </>
      ) : null}

      <Field label="图像模型 ID">
        <input
          type="text"
          value={draft.imageModelID}
          placeholder="留空=默认 gpt-image-2"
          onChange={(e) => onPatchDraft({ imageModelID: e.target.value })}
          spellCheck={false}
          className={`focus-ring w-full min-w-0 border border-black/[0.08] bg-[var(--surface)] px-3 py-2 text-sm text-zinc-900 placeholder:text-zinc-400 dark:border-white/[0.08] dark:text-zinc-100 dark:placeholder:text-zinc-500 font-mono-token ${usesFluentUI ? "rounded-[10px]" : "rounded-[14px]"}`}
        />
        {preferredModels && preferredModels.image.length > 0 ? (
          <ModelSuggestions
            title="推荐图像模型"
            models={preferredModels.image}
            selectedID={draft.imageModelID}
            usesFluentUI={usesFluentUI}
            onSelect={(id) => onPatchDraft({ imageModelID: id })}
          />
        ) : null}
      </Field>

      <Field label="并发数量限制">
        <input
          type="number"
          value={draft.concurrencyLimit || ""}
          placeholder="留空=不限制"
          min={0}
          step={1}
          onChange={(e) => onPatchDraft({ concurrencyLimit: Math.max(0, Math.floor(Number(e.target.value) || 0)) })}
          className={`focus-ring w-full min-w-0 border border-black/[0.08] bg-[var(--surface)] px-3 py-2 text-sm text-zinc-900 placeholder:text-zinc-400 dark:border-white/[0.08] dark:text-zinc-100 dark:placeholder:text-zinc-500 font-mono-token ${usesFluentUI ? "rounded-[10px]" : "rounded-[14px]"}`}
        />
        <Hint>0/留空 = 不限制。填正整数后,此 profile 跨所有标签页最多同时运行这么多任务。</Hint>
      </Field>

      <Field label="失败重试路由到">
        <select
          value={draft.fallbackProfileId ?? ""}
          onChange={(e) => onPatchDraft({ fallbackProfileId: e.target.value || undefined })}
          className={`focus-ring w-full min-w-0 border border-black/[0.08] bg-[var(--surface)] px-3 py-2 text-sm text-zinc-900 dark:border-white/[0.08] dark:text-zinc-100 ${usesFluentUI ? "rounded-[10px]" : "rounded-[14px]"}`}
        >
          <option value="">不自动切备用上游</option>
          {fallbackCandidates.map((profile) => (
            <option key={profile.id} value={profile.id}>
              {profile.name} · {profile.apiMode === "responses" ? "Responses" : "Images"}
            </option>
          ))}
        </select>
        <Hint>
          当前上游自动重试仍失败后，可额外切到这里选定的备用 profile 再尝试一次。默认关闭。
        </Hint>
      </Field>

      {draft.apiMode === "images" ? (
        <Field label="Images API 中转兼容">
          <button
            type="button"
            role="switch"
            aria-checked={draft.imagesNewAPICompat === true}
            onClick={() => onPatchDraft({ imagesNewAPICompat: !(draft.imagesNewAPICompat === true) })}
            className={`flex w-full items-center justify-between gap-3 border border-black/[0.08] bg-[var(--surface)] px-3 py-2.5 text-left text-sm text-zinc-900 transition-colors hover:border-[color:var(--accent)]/35 dark:border-white/[0.08] dark:text-zinc-100 ${usesFluentUI ? "rounded-[10px]" : "rounded-[14px]"}`}
          >
            <span className="min-w-0">
              <span className="block min-w-0 break-words font-medium">开启此开关可能可以解决newapi生图问题</span>
              <span className="mt-1 block min-w-0 break-words text-[11px] leading-relaxed text-zinc-500 dark:text-zinc-400">
                开启后会为 Images API 强制发送 <code className="font-mono-token">response_format=b64_json</code>，并关闭 <code className="font-mono-token">stream</code> / <code className="font-mono-token">partial_images</code>，更适合部分 NewAPI / Packy 风格中转站。
              </span>
            </span>
            <span
              className={`inline-flex min-h-[26px] min-w-[58px] shrink-0 items-center justify-center border px-2.5 text-[11px] font-semibold tracking-[0.04em] ${
                draft.imagesNewAPICompat
                  ? "border-[color:var(--accent)]/20 bg-[var(--accent-soft)] text-[var(--accent)]"
                  : "border-black/[0.08] bg-black/[0.04] text-zinc-500 dark:border-white/[0.08] dark:bg-white/[0.05] dark:text-zinc-300"
              } ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
            >
              {draft.imagesNewAPICompat ? "已开启" : "已关闭"}
            </span>
          </button>
          <Hint>
            默认关闭，保持 OpenAI 标准 Images API 请求。只有默认标准参数用不了时，再尝试开启。
          </Hint>
        </Field>
      ) : null}

      <button
        type="button"
        onClick={() => void onTest()}
        disabled={!canSave || isTestingKey}
        className={`platform-action-btn w-full inline-flex items-center justify-center gap-2 border border-black/[0.08] px-3 py-2 text-sm text-zinc-700 transition-colors hover:border-[color:var(--accent)]/35 hover:text-[var(--accent)] disabled:cursor-not-allowed disabled:opacity-50 dark:border-white/[0.08] dark:text-zinc-300 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
      >
        <Plug className={`h-3.5 w-3.5 ${isTestingKey ? "animate-spin" : ""}`} />
        {isTestingKey ? "测试中..." : "保存并测试连接"}
      </button>

      <div className={`flex gap-2 pt-1 ${isAndroidPhone ? "sticky bottom-0 -mx-4 mt-1 border-t border-black/[0.06] bg-white/92 px-4 pb-4 pt-3 dark:border-white/[0.04] dark:bg-zinc-900/92" : "justify-end"}`}>
        <button
          type="button"
          onClick={onClose}
          className={`platform-action-btn border border-black/[0.08] px-4 py-2 text-sm text-zinc-700 transition-colors hover:bg-black/[0.04] dark:border-white/[0.08] dark:text-zinc-300 dark:hover:bg-white/[0.06] ${isAndroidPhone ? "flex-1 rounded-full" : usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
        >
          关闭
        </button>
        <button
          type="button"
          onClick={() => void onSaveAndClose()}
          disabled={!canSave}
          className={`liquid-primary-button bg-[var(--accent)] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--accent-2)] disabled:cursor-not-allowed disabled:bg-zinc-200 disabled:text-zinc-500 dark:disabled:bg-zinc-800 ${isAndroidPhone ? "flex-[1.2] rounded-full" : usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
        >
          保存
        </button>
      </div>

      {!canSave ? <p className="min-w-0 break-words text-[11px] text-zinc-500 [overflow-wrap:anywhere]">BASE_URL 和 API Key 必须填齐才能保存。</p> : null}

      {draft.apiMode === "images" ? (
        <div className={`${usesAppleUI ? "liquid-glass-panel" : ""} flex items-start gap-2 border border-[color:var(--accent)]/20 bg-[var(--accent-soft)] px-3 py-2 text-[11px] text-[var(--accent)] ${usesFluentUI ? "rounded-[10px]" : "rounded-[14px]"}`}>
          <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          <span className="min-w-0 break-words [overflow-wrap:anywhere]">Images API 通常走标准 <code className="font-mono-token">/v1/images/generations</code> + <code className="font-mono-token">/v1/images/edits</code>；Google 官方 <code className="font-mono-token">gemini-3.1-flash-image</code> 例外走 Interactions API。两者都没有 SSE 保活。</span>
        </div>
      ) : null}
    </div>
  );
}

function Field({ label, children }: { label: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="upstream-field min-w-0">
      <label className="mb-1.5 block min-w-0 break-words text-xs text-zinc-600 [overflow-wrap:anywhere] dark:text-zinc-400">{label}</label>
      {children}
    </div>
  );
}

function Hint({ children }: { children: React.ReactNode }) {
  return (
    <p className="mt-1.5 min-w-0 break-words text-[11px] leading-relaxed text-zinc-500 [overflow-wrap:anywhere] dark:text-zinc-500">{children}</p>
  );
}

function OptionCard({
  active,
  usesFluentUI,
  title,
  sub,
  onClick,
}: {
  active: boolean;
  usesFluentUI: boolean;
  title: string;
  sub?: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={`upstream-option-card platform-card flex flex-col items-start gap-1 border p-2.5 text-left transition-all ${
        active
          ? "active border-[color:var(--accent)]/55 bg-[var(--accent-soft)] text-zinc-950 shadow-[0_0_0_1px_rgb(0_122_255_/_0.12)] dark:text-zinc-50"
          : "border-black/[0.08] text-zinc-700 hover:border-[color:var(--accent)]/30 hover:bg-white/80 dark:border-white/[0.06] dark:text-zinc-300 dark:hover:bg-white/[0.05]"
      } ${usesFluentUI ? "rounded-[8px]" : "rounded-[14px]"}`}
    >
      <span className="upstream-option-head flex w-full items-start justify-between gap-2">
        <span className="upstream-option-title min-w-0 text-[12px] font-semibold">{title}</span>
        {active ? (
          <span className="upstream-option-check inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-white/78 text-[var(--accent)] shadow-[0_4px_10px_-8px_rgb(0_122_255_/_0.85)] dark:bg-white/[0.08]">
            <CheckCircle2 className="h-3.5 w-3.5" />
          </span>
        ) : null}
      </span>
      {sub ? <span className={`upstream-option-sub min-w-0 text-[10px] ${active ? "text-[var(--accent)]/90" : "text-zinc-500"}`}>{sub}</span> : null}
    </button>
  );
}

function ModelSuggestions({
  title,
  models,
  selectedID,
  usesFluentUI,
  onSelect,
}: {
  title: string;
  models: UpstreamModelDescriptor[];
  selectedID: string;
  usesFluentUI: boolean;
  onSelect: (id: string) => void;
}) {
  return (
    <div className="mt-2 flex flex-col gap-2">
      <span className="text-[11px] font-medium text-zinc-500 dark:text-zinc-400">{title}</span>
      <div className="flex flex-wrap gap-2">
        {models.slice(0, 12).map((model) => {
          const active = model.id === selectedID.trim();
          return (
            <button
              key={model.id}
              type="button"
              onClick={() => onSelect(model.id)}
              className={`inline-flex max-w-full items-center gap-1.5 border px-2.5 py-1.5 text-left text-[11px] transition-colors ${
                active
                  ? "border-[color:var(--accent)]/45 bg-[var(--accent-soft)] text-[var(--accent)]"
                  : "border-black/[0.08] bg-black/[0.03] text-zinc-600 hover:border-[color:var(--accent)]/30 hover:text-[var(--accent)] dark:border-white/[0.08] dark:bg-white/[0.04] dark:text-zinc-300"
              } ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
            >
              <span className="min-w-0 break-words [overflow-wrap:anywhere]">{formatUpstreamModelLabel(model)}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

function Req() {
  return <span className="text-red-500">*</span>;
}
