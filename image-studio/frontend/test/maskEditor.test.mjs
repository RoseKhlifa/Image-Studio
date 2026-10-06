import assert from "node:assert/strict";
import test from "node:test";

const {
  invertMaskPixels,
  normalizeMaskPixels,
} = await import("../src/lib/maskEditor.ts");

test("opaque black and white masks normalize to alpha edit semantics", () => {
  const pixels = new Uint8ClampedArray([
    0, 0, 0, 255,
    255, 255, 255, 255,
  ]);

  assert.equal(normalizeMaskPixels(pixels), true);
  assert.deepEqual(Array.from(pixels), [
    0, 0, 0, 255,
    0, 0, 0, 0,
  ]);
});

test("alpha masks preserve alpha and discard irrelevant RGB channels", () => {
  const pixels = new Uint8ClampedArray([
    240, 10, 80, 255,
    12, 180, 90, 96,
  ]);

  assert.equal(normalizeMaskPixels(pixels), true);
  assert.deepEqual(Array.from(pixels), [
    0, 0, 0, 255,
    0, 0, 0, 96,
  ]);
});

test("mask inversion swaps protected and editable alpha", () => {
  const pixels = new Uint8ClampedArray([
    0, 0, 0, 255,
    0, 0, 0, 0,
    0, 0, 0, 64,
  ]);

  assert.equal(invertMaskPixels(pixels), true);
  assert.deepEqual(Array.from(pixels), [
    0, 0, 0, 0,
    0, 0, 0, 255,
    0, 0, 0, 191,
  ]);
});

test("inverting a fully editable mask reports an empty selection", () => {
  const pixels = new Uint8ClampedArray([0, 0, 0, 0]);
  assert.equal(invertMaskPixels(pixels), false);
  assert.deepEqual(Array.from(pixels), [0, 0, 0, 255]);
});

test("fully protected masks report no editable pixels", () => {
  const pixels = new Uint8ClampedArray([0, 0, 0, 255]);
  assert.equal(normalizeMaskPixels(pixels), false);
});
