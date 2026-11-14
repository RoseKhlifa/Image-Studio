import assert from "node:assert/strict";
import test from "node:test";

const runtime = await import("../src/state/workspaceRuntime.ts");

function makeWorkspace(id, overrides = {}) {
  return {
    id,
    name: `tab-${id}`,
    prompt: "",
    negativePrompt: "",
    mode: "generate",
    size: "1024x1024",
    quality: "medium",
    outputFormat: "png",
    seed: 0,
    batchCount: 1,
    sources: [],
    currentImageId: null,
    batchResultIds: [],
    resultGridOpen: false,
    runningJobIds: [],
    jobsTotal: 0,
    jobsCompleted: 0,
    progress: null,
    streamPreview: null,
    lastLogLine: "",
    errorMessage: null,
    lastPayload: null,
    ...overrides,
  };
}

test("normalizes api mode and concurrency values", () => {
  assert.equal(runtime.normalizeAPIMode("images"), "images");
  assert.equal(runtime.normalizeAPIMode("responses"), "responses");
  assert.equal(runtime.apiModeLabel("images"), "Images API");
  assert.equal(runtime.apiModeLabel("responses"), "Responses API");
  assert.equal(runtime.normalizeConcurrencyLimit(3.8), 3);
  assert.equal(runtime.normalizeConcurrencyLimit(0), 0);
  assert.equal(runtime.normalizeConcurrencyLimit(-2), 0);
  assert.equal(runtime.normalizeBatchCount(3.8), 3);
  assert.equal(runtime.normalizeBatchCount(0), 1);
  assert.equal(runtime.normalizeBatchCount(99), 9);
  assert.equal(runtime.normalizeBatchProcessAutoAspectResolution("2k"), "2k");
  assert.equal(runtime.normalizeBatchProcessAutoAspectResolution("oops"), "");
});

test("normalizes batch process config and keeps scanned image dimensions", () => {
  const value = runtime.normalizeBatchProcessConfig({
    enabled: true,
    inputDir: " /tmp/src ",
    outputMode: "custom_dir",
    outputDir: " /tmp/out ",
    concurrency: 3.8,
    retryOnFailure: true,
    fileNamePrefix: "processed-",
    autoAspectResolution: "4k",
    discoveredSources: [
      { path: "/tmp/a.png", name: "a.png", size: 12, width: 1200, height: 800 },
      { path: " ", name: "bad", size: 0 },
    ],
  });
  assert.equal(value.enabled, true);
  assert.equal(value.inputDir, "/tmp/src");
  assert.equal(value.outputDir, "/tmp/out");
  assert.equal(value.concurrency, 3);
  assert.equal(value.retryOnFailure, true);
  assert.equal(value.autoAspectResolution, "4k");
  assert.deepEqual(value.discoveredSources, [
    { path: "/tmp/a.png", name: "a.png", size: 12, width: 1200, height: 800, previewUrl: undefined, previewWidth: undefined, previewHeight: undefined },
  ]);
});

test("patches only the target workspace runtime", () => {
  const workspaces = [makeWorkspace("a"), makeWorkspace("b")];
  const next = runtime.patchWorkspaceRuntime(workspaces, "b", {
    runningJobs: ["job-1"],
    jobsCompleted: 1,
    batchResultIds: ["img-1", "img-2"],
    resultGridOpen: true,
  });
  assert.deepEqual(next[0], workspaces[0]);
  assert.deepEqual(next[1].runningJobIds, ["job-1"]);
  assert.equal(next[1].jobsCompleted, 1);
  assert.deepEqual(next[1].batchResultIds, ["img-1", "img-2"]);
  assert.equal(next[1].resultGridOpen, true);
});

test("reads runtime from the active workspace mirror and background tabs", () => {
  const state = {
    activeWorkspaceId: "a",
    runningJobs: ["job-a"],
    jobsTotal: 1,
    jobsCompleted: 0,
    progress: { stage: "x", elapsed: 1, bytes: 2 },
    streamPreview: { jobId: "job-a", imageB64: "abc", updatedAt: 1 },
    lastLogLine: "log",
    errorMessage: null,
    lastPayload: null,
    workspaces: [
      makeWorkspace("a"),
      makeWorkspace("b", { runningJobIds: ["job-b"], jobsCompleted: 2 }),
    ],
  };
  const active = runtime.workspaceRuntimeFromState(state, "a");
  const bg = runtime.workspaceRuntimeFromState(state, "b");
  assert.equal(active.isRunning, true);
  assert.deepEqual(active.runningJobs, ["job-a"]);
  assert.equal(active.streamPreview.imageB64, "abc");
  assert.equal(bg.isRunning, true);
  assert.deepEqual(bg.runningJobs, ["job-b"]);
  assert.equal(bg.jobsCompleted, 2);
});

test("tracks per-api-mode running counts and active patches", () => {
  const state = { runningJobMeta: { a: { workspaceId: "a", apiMode: "responses" }, b: { workspaceId: "b", apiMode: "images" } } };
  assert.equal(runtime.workspaceRunningCount(state, "responses"), 1);
  assert.equal(runtime.workspaceRunningCount(state, "images"), 1);
  assert.equal(runtime.activeRuntimePatch({ runningJobs: ["x"], jobsCompleted: 2 }).isRunning, true);
});

test("a queue survives the gap between waves and finishes only after every job settles", () => {
  let state = { runningJobs: [], jobsCompleted: 0, jobsTotal: 650 };
  for (let i = 0; i < 650; i++) {
    const job = `job-${i}`;
    state.runningJobs.push(job);
    const result = runtime.completeWorkspaceJob(state, job);
    assert.equal(result.completed, i + 1);
    assert.equal(result.jobsTotal, i === 649 ? 0 : 650);
    assert.equal(runtime.activeRuntimePatch({ runningJobs: result.runningJobs, jobsTotal: result.jobsTotal, jobsCompleted: result.completed }).isRunning, i !== 649);
    state = { runningJobs: result.runningJobs, jobsCompleted: result.completed, jobsTotal: result.jobsTotal };
    assert.equal(runtime.completeWorkspaceJob(state, job).completed, i + 1, "duplicate settlement must not increment the count");
  }
});
