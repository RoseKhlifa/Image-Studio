import type { HistoryItem, OutputFormatValue } from "../types/domain";

function extensionForFormat(format?: OutputFormatValue): string {
  if (format === "jpeg") return "jpg";
  if (format === "webp") return "webp";
  if (format === "gif") return "gif";
  if (format === "svg") return "svg";
  return "png";
}

function extensionFromPath(path?: string): string {
  const match = path?.match(/\.([a-z0-9]+)$/i);
  return match?.[1]?.toLowerCase() ?? "";
}

export function suggestedImageName(item: Pick<HistoryItem, "id" | "mode" | "outputFormat" | "savedPath">): string {
  const ext = extensionFromPath(item.savedPath) || extensionForFormat(item.outputFormat);
  return `image-${item.mode}-${item.id.slice(0, 8)}.${ext}`;
}
