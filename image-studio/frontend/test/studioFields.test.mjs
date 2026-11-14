import assert from "node:assert/strict";
import test from "node:test";
import { build } from "esbuild";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { fileURLToPath, pathToFileURL } from "node:url";

test("static UI subscriptions skip 200 progress/preview updates and still render changed history", async () => {
  const dir = await mkdtemp(`${tmpdir()}/image-studio-field-render-`);
  const storageDescriptor = Object.getOwnPropertyDescriptor(globalThis, "localStorage");
  Object.defineProperty(globalThis, "localStorage", { configurable: true, value: { getItem: () => null } });
  let renderer;
  try {
    await build({
      stdin: {
        contents: `export { useStudioStore } from "./src/state/studioStore.ts";
          export { useStudioFields } from "./src/state/useStudioFields.ts";
          export { createElement, memo } from "react";
          export { default as renderer } from "react-test-renderer";`,
        resolveDir: fileURLToPath(new URL("..", import.meta.url)),
      },
      outfile: `${dir}/fields.mjs`, bundle: true, platform: "node", format: "esm",
      define: { "import.meta.env": JSON.stringify({ VITE_TARGET_PLATFORM: "macos", PACKAGE_VERSION: "test" }) },
    });
    const { useStudioStore: store, useStudioFields, createElement, memo, renderer: { create, act } } = await import(pathToFileURL(`${dir}/fields.mjs`));
    store.setState({ currentImage: { id: "same-image" } });
    let fieldRenders = 0;
    let fullRenders = 0;
    const StaticRegion = memo(() => {
      const { history } = useStudioFields(["history", "historyHasMore"]);
      const id = store((state) => state.currentImage?.id);
      fieldRenders += 1;
      return createElement("span", null, `${history.length}:${id}`);
    });
    function WholeStoreRegion() {
      store();
      fullRenders += 1;
      return createElement(StaticRegion);
    }
    // These updates are synchronous; async act in a bundled Node runtime
    // otherwise falls back to browser MessageChannels that keep ports alive.
    act(() => { renderer = create(createElement(WholeStoreRegion)); });
    for (let i = 0; i < 200; i++) {
      act(() => { store.setState({ progress: { bytes: i }, currentImage: { id: "same-image", imageB64: `frame-${i}` } }); });
    }
    assert.equal(fullRenders, 201);
    assert.equal(fieldRenders, 1);
    act(() => { store.setState({ history: [{ id: "new", createdAt: 1000 }] }); });
    assert.equal(fieldRenders, 2);
    assert.equal(renderer.toJSON().children[0], "1:same-image");
    act(() => { store.setState({ currentImage: { id: "selected-image" } }); });
    assert.equal(fieldRenders, 3);
    assert.equal(renderer.toJSON().children[0], "1:selected-image");
  } finally {
    renderer?.unmount();
    if (storageDescriptor) Object.defineProperty(globalThis, "localStorage", storageDescriptor);
    else delete globalThis.localStorage;
    await rm(dir, { recursive: true, force: true });
  }
});
