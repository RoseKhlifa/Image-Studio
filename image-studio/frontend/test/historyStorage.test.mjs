import assert from "node:assert/strict";
import test from "node:test";
import "fake-indexeddb/auto";
import { IDBCursor, IDBFactory, IDBIndex, IDBObjectStore } from "fake-indexeddb";
import * as storage from "../src/lib/storage.ts";
import { mergeHistoryItems } from "../src/lib/history.ts";

const legacy = await new Promise((resolve, reject) => {
  const req = indexedDB.open("keyval-store");
  req.onupgradeneeded = () => req.result.createObjectStore("keyval");
  req.onsuccess = () => resolve(req.result);
  req.onerror = () => reject(req.error);
});

function item(id, createdAt) {
  return { id, createdAt, prompt: `prompt ${id}`, mode: "generate", size: "1024x1024", quality: "high" };
}

test.beforeEach(() => storage.clearHistoryStorage());
test.after(() => legacy.close());

test("hundreds of same-day results with duplicate timestamps remain reachable through bounded pages", async () => {
  const now = new Date(2026, 9, 6, 12).getTime();
  const items = Array.from({ length: 650 }, (_, index) => item(`image-${String(index).padStart(4, "0")}`, now - Math.floor(index / 220) * 86400000));
  await storage.persistHistoryItems(items);
  let cursor = null;
  const loaded = [];
  do {
    const page = await storage.loadHistoryPage({ limit: 18, cursor });
    assert.ok(page.items.length <= 18);
    loaded.push(...page.items);
    cursor = page.nextCursor;
  } while (cursor);
  assert.equal(loaded.length, 650);
  assert.deepEqual(new Set(loaded.map((i) => i.id)), new Set(items.map((i) => i.id)));
  assert.equal(loaded.find((i) => i.id === "image-0649").prompt, "prompt image-0649");
});

test("saving a new result after loading one page preserves older metadata and full images", async () => {
  await storage.persistHistoryItems(Array.from({ length: 300 }, (_, i) => item(`old-${i}`, 1000 + i)));
  await storage.persistHistoryFullImage("old-0", "AQID");
  const page = await storage.loadHistoryPage({ limit: 18 });
  const newest = item("new", 2000);
  const visible = mergeHistoryItems([newest, ...page.items, page.items[0]]);
  assert.equal(visible.length, 19);
  await storage.persistHistoryItem(newest);
  assert.equal((await storage.loadAllHistory()).length, 301);
  assert.equal(await storage.loadHistoryFullImage("old-0"), "AQID");
});

test("date cleanup deletes unloaded old records and their full images while retaining unloaded newer results", async () => {
  const items = Array.from({ length: 300 }, (_, i) => item(`image-${i}`, 1000 + i));
  await storage.persistHistoryItems(items);
  await storage.persistHistoryFullImages([{ id: "image-0", imageB64: "AQID" }, { id: "image-100", imageB64: "BAUG" }]);
  assert.equal((await storage.loadHistoryPage({ limit: 18 })).items.length, 18);
  const removed = await storage.removeHistoryOlderThan(1100);
  assert.equal(removed.length, 100);
  const remaining = await storage.loadAllHistory();
  assert.equal(remaining.length, 200);
  assert.ok(remaining.some((i) => i.id === "image-100"));
  assert.equal(await storage.loadHistoryFullImage("image-0"), "");
  assert.equal(await storage.loadHistoryFullImage("image-100"), "BAUG");
});

test("single history deletion also removes legacy copies so migration cannot restore deleted records", async () => {
  // A fresh module models restarting after an older client wrote its archive.
  const storage = await import(`../src/lib/storage.ts?legacy-restart=${Date.now()}`);
  const old = item("legacy", 1000);
  await new Promise((resolve, reject) => {
    const tx = legacy.transaction("keyval", "readwrite");
    tx.objectStore("keyval").put(old, "history:legacy");
    tx.objectStore("keyval").put("AQID", "history-full:legacy");
    tx.oncomplete = resolve;
    tx.onerror = () => reject(tx.error);
  });
  assert.equal((await storage.loadAllHistory()).length, 1);
  await storage.removeHistoryItem("legacy");
  assert.deepEqual(await storage.loadAllHistory(), []);
  assert.equal(await storage.loadHistoryFullImage("legacy"), "");
});

test("history merge keeps every batch result beyond the former 120-item limit", () => {
  const all = Array.from({ length: 650 }, (_, i) => item(`image-${i}`, 1000 + i));
  const merged = mergeHistoryItems([...all, all[0], { ...all[1], prompt: "duplicate" }]);
  assert.equal(merged.length, 650);
  assert.equal(merged[0].id, "image-649");
  assert.equal(merged.at(-1).id, "image-0");
  assert.equal(merged.find((i) => i.id === "image-1").prompt, "prompt image-1");
});

test("paging never rereads earlier timestamp ties and migrates the legacy database once", async () => {
  const current = await import(`../src/lib/storage.ts?bounded-reads=${Date.now()}`);
  const items = Array.from({ length: 300 }, (_, i) => item(`tie-${String(i).padStart(4, "0")}`, 1000));
  await current.persistHistoryItems(items);
  const open = indexedDB.open;
  const openCursor = IDBIndex.prototype.openCursor;
  let legacyOpens = 0;
  let rows = 0;
  indexedDB.open = function (name, ...args) {
    if (name === "keyval-store") legacyOpens += 1;
    return open.call(this, name, ...args);
  };
  IDBIndex.prototype.openCursor = function (...args) {
    const request = openCursor.apply(this, args);
    request.addEventListener("success", () => { if (request.result) rows += 1; });
    return request;
  };
  try {
    let cursor = null;
    let loaded = 0;
    do {
      rows = 0;
      const page = await current.loadHistoryPage({ cursor, limit: 18 });
      assert.ok(rows <= 21, `a page inspected ${rows} records`);
      loaded += page.items.length;
      cursor = page.nextCursor;
    } while (cursor);
    assert.equal(loaded, 300);
    assert.equal(legacyOpens, 1);
  } finally {
    indexedDB.open = open;
    IDBIndex.prototype.openCursor = openCursor;
  }
});

test("a version-2 archive stays readable with unchanged metadata and full images", async () => {
  const original = globalThis.indexedDB;
  const factory = new IDBFactory();
  globalThis.indexedDB = factory;
  try {
    const old = await new Promise((resolve, reject) => {
      const request = factory.open("image-studio", 2);
      request.onupgradeneeded = () => {
        request.result.createObjectStore("history", { keyPath: "id" }).createIndex("createdAt", "createdAt");
        request.result.createObjectStore("historyFull", { keyPath: "id" });
      };
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error);
    });
    await new Promise((resolve, reject) => {
      const tx = old.transaction(["history", "historyFull"], "readwrite");
      tx.objectStore("history").put(item("before-upgrade", 1000));
      tx.objectStore("historyFull").put({ id: "before-upgrade", image: new Blob([new Uint8Array([1, 2, 3])]) });
      tx.oncomplete = resolve;
      tx.onerror = () => reject(tx.error);
    });
    old.close();
    const upgraded = await import(`../src/lib/storage.ts?upgrade-v2=${Date.now()}`);
    const page = await upgraded.loadHistoryPage({ limit: 18 });
    assert.equal(page.items[0].id, "before-upgrade");
    assert.equal(page.items[0].prompt, "prompt before-upgrade");
    assert.equal(await upgraded.loadHistoryFullImage("before-upgrade"), "AQID");
    await new Promise((resolve, reject) => {
      const request = factory.deleteDatabase("image-studio");
      request.onsuccess = resolve;
      request.onerror = () => reject(request.error);
    });
  } finally {
    globalThis.indexedDB = original;
  }
});

test("older webviews without primary-key seeking still return complete timestamp-tied pages", async () => {
  await storage.persistHistoryItems(Array.from({ length: 60 }, (_, i) => item(`fallback-${i}`, 1000)));
  const original = IDBCursor.prototype.continuePrimaryKey;
  IDBCursor.prototype.continuePrimaryKey = undefined;
  try {
    const ids = new Set();
    let cursor = null;
    do {
      const page = await storage.loadHistoryPage({ cursor, limit: 18 });
      for (const item of page.items) { assert.equal(ids.has(item.id), false); ids.add(item.id); }
      cursor = page.nextCursor;
    } while (cursor);
    assert.equal(ids.size, 60);
  } finally {
    IDBCursor.prototype.continuePrimaryKey = original;
  }
});

test("a transient legacy read failure is retried instead of caching an empty archive", async () => {
  await new Promise((resolve, reject) => {
    const tx = legacy.transaction("keyval", "readwrite");
    tx.objectStore("keyval").put(item("retry-legacy", 1000), "history:retry-legacy");
    tx.oncomplete = resolve;
    tx.onerror = () => reject(tx.error);
  });
  const fresh = await import(`../src/lib/storage.ts?migration-retry=${Date.now()}`);
  const original = IDBObjectStore.prototype.getAll;
  let failed = false;
  IDBObjectStore.prototype.getAll = function (...args) {
    if (this.name === "keyval" && !failed) { failed = true; throw new Error("temporary read failure"); }
    return original.apply(this, args);
  };
  try {
    assert.deepEqual(await fresh.loadAllHistory(), []);
    assert.equal(failed, true);
    assert.equal((await fresh.loadAllHistory())[0].id, "retry-legacy");
  } finally {
    IDBObjectStore.prototype.getAll = original;
  }
});
