import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtemp, readFile, rm, mkdir, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { performance } from "node:perf_hooks";
import { build } from "esbuild";
import { IDBFactory, IDBKeyRange, IDBIndex } from "fake-indexeddb";

const frontend = fileURLToPath(new URL("..", import.meta.url));
const root = path.resolve(frontend, "../..");
const args = process.argv.slice(2);
const option = (name, fallback) => args.find((arg) => arg.startsWith(`${name}=`))?.slice(name.length + 1) ?? fallback;
const revision = option("--revision", "");
if (revision && !/^[0-9a-f]{7,40}$/i.test(revision)) throw new Error("--revision requires a commit hash");
const output = option("--output", "");
const records = Number(option("--records", "1000"));
const rounds = Number(option("--rounds", "3"));
const timestamps = option("--timestamps", "ties");
assert.ok(Number.isSafeInteger(records) && records > 0 && Number.isSafeInteger(rounds) && rounds > 0);
assert.ok(timestamps === "ties" || timestamps === "unique");
const temp = await mkdtemp(path.join(tmpdir(), "image-studio-performance-"));
const median = (values) => [...values].sort((a, b) => a - b)[Math.floor(values.length / 2)];

try {
  await build({
    stdin: {
      contents: `export * as storage from "./src/lib/storage.ts"; export * as compat from "./src/lib/compatState.ts";`,
      resolveDir: frontend,
    },
    outfile: `${temp}/runtime.mjs`, bundle: true, format: "esm", platform: "node",
    define: { "import.meta.env": JSON.stringify({ VITE_TARGET_PLATFORM: "macos", PACKAGE_VERSION: "benchmark" }) },
    plugins: revision ? [{
      name: "revision-source",
      setup(builder) {
        builder.onLoad({ filter: /\.(?:ts|tsx|js)$/ }, ({ path: filename }) => {
          const relative = path.relative(root, filename).replaceAll(path.sep, "/");
          if (!relative.startsWith("image-studio/frontend/src/") && !relative.startsWith("shared/kernel/")) return;
          const contents = execFileSync("git", ["show", `${revision}:${relative}`], { cwd: root, encoding: "utf8" });
          return { contents, loader: filename.endsWith(".tsx") ? "tsx" : filename.endsWith(".ts") ? "ts" : "js", resolveDir: path.dirname(filename) };
        });
      },
    }] : [],
  });
  globalThis.indexedDB = new IDBFactory();
  globalThis.IDBKeyRange = IDBKeyRange;
  globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };
  const legacy = await new Promise((resolve, reject) => {
    const request = indexedDB.open("keyval-store");
    request.onupgradeneeded = () => request.result.createObjectStore("keyval");
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
  legacy.close();
  const { storage, compat } = await import(pathToFileURL(`${temp}/runtime.mjs`));
  const items = Array.from({ length: records }, (_, i) => ({
    id: `image-${String(i).padStart(6, "0")}`, createdAt: timestamps === "ties" ? 1000 : 1000 + i, prompt: `fixture ${i}`, mode: "generate", size: "1024x1024", quality: "high",
  }));
  await storage.persistHistoryItems(items);
  const originalOpenCursor = IDBIndex.prototype.openCursor;
  let rows = 0;
  IDBIndex.prototype.openCursor = function (...parameters) {
    const request = originalOpenCursor.apply(this, parameters);
    request.addEventListener("success", () => { if (request.result) rows += 1; });
    return request;
  };
  const pagination = [];
  for (let round = 0; round < rounds; round++) {
    rows = 0;
    let cursor = null;
    const ids = new Set();
    let pages = 0;
    const started = performance.now();
    do {
      const page = await storage.loadHistoryPage({ cursor, limit: 24 });
      for (const item of page.items) { assert.equal(ids.has(item.id), false); ids.add(item.id); }
      cursor = page.nextCursor;
      pages += 1;
    } while (cursor);
    assert.equal(ids.size, records);
    pagination.push({ elapsedMs: performance.now() - started, indexRowsRead: rows, pages });
  }
  IDBIndex.prototype.openCursor = originalOpenCursor;

  const input = {
    history: items.map((item) => ({ ...item, imageB64: "A".repeat(4096) })), profiles: [], activeProfileId: "", aiProfileId: "",
    proxyMode: "system", proxyURL: "", theme: "system", fontScale: 1, outputFormat: "png", background: "auto", outputCompression: 100,
    inputFidelity: "auto", imageStyle: "default", moderation: "low", userIdentifier: "", partialImages: 1, protectStreamPreview: true,
    autoRetryEnabled: true, autoRetryCount: 5, promptTemplates: [], promptHistory: [], presets: [], customAspectRatios: [], kernelRuntimeMode: "auto",
    keepLogs: false, cleanupPreviewCacheOnExit: false, ignoredReleaseTag: "", completionSound: { enabled: true, mode: "default", customName: "", customDataURL: "" },
    completionNotification: { enabled: true },
  };
  const sync = [];
  for (let round = 0; round < rounds; round++) {
    let fingerprint = compat.compatibilityExportFingerprint(input);
    const changed = compat.createCompatibilityExportChangeDetector?.() ?? ((value) => {
      const next = compat.compatibilityExportFingerprint(value);
      if (next === fingerprint) return false;
      fingerprint = next;
      return true;
    });
    changed(input);
    const started = performance.now();
    for (let event = 0; event < 200; event++) {
      assert.equal(changed({ ...input, progress: { bytes: event, elapsed: event }, viewZoom: event / 10 }), false);
    }
    sync.push(performance.now() - started);
    assert.equal(changed({ ...input, theme: "dark" }), true);
  }
  const result = {
    node: process.version, source: revision || "working-tree", records, rounds,
    pagination: { fixture: `${timestamps === "ties" ? "identical" : "distinct"} timestamps, 24 records per page`, medianMs: median(pagination.map((r) => r.elapsedMs)), runs: pagination },
    compatibilityUpdates: { events: 200, base64CharactersPerRecord: 4096, medianMs: median(sync), runsMs: sync },
    scope: "Node + fake-indexeddb microbenchmark; timings do not represent a browser or upstream inference latency.",
  };
  if (output) { await mkdir(path.dirname(path.resolve(output)), { recursive: true }); await writeFile(output, `${JSON.stringify(result, null, 2)}\n`); }
  console.log(JSON.stringify(result, null, 2));
} finally {
  await rm(temp, { recursive: true, force: true });
}
