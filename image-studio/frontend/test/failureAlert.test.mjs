import assert from "node:assert/strict";
import test from "node:test";
import { createFailureAlertGate } from "../src/lib/failureAlert.ts";

test("a failed concurrent batch alerts once and the next submission can alert again", () => {
  const first = createFailureAlertGate();
  assert.equal(first("HTTP 503"), true);
  assert.equal(first("HTTP 401"), false);
  assert.equal(first("处理结果失败"), false);
  assert.equal(createFailureAlertGate()("HTTP 503"), true);
});

test("user cancellation stays silent and does not consume the failure alert", () => {
  for (const message of ["context canceled", "AbortError", "已取消", "提交失败:context canceled"]) {
    const gate = createFailureAlertGate();
    assert.equal(gate(message), false);
    assert.equal(gate("HTTP 503"), true);
  }
});
