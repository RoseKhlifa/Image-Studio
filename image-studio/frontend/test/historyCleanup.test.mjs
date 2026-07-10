import assert from "node:assert/strict";
import test from "node:test";

const {
  buildHistoryCleanupPatch,
  waitForActiveHistoryLoad,
} = await import("../src/state/historyCleanup.ts");

test("history clear waits only for an already active page load", async () => {
  let calls = 0;
  await waitForActiveHistoryLoad(() => ({
    historyLoading: false,
    loadMoreHistory: async () => { calls += 1; },
  }));
  assert.equal(calls, 0);

  let releaseLoad;
  let settled = false;
  const activeLoad = new Promise((resolve) => { releaseLoad = resolve; });
  const waiting = waitForActiveHistoryLoad(() => ({
    historyLoading: true,
    loadMoreHistory: async () => {
      calls += 1;
      await activeLoad;
    },
  })).then(() => { settled = true; });

  await Promise.resolve();
  assert.equal(calls, 1);
  assert.equal(settled, false);
  releaseLoad();
  await waiting;
  assert.equal(settled, true);
});

test("history cleanup removes persisted references across every workspace", () => {
  const current = { id: "history-a" };
  const pending = { id: "history-pending" };
  const compare = { id: "history-b" };
  const sourcePreview = { id: "source-preview:/tmp/source.png" };
  const patch = buildHistoryCleanupPatch({
    history: [current, pending, compare],
    historyHasMore: true,
    historyLoading: true,
    historyCursorBeforeDayStart: 123,
    currentImage: current,
    batchResults: [current, pending],
    resultGridOpen: true,
    compareB: compare,
    resultDetail: compare,
    savePromptRequest: { kind: "single", item: current },
    savePromptQueue: [
      { kind: "batch", workspaceId: "workspace-a", items: [compare, pending] },
      { kind: "single", item: compare },
    ],
    workspaces: [
      {
        id: "workspace-a",
        currentImageId: current.id,
        batchResultIds: [current.id, pending.id],
        resultGridOpen: true,
      },
      {
        id: "workspace-source",
        currentImageId: sourcePreview.id,
        batchResultIds: [],
        resultGridOpen: false,
      },
    ],
  }, [current.id, compare.id]);

  assert.deepEqual(patch.history, [pending]);
  assert.equal(patch.historyHasMore, false);
  assert.equal(patch.historyLoading, false);
  assert.equal(patch.historyCursorBeforeDayStart, null);
  assert.equal(patch.currentImage, null);
  assert.deepEqual(patch.batchResults, [pending]);
  assert.equal(patch.resultGridOpen, false);
  assert.equal(patch.compareB, null);
  assert.equal(patch.resultDetail, null);
  assert.deepEqual(patch.savePromptRequest, {
    kind: "batch",
    workspaceId: "workspace-a",
    items: [pending],
  });
  assert.deepEqual(patch.savePromptQueue, []);
  assert.equal(patch.workspaces[0].currentImageId, null);
  assert.deepEqual(patch.workspaces[0].batchResultIds, [pending.id]);
  assert.equal(patch.workspaces[0].resultGridOpen, false);
  assert.equal(patch.workspaces[1].currentImageId, sourcePreview.id);
});
