import assert from "node:assert/strict";
import test from "node:test";

const images = await import("../src/lib/images.ts");

test("historyFullSrc prefers full image sources for selected canvas images", () => {
  const item = {
    id: "img-1",
    previewOnly: true,
    fullUrl: "/media/full/img-1",
    previewUrl: "/media/preview/img-1",
  };

  assert.equal(images.historyFullSrc(item, null), "/media/full/img-1");
});

test("historyFullSrc derives full media URL from persisted image id", () => {
  const item = {
    id: "img-2",
    imageId: "asset-2",
    previewOnly: true,
    previewUrl: "/media/preview/asset-2",
  };

  assert.equal(images.historyFullSrc(item, null), "/media/full/asset-2");
});

test("historyFullSrc keeps transient stream previews on preview media", () => {
  const item = {
    id: "preview-job-1",
    imageId: "partial-1",
    previewOnly: true,
    previewUrl: "/media/preview/partial-1",
  };

  assert.equal(images.historyFullSrc(item, null), "/media/preview/partial-1");
});

test("historyPreviewSrc remains preview-first for grids and thumbnails", () => {
  const item = {
    id: "img-3",
    previewOnly: true,
    fullUrl: "/media/full/img-3",
    previewUrl: "/media/preview/img-3",
  };

  assert.equal(images.historyPreviewSrc(item, null), "/media/preview/img-3");
});

test("annotated canvas target stays first when another reference is added", () => {
  const annotated = { path: "/tmp/annotated.png", name: "annotated.png", size: 1 };
  const added = { path: "/tmp/new-reference.png", name: "new-reference.png", size: 1 };
  assert.deepEqual(
    images.prioritizeImageSource([added], annotated).map((source) => source.path),
    ["/tmp/annotated.png", "/tmp/new-reference.png"],
  );
  assert.deepEqual(
    images.prioritizeImageSource([annotated, added], annotated).map((source) => source.path),
    ["/tmp/annotated.png", "/tmp/new-reference.png"],
  );
});
