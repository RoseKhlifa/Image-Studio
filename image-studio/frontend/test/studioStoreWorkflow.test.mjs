import assert from "node:assert/strict";
import test from "node:test";
import { build } from "esbuild";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { fileURLToPath, pathToFileURL } from "node:url";
import { setTimeout as delay } from "node:timers/promises";
import "fake-indexeddb/auto";

// Exercise the real store and runtime event wiring, resolving the same TS
// imports as the application build. Only the native service is a fixture.
const output = await mkdtemp(`${tmpdir()}/image-studio-store-test-`);
await build({
  stdin: {
    contents: `export { useStudioStore } from "./src/state/studioStore.ts";
      export { emitLocalEvent } from "./src/platform/runtime/hostEvents.ts";
      export * as storage from "./src/lib/storage.ts";`,
    resolveDir: fileURLToPath(new URL("..", import.meta.url)),
  },
  outfile: `${output}/store.mjs`, bundle: true, format: "esm", platform: "node",
  define: { "import.meta.env": JSON.stringify({ VITE_TARGET_PLATFORM: "windows", PACKAGE_VERSION: "test" }) },
});

const preferences = new Map([["gptcodex.starPrompted", "1"]]);
globalThis.localStorage = {
  getItem: (key) => preferences.get(key) ?? null,
  setItem: (key, value) => preferences.set(key, String(value)),
  removeItem: (key) => preferences.delete(key),
};
globalThis.document = { visibilityState: "hidden", querySelector: () => null };
const sounds = [];
const notifications = [];
globalThis.Audio = class {
  currentTime = 0;
  play() { sounds.push("played"); return Promise.resolve(); }
};
globalThis.Notification = class {
  static permission = "granted";
  constructor(title, options) { notifications.push({ title, ...options }); }
};

let mode = "success";
let active = 0;
let peak = 0;
const requests = [];
const timers = new Set();
let emitLocalEvent;
function startJob(options) {
  requests.push(options);
  if (mode === "submit-error") throw new Error("service unavailable");
  active += 1;
  peak = Math.max(peak, active);
  if (mode !== "hold") {
    const timer = setTimeout(() => {
      timers.delete(timer);
      active -= 1;
      if (mode === "error") {
        emitLocalEvent(`error:${options.requestedJobId}`, { message: "HTTP 503" });
      } else {
        emitLocalEvent(`result:${options.requestedJobId}`, {
          mode: "generate", prompt: options.prompt, outputFormat: "png", imageB64: "AQID",
          savedPath: `/tmp/fixture-${options.requestedJobId}.png`,
        });
      }
    }, 8);
    timers.add(timer);
  }
  return { jobId: options.requestedJobId };
}
globalThis.window = { runtime: {}, go: { backend: { Service: {
  Generate: startJob, Edit: startJob,
  Cancel(jobId) { emitLocalEvent(`error:${jobId}`, { message: "context canceled" }); },
} } } };

const bundle = await import(pathToFileURL(`${output}/store.mjs`));
const { useStudioStore: store, storage } = bundle;
emitLocalEvent = bundle.emitLocalEvent;
const baseline = store.getState();
test.beforeEach(async () => {
  for (const timer of timers) clearTimeout(timer);
  timers.clear(); requests.length = 0; sounds.length = 0; notifications.length = 0;
  mode = "success"; active = 0; peak = 0;
  await storage.clearHistoryStorage();
  store.setState({
    ...baseline, history: [], batchResults: [], toasts: [], runningJobs: [], runningJobMeta: {},
    apiKey: "fixture-key", baseURL: "https://fixture.invalid", prompt: "fixture prompt", apiMode: "images",
    imageModelID: "gpt-image-1", mode: "generate", batchCount: 1, savePromptSuppressed: true,
    completionSound: { enabled: true, mode: "default", customName: "", customDataURL: "" },
    completionNotification: { enabled: true },
    pushToast(text, kind) {
      store.setState((state) => ({ toasts: [...state.toasts, { text, kind }] }));
    },
  });
});
test.after(async () => {
  for (const timer of timers) clearTimeout(timer);
  await rm(output, { recursive: true, force: true });
});
async function waitFor(check) {
  const deadline = Date.now() + 4000;
  while (!check()) {
    if (Date.now() > deadline) throw new Error(`store did not settle: ${store.getState().errorMessage}`);
    await delay(5);
  }
}

test("the real loop queue completes every wave and persists every result", async () => {
  store.setState({ loopGeneration: { enabled: true, totalCount: 7, concurrency: 2, autoSave: false, autoSaveDir: "", livePreview: false } });
  await store.getState().submit();
  await waitFor(() => !store.getState().isRunning && store.getState().jobsCompleted === 7 && sounds.length === 1);
  assert.equal(requests.length, 7);
  assert.equal(peak, 2);
  assert.equal(store.getState().history.length, 7);
  assert.equal((await storage.loadAllHistory()).length, 7);
  assert.equal(notifications.length, 1);
  assert.match(notifications[0].title, /已完成/);
});

test("concurrent generation failures alert once and settle the real counters", async () => {
  mode = "error";
  store.setState({ batchCount: 3 });
  await store.getState().submit();
  await waitFor(() => !store.getState().isRunning);
  assert.equal(store.getState().jobsCompleted, 3);
  assert.equal(requests.length, 3);
  assert.equal(sounds.length, 1);
  assert.equal(notifications.length, 1);
  assert.match(notifications[0].title, /生成失败/);
  assert.equal(notifications[0].body, "HTTP 503");
  assert.equal(store.getState().toasts.filter((toast) => toast.kind === "error").length, 1);
});

test("submission rejection counts each failed job exactly once", async () => {
  mode = "submit-error";
  store.setState({ batchCount: 3 });
  await store.getState().submit();
  await waitFor(() => !store.getState().isRunning);
  assert.equal(store.getState().jobsCompleted, 3);
  assert.equal(requests.length, 3);
  assert.equal(store.getState().jobsTotal, 0);
  assert.equal(sounds.length, 1);
  assert.equal(notifications.length, 1);
});

test("user cancellation through the real native event route stays silent", async () => {
  mode = "hold";
  store.setState({ batchCount: 3 });
  await store.getState().submit();
  await store.getState().cancel();
  assert.equal(requests.length, 3);
  assert.equal(store.getState().isRunning, false);
  assert.equal(sounds.length, 0);
  assert.equal(notifications.length, 0);
});
