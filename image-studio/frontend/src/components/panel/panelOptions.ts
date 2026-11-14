import type { QualityValue, SizeValue } from "../../types/domain";
import { classifyImageModel } from "../../../../../shared/kernel/requestModel.js";

export const STYLE_CHIPS: { id: string; label: string; hint: string }[] = [
  { id: "cyberpunk", label: "赛博朋克", hint: "霓虹夜景" },
  { id: "anime", label: "二次元", hint: "动画上色" },
  { id: "illust", label: "插画", hint: "扁平绘制" },
  { id: "3d", label: "3D 渲染", hint: "体积光泽" },
  { id: "chinese", label: "国风", hint: "水墨意境" },
];

// auto 不展示具体方框形状,留给上游决定。
export const ASPECT_OPTIONS: { value: SizeValue; label: string; w: number; h: number; auto?: boolean }[] = [
  { value: "auto", label: "Auto", w: 18, h: 18, auto: true },
  { value: "1024x1024", label: "1:1", w: 18, h: 18 },
  { value: "1024x1536", label: "2:3", w: 14, h: 20 },
  { value: "1152x2048", label: "9:16", w: 12, h: 22 },
  { value: "1536x1024", label: "3:2", w: 22, h: 14 },
  { value: "2048x1152", label: "16:9", w: 24, h: 13 },
];

export const QUALITY_TIERS: { value: QualityValue; label: string }[] = [
  { value: "auto", label: "自动" },
  { value: "max", label: "极高" },
  { value: "xhigh", label: "超高" },
  { value: "low", label: "快速" },
  { value: "medium", label: "标准" },
  { value: "high", label: "精修" },
  { value: "standard", label: "standard" },
  { value: "hd", label: "hd" },
];

export function availableQualityOptions(imageModelID?: string): Array<{ value: QualityValue; label: string }> {
  const family = classifyImageModel(imageModelID || "");
  if (family === "dalle2") {
    return QUALITY_TIERS.filter((item) => item.value === "auto" || item.value === "standard");
  }
  if (family === "dalle3") {
    return QUALITY_TIERS.filter((item) => item.value === "auto" || item.value === "standard" || item.value === "hd");
  }
  if ((imageModelID || "").toLowerCase().startsWith("gpt-image-2.6-")) {
    return QUALITY_TIERS.filter((item) => ["auto", "low", "medium", "high", "xhigh", "max"].includes(item.value));
  }
  return QUALITY_TIERS.filter((item) => item.value === "auto" || item.value === "low" || item.value === "medium" || item.value === "high");
}

export function normalizeQualitySelection(value: string, imageModelID?: string): QualityValue {
  const allowed = availableQualityOptions(imageModelID);
  return allowed.some((item) => item.value === value) ? value as QualityValue : allowed[0].value;
}
