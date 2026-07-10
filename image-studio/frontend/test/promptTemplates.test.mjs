import assert from "node:assert/strict";
import test from "node:test";

const promptTemplates = await import("../src/lib/promptTemplates.ts");

test("buildPromptHistoryEntries numbers prompts in display order", () => {
  assert.deepEqual(promptTemplates.buildPromptHistoryEntries(["newest", "older"]), [
    { position: 1, text: "newest" },
    { position: 2, text: "older" },
  ]);
});

test("normalizePromptTemplates keeps only valid templates", () => {
  const normalized = promptTemplates.normalizePromptTemplates([
    { id: "a", label: "模板1", text: "cat", createdAt: 1, updatedAt: 2 },
    { id: "", label: "bad", text: "x" },
    { id: "b", label: " ", text: "x" },
  ]);
  assert.deepEqual(normalized, [
    { id: "a", label: "模板1", text: "cat", createdAt: 1, updatedAt: 2 },
  ]);
});

test("nextDefaultPromptTemplateLabel uses first available numeric slot", () => {
  const label = promptTemplates.nextDefaultPromptTemplateLabel([
    { id: "1", label: "模板1", text: "a", createdAt: 1, updatedAt: 1 },
    { id: "3", label: "模板3", text: "b", createdAt: 1, updatedAt: 1 },
  ]);
  assert.equal(label, "模板2");
});

test("resolvePromptTemplateManagerSelection keeps explicit new mode", () => {
  const resolved = promptTemplates.resolvePromptTemplateManagerSelection([
    { id: "1", label: "模板1", text: "a", createdAt: 1, updatedAt: 1 },
  ], promptTemplates.NEW_PROMPT_TEMPLATE_ID);
  assert.deepEqual(resolved, {
    mode: "new",
    selectedId: promptTemplates.NEW_PROMPT_TEMPLATE_ID,
    initializeDraft: false,
  });
});

test("resolvePromptTemplateManagerSelection falls back to first template", () => {
  const first = { id: "1", label: "模板1", text: "a", createdAt: 1, updatedAt: 1 };
  const resolved = promptTemplates.resolvePromptTemplateManagerSelection([first], "");
  assert.deepEqual(resolved, {
    mode: "selected",
    selectedId: "1",
    template: first,
    initializeDraft: true,
  });
});

test("resolvePromptTemplateManagerSelection initializes new mode when empty", () => {
  const resolved = promptTemplates.resolvePromptTemplateManagerSelection([], "");
  assert.deepEqual(resolved, {
    mode: "new",
    selectedId: promptTemplates.NEW_PROMPT_TEMPLATE_ID,
    initializeDraft: true,
  });
});
