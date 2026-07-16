import type { HistoryItem } from "../../types/domain";

export interface HistoryPromptGroup {
  key: string;
  normalizedPrompt: string;
  prompt: string;
  representative: HistoryItem;
  items: HistoryItem[];
}

export type HistoryPromptEntry =
  | { kind: "item"; key: string; item: HistoryItem; group: HistoryPromptGroup }
  | { kind: "group"; key: string; group: HistoryPromptGroup };

export function normalizeHistoryPrompt(prompt: string | null | undefined): string {
  return (prompt ?? "").trim().replace(/\s+/g, " ").toLowerCase();
}

export function compactHistoryPrompt(prompt: string | null | undefined): string {
  return (prompt ?? "").trim().replace(/\s+/g, " ");
}

export function historyPromptGroupLabel(group: HistoryPromptGroup): string {
  return group.prompt || "(无 prompt)";
}

export function historyPromptGroupIndexSummary(group: HistoryPromptGroup): string {
  if (group.items.length <= 0) return "#0";
  if (group.items.length === 1) return "#1";
  return `#1-#${group.items.length}`;
}

export function buildHistoryPromptGroups(items: HistoryItem[]): HistoryPromptGroup[] {
  const groups = new Map<string, HistoryPromptGroup>();

  for (const item of items) {
    const normalizedPrompt = normalizeHistoryPrompt(item.prompt);
    const key = `prompt:${normalizedPrompt}`;
    const group = groups.get(key);
    if (group) {
      group.items.push(item);
      continue;
    }

    groups.set(key, {
      key,
      normalizedPrompt,
      prompt: compactHistoryPrompt(item.prompt),
      representative: item,
      items: [item],
    });
  }

  return Array.from(groups.values());
}

export function buildHistoryPromptEntries(items: HistoryItem[]): HistoryPromptEntry[] {
  return buildHistoryPromptGroups(items).map((group) => {
    if (group.items.length > 1) {
      return { kind: "group", key: group.key, group };
    }
    return { kind: "item", key: group.representative.id, item: group.representative, group };
  });
}

export function historyPromptGroupContains(group: HistoryPromptGroup, itemId: string | null | undefined): boolean {
  if (!itemId) return false;
  return group.items.some((item) => item.id === itemId);
}
