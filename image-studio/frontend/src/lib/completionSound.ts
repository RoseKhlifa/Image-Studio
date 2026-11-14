import type { CompletionSoundConfig } from "../types/domain";
import { blobToBase64 } from "./images.ts";

export const DEFAULT_COMPLETION_SOUND_DATA_URL = "data:audio/wav;base64,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";
export const MAX_COMPLETION_SOUND_BYTES = 256 * 1024;

const COMPLETION_SOUND_ENABLED_KEY = "gptcodex.completionSound.enabled";
const COMPLETION_SOUND_MODE_KEY = "gptcodex.completionSound.mode";
const COMPLETION_SOUND_NAME_KEY = "gptcodex.completionSound.customName";
const COMPLETION_SOUND_DATA_KEY = "gptcodex.completionSound.customDataURL";

const AUDIO_EXT_MIME: Record<string, string> = {
  mp3: "audio/mpeg",
  wav: "audio/wav",
  ogg: "audio/ogg",
  m4a: "audio/mp4",
  aac: "audio/aac",
  webm: "audio/webm",
};

export function normalizeCompletionSoundConfig(raw: unknown): CompletionSoundConfig {
  const source = raw && typeof raw === "object" ? raw as Record<string, any> : {};
  const customDataURL = typeof source.customDataURL === "string" ? source.customDataURL.trim() : "";
  const mode = source.mode === "custom" && customDataURL ? "custom" : "default";
  return {
    enabled: source.enabled !== false,
    mode,
    customName: typeof source.customName === "string" ? source.customName.trim() : "",
    customDataURL,
  };
}

export function readCompletionSoundConfig(): CompletionSoundConfig {
  try {
    return normalizeCompletionSoundConfig({
      enabled: localStorage.getItem(COMPLETION_SOUND_ENABLED_KEY) !== "0",
      mode: localStorage.getItem(COMPLETION_SOUND_MODE_KEY) === "custom" ? "custom" : "default",
      customName: localStorage.getItem(COMPLETION_SOUND_NAME_KEY) ?? "",
      customDataURL: localStorage.getItem(COMPLETION_SOUND_DATA_KEY) ?? "",
    });
  } catch {
    return normalizeCompletionSoundConfig(null);
  }
}

export function persistCompletionSoundConfig(value: CompletionSoundConfig): void {
  const next = normalizeCompletionSoundConfig(value);
  try {
    localStorage.setItem(COMPLETION_SOUND_ENABLED_KEY, next.enabled ? "1" : "0");
    localStorage.setItem(COMPLETION_SOUND_MODE_KEY, next.mode);
    if (next.customName) localStorage.setItem(COMPLETION_SOUND_NAME_KEY, next.customName);
    else localStorage.removeItem(COMPLETION_SOUND_NAME_KEY);
    if (next.customDataURL) localStorage.setItem(COMPLETION_SOUND_DATA_KEY, next.customDataURL);
    else localStorage.removeItem(COMPLETION_SOUND_DATA_KEY);
  } catch {
    // localStorage can be unavailable in tests/previews.
  }
}

export function resolveCompletionSoundSource(value: CompletionSoundConfig): string {
  const next = normalizeCompletionSoundConfig(value);
  if (next.mode === "custom" && next.customDataURL) return next.customDataURL;
  return DEFAULT_COMPLETION_SOUND_DATA_URL;
}

export function shouldPlayCompletionSound(input: {
  config: CompletionSoundConfig;
  completedNow: number;
  totalNow: number;
}): boolean {
  const next = normalizeCompletionSoundConfig(input.config);
  return next.enabled && input.totalNow > 0 && input.completedNow === input.totalNow;
}

export async function importCompletionSoundFile(file: File): Promise<{ name: string; dataURL: string }> {
  if (file.size <= 0) throw new Error("音频文件为空");
  if (file.size > MAX_COMPLETION_SOUND_BYTES) {
    throw new Error(`音频文件不能超过 ${Math.round(MAX_COMPLETION_SOUND_BYTES / 1024)} KB`);
  }
  const mimeType = normalizeAudioMimeType(file.type, file.name);
  if (!mimeType) throw new Error("仅支持 mp3、wav、ogg、m4a、aac、webm 音频");
  const imageB64 = await blobToBase64(file);
  return {
    name: file.name.trim() || "custom-sound",
    dataURL: `data:${mimeType};base64,${imageB64}`,
  };
}

export async function playCompletionSound(
  config: CompletionSoundConfig,
  options: {
    force?: boolean;
    createAudio?: (src: string) => { currentTime: number; play: () => Promise<unknown> | unknown };
  } = {},
): Promise<boolean> {
  const next = normalizeCompletionSoundConfig(config);
  if (!options.force && !next.enabled) return false;
  const src = resolveCompletionSoundSource(next);
  const createAudio = options.createAudio ?? ((source) => {
    if (typeof Audio === "undefined") return null;
    return new Audio(source);
  });
  const audio = createAudio(src);
  if (!audio) return false;
  try {
    audio.currentTime = 0;
    await audio.play();
    return true;
  } catch {
    return false;
  }
}

function normalizeAudioMimeType(type: string | null | undefined, name: string | null | undefined): string | null {
  const cleaned = (type ?? "").trim().toLowerCase();
  if (cleaned.startsWith("audio/")) return cleaned;
  const match = (name ?? "").trim().toLowerCase().match(/\.([a-z0-9]+)$/);
  if (!match) return null;
  return AUDIO_EXT_MIME[match[1]] ?? null;
}
