import assert from "node:assert/strict";
import test from "node:test";
import { setTimeout as delay } from "node:timers/promises";
import { trackNativeFileDragGesture } from "../src/platform/runtime/nativeFileDragGesture.ts";

function target() {
  const listeners = new Map();
  return {
    listeners,
    addEventListener(name, handler) { listeners.set(name, handler); },
    removeEventListener(name, handler) { if (listeners.get(name) === handler) listeners.delete(name); },
    emit(name, event) { listeners.get(name)?.(event); },
  };
}

test("native drag starts once after movement and after the browser input handler returns", async () => {
  const events = target();
  let calls = 0;
  trackNativeFileDragGesture(events, { clientX: 10, clientY: 20 }, () => { calls += 1; });
  events.emit("mousemove", { clientX: 12, clientY: 21, buttons: 1 });
  await delay(5);
  assert.equal(calls, 0);
  events.emit("mousemove", { clientX: 20, clientY: 20, buttons: 1 });
  events.emit("mousemove", { clientX: 30, clientY: 20, buttons: 1 });
  assert.equal(calls, 0);
  await delay(5);
  assert.equal(calls, 1);
  assert.equal(events.listeners.size, 0);
});

test("mouseup, Escape, blur and component cleanup cancel pending native capture", async () => {
  for (const cancel of ["mouseup", "keydown", "blur", "cleanup", "released-button"]) {
    const events = target();
    let calls = 0;
    const cleanup = trackNativeFileDragGesture(events, { clientX: 0, clientY: 0 }, () => { calls += 1; });
    events.emit("mousemove", { clientX: 10, clientY: 0, buttons: 1 });
    if (cancel === "cleanup") cleanup();
    else if (cancel === "released-button") events.emit("mousemove", { clientX: 11, clientY: 0, buttons: 0 });
    else events.emit(cancel, { key: "Escape" });
    await delay(5);
    assert.equal(calls, 0, cancel);
    assert.equal(events.listeners.size, 0, cancel);
  }
});
