import assert from "node:assert/strict";
import test from "node:test";
import { setTimeout as delay } from "node:timers/promises";
import "fake-indexeddb/auto";
import { clearHistoryStorage, loadAllHistory, persistHistoryItems } from "../src/lib/storage.ts";

const realWindow = globalThis.window;
const realLocalStorage = globalThis.localStorage;

test.beforeEach(() => clearHistoryStorage());

function installStorage() {
  const store = new Map();
  globalThis.localStorage = {
    getItem(key) {
      return store.has(key) ? store.get(key) : null;
    },
    setItem(key, value) {
      store.set(key, String(value));
    },
    removeItem(key) {
      store.delete(key);
    },
  };
  return store;
}

function installService(methods = {}) {
  globalThis.window = {
    go: {
      backend: {
        Service: methods,
      },
    },
  };
}

test.afterEach(() => {
  globalThis.window = realWindow;
  globalThis.localStorage = realLocalStorage;
});

test("compat export preserves previewPath sourcePaths and parentId in history", async () => {
  installStorage();
  await persistHistoryItems(Array.from({ length: 250 }, (_, i) => ({
    id: `unloaded-${i}`, prompt: `older prompt ${i}`, mode: "generate", size: "1024x1024", quality: "high", createdAt: 100,
  })));
  let savedState = null;
  installService({
    SaveCompatibilityState(state) {
      savedState = state;
    },
  });
  const compatState = await import(`../src/lib/compatState.ts?compat-export=${Date.now()}-${Math.random().toString(36).slice(2)}`);

  await compatState.exportCompatibilityStateNow({
    history: [{
      id: "hist-1",
      prompt: "edit cat",
      mode: "edit",
      size: "1024x1024",
      quality: "high",
      outputFormat: "png",
      createdAt: 123,
      savedPath: "/tmp/result.png",
      previewPath: "/tmp/previews/result.png",
      thumbPath: "/tmp/thumbs/result.png",
      parentId: "/tmp/source-a.png",
      sourcePaths: ["/tmp/source-a.png", "/tmp/source-b.png"],
      imageB64: "AAAA",
    }],
    profiles: [],
    activeProfileId: "",
    proxyMode: "system",
    proxyURL: "",
    theme: "system",
    fontScale: 1,
    outputFormat: "png",
    background: "auto",
    outputCompression: 100,
    inputFidelity: "auto",
    imageStyle: "default",
    moderation: "low",
    userIdentifier: "",
    partialImages: 1,
    protectStreamPreview: true,
    autoRetryEnabled: true,
    autoRetryCount: 5,
    promptTemplates: [],
    promptHistory: [],
    presets: [],
    customAspectRatios: [],
    kernelRuntimeMode: "remote",
    keepLogs: false,
    cleanupPreviewCacheOnExit: false,
    ignoredReleaseTag: "",
    completionSound: { enabled: true, mode: "default", customName: "", customDataURL: "" },
    completionNotification: { enabled: false },
  });

  assert.ok(savedState, "compat state should be exported");
  assert.equal(savedState.history.length, 251, "compatibility export must include unloaded history");
  assert.equal(savedState.history[0].previewPath, "/tmp/previews/result.png");
  assert.equal(savedState.history[0].parentId, "/tmp/source-a.png");
  assert.deepEqual(savedState.history[0].sourcePaths, ["/tmp/source-a.png", "/tmp/source-b.png"]);
});

test("importing a partial snapshot from an older client cannot delete the local archive", async () => {
  installStorage();
  await persistHistoryItems(Array.from({ length: 250 }, (_, i) => ({
    id: `old-${i}`, prompt: `older prompt ${i}`, mode: "generate", size: "1024x1024", quality: "high", createdAt: 100,
  })));
  installService({ LoadCompatibilityState() {
    return {
      updatedAt: Date.now(), settings: {}, profiles: [], activeProfileId: "",
      history: [{ id: "new", prompt: "new prompt", mode: "generate", size: "1024x1024", quality: "high", createdAt: 200 }],
    };
  } });
  const compat = await import(`../src/lib/compatState.ts?partial-import=${Date.now()}`);
  assert.equal(await compat.importCompatibilityStateIfNewer(), true);
  const all = await loadAllHistory();
  assert.equal(all.length, 251);
  assert.ok(all.some((item) => item.id === "old-0"));
});

test("compat fingerprint changes when previewPath or sourcePaths change", async () => {
  installStorage();
  installService();
  const compatState = await import(`../src/lib/compatState.ts?compat-fingerprint=${Date.now()}-${Math.random().toString(36).slice(2)}`);

  const base = {
    history: [{
      id: "hist-1",
      prompt: "edit cat",
      mode: "edit",
      size: "1024x1024",
      quality: "high",
      outputFormat: "png",
      createdAt: 123,
      savedPath: "/tmp/result.png",
      previewPath: "/tmp/previews/result-a.png",
      sourcePaths: ["/tmp/source-a.png"],
    }],
    profiles: [],
    activeProfileId: "",
    proxyMode: "system",
    proxyURL: "",
    theme: "system",
    fontScale: 1,
    outputFormat: "png",
    background: "auto",
    outputCompression: 100,
    inputFidelity: "auto",
    imageStyle: "default",
    moderation: "low",
    userIdentifier: "",
    partialImages: 1,
    protectStreamPreview: true,
    autoRetryEnabled: true,
    autoRetryCount: 5,
    promptTemplates: [],
    promptHistory: [],
    presets: [],
    customAspectRatios: [],
    kernelRuntimeMode: "remote",
    keepLogs: false,
    cleanupPreviewCacheOnExit: false,
    ignoredReleaseTag: "",
    completionSound: { enabled: true, mode: "default", customName: "", customDataURL: "" },
    completionNotification: { enabled: false },
  };

  const fingerprintA = compatState.compatibilityExportFingerprint(base);
  const fingerprintB = compatState.compatibilityExportFingerprint({
    ...base,
    history: [{ ...base.history[0], previewPath: "/tmp/previews/result-b.png" }],
  });
  const fingerprintC = compatState.compatibilityExportFingerprint({
    ...base,
    history: [{ ...base.history[0], sourcePaths: ["/tmp/source-a.png", "/tmp/source-b.png"] }],
  });

  assert.notEqual(fingerprintA, fingerprintB);
  assert.notEqual(fingerprintA, fingerprintC);
});

function exportInput(overrides = {}) {
  return {
    history: [], profiles: [], activeProfileId: "", aiProfileId: "", proxyMode: "system", proxyURL: "", theme: "system", fontScale: 1,
    outputFormat: "png", background: "auto", outputCompression: 100, inputFidelity: "auto", imageStyle: "default", moderation: "low",
    userIdentifier: "", partialImages: 1, protectStreamPreview: true, autoRetryEnabled: true, autoRetryCount: 5,
    promptTemplates: [], promptHistory: [], presets: [], customAspectRatios: [], kernelRuntimeMode: "auto", keepLogs: false,
    cleanupPreviewCacheOnExit: false, ignoredReleaseTag: "", completionSound: { enabled: true, mode: "default", customName: "", customDataURL: "" },
    completionNotification: { enabled: true }, ...overrides,
  };
}

test("progress and preview updates do not serialize history; persisted preferences still trigger export", async () => {
  const preferences = installStorage();
  installService();
  const compat = await import("../src/lib/compatState.ts");
  let imageReads = 0;
  const image = { id: "image", prompt: "fixture", createdAt: 1000, get imageB64() { imageReads += 1; return "AAAA"; } };
  const input = exportInput({ history: [image] });
  const changed = compat.createCompatibilityExportChangeDetector();
  assert.equal(changed(input), true);
  assert.equal(imageReads, 1);
  for (let i = 0; i < 200; i++) {
    assert.equal(changed({ ...input, progress: { bytes: i }, viewZoom: i, currentImage: { imageB64: `preview-${i}` } }), false);
  }
  assert.equal(imageReads, 1, "unrelated events must not inspect full images");
  preferences.set("gptcodex.outputDir", "/tmp/new-output");
  assert.equal(changed(input), true);
  assert.equal(imageReads, 2);
  assert.equal(changed(input), false);
  assert.equal(changed({ ...input, history: [{ ...image, prompt: "changed prompt" }] }), true);
});

test("scheduled exports clone only the latest snapshot and serialize writes without losing pending state", async () => {
  installStorage();
  let releaseFirst;
  let firstStarted;
  const started = new Promise((resolve) => { firstStarted = resolve; });
  const held = new Promise((resolve) => { releaseFirst = resolve; });
  const saved = [];
  let active = 0;
  let peak = 0;
  installService({ async SaveCompatibilityState(state) {
    active += 1;
    peak = Math.max(peak, active);
    saved.push(state);
    if (saved.length === 1) { firstStarted(); await held; }
    active -= 1;
  } });
  const compat = await import(`../src/lib/compatState.ts?scheduled=${Date.now()}`);
  let reads = 0;
  const image = {
    id: "scheduled", prompt: "fixture", mode: "generate", size: "1024x1024", quality: "high", createdAt: 1000,
    get imageB64() { reads += 1; return "AAAA"; },
  };
  const input = exportInput({ history: [image] });
  for (let i = 0; i < 50; i++) compat.scheduleCompatibilityExport({ ...input, theme: i === 49 ? "light" : "system" });
  assert.equal(reads, 0, "superseded schedules must not copy the archive");
  await Promise.race([started, delay(2000).then(() => { throw new Error("scheduled export did not start"); })]);
  compat.scheduleCompatibilityExport({ ...input, theme: "system" });
  compat.scheduleCompatibilityExport({ ...input, theme: "dark" });
  await delay(300);
  assert.equal(saved.length, 1, "a pending write must wait for the first one");
  releaseFirst();
  const deadline = Date.now() + 2000;
  while (saved.length < 2 || active > 0) {
    if (Date.now() > deadline) throw new Error("latest snapshot was not persisted");
    await delay(5);
  }
  assert.equal(peak, 1);
  assert.deepEqual(saved.map((state) => state.settings.theme), ["light", "dark"]);
  assert.equal(saved[1].history[0].id, "scheduled");
});
