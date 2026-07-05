import assert from "node:assert/strict";
import test from "node:test";

import {
  DEFAULT_AUTO_RETRY_COUNT,
  DEFAULT_PARTIAL_IMAGES,
  DEFAULT_REASONING_EFFORT,
  buildResponsesPayload,
  describeProblem,
  extractInvalidSize,
  isRetryableRaw,
  normalizeAutoRetryCount,
  normalizeOpenAIImageSize,
  openAIAPIEndpoint,
  repairSizeForOpenAI,
  normalizePartialImages,
  shouldUseImagesNewAPICompat,
} from "../../../shared/kernel/requestModel.js";

test("Responses payload defaults partial_images to streaming preview count", () => {
  const payload = buildResponsesPayload({
    prompt: "cat",
    size: "1024x1024",
    quality: "low",
    outputFormat: "png",
    imageModelID: "gpt-image-2",
    textModelID: "gpt-5.5",
    requestPolicy: "openai",
  }, []);
  assert.equal(payload.tools[0].partial_images, DEFAULT_PARTIAL_IMAGES);
  assert.equal(payload.reasoning.effort, DEFAULT_REASONING_EFFORT);
});

test("normalizePartialImages clamps OpenAI range", () => {
  assert.equal(normalizePartialImages(0), 0);
  assert.equal(normalizePartialImages(-1), DEFAULT_PARTIAL_IMAGES);
  assert.equal(normalizePartialImages(2.8), 2);
  assert.equal(normalizePartialImages(9), 3);
});

test("normalizeAutoRetryCount clamps retry count range", () => {
  assert.equal(normalizeAutoRetryCount(undefined), DEFAULT_AUTO_RETRY_COUNT);
  assert.equal(normalizeAutoRetryCount(-1), DEFAULT_AUTO_RETRY_COUNT);
  assert.equal(normalizeAutoRetryCount(3.8), 3);
  assert.equal(normalizeAutoRetryCount(99), 10);
});

test("OpenAI endpoint helper preserves Google compatibility base path", () => {
  assert.equal(
    openAIAPIEndpoint("https://generativelanguage.googleapis.com/v1beta/openai", "images/generations"),
    "https://generativelanguage.googleapis.com/v1beta/openai/images/generations",
  );
  assert.equal(
    openAIAPIEndpoint("https://relay.example.com/api/v1", "/images/edits"),
    "https://relay.example.com/api/v1/images/edits",
  );
});

test("Gemini and Imagen image models use non-streaming Images compat mode", () => {
  assert.equal(shouldUseImagesNewAPICompat({ imageModelID: "gemini-3.1-flash-image" }), true);
  assert.equal(shouldUseImagesNewAPICompat({ imageModelID: "imagen-4.0-generate-001" }), true);
  assert.equal(shouldUseImagesNewAPICompat({ imageModelID: "gpt-image-2" }), false);
});

test("Responses payload uses configured reasoning effort", () => {
  const payload = buildResponsesPayload({
    prompt: "cat",
    imageModelID: "gpt-image-2",
    textModelID: "gpt-5.5",
    reasoningEffort: "high",
  }, []);
  assert.equal(payload.reasoning.effort, "high");
});

test("describeProblem extracts refusal text from Responses SSE message events", () => {
  const raw = [
    'data: {"type":"response.output_item.done","item":{"type":"message","status":"completed","content":[{"type":"output_text","text":"抱歉，这个请求包含成人裸露，我无法生成这类真实照片风格图片。"}]}}',
    'data: {"type":"response.completed","response":{"status":"completed","output":[{"type":"image_generation_call","status":"failed"}]}}',
  ].join("\n");
  assert.equal(describeProblem(raw), "抱歉，这个请求包含成人裸露，我无法生成这类真实照片风格图片。");
});

test("isRetryableRaw treats upstream_error and 403 as retryable", () => {
  assert.equal(isRetryableRaw(JSON.stringify({
    error: {
      message: "Upstream request failed",
      type: "upstream_error",
      upstreamStatus: 403,
    },
  })), true);
  assert.equal(isRetryableRaw(JSON.stringify({ status: 403 })), true);
});

test("repairSizeForOpenAI snaps invalid sizes to nearest legal 16-aligned value", () => {
  assert.deepEqual(normalizeOpenAIImageSize("872x2048"), { width: 880, height: 2048 });
  assert.deepEqual(extractInvalidSize(`{"error":{"message":"Invalid size '872x2048'. Width and height must both be divisible by 16."}}`), {
    original: "872x2048",
    reason: "divisible_by_16",
  });
  assert.deepEqual(repairSizeForOpenAI({ size: "872x2048", prompt: "cat" }), {
    size: "880x2048",
    prompt: "cat",
  });
});
