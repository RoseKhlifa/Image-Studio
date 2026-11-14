import type { HistoryItem } from "../types/domain";

// A loaded page is only a view of the archive. Never use it as a retention list.
export function mergeHistoryItems(items: HistoryItem[]): HistoryItem[] {
  const seen = new Set<string>();
  return items.filter((item) => {
    if (!item.id || seen.has(item.id)) return false;
    seen.add(item.id);
    return true;
  }).sort((a, b) => b.createdAt - a.createdAt);
}
