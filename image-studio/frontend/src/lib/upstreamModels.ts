import type { APIMode } from "../types/domain";
import type { UpstreamModelDescriptorLike } from "../platform/runtime/hostTypes";

export type UpstreamModelDescriptor = {
  id: string;
  object: string;
  ownedBy: string;
  displayName: string;
};

export type UpstreamModelCatalog = {
  all: UpstreamModelDescriptor[];
  text: UpstreamModelDescriptor[];
  image: UpstreamModelDescriptor[];
};

function normalizeModelDescriptor(input: UpstreamModelDescriptorLike): UpstreamModelDescriptor | null {
  const id = typeof input.id === "string" ? input.id.trim() : "";
  if (!id) return null;
  return {
    id,
    object: typeof input.object === "string" ? input.object.trim() : "",
    ownedBy: typeof input.ownedBy === "string" ? input.ownedBy.trim() : "",
    displayName: typeof input.displayName === "string" ? input.displayName.trim() : "",
  };
}

function uniqueModels(input: UpstreamModelDescriptorLike[] = []): UpstreamModelDescriptor[] {
  const seen = new Set<string>();
  const result: UpstreamModelDescriptor[] = [];
  for (const item of input) {
    const normalized = normalizeModelDescriptor(item);
    if (!normalized) continue;
    const key = normalized.id.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    result.push(normalized);
  }
  return result;
}

function looksLikeImageModel(model: UpstreamModelDescriptor): boolean {
  const haystack = `${model.id} ${model.displayName} ${model.object} ${model.ownedBy}`.toLowerCase();
  return haystack.includes("gpt-image")
    || haystack.includes("gemini-")
    || haystack.includes("imagen-")
    || haystack.includes("nano-banana")
    || haystack.includes("image-")
    || haystack.includes("images")
    || haystack.includes("dall-e")
    || haystack.includes("vision-image");
}

function looksLikeTextModel(model: UpstreamModelDescriptor): boolean {
  return !looksLikeImageModel(model);
}

function scoreTextModel(model: UpstreamModelDescriptor): number {
  const id = model.id.toLowerCase();
  if (id === "gpt-5.5") return 0;
  if (id.startsWith("gpt-5.5")) return 1;
  if (id.startsWith("gpt-5")) return 2;
  if (id.startsWith("gpt-4.1")) return 3;
  if (id.startsWith("o3")) return 4;
  if (id.startsWith("o4")) return 5;
  return 100;
}

function scoreImageModel(model: UpstreamModelDescriptor): number {
  const id = model.id.toLowerCase();
  if (id === "gpt-image-2") return 0;
  if (id === "gpt-image-1") return 1;
  if (id.startsWith("gpt-image")) return 2;
  if (id.startsWith("gemini-3.1-flash-image")) return 3;
  if (id.startsWith("gemini-")) return 4;
  if (id.startsWith("imagen-")) return 5;
  if (id.startsWith("dall-e")) return 6;
  return 100;
}

function sortModels(models: UpstreamModelDescriptor[], scorer: (model: UpstreamModelDescriptor) => number) {
  return [...models].sort((a, b) => {
    const scoreDiff = scorer(a) - scorer(b);
    if (scoreDiff !== 0) return scoreDiff;
    return a.id.localeCompare(b.id, "en", { sensitivity: "base" });
  });
}

export function buildUpstreamModelCatalog(input: UpstreamModelDescriptorLike[] = []): UpstreamModelCatalog {
  const all = sortModels(uniqueModels(input), (model) => {
    if (looksLikeImageModel(model)) return 50 + scoreImageModel(model);
    return scoreTextModel(model);
  });
  const text = sortModels(all.filter(looksLikeTextModel), scoreTextModel);
  const image = sortModels(all.filter(looksLikeImageModel), scoreImageModel);
  return { all, text, image };
}

export function preferredModelsForAPIMode(catalog: UpstreamModelCatalog, apiMode: APIMode) {
  return apiMode === "images"
    ? { text: [], image: catalog.image.length > 0 ? catalog.image : catalog.all }
    : {
        text: catalog.text.length > 0 ? catalog.text : catalog.all,
        image: catalog.image.length > 0 ? catalog.image : catalog.all,
      };
}

export function formatUpstreamModelLabel(model: UpstreamModelDescriptor): string {
  if (model.displayName && model.displayName !== model.id) {
    return `${model.displayName} (${model.id})`;
  }
  return model.id;
}
