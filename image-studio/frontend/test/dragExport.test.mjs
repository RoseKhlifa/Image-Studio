import assert from "node:assert/strict";
import test from "node:test";

const realWindow = globalThis.window;

function installWindow(href = "http://wails.localhost/") {
  globalThis.window = {
    location: { href },
  };
}

function restoreWindow() {
  globalThis.window = realWindow;
}

test("buildHistoryItemDragExport prefers the saved file path for desktop file drags", async () => {
  installWindow();
  try {
    const dragExport = await import(`../src/lib/dragExport.ts?drag-export-test=${Date.now()}-${Math.random().toString(36).slice(2)}`);
    const spec = dragExport.buildHistoryItemDragExport({
      id: "result-12345678",
      mode: "generate",
      outputFormat: "png",
      savedPath: "/tmp/image-generate-cat.png",
      fullUrl: "/media/full/abc123",
      imageId: "abc123",
      imageB64: "",
      previewOnly: false,
    });
    assert.deepEqual(spec, {
      href: "file:///tmp/image-generate-cat.png",
      fileName: "image-generate-cat.png",
      mimeType: "image/png",
      downloadURL: "image/png:image-generate-cat.png:file:///tmp/image-generate-cat.png",
    });
  } finally {
    restoreWindow();
  }
});

test("buildHistoryItemDragExport falls back to the saved file path when no media route exists", async () => {
  installWindow();
  try {
    const dragExport = await import(`../src/lib/dragExport.ts?drag-export-test=${Date.now()}-${Math.random().toString(36).slice(2)}`);
    const spec = dragExport.buildHistoryItemDragExport({
      id: "result-22334455",
      mode: "generate",
      outputFormat: "png",
      savedPath: "/tmp/image-generate-only-path.png",
      fullUrl: "",
      imageId: "",
      imageB64: "",
      previewOnly: false,
    });
    assert.deepEqual(spec, {
      href: "file:///tmp/image-generate-only-path.png",
      fileName: "image-generate-only-path.png",
      mimeType: "image/png",
      downloadURL: "image/png:image-generate-only-path.png:file:///tmp/image-generate-only-path.png",
    });
  } finally {
    restoreWindow();
  }
});

test("buildHistoryItemDragExport falls back to the generated media route and suggested name", async () => {
  installWindow("http://wails.localhost/app/");
  try {
    const dragExport = await import(`../src/lib/dragExport.ts?drag-export-test=${Date.now()}-${Math.random().toString(36).slice(2)}`);
    const spec = dragExport.buildHistoryItemDragExport({
      id: "abcdef123456",
      mode: "edit",
      outputFormat: "jpeg",
      savedPath: "",
      fullUrl: "",
      imageId: "feedbeef",
      imageB64: "",
      previewOnly: false,
    });
    assert.deepEqual(spec, {
      href: "http://wails.localhost/media/full/feedbeef",
      fileName: "image-edit-abcdef12.jpg",
      mimeType: "image/jpeg",
      downloadURL: "image/jpeg:image-edit-abcdef12.jpg:http://wails.localhost/media/full/feedbeef",
    });
  } finally {
    restoreWindow();
  }
});

test("buildHistoryItemDragExport rewrites wails asset URLs for drag export", async () => {
  installWindow("wails://wails/index.html");
  try {
    const dragExport = await import(`../src/lib/dragExport.ts?drag-export-test=${Date.now()}-${Math.random().toString(36).slice(2)}`);
    const spec = dragExport.buildHistoryItemDragExport({
      id: "result-abcdef12",
      mode: "generate",
      outputFormat: "png",
      savedPath: "",
      fullUrl: "/media/full/ff00ff00",
      imageId: "ff00ff00",
      imageB64: "",
      previewOnly: false,
    });
    assert.deepEqual(spec, {
      href: "http://wails.localhost/media/full/ff00ff00",
      fileName: "image-generate-result-a.png",
      mimeType: "image/png",
      downloadURL: "image/png:image-generate-result-a.png:http://wails.localhost/media/full/ff00ff00",
    });
  } finally {
    restoreWindow();
  }
});

test("buildHistoryItemDragExport still prefers saved paths for persisted preview-only history items", async () => {
  installWindow("http://wails.localhost/app/");
  try {
    const dragExport = await import(`../src/lib/dragExport.ts?drag-export-test=${Date.now()}-${Math.random().toString(36).slice(2)}`);
    const spec = dragExport.buildHistoryItemDragExport({
      id: "history-persisted-1",
      mode: "generate",
      outputFormat: "webp",
      savedPath: "/tmp/image-generate-history.webp",
      fullUrl: "/media/full/history-full-1",
      imageId: "history-full-1",
      imageB64: "",
      previewOnly: true,
    });
    assert.deepEqual(spec, {
      href: "file:///tmp/image-generate-history.webp",
      fileName: "image-generate-history.webp",
      mimeType: "image/webp",
      downloadURL: "image/webp:image-generate-history.webp:file:///tmp/image-generate-history.webp",
    });
  } finally {
    restoreWindow();
  }
});

test("writeImageFileDragData writes the expected drag payload formats", async () => {
  const dragExport = await import(`../src/lib/dragExport.ts?drag-export-test=${Date.now()}-${Math.random().toString(36).slice(2)}`);
  const writes = [];
  dragExport.writeImageFileDragData({
    clearData() {
      writes.push(["clearData"]);
    },
    setData(format, value) {
      writes.push([format, value]);
    },
  }, {
    href: "http://wails.localhost/media/full/abc123",
    fileName: "image.png",
    mimeType: "image/png",
    downloadURL: "image/png:image.png:http://wails.localhost/media/full/abc123",
  });
  assert.deepEqual(writes, [
    ["clearData"],
    ["DownloadURL", "image/png:image.png:http://wails.localhost/media/full/abc123"],
    ["text/uri-list", "http://wails.localhost/media/full/abc123"],
    ["text/plain", "http://wails.localhost/media/full/abc123"],
  ]);
});

test("internal history drag payload round-trips through dataTransfer", async () => {
  const dragExport = await import(`../src/lib/dragExport.ts?drag-export-test=${Date.now()}-${Math.random().toString(36).slice(2)}`);
  const store = new Map();
  const dataTransfer = {
    setData(format, value) {
      store.set(format, value);
    },
    getData(format) {
      return store.get(format) ?? "";
    },
  };
  dragExport.writeInternalHistoryItemDragData(dataTransfer, {
    id: "history-1",
    imageId: "img-1",
    previewUrl: "/media/preview/img-1",
    fullUrl: "/media/full/img-1",
    previewOnly: true,
    prompt: "cat",
    mode: "edit",
    size: "1024x1024",
    quality: "medium",
    createdAt: 123,
    savedPath: "/tmp/cat.png",
  });
  assert.deepEqual(dragExport.readInternalHistoryItemDragData(dataTransfer), {
    id: "history-1",
    imageId: "img-1",
    previewUrl: "/media/preview/img-1",
    fullUrl: "/media/full/img-1",
    previewOnly: true,
    prompt: "cat",
    mode: "edit",
    size: "1024x1024",
    quality: "medium",
    createdAt: 123,
    savedPath: "/tmp/cat.png",
  });
});
