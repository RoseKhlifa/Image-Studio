import assert from "node:assert/strict";
import test from "node:test";

const groups = await import("../src/components/history/historyPromptGroups.ts");

function item(id, prompt, createdAt) {
  return {
    id,
    prompt,
    mode: "generate",
    size: "1024x1024",
    quality: "medium",
    createdAt,
  };
}

test("groups history records by normalized prompt while preserving newest representative", () => {
  const entries = groups.buildHistoryPromptEntries([
    item("newest-cat", "  Cyber cat\nportrait  ", 30),
    item("dog", "dog portrait", 20),
    item("older-cat", "cyber   cat portrait", 10),
  ]);

  assert.equal(entries.length, 2);
  assert.equal(entries[0].kind, "group");
  assert.equal(entries[0].group.representative.id, "newest-cat");
  assert.deepEqual(entries[0].group.items.map((entry) => entry.id), ["newest-cat", "older-cat"]);
  assert.equal(entries[0].group.prompt, "Cyber cat portrait");
  assert.equal(groups.historyPromptGroupIndexSummary(entries[0].group), "#1-#2");
  assert.equal(entries[1].kind, "item");
  assert.equal(entries[1].item.id, "dog");
});

test("keeps empty prompt records foldable instead of generating unstable keys", () => {
  const [entry] = groups.buildHistoryPromptEntries([
    item("blank-a", " ", 20),
    item("blank-b", "", 10),
  ]);

  assert.equal(entry.kind, "group");
  assert.equal(entry.group.key, "prompt:");
  assert.equal(groups.historyPromptGroupLabel(entry.group), "(无 prompt)");
  assert.equal(groups.historyPromptGroupContains(entry.group, "blank-b"), true);
  assert.equal(groups.historyPromptGroupContains(entry.group, "missing"), false);
});
