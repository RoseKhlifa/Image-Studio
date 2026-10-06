import {
  DeleteStoredAPIKey,
  GetStoredAPIKey,
  SetStoredAPIKey,
} from "../platform/runtime/host";
import type { APIMode, ReasoningEffortValue, RequestPolicy, UpstreamProfile, UpstreamProvider } from "../types/domain";
import type { StudioState } from "./studioStore.types";
import {
  duplicateProfile as cloneProfile,
  genProfileId,
  keyringUserFor,
  nextDefaultProfileName,
  normalizeResponsesTransport,
  pickActiveProfile,
  pickAIProfile,
  persistAIProfileId,
} from "../lib/profiles";
import { cleanBaseURL } from "../lib/security";
import { normalizeConcurrencyLimit } from "./workspaceRuntime";
import { persistActiveProfileId, persistProfiles } from "./studioStore.shared";

type StateAdapter = {
  getState: () => StudioState;
  setState: (patch: Partial<StudioState> | ((state: StudioState) => Partial<StudioState>)) => void;
};

export function createProfileActions(store: StateAdapter) {
  return {
    async createProfile(input: {
      name?: string;
      provider?: UpstreamProvider;
      apiMode: APIMode;
      responsesTransport?: UpstreamProfile["responsesTransport"];
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
    }) {
      const list = store.getState().profiles;
      const id = genProfileId();
      const profile: UpstreamProfile = {
        id,
        name: input.name?.trim() || nextDefaultProfileName(list),
        provider: input.provider ?? "openai",
        apiMode: input.provider && input.provider !== "openai" ? "images" : input.apiMode,
        responsesTransport: normalizeResponsesTransport(input.apiMode === "responses" ? input.responsesTransport : "sse"),
        requestPolicy: input.requestPolicy ?? "openai",
        imagesNewAPICompat: input.imagesNewAPICompat === true,
        allowInsecureConnection: input.allowInsecureConnection === true,
        baseURL: cleanBaseURL(input.baseURL ?? ""),
        textModelID: (input.textModelID ?? "").trim(),
        imageModelID: (input.imageModelID ?? "").trim(),
        reasoningEffort: input.reasoningEffort ?? "xhigh",
        concurrencyLimit: normalizeConcurrencyLimit(input.concurrencyLimit ?? 0),
        fallbackProfileId: undefined,
        createdAt: Date.now(),
      };
      if ((input.apiKey ?? "").trim()) {
        try { await SetStoredAPIKey(keyringUserFor(id), input.apiKey!.trim()); }
        catch (e: any) {
          if (typeof console !== "undefined") console.error("写 keyring 失败", e);
        }
      }
      const next = [...list, profile];
      persistProfiles(next);
      const aiProfile = pickAIProfile(next, store.getState().aiProfileId, store.getState().activeProfileId);
      if (aiProfile?.id && aiProfile.id !== store.getState().aiProfileId) {
        persistAIProfileId(aiProfile.id);
      }
      store.setState({ profiles: next, aiProfileId: aiProfile?.id ?? "" });
      if (input.setActive ?? true) {
        await this.setActiveProfile(id);
      }
      return id;
    },

    async updateProfile(id: string, patch: Partial<Omit<UpstreamProfile, "id" | "createdAt">> & { apiKey?: string }) {
      const list = store.getState().profiles;
      const index = list.findIndex((profile) => profile.id === id);
      if (index < 0) return false;
      const current = list[index];
      const next: UpstreamProfile = {
        ...current,
        name: patch.name !== undefined ? patch.name.trim() : current.name,
        provider: patch.provider ?? current.provider ?? "openai",
        apiMode: (patch.provider ?? current.provider ?? "openai") !== "openai"
          ? "images"
          : patch.apiMode ?? current.apiMode,
        responsesTransport: patch.responsesTransport !== undefined
          ? normalizeResponsesTransport(patch.responsesTransport)
          : normalizeResponsesTransport(current.responsesTransport),
        requestPolicy: patch.requestPolicy ?? current.requestPolicy,
        imagesNewAPICompat: patch.imagesNewAPICompat ?? current.imagesNewAPICompat ?? false,
        allowInsecureConnection: patch.allowInsecureConnection ?? current.allowInsecureConnection ?? false,
        baseURL: patch.baseURL !== undefined ? cleanBaseURL(patch.baseURL) : current.baseURL,
        textModelID: patch.textModelID !== undefined ? patch.textModelID.trim() : current.textModelID,
        imageModelID: patch.imageModelID !== undefined ? patch.imageModelID.trim() : current.imageModelID,
        reasoningEffort: patch.reasoningEffort ?? current.reasoningEffort ?? "xhigh",
        concurrencyLimit: patch.concurrencyLimit !== undefined
          ? normalizeConcurrencyLimit(patch.concurrencyLimit) : current.concurrencyLimit,
        fallbackProfileId: patch.fallbackProfileId !== undefined ? patch.fallbackProfileId || undefined : current.fallbackProfileId,
        lastUsedAt: patch.lastUsedAt ?? current.lastUsedAt,
      };
      const nextList = list.map((profile, idx) => (idx === index ? next : profile));
      persistProfiles(nextList);
      const aiProfile = pickAIProfile(nextList, store.getState().aiProfileId, store.getState().activeProfileId);
      if ((aiProfile?.id ?? "") !== store.getState().aiProfileId) {
        persistAIProfileId(aiProfile?.id ?? "");
      }
      store.setState({ profiles: nextList, aiProfileId: aiProfile?.id ?? "" });
      if (patch.apiKey !== undefined) {
        try { await SetStoredAPIKey(keyringUserFor(id), patch.apiKey); }
        catch (e: any) {
          if (typeof console !== "undefined") console.error("写 keyring 失败", e);
        }
      }
      if (id === store.getState().activeProfileId) {
        const apiKey = patch.apiKey !== undefined ? patch.apiKey.trim() : store.getState().apiKey;
        store.setState({
          provider: next.provider,
          apiMode: next.apiMode,
          responsesTransport: next.responsesTransport ?? "sse",
          requestPolicy: next.requestPolicy,
          imagesNewAPICompat: next.imagesNewAPICompat ?? false,
          baseURL: next.baseURL,
          textModelID: next.textModelID,
          imageModelID: next.imageModelID,
          reasoningEffort: next.reasoningEffort,
          apiKey,
        });
      }
      return true;
    },

    async deleteProfile(id: string) {
      const list = store.getState().profiles;
      const index = list.findIndex((profile) => profile.id === id);
      if (index < 0) return;
      const nextList = list.filter((_, i) => i !== index);
      persistProfiles(nextList);
      try { await DeleteStoredAPIKey(keyringUserFor(id)); }
      catch (e: any) {
        if (typeof console !== "undefined") console.warn("删 keyring 项失败(继续)", e);
      }
      const aiProfile = pickAIProfile(nextList, store.getState().aiProfileId === id ? "" : store.getState().aiProfileId, store.getState().activeProfileId);
      persistAIProfileId(aiProfile?.id ?? "");
      store.setState({ profiles: nextList, aiProfileId: aiProfile?.id ?? "" });
      if (store.getState().activeProfileId === id) {
        const fallback = pickActiveProfile(nextList, "");
        if (fallback) {
          await this.setActiveProfile(fallback.id);
        } else {
          persistActiveProfileId("");
          store.setState({
            profiles: nextList,
            activeProfileId: "",
            apiKey: "",
            baseURL: "",
            textModelID: "",
            imageModelID: "",
            reasoningEffort: "xhigh",
            provider: "openai",
            apiMode: "responses",
            responsesTransport: "sse",
            requestPolicy: "openai",
            imagesNewAPICompat: false,
            upstreamModalOpen: false,
            settingsOpen: true,
            upstreamReturnTarget: "settings",
          });
        }
      }
    },

    async duplicateProfile(id: string) {
      const current = store.getState().profiles.find((profile) => profile.id === id);
      if (!current) return null;
      const cloned = cloneProfile(current);
      try {
        const existingKey = await GetStoredAPIKey(keyringUserFor(id)).catch(() => "");
        if (existingKey) {
          await SetStoredAPIKey(keyringUserFor(cloned.id), existingKey);
        }
      } catch {}
      const next = [...store.getState().profiles, cloned];
      persistProfiles(next);
      store.setState({ profiles: next });
      return cloned.id;
    },

    async setActiveProfile(id: string) {
      const profile = store.getState().profiles.find((p) => p.id === id);
      if (!profile) return;
      persistActiveProfileId(id);
      const apiKey = await GetStoredAPIKey(keyringUserFor(id)).catch(() => "");
      const refreshed: UpstreamProfile = { ...profile, lastUsedAt: Date.now() };
      const nextProfiles = store.getState().profiles.map((p) => p.id === id ? refreshed : p);
      persistProfiles(nextProfiles);
      store.setState({
        profiles: nextProfiles,
        activeProfileId: id,
        provider: profile.provider ?? "openai",
        apiMode: profile.apiMode,
        responsesTransport: profile.responsesTransport ?? "sse",
        requestPolicy: profile.requestPolicy,
        imagesNewAPICompat: profile.imagesNewAPICompat ?? false,
        baseURL: profile.baseURL,
        textModelID: profile.textModelID,
        imageModelID: profile.imageModelID,
        reasoningEffort: profile.reasoningEffort,
        apiKey,
      });
    },

    async setAIProfile(id: string) {
      const profile = store.getState().profiles.find((item) => item.id === id);
      if (!profile || profile.apiMode !== "responses") return false;
      persistAIProfileId(id);
      store.setState({ aiProfileId: id });
      return true;
    },
  };
}
