import {
  WindowSetDarkTheme,
  WindowSetLightTheme,
  WindowSetSystemDefaultTheme,
  RegisterTrustedOutputDir,
} from "../platform/runtime/host";
import type { ThemeMode, HistoryItem, Annotation } from "../types/domain";
import type { ModeConfig, Stroke } from "./studioStore.types";
import { isWindows } from "../platform";
import { ACTIVE_PROFILE_LS_KEY, PROFILES_LS_KEY, tryParseProfile } from "../lib/profiles";
export { loadStoredAIProfileId, persistAIProfileId } from "../lib/profiles";
import type { UpstreamProfile } from "../types/domain";
import { pruneHistoryStorage } from "../lib/storage";
import { dataURLFromBase64, getImageDimensionsFromBase64 } from "../lib/images";

export const EMPTY_MODE_CFG: ModeConfig = {
  baseURL: "",
  apiKey: "",
  textModelID: "",
  imageModelID: "",
  concurrencyLimit: 0,
};

export const MAX_HISTORY_ITEMS = 120;

let detachSystemThemeListener: (() => void) | null = null;

export function resolvedTheme(theme: ThemeMode): "light" | "dark" {
  if (theme === "dark" || theme === "light") return theme;
  if (typeof window !== "undefined" && typeof window.matchMedia === "function") {
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }
  return "dark";
}

export function unbindSystemThemeListener() {
  if (detachSystemThemeListener) {
    detachSystemThemeListener();
    detachSystemThemeListener = null;
  }
}

export function writeResolvedTheme(theme: "light" | "dark") {
  document.documentElement.setAttribute("data-theme", theme);
  document.documentElement.classList.toggle("dark", theme === "dark");
  document.documentElement.style.colorScheme = theme;
}

export function bindSystemThemeListener() {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return;
  const media = window.matchMedia("(prefers-color-scheme: dark)");
  const apply = (matches: boolean) => writeResolvedTheme(matches ? "dark" : "light");
  const onChange = (event: MediaQueryListEvent) => apply(event.matches);
  apply(media.matches);
  if (typeof media.addEventListener === "function") {
    media.addEventListener("change", onChange);
    detachSystemThemeListener = () => media.removeEventListener("change", onChange);
    return;
  }
  media.addListener(onChange);
  detachSystemThemeListener = () => media.removeListener(onChange);
}

export function applyTheme(theme: ThemeMode) {
  unbindSystemThemeListener();
  document.documentElement.setAttribute("data-appearance", theme);
  writeResolvedTheme(resolvedTheme(theme));
  if (isWindows) {
    if (theme === "system") WindowSetSystemDefaultTheme();
    else if (theme === "dark") WindowSetDarkTheme();
    else WindowSetLightTheme();
  }
  if (theme === "system") bindSystemThemeListener();
}

export function loadModeConfig(mode: "responses" | "images"): ModeConfig {
  const r = (k: Exclude<keyof ModeConfig, "apiKey" | "concurrencyLimit">): string => {
    try { return localStorage.getItem(`gptcodex.${mode}.${k}`) ?? ""; } catch { return ""; }
  };
  const limit = (() => {
    try {
      const raw = localStorage.getItem(`gptcodex.${mode}.concurrencyLimit`) ?? "";
      const n = Number(raw);
      return Number.isFinite(n) && n > 0 ? Math.floor(n) : 0;
    } catch {
      return 0;
    }
  })();
  return {
    baseURL: r("baseURL"),
    apiKey: "",
    textModelID: r("textModelID"),
    imageModelID: r("imageModelID"),
    concurrencyLimit: limit,
  };
}

export function persistProfiles(list: UpstreamProfile[]) {
  try { localStorage.setItem(PROFILES_LS_KEY, JSON.stringify(list)); } catch {}
}

export function persistActiveProfileId(id: string) {
  try {
    if (id) localStorage.setItem(ACTIVE_PROFILE_LS_KEY, id);
    else localStorage.removeItem(ACTIVE_PROFILE_LS_KEY);
  } catch {}
}

export function loadStoredProfiles(): UpstreamProfile[] {
  try {
    const raw = localStorage.getItem(PROFILES_LS_KEY);
    if (!raw) return [];
    const arr = JSON.parse(raw);
    if (!Array.isArray(arr)) return [];
    return arr.map((x) => tryParseProfile(x)).filter((p): p is UpstreamProfile => p !== null);
  } catch {
    return [];
  }
}

export function loadStoredActiveProfileId(): string {
  try { return localStorage.getItem(ACTIVE_PROFILE_LS_KEY) ?? ""; } catch { return ""; }
}

export function clearLegacyModeLocalStorage() {
  for (const mode of ["responses", "images"] as const) {
    for (const field of ["baseURL", "textModelID", "imageModelID", "concurrencyLimit"]) {
      try { localStorage.removeItem(`gptcodex.${mode}.${field}`); } catch {}
    }
  }
  try { localStorage.removeItem("gptcodex.apiMode"); } catch {}
}

export function genId(): string {
  try {
    if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
      return crypto.randomUUID();
    }
  } catch {}
  return "id-" + Date.now().toString(36) + "-" + Math.random().toString(36).slice(2, 10);
}

export function tempDataURLFromB64(b64: string): string {
  return dataURLFromBase64(b64);
}

export function stripDataURLPrefix(dataURL: string): string {
  const idx = dataURL.indexOf(",");
  return idx >= 0 ? dataURL.slice(idx + 1) : dataURL;
}

export function buildMaskPNGDataURL(strokes: Stroke[], dims: { w: number; h: number } | null): string | null {
  if (!dims || strokes.length === 0) return null;
  const c = document.createElement("canvas");
  c.width = dims.w;
  c.height = dims.h;
  const ctx = c.getContext("2d");
  if (!ctx) return null;
  ctx.fillStyle = "#000";
  ctx.fillRect(0, 0, c.width, c.height);
  ctx.lineCap = "round";
  ctx.lineJoin = "round";
  let hasWhite = false;
  for (const s of strokes) {
    ctx.strokeStyle = s.erase ? "#000" : "#fff";
    ctx.lineWidth = s.size;
    ctx.beginPath();
    for (let i = 0; i < s.points.length; i += 2) {
      const x = s.points[i];
      const y = s.points[i + 1];
      if (i === 0) ctx.moveTo(x, y);
      else ctx.lineTo(x, y);
    }
    ctx.stroke();
    if (!s.erase) hasWhite = true;
  }
  return hasWhite ? c.toDataURL("image/png") : null;
}

export async function registerTrustedOutputRoots(roots: string[]): Promise<void> {
  for (const root of roots) {
    if (!root.trim()) continue;
    await RegisterTrustedOutputDir(root).catch(() => undefined);
  }
}

export function trimHistory(items: HistoryItem[]): HistoryItem[] {
  if (items.length <= MAX_HISTORY_ITEMS) return items;
  return items.slice(0, MAX_HISTORY_ITEMS);
}

export function persistTrimmedHistory(items: HistoryItem[]): void {
  const keptIDs = items.map((item) => item.id);
  void pruneHistoryStorage(keptIDs);
}

export function imageDims(b64: string): { w: number; h: number } | null {
  return getImageDimensionsFromBase64(b64);
}

export function augmentPromptWithAnnotations(
  prompt: string,
  annotations: Annotation[],
  dims: { w: number; h: number } | null,
): string {
  if (!annotations || annotations.length === 0) return prompt;
  const rects = annotations.filter((a) => a.kind === "rect");
  if (rects.length === 0) return prompt;
  const describe = (a: Annotation): string => {
    if (!dims) return `区域 ${rects.indexOf(a) + 1}`;
    const cx = (a.x + (a.width ?? 0) / 2) / dims.w;
    const cy = (a.y + (a.height ?? 0) / 2) / dims.h;
    const hPart = cx < 0.34 ? "左" : cx > 0.66 ? "右" : "中";
    const vPart = cy < 0.34 ? "上" : cy > 0.66 ? "下" : "中";
    return `${vPart}${hPart}部`;
  };
  const positions = rects.map(describe).join("、");
  return `${prompt}\n(请重点关注${positions}标注区域)`;
}
