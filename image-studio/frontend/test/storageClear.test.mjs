import assert from "node:assert/strict";
import test from "node:test";

class FakeObjectStore {
  constructor(keys) {
    this.keys = new Set(keys);
    this.clearCalls = 0;
    this.deleteCalls = 0;
  }

  getAllKeys() {
    const request = {};
    const snapshot = Array.from(this.keys);
    queueMicrotask(() => {
      request.result = snapshot;
      request.onsuccess?.();
    });
    return request;
  }

  clear() {
    this.clearCalls += 1;
    this.keys.clear();
  }

  delete(key) {
    this.deleteCalls += 1;
    this.keys.delete(key);
  }
}

class FakeDatabase {
  constructor(stores) {
    this.stores = stores;
    this.objectStoreNames = {
      contains: (name) => Object.hasOwn(stores, name),
    };
  }

  transaction() {
    const transaction = {
      error: null,
      objectStore: (name) => this.stores[name],
    };
    setTimeout(() => transaction.oncomplete?.(), 0);
    return transaction;
  }
}

function fakeIndexedDB(databases) {
  return {
    open(name) {
      const request = {};
      queueMicrotask(() => {
        request.result = databases[name];
        request.onsuccess?.();
      });
      return request;
    },
  };
}

test("clearHistoryStorage bulk-clears current and legacy history without deleting unrelated legacy state", async () => {
  const currentHistory = new FakeObjectStore(["new-a", "new-b"]);
  const currentFull = new FakeObjectStore(["new-a", "new-b", "orphan-full"]);
  const legacy = new FakeObjectStore([
    "history:legacy-a",
    "history-full:legacy-a",
    "settings:keep-me",
  ]);
  globalThis.indexedDB = fakeIndexedDB({
    "image-studio": new FakeDatabase({ history: currentHistory, historyFull: currentFull }),
    "keyval-store": new FakeDatabase({ keyval: legacy }),
  });

  const { clearHistoryStorage } = await import("../src/lib/storage.ts");
  const clearedIDs = await clearHistoryStorage();

  assert.deepEqual(new Set(clearedIDs), new Set(["new-a", "new-b", "orphan-full", "legacy-a"]));
  assert.deepEqual(Array.from(currentHistory.keys), []);
  assert.deepEqual(Array.from(currentFull.keys), []);
  assert.equal(currentHistory.clearCalls, 1);
  assert.equal(currentFull.clearCalls, 1);
  assert.equal(currentHistory.deleteCalls, 0);
  assert.equal(currentFull.deleteCalls, 0);
  assert.deepEqual(Array.from(legacy.keys), ["settings:keep-me"]);
});
