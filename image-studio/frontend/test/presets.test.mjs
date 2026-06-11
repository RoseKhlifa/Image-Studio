import assert from "node:assert/strict";
import test from "node:test";

const presets = await import("../src/lib/presets.ts");

function makeState(overrides = {}) {
  return {
    size: "1536x1024",
    quality: "high",
    outputFormat: "png",
    negativePrompt: "",
    background: "auto",
    outputCompression: 100,
    inputFidelity: "auto",
    imageStyle: "default",
    moderation: "low",
    batchCount: 4,
    styleTag: "anime",
    editAutoAspectResolution: "",
    ...overrides,
  };
}

test("buildPresetFromSnapshot keeps styleTag for quick switching", () => {
  const preset = presets.buildPresetFromSnapshot("插画", "p1", makeState());
  assert.equal(preset.name, "插画");
  assert.equal(preset.styleTag, "anime");
  assert.equal(preset.batchCount, 4);
});

test("buildPresetPatch applies explicit styleTag and preserves legacy missing fields", () => {
  const current = makeState({ styleTag: "anime", outputFormat: "webp" });
  const legacyPatch = presets.buildPresetPatch({
    id: "legacy",
    name: "旧预设",
    size: current.size,
    quality: current.quality,
    negativePrompt: current.negativePrompt,
    batchCount: current.batchCount,
  }, current);
  assert.equal(legacyPatch.styleTag, "anime");
  assert.equal(legacyPatch.outputFormat, "webp");

  const nextPatch = presets.buildPresetPatch({
    id: "fresh",
    name: "新预设",
    ...makeState({ styleTag: "", outputFormat: "png" }),
  }, current);
  assert.equal(nextPatch.styleTag, "");
  assert.equal(nextPatch.outputFormat, "png");
});

test("findMatchingPresetId requires current parameters to fully match new presets", () => {
  const current = makeState();
  const preset = presets.buildPresetFromSnapshot("插画", "match", current);
  assert.equal(presets.findMatchingPresetId([preset], current), "match");
  assert.equal(
    presets.findMatchingPresetId([preset], makeState({ styleTag: "" })),
    null,
  );
});

test("normalizeSelectedPresetId keeps explicit selection separate from passive parameter match", () => {
  const current = makeState();
  const preset = presets.buildPresetFromSnapshot("插画", "match", current);
  assert.equal(presets.normalizeSelectedPresetId([preset], null), null);
  assert.equal(presets.normalizeSelectedPresetId([preset], "match"), "match");
  assert.equal(presets.normalizeSelectedPresetId([preset], "missing"), null);
});

test("nextDefaultPresetName fills the first available 配置序号", () => {
  assert.equal(presets.nextDefaultPresetName([]), "配置1");
  assert.equal(
    presets.nextDefaultPresetName([
      { id: "p1", name: "配置1" },
      { id: "p3", name: "配置3" },
      { id: "other", name: "插画" },
    ]),
    "配置2",
  );
});

test("preset snapshots preserve ordinary img2img auto-aspect sizing mode", () => {
  const current = makeState({ editAutoAspectResolution: "2k" });
  const preset = presets.buildPresetFromSnapshot("源图适配", "auto-source", current);
  const applied = presets.buildPresetPatch(preset, makeState());

  assert.equal(preset.editAutoAspectResolution, "2k");
  assert.equal(applied.editAutoAspectResolution, "2k");
  assert.equal(presets.findMatchingPresetId([preset], current), "auto-source");
  assert.equal(presets.findMatchingPresetId([preset], makeState({ editAutoAspectResolution: "" })), null);
});

test("legacy presets without auto-aspect mode clear ordinary img2img override", () => {
  const legacyPatch = presets.buildPresetPatch({
    id: "legacy",
    name: "旧预设",
    size: "1024x1024",
    quality: "medium",
    negativePrompt: "",
    batchCount: 1,
  }, makeState({ editAutoAspectResolution: "4k" }));

  assert.equal(legacyPatch.editAutoAspectResolution, "");
});
