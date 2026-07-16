import assert from "node:assert/strict";
import test from "node:test";

const backdrop = await import("../src/components/common/modalBackdrop.ts");

test("modal backdrop closes only when the same pointer starts and ends on it", () => {
  const gesture = backdrop.beginBackdropPointerGesture(7, true);

  assert.equal(backdrop.shouldDismissFromBackdropPointer(gesture, 7, true), true);
  assert.equal(backdrop.shouldDismissFromBackdropPointer(gesture, 7, false), false);
  assert.equal(backdrop.shouldDismissFromBackdropPointer(gesture, 8, true), false);
});

test("dragging a control outside the modal does not dismiss it", () => {
  const gesture = backdrop.beginBackdropPointerGesture(7, false);

  assert.equal(backdrop.shouldDismissFromBackdropPointer(gesture, 7, true), false);
  assert.equal(backdrop.shouldDismissFromBackdropPointer(null, 7, true), false);
});
