import type { APIMode, RequestPolicy, ReasoningEffortValue, ResponsesTransport, UpstreamProfile, UpstreamProvider } from "../types/domain";
import { normalizeUpstreamProvider } from "./profiles.ts";
import { buildUpstreamModelCatalog } from "./upstreamModels.ts";
import { cleanBaseURL } from "./security.ts";

export type UpstreamConfigExportProfile = {
  id: string;
  name: string;
  provider?: UpstreamProvider;
  apiMode: APIMode;
  responsesTransport: ResponsesTransport;
  requestPolicy: RequestPolicy;
  imagesNewAPICompat: boolean;
  allowInsecureConnection: boolean;
  baseURL: string;
  textModelID: string;
  imageModelID: string;
  reasoningEffort: ReasoningEffortValue;
  concurrencyLimit: number;
  fallbackProfileId?: string;
  createdAt: number;
  lastUsedAt?: number;
  apiKey?: string;
};

export type UpstreamConfigExportFile = {
  version: 1;
  exportedAt: string;
  activeProfileId: string;
  aiProfileId?: string;
  profiles: UpstreamConfigExportProfile[];
};

export type ParsedUpstreamConfigImport = {
  activeProfileId: string;
  aiProfileId?: string;
  profiles: UpstreamConfigExportProfile[];
};

export type UpstreamConfigImportActions = {
  getProfiles: () => UpstreamProfile[];
  createProfile: (input: {
    name?: string;
    provider?: UpstreamProvider;
    apiMode: APIMode;
    responsesTransport?: ResponsesTransport;
    baseURL?: string;
    requestPolicy?: RequestPolicy;
    imagesNewAPICompat?: boolean;
    allowInsecureConnection?: boolean;
    textModelID?: string;
    imageModelID?: string;
    reasoningEffort?: ReasoningEffortValue;
    concurrencyLimit?: number;
    apiKey?: string;
    setActive?: boolean;
  }) => Promise<string>;
  updateProfile: (
    id: string,
    patch: Partial<Omit<UpstreamProfile, "id" | "createdAt">> & { apiKey?: string },
  ) => Promise<boolean>;
  setActiveProfile: (id: string) => Promise<void>;
  setAIProfile?: (id: string) => Promise<boolean>;
};

export type AppliedUpstreamConfigImport = {
  importedCount: number;
  activeProfileId: string;
  aiProfileId?: string;
  importedProfileIds: string[];
};

function toProfileSnapshot(input: UpstreamConfigExportProfile, actualId: string): UpstreamProfile {
  return {
    id: actualId,
    name: input.name,
    provider: input.provider ?? "openai",
    apiMode: input.apiMode,
    responsesTransport: input.responsesTransport ?? "sse",
    requestPolicy: input.requestPolicy,
    imagesNewAPICompat: input.imagesNewAPICompat,
    allowInsecureConnection: input.allowInsecureConnection,
    baseURL: input.baseURL,
    textModelID: input.textModelID,
    imageModelID: input.imageModelID,
    reasoningEffort: input.reasoningEffort,
    concurrencyLimit: input.concurrencyLimit,
    fallbackProfileId: input.fallbackProfileId,
    createdAt: input.createdAt,
    lastUsedAt: input.lastUsedAt,
  };
}

function normalizeAPIMode(value: unknown): APIMode {
  return value === "images" ? "images" : "responses";
}

function normalizeRequestPolicy(value: unknown): RequestPolicy {
  return value === "compat" ? "compat" : "openai";
}

function normalizeResponsesTransport(value: unknown): ResponsesTransport {
  return value === "websocket" ? "websocket" : "sse";
}

function normalizeReasoningEffort(value: unknown): ReasoningEffortValue {
  return value === "low" || value === "medium" || value === "high" || value === "xhigh"
    ? value
    : "xhigh";
}

function normalizeConcurrencyLimit(value: unknown): number {
  if (typeof value !== "number" || !Number.isFinite(value) || value < 0) return 0;
  return Math.floor(value);
}

function normalizeImportedBaseURL(value: unknown): string {
  const trimmed = typeof value === "string" ? value.trim() : "";
  if (!trimmed) return "";
  return cleanBaseURL(trimmed).replace(/\/v1$/i, "");
}

function stripWrappedCodeFence(rawJSON: string): string {
  const trimmed = rawJSON.trim();
  const fenced = trimmed.match(/^```(?:json)?\s*([\s\S]*?)\s*```$/i);
  return fenced ? fenced[1].trim() : trimmed;
}

function hostLabelFromBaseURL(baseURL: string): string {
  try {
    return new URL(baseURL).host;
  } catch {
    return baseURL.replace(/^https?:\/\//i, "");
  }
}

function buildTemplateProfileName(prefix: string, baseURL: string, providerName = ""): string {
  const host = hostLabelFromBaseURL(baseURL);
  const parts = [prefix.trim(), providerName.trim(), host.trim()].filter(Boolean);
  return parts.join(" · ") || "快捷导入";
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}

function nextTemplateProfileId(index: number): string {
  return `template-${index}`;
}

function parseExportProfile(raw: unknown): UpstreamConfigExportProfile | null {
  if (!raw || typeof raw !== "object") return null;
  const source = raw as Record<string, unknown>;
  const id = typeof source.id === "string" ? source.id.trim() : "";
  const name = typeof source.name === "string" ? source.name.trim() : "";
  if (!id || !name) return null;
  const provider = normalizeUpstreamProvider(source.provider);
  return {
    id,
    name,
    ...(source.provider !== undefined ? { provider } : {}),
    apiMode: provider === "openai" ? normalizeAPIMode(source.apiMode) : "images",
    responsesTransport: normalizeResponsesTransport(source.responsesTransport),
    requestPolicy: normalizeRequestPolicy(source.requestPolicy),
    imagesNewAPICompat: source.imagesNewAPICompat === true,
    allowInsecureConnection: source.allowInsecureConnection === true,
    baseURL: normalizeImportedBaseURL(source.baseURL),
    textModelID: typeof source.textModelID === "string" ? source.textModelID.trim() : "",
    imageModelID: typeof source.imageModelID === "string" ? source.imageModelID.trim() : "",
    reasoningEffort: normalizeReasoningEffort(source.reasoningEffort),
    concurrencyLimit: normalizeConcurrencyLimit(source.concurrencyLimit),
    fallbackProfileId: typeof source.fallbackProfileId === "string" ? source.fallbackProfileId.trim() || undefined : undefined,
    createdAt: typeof source.createdAt === "number" && Number.isFinite(source.createdAt) ? source.createdAt : Date.now(),
    lastUsedAt: typeof source.lastUsedAt === "number" && Number.isFinite(source.lastUsedAt) ? source.lastUsedAt : undefined,
    apiKey: typeof source.apiKey === "string" ? source.apiKey.trim() : undefined,
  };
}

function parseNewAPIChannelConnTemplate(raw: Record<string, unknown>): ParsedUpstreamConfigImport | null {
  if (String(raw._type || "").trim() !== "newapi_channel_conn") return null;
  const apiKey = typeof raw.key === "string" ? raw.key.trim() : "";
  const baseURL = normalizeImportedBaseURL(raw.url);
  if (!apiKey || !baseURL) {
    throw new Error("newapi 模板缺少 key 或 url");
  }
  const profileId = nextTemplateProfileId(1);
  return {
    activeProfileId: profileId,
    profiles: [
      {
        id: profileId,
        name: buildTemplateProfileName("NewAPI", baseURL),
        apiMode: "responses",
        responsesTransport: "sse",
        requestPolicy: "openai",
        imagesNewAPICompat: false,
        allowInsecureConnection: false,
        baseURL,
        textModelID: "",
        imageModelID: "",
        reasoningEffort: "xhigh",
        concurrencyLimit: 0,
        createdAt: Date.now(),
        apiKey,
      },
    ],
  };
}

function buildOpenCodeModelCatalog(rawModels: unknown) {
  const models = asRecord(rawModels);
  if (!models) return buildUpstreamModelCatalog([]);
  return buildUpstreamModelCatalog(
    Object.entries(models).map(([id, raw]) => {
      const source = asRecord(raw);
      return {
        id,
        displayName: typeof source?.name === "string" ? source.name.trim() : "",
      };
    }),
  );
}

function inferReasoningEffortFromOpenCodeModel(rawModels: unknown, textModelID: string): ReasoningEffortValue {
  const models = asRecord(rawModels);
  const model = models ? asRecord(models[textModelID]) : null;
  const variants = model ? asRecord(model.variants) : null;
  const available = new Set(Object.keys(variants ?? {}));
  if (available.has("xhigh")) return "xhigh";
  if (available.has("high")) return "high";
  if (available.has("medium")) return "medium";
  if (available.has("low")) return "low";
  return "xhigh";
}

function parseOpenCodeProviderTemplate(raw: Record<string, unknown>): ParsedUpstreamConfigImport | null {
  const providerRoot = asRecord(raw.provider);
  if (!providerRoot) return null;

  const profiles: UpstreamConfigExportProfile[] = [];
  let nextIndex = 1;
  for (const [providerName, providerValue] of Object.entries(providerRoot)) {
    const provider = asRecord(providerValue);
    const options = provider ? asRecord(provider.options) : null;
    const apiKey = typeof options?.apiKey === "string" ? options.apiKey.trim() : "";
    const baseURL = normalizeImportedBaseURL(options?.baseURL ?? options?.baseUrl);
    if (!apiKey || !baseURL) continue;

    const catalog = buildOpenCodeModelCatalog(provider?.models);
    const hasTextModels = catalog.text.length > 0;
    const hasImageModels = catalog.image.length > 0;
    const apiMode: APIMode = hasTextModels ? "responses" : hasImageModels ? "images" : "responses";
    const textModelID = hasTextModels ? catalog.text[0]?.id ?? "" : "";
    const imageModelID = hasImageModels ? catalog.image[0]?.id ?? "" : "";
    const profileId = nextTemplateProfileId(nextIndex);
    nextIndex += 1;

    profiles.push({
      id: profileId,
      name: buildTemplateProfileName("OpenCode", baseURL, providerName),
      apiMode,
      responsesTransport: "sse",
      requestPolicy: "openai",
      imagesNewAPICompat: false,
      allowInsecureConnection: false,
      baseURL,
      textModelID,
      imageModelID,
      reasoningEffort: inferReasoningEffortFromOpenCodeModel(provider?.models, textModelID),
      concurrencyLimit: 0,
      createdAt: Date.now(),
      apiKey,
    });
  }

  if (profiles.length === 0) {
    throw new Error("OpenCode 模板里没有可用的 provider 配置");
  }

  return {
    activeProfileId: profiles[0]?.id ?? "",
    profiles,
  };
}

export function buildUpstreamConfigExportFile(
  profiles: UpstreamProfile[],
  activeProfileId: string,
  apiKeysById: Record<string, string>,
  aiProfileId = "",
): UpstreamConfigExportFile {
  return {
    version: 1,
    exportedAt: new Date().toISOString(),
    activeProfileId,
    ...(aiProfileId ? { aiProfileId } : {}),
    profiles: profiles.map((profile) => ({
      ...profile,
      responsesTransport: profile.responsesTransport === "websocket" ? "websocket" : "sse",
      imagesNewAPICompat: profile.imagesNewAPICompat === true,
      allowInsecureConnection: profile.allowInsecureConnection === true,
      apiKey: (apiKeysById[profile.id] ?? "").trim(),
    })),
  };
}

export function parseUpstreamConfigImportFile(rawJSON: string): ParsedUpstreamConfigImport {
  const parsed = JSON.parse(stripWrappedCodeFence(rawJSON)) as Record<string, unknown>;
  const fromNewAPI = parseNewAPIChannelConnTemplate(parsed);
  if (fromNewAPI) return fromNewAPI;

  const fromOpenCode = parseOpenCodeProviderTemplate(parsed);
  if (fromOpenCode) return fromOpenCode;

  const profiles = Array.isArray(parsed?.profiles)
    ? parsed.profiles.map(parseExportProfile).filter((item): item is UpstreamConfigExportProfile => item !== null)
    : [];
  if (profiles.length === 0) {
    throw new Error("暂不支持这类 JSON。当前支持：本应用导出文件、newapi_channel_conn、OpenCode provider 配置");
  }
  const activeProfileId = typeof parsed?.activeProfileId === "string" ? parsed.activeProfileId.trim() : "";
  const aiProfileId = typeof parsed?.aiProfileId === "string" ? parsed.aiProfileId.trim() : "";
  return {
    activeProfileId,
    ...(aiProfileId ? { aiProfileId } : {}),
    profiles,
  };
}

function buildProfilePatch(
  incoming: UpstreamConfigExportProfile,
): Partial<Omit<UpstreamProfile, "id" | "createdAt">> & { apiKey?: string } {
  return {
    name: incoming.name,
    provider: incoming.provider ?? "openai",
    apiMode: incoming.apiMode,
    responsesTransport: incoming.responsesTransport ?? "sse",
    requestPolicy: incoming.requestPolicy,
    imagesNewAPICompat: incoming.imagesNewAPICompat,
    allowInsecureConnection: incoming.allowInsecureConnection,
    baseURL: incoming.baseURL,
    textModelID: incoming.textModelID,
    imageModelID: incoming.imageModelID,
    reasoningEffort: incoming.reasoningEffort,
    concurrencyLimit: incoming.concurrencyLimit,
    lastUsedAt: incoming.lastUsedAt,
    apiKey: incoming.apiKey,
  };
}

export async function applyParsedUpstreamConfigImport(
  parsed: ParsedUpstreamConfigImport,
  actions: UpstreamConfigImportActions,
): Promise<AppliedUpstreamConfigImport> {
  const existingByName = new Map(actions.getProfiles().map((profile) => [profile.name, profile]));
  const originalToActualID = new Map<string, string>();
  const fallbackLinks: Array<{ actualId: string; originalFallbackId?: string }> = [];
  const importedProfileIds: string[] = [];

  for (const incoming of parsed.profiles) {
    const match = existingByName.get(incoming.name) ?? null;
    const patch = buildProfilePatch(incoming);
    if (match) {
      await actions.updateProfile(match.id, patch);
      originalToActualID.set(incoming.id, match.id);
      importedProfileIds.push(match.id);
      fallbackLinks.push({ actualId: match.id, originalFallbackId: incoming.fallbackProfileId });
      existingByName.set(incoming.name, toProfileSnapshot(incoming, match.id));
      continue;
    }

    const newId = await actions.createProfile({
      name: incoming.name,
      provider: incoming.provider ?? "openai",
      apiMode: incoming.apiMode,
      responsesTransport: incoming.responsesTransport ?? "sse",
      requestPolicy: incoming.requestPolicy,
      imagesNewAPICompat: incoming.imagesNewAPICompat,
      allowInsecureConnection: incoming.allowInsecureConnection,
      baseURL: incoming.baseURL,
      textModelID: incoming.textModelID,
      imageModelID: incoming.imageModelID,
      reasoningEffort: incoming.reasoningEffort,
      concurrencyLimit: incoming.concurrencyLimit,
      apiKey: incoming.apiKey,
      setActive: false,
    });
    originalToActualID.set(incoming.id, newId);
    importedProfileIds.push(newId);
    fallbackLinks.push({ actualId: newId, originalFallbackId: incoming.fallbackProfileId });
    existingByName.set(incoming.name, toProfileSnapshot(incoming, newId));
  }

  const currentProfiles = actions.getProfiles();
  for (const link of fallbackLinks) {
    const requestedFallbackId = link.originalFallbackId?.trim();
    if (!requestedFallbackId) continue;
    const resolvedFallbackId = originalToActualID.get(requestedFallbackId)
      ?? currentProfiles.find((profile) => profile.id === requestedFallbackId)?.id
      ?? "";
    if (!resolvedFallbackId) continue;
    await actions.updateProfile(link.actualId, { fallbackProfileId: resolvedFallbackId });
  }

  const nextActiveProfileId = parsed.activeProfileId.trim()
    ? (originalToActualID.get(parsed.activeProfileId.trim()) ?? "")
    : "";
  if (nextActiveProfileId) {
    await actions.setActiveProfile(nextActiveProfileId);
  }
  const nextAIProfileId = parsed.aiProfileId?.trim()
    ? (originalToActualID.get(parsed.aiProfileId.trim()) ?? "")
    : "";
  if (nextAIProfileId && actions.setAIProfile) {
    await actions.setAIProfile(nextAIProfileId);
  }

  return {
    importedCount: parsed.profiles.length,
    activeProfileId: nextActiveProfileId,
    ...(nextAIProfileId ? { aiProfileId: nextAIProfileId } : {}),
    importedProfileIds,
  };
}
