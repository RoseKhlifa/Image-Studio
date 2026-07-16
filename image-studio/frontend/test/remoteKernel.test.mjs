import assert from "node:assert/strict";
import test from "node:test";
import { DEFAULT_AUTO_RETRY_COUNT } from "../../../shared/kernel/requestModel.js";

const realFetch = globalThis.fetch;
const realSetTimeout = globalThis.setTimeout;
const realClearTimeout = globalThis.clearTimeout;
const realSetInterval = globalThis.setInterval;
const realClearInterval = globalThis.clearInterval;
const realLocalStorage = globalThis.localStorage;
const realDocument = globalThis.document;
const realWindow = globalThis.window;
const realURL = globalThis.URL;
const realCreateObjectURL = globalThis.URL?.createObjectURL;
const realRevokeObjectURL = globalThis.URL?.revokeObjectURL;
const realAtob = globalThis.atob;
const realBtoa = globalThis.btoa;

function installBase64() {
  globalThis.atob = (value) => Buffer.from(value, "base64").toString("binary");
  globalThis.btoa = (value) => Buffer.from(value, "binary").toString("base64");
}

test("remote kernel keeps remote HTTP behind the insecure-channel opt-in", async () => {
  const common = await import(`../src/platform/runtime/remote-kernel/common.ts?remote-security=${Date.now()}-${Math.random()}`);
  assert.throws(
    () => common.validateRemoteBaseURL("http://relay.example.com", false),
    /拒绝使用非 TLS 上游/,
  );
  assert.equal(
    common.validateRemoteBaseURL("http://relay.example.com/v1", true),
    "http://relay.example.com",
  );
  assert.equal(
    common.validateRemoteBaseURL("http://127.0.0.1:8787", false),
    "http://127.0.0.1:8787",
  );
});

function installURLStubs() {
  const NativeURL = URL;
  class FakeURL extends NativeURL {}
  FakeURL.createObjectURL = () => "blob:mock";
  FakeURL.revokeObjectURL = () => {};
  globalThis.URL = FakeURL;
}

function installStorage() {
  const store = new Map();
  globalThis.localStorage = {
    getItem(key) {
      return store.has(key) ? store.get(key) : null;
    },
    setItem(key, value) {
      store.set(key, String(value));
    },
    removeItem(key) {
      store.delete(key);
    },
  };
}

function installDocument() {
  globalThis.document = {
    body: {
      appendChild() {},
    },
    createElement(tag) {
      if (tag === "a") {
        return {
          href: "",
          download: "",
          click() {},
          remove() {},
        };
      }
      if (tag === "input") {
        return {
          type: "",
          accept: "",
          style: {},
          files: [],
          addEventListener() {},
          click() {},
          remove() {},
        };
      }
      if (tag === "canvas") {
        return {
          width: 0,
          height: 0,
          getContext() {
            return {
              translate() {},
              rotate() {},
              drawImage() {},
              scale() {},
            };
          },
          toBlob(callback) {
            callback(new Blob(["canvas"], { type: "image/png" }));
          },
        };
      }
      return {};
    },
  };
  globalThis.window = {
    open() {
      return { closed: false };
    },
    location: { href: "" },
  };
}

function installImmediateTimers() {
  globalThis.setTimeout = (fn, _ms, ...args) => {
    queueMicrotask(() => fn(...args));
    return 0;
  };
  globalThis.clearTimeout = () => {};
  globalThis.setInterval = () => 0;
  globalThis.clearInterval = () => {};
}

async function withPatchedGlobals(setup, run) {
  try {
    installBase64();
    installURLStubs();
    installStorage();
    installDocument();
    installImmediateTimers();
    await setup();
    return await run();
  } finally {
    globalThis.fetch = realFetch;
    globalThis.setTimeout = realSetTimeout;
    globalThis.clearTimeout = realClearTimeout;
    globalThis.setInterval = realSetInterval;
    globalThis.clearInterval = realClearInterval;
    globalThis.localStorage = realLocalStorage;
    globalThis.document = realDocument;
    globalThis.window = realWindow;
    globalThis.URL = realURL;
    if (globalThis.URL && realCreateObjectURL) globalThis.URL.createObjectURL = realCreateObjectURL;
    if (globalThis.URL && realRevokeObjectURL) globalThis.URL.revokeObjectURL = realRevokeObjectURL;
    globalThis.atob = realAtob;
    globalThis.btoa = realBtoa;
  }
}

function loadRemoteKernel() {
  return import(`../src/platform/runtime/remoteKernel.ts?test=${Date.now()}-${Math.random().toString(36).slice(2)}`);
}

test("runRemoteImageJob retries retryable responses and returns parsed SSE image", async () => {
  let calls = 0;
  await withPatchedGlobals(async () => {
    globalThis.fetch = async () => {
      calls += 1;
      if (calls === 1) {
        return new Response("<html>Error code 524 | 524: A timeout occurred</html>", {
          status: 524,
          headers: { "content-type": "text/html" },
        });
      }
      return new Response(
        'data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"YWJj","revised_prompt":"rev"}}\n',
        { status: 200, headers: { "content-type": "text/event-stream" } },
      );
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    const result = await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "generate",
          prompt: "cat",
          size: "1024x1024",
          quality: "low",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 0,
          negativePrompt: "",
          userIdentifier: "user-hash-123",
          baseURL: "https://upstream.example",
          textModelID: "gpt-5.5",
          imageModelID: "gpt-image-2",
          apiMode: "responses",
          noPromptRevision: false,
        },
      },
      { signal: new AbortController().signal },
    );
    assert.equal(calls, 2);
    assert.equal(result.imageB64, "YWJj");
    assert.equal(result.revisedPrompt, "rev");
    assert.equal(result.sourceEvent, "final");
    assert.ok(result.rawPath?.startsWith("memory://text/"));
  });
});

test("runRemoteImageJob can fall back to a backup upstream profile after main retries fail", async () => {
  const seen = [];
  await withPatchedGlobals(async () => {
    globalThis.fetch = async (url) => {
      const urlText = String(url);
      seen.push(urlText);
      if (urlText.startsWith("https://primary.example")) {
        return new Response("<html>Error code 524 | 524: A timeout occurred</html>", {
          status: 524,
          headers: { "content-type": "text/html" },
        });
      }
      return new Response(
        'data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"YmFja3Vw","revised_prompt":"backup-rev"}}\n',
        { status: 200, headers: { "content-type": "text/event-stream" } },
      );
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    const result = await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "generate",
          prompt: "cat",
          size: "1024x1024",
          quality: "low",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 0,
          negativePrompt: "",
          userIdentifier: "user-hash-123",
          baseURL: "https://primary.example",
          textModelID: "gpt-5.5",
          imageModelID: "gpt-image-2",
          apiMode: "responses",
          requestPolicy: "openai",
          noPromptRevision: false,
          fallbackProfile: {
            baseURL: "https://backup.example",
            apiKey: "backup-key",
            textModelID: "gpt-5.5-mini",
            imageModelID: "gpt-image-2",
            apiMode: "responses",
            requestPolicy: "openai",
          },
        },
      },
      { signal: new AbortController().signal },
    );
    assert.equal(result.imageB64, "YmFja3Vw");
    assert.equal(result.revisedPrompt, "backup-rev");
    assert.equal(seen.filter((url) => url.startsWith("https://primary.example")).length, DEFAULT_AUTO_RETRY_COUNT + 1);
    assert.equal(seen.filter((url) => url.startsWith("https://backup.example")).length, 1);
  });
});

test("runRemoteImageJob emits Responses API partial image previews", async () => {
  let capturedBody = null;
  const partials = [];
  await withPatchedGlobals(async () => {
    globalThis.fetch = async (_url, init) => {
      capturedBody = JSON.parse(init.body);
      return new Response(
        'data: {"type":"response.image_generation_call.partial_image","partial_image_index":1,"partial_image_b64":"cGFydGlhbA==","revised_prompt":"partial rev"}\n' +
        'data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"ZmluYWw=","revised_prompt":"final rev"}}\n',
        { status: 200, headers: { "content-type": "text/event-stream" } },
      );
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    const result = await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "generate",
          prompt: "cat",
          size: "1024x1024",
          quality: "low",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 0,
          negativePrompt: "",
          userIdentifier: "user-hash-123",
          baseURL: "https://upstream.example",
          textModelID: "gpt-5.5",
          imageModelID: "gpt-image-2",
          apiMode: "responses",
          requestPolicy: "openai",
          noPromptRevision: false,
          partialImages: 2,
        },
      },
      {
        signal: new AbortController().signal,
        onPartialImage: (partial) => partials.push(partial),
      },
    );
    assert.equal(capturedBody.tools[0].partial_images, 2);
    assert.equal(capturedBody.safety_identifier, "user-hash-123");
    assert.equal(result.imageB64, "ZmluYWw=");
    assert.deepEqual(partials, [
      {
        imageB64: "cGFydGlhbA==",
        revisedPrompt: "partial rev",
        partialImageIndex: 1,
        sourceEvent: "responses_partial",
      },
    ]);
  });
});

test("runRemoteImageJob retries when Responses API only returns partial previews", async () => {
  let calls = 0;
  const partials = [];
  await withPatchedGlobals(async () => {
    globalThis.fetch = async () => {
      calls += 1;
      if (calls === 1) {
        return new Response(
          'data: {"type":"response.image_generation_call.partial_image","partial_image_index":0,"partial_image_b64":"cGFydGlhbA==","revised_prompt":"partial rev"}\n' +
          'data: {"type":"response.completed","response":{"status":"completed"}}\n',
          { status: 200, headers: { "content-type": "text/event-stream" } },
        );
      }
      return new Response(
        'data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"ZmluYWw=","revised_prompt":"final rev"}}\n',
        { status: 200, headers: { "content-type": "text/event-stream" } },
      );
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    const result = await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "generate",
          prompt: "cat",
          size: "1024x1024",
          quality: "low",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 0,
          negativePrompt: "",
          userIdentifier: "user-hash-123",
          baseURL: "https://upstream.example",
          textModelID: "gpt-5.5",
          imageModelID: "gpt-image-2",
          apiMode: "responses",
          requestPolicy: "openai",
          noPromptRevision: false,
          partialImages: 1,
        },
      },
      {
        signal: new AbortController().signal,
        onPartialImage: (partial) => partials.push(partial),
      },
    );
    assert.equal(calls, 2);
    assert.equal(result.imageB64, "ZmluYWw=");
    assert.equal(result.sourceEvent, "final");
    assert.equal(partials.length, 1);
    assert.equal(partials[0].imageB64, "cGFydGlhbA==");
  });
});

test("runRemoteImageJob parses Images API JSON mode", async () => {
  let captured = null;
  await withPatchedGlobals(async () => {
    globalThis.fetch = async (url, init) => {
      captured = {
        url: String(url),
        contentType: init.headers["Content-Type"] || init.headers["content-type"] || null,
        body: JSON.parse(init.body),
      };
      return new Response('{"data":[{"b64_json":"img-data","revised_prompt":"img-rev"}]}', {
        status: 200,
        headers: { "content-type": "application/json" },
      });
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    const result = await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "generate",
          prompt: "bird",
          size: "1024x1024",
          quality: "medium",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 0,
          negativePrompt: "",
          userIdentifier: "user-hash-123",
          baseURL: "https://upstream.example",
          textModelID: "",
          imageModelID: "gpt-image-2",
          apiMode: "images",
          requestPolicy: "openai",
          noPromptRevision: false,
        },
      },
      { signal: new AbortController().signal },
    );
    assert.equal(captured.url, "https://upstream.example/v1/images/generations");
    assert.equal(captured.body.prompt, "bird");
    assert.equal(captured.body.user, "user-hash-123");
    assert.equal(result.imageB64, "img-data");
    assert.equal(result.revisedPrompt, "img-rev");
    assert.equal(result.sourceEvent, "images_api");
  });
});

test("runRemoteImageJob uses the official Google Interactions fixture for Nano Banana 2", async () => {
  const imageB64 = "iVBORw0KGgoAAA==";
  let captured = null;
  await withPatchedGlobals(async () => {
    globalThis.fetch = async (url, init) => {
      captured = {
        url: String(url),
        headers: init.headers,
        body: JSON.parse(init.body),
      };
      return new Response(JSON.stringify({
        object: "interaction",
        status: "completed",
        steps: [{
          type: "model_output",
          content: [{ type: "image", data: imageB64, mime_type: "image/png" }],
        }],
      }), {
        status: 200,
        headers: { "content-type": "application/json" },
      });
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    const result = await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "google-key",
          mode: "generate",
          prompt: "nano banana",
          size: "2048x1152",
          quality: "medium",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 0,
          negativePrompt: "",
          baseURL: "https://generativelanguage.googleapis.com/v1beta/openai",
          textModelID: "",
          imageModelID: "gemini-3.1-flash-image",
          apiMode: "images",
          requestPolicy: "openai",
          noPromptRevision: false,
        },
      },
      { signal: new AbortController().signal },
    );
    assert.equal(captured.url, "https://generativelanguage.googleapis.com/v1beta/interactions");
    assert.equal(captured.headers["x-goog-api-key"], "google-key");
    assert.equal("Authorization" in captured.headers, false);
    assert.deepEqual(captured.body, {
      model: "gemini-3.1-flash-image",
      input: "nano banana",
      response_format: {
        type: "image",
        delivery: "inline",
        mime_type: "image/png",
        aspect_ratio: "16:9",
        image_size: "2K",
      },
      store: false,
    });
    assert.equal(result.imageB64, imageB64);
    assert.equal(result.sourceEvent, "google_interactions");
  });
});

test("Google Interactions protocol errors do not silently fall back to Images API", async () => {
  let calls = 0;
  await withPatchedGlobals(async () => {
    globalThis.fetch = async () => {
      calls += 1;
      return new Response('{"error":{"code":400,"message":"model is not enabled","status":"INVALID_ARGUMENT"}}', {
        status: 400,
        headers: { "content-type": "application/json" },
      });
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    await assert.rejects(
      kernel.runRemoteImageJob(
        {
          payload: {
            apiKey: "google-key",
            mode: "generate",
            prompt: "nano banana",
            size: "1024x1024",
            quality: "medium",
            outputFormat: "png",
            imagePaths: [],
            imagePath: "",
            maskB64: "",
            seed: 0,
            negativePrompt: "",
            baseURL: "https://generativelanguage.googleapis.com/v1beta/openai",
            textModelID: "",
            imageModelID: "gemini-3.1-flash-image",
            apiMode: "images",
            requestPolicy: "openai",
            noPromptRevision: false,
            autoRetryEnabled: false,
          },
        },
        { signal: new AbortController().signal },
      ),
      /Google Interactions 返回错误:model is not enabled/,
    );
    assert.equal(calls, 1);
  });
});

test("runRemoteImageJob emits Images API stream partial image previews", async () => {
  let captured = null;
  const partials = [];
  await withPatchedGlobals(async () => {
    globalThis.fetch = async (url, init) => {
      captured = {
        url: String(url),
        body: JSON.parse(init.body),
      };
      return new Response(
        'data: {"type":"image_generation.partial_image","partial_image_index":0,"b64_json":"cGFydGlhbA=="}\n' +
        'data: {"type":"image_generation.completed","b64_json":"ZmluYWw="}\n',
        { status: 200, headers: { "content-type": "text/event-stream" } },
      );
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    const result = await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "generate",
          prompt: "bird",
          size: "1024x1024",
          quality: "medium",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 0,
          negativePrompt: "",
          userIdentifier: "user-hash-123",
          baseURL: "https://upstream.example",
          textModelID: "",
          imageModelID: "gpt-image-2",
          apiMode: "images",
          requestPolicy: "openai",
          noPromptRevision: false,
          partialImages: 3,
        },
      },
      {
        signal: new AbortController().signal,
        onPartialImage: (partial) => partials.push(partial),
      },
    );
    assert.equal(captured.url, "https://upstream.example/v1/images/generations");
    assert.equal(captured.body.stream, true);
    assert.equal(captured.body.partial_images, 3);
    assert.equal(result.imageB64, "ZmluYWw=");
    assert.equal(result.sourceEvent, "images_api");
    assert.deepEqual(partials, [
      {
        imageB64: "cGFydGlhbA==",
        partialImageIndex: 0,
        sourceEvent: "images_partial",
      },
    ]);
  });
});

test("runRemoteImageJob retries when Images API only returns partial previews", async () => {
  let calls = 0;
  const partials = [];
  await withPatchedGlobals(async () => {
    globalThis.fetch = async () => {
      calls += 1;
      if (calls === 1) {
        return new Response(
          'data: {"type":"image_generation.partial_image","partial_image_index":0,"b64_json":"aW1hZ2VzLXBhcnRpYWw="}\n' +
          'data: {"type":"response.completed","response":{"status":"completed"}}\n',
          { status: 200, headers: { "content-type": "text/event-stream" } },
        );
      }
      return new Response(
        'data: {"type":"image_generation.completed","b64_json":"aW1hZ2VzLWZpbmFs"}\n',
        { status: 200, headers: { "content-type": "text/event-stream" } },
      );
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    const result = await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "generate",
          prompt: "cat",
          size: "1024x1024",
          quality: "low",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 0,
          negativePrompt: "",
          userIdentifier: "user-hash-123",
          baseURL: "https://upstream.example",
          textModelID: "",
          imageModelID: "gpt-image-2",
          apiMode: "images",
          requestPolicy: "openai",
          noPromptRevision: false,
          partialImages: 1,
        },
      },
      {
        signal: new AbortController().signal,
        onPartialImage: (partial) => partials.push(partial),
      },
    );
    assert.equal(calls, 2);
    assert.equal(result.imageB64, "aW1hZ2VzLWZpbmFs");
    assert.equal(result.sourceEvent, "images_api");
    assert.equal(partials.length, 1);
    assert.equal(partials[0].imageB64, "aW1hZ2VzLXBhcnRpYWw=");
  });
});

test("runRemoteImageJob repairs invalid 16-alignment size errors and retries once", async () => {
  let calls = 0;
  const bodies = [];
  await withPatchedGlobals(async () => {
    globalThis.fetch = async (_url, init) => {
      calls += 1;
      bodies.push(JSON.parse(init.body));
      if (calls === 1) {
        return new Response(
          JSON.stringify({
            error: {
              message: "Invalid size '872x2048'. Width and height must both be divisible by 16.",
              type: "image_generation_user_error",
              param: "tools",
              code: "invalid_value",
            },
          }),
          { status: 400, headers: { "content-type": "application/json" } },
        );
      }
      return new Response(
        'data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"ZmluYWw=","revised_prompt":"fixed"}}\n',
        { status: 200, headers: { "content-type": "text/event-stream" } },
      );
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    const result = await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "generate",
          prompt: "cat",
          size: "872x2048",
          quality: "low",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 0,
          negativePrompt: "",
          userIdentifier: "user-hash-123",
          baseURL: "https://upstream.example",
          textModelID: "gpt-5.5",
          imageModelID: "gpt-image-2",
          apiMode: "responses",
          requestPolicy: "openai",
          noPromptRevision: false,
        },
      },
      { signal: new AbortController().signal },
    );
    assert.equal(calls, 2);
    assert.equal(bodies[0].tools[0].size, "872x2048");
    assert.equal(bodies[1].tools[0].size, "880x2048");
    assert.equal(result.imageB64, "ZmluYWw=");
  });
});

test("runRemoteImageJob uses NewAPI images compat mode without stream fields", async () => {
  let captured = null;
  await withPatchedGlobals(async () => {
    globalThis.fetch = async (url, init) => {
      captured = {
        url: String(url),
        body: JSON.parse(init.body),
      };
      return new Response('{"data":[{"b64_json":"img-data","revised_prompt":"img-rev"}]}', {
        status: 200,
        headers: { "content-type": "application/json" },
      });
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    const result = await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "generate",
          prompt: "bird",
          size: "1024x1024",
          quality: "medium",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 0,
          negativePrompt: "",
          baseURL: "https://upstream.example",
          textModelID: "",
          imageModelID: "gpt-image-2",
          apiMode: "images",
          requestPolicy: "openai",
          imagesNewAPICompat: true,
          noPromptRevision: false,
          partialImages: 3,
        },
      },
      { signal: new AbortController().signal },
    );
    assert.equal(captured.url, "https://upstream.example/v1/images/generations");
    assert.equal(captured.body.response_format, "b64_json");
    assert.equal("stream" in captured.body, false);
    assert.equal("partial_images" in captured.body, false);
    assert.equal(result.imageB64, "img-data");
    assert.equal(result.revisedPrompt, "img-rev");
  });
});

test("runRemoteImageJob skips NewAPI empty JSON keepalives before the final image", async () => {
  await withPatchedGlobals(async () => {
    globalThis.fetch = async () => new Response(
      '{}\n[]\n""\nnull\n{"data":[]}\n{"data":[{"b64_json":"img-data","revised_prompt":"kept alive"}]}\n',
      { status: 200, headers: { "content-type": "application/json" } },
    );
  }, async () => {
    const kernel = await loadRemoteKernel();
    const result = await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "generate",
          prompt: "bird",
          size: "1024x1024",
          quality: "medium",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 0,
          negativePrompt: "",
          baseURL: "https://upstream.example",
          textModelID: "",
          imageModelID: "gpt-image-2",
          apiMode: "images",
          requestPolicy: "openai",
          imagesNewAPICompat: true,
          noPromptRevision: false,
        },
      },
      { signal: new AbortController().signal },
    );
    assert.equal(result.imageB64, "img-data");
    assert.equal(result.revisedPrompt, "kept alive");
  });
});

test("runRemoteImageJob skips NewAPI empty SSE keepalives before the final image", async () => {
  await withPatchedGlobals(async () => {
    globalThis.fetch = async () => new Response(
      'data:\n\ndata:{}\n\ndata:[]\n\ndata:null\n\ndata: {"data":[]}\n\ndata:{"data":[{"b64_json":"img-data"}]}\n\n',
      { status: 200, headers: { "content-type": "text/event-stream" } },
    );
  }, async () => {
    const kernel = await loadRemoteKernel();
    const result = await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "generate",
          prompt: "bird",
          size: "1024x1024",
          quality: "medium",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 0,
          negativePrompt: "",
          baseURL: "https://upstream.example",
          textModelID: "",
          imageModelID: "gpt-image-2",
          apiMode: "images",
          requestPolicy: "openai",
          imagesNewAPICompat: true,
          noPromptRevision: false,
        },
      },
      { signal: new AbortController().signal },
    );
    assert.equal(result.imageB64, "img-data");
  });
});

test("runRemoteImageJob rejects NewAPI responses containing only empty keepalives", async () => {
  await withPatchedGlobals(async () => {
    globalThis.fetch = async () => new Response(
      '{}\n[]\n""\nnull\n{"data":[]}\n',
      { status: 200, headers: { "content-type": "application/json" } },
    );
  }, async () => {
    const kernel = await loadRemoteKernel();
    await assert.rejects(
      kernel.runRemoteImageJob(
        {
          payload: {
            apiKey: "key",
            mode: "generate",
            prompt: "bird",
            size: "1024x1024",
            quality: "medium",
            outputFormat: "png",
            imagePaths: [],
            imagePath: "",
            maskB64: "",
            seed: 0,
            negativePrompt: "",
            baseURL: "https://upstream.example",
            textModelID: "",
            imageModelID: "gpt-image-2",
            apiMode: "images",
            requestPolicy: "openai",
            imagesNewAPICompat: true,
            noPromptRevision: false,
          },
        },
        { signal: new AbortController().signal },
      ),
      /上游没有返回可用图片/,
    );
  });
});

test("runRemoteImageJob sends Responses API mask as input_image_mask data URL", async () => {
  let captured = null;
  await withPatchedGlobals(async () => {
    globalThis.fetch = async (_url, init) => {
      captured = JSON.parse(init.body);
      return new Response(
        'data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"YWJj","revised_prompt":"rev"}}\n',
        { status: 200, headers: { "content-type": "text/event-stream" } },
      );
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "edit",
          prompt: "cat",
          size: "1024x1024",
          quality: "low",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "iVBORw0KGgp0ZXN0",
          seed: 0,
          negativePrompt: "",
          baseURL: "https://upstream.example",
          textModelID: "gpt-5.5",
          imageModelID: "gpt-image-2",
          apiMode: "responses",
          requestPolicy: "openai",
          noPromptRevision: false,
        },
        sourceImages: [
          { imageB64: "iVBORw0KGgpzb3VyY2U=", name: "source.png", mimeType: "image/png" },
        ],
      },
      { signal: new AbortController().signal },
    );
    assert.equal(captured.tools[0].input_image_mask.image_url, "data:image/png;base64,iVBORw0KGgp0ZXN0");
    assert.equal(captured.tools[0].action, "edit");
  });
});

test("runRemoteImageJob sends Images API edit mask with image MIME type", async () => {
  let captured = null;
  await withPatchedGlobals(async () => {
    globalThis.fetch = async (url, init) => {
      captured = {
        url: String(url),
        body: init.body,
      };
      return new Response('{"data":[{"b64_json":"img-data","revised_prompt":"img-rev"}]}', {
        status: 200,
        headers: { "content-type": "application/json" },
      });
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "edit",
          prompt: "bird",
          size: "1024x1024",
          quality: "medium",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "iVBORw0KGgpmYWtl",
          seed: 0,
          negativePrompt: "",
          userIdentifier: "user-hash-123",
          baseURL: "https://upstream.example",
          textModelID: "",
          imageModelID: "gpt-image-2",
          apiMode: "images",
          requestPolicy: "openai",
          noPromptRevision: false,
        },
        sourceImages: [
          { imageB64: "iVBORw0KGgpzb3VyY2U=", name: "source.png", mimeType: "image/png" },
        ],
      },
      { signal: new AbortController().signal },
    );
    assert.equal(captured.url, "https://upstream.example/v1/images/edits");
    assert.ok(captured.body instanceof FormData);
    assert.equal(captured.body.get("user"), "user-hash-123");
    const mask = captured.body.get("mask");
    assert.ok(mask instanceof Blob);
    assert.equal(mask.type, "image/png");
  });
});

test("runRemoteImageJob omits relay-only fields by default and includes them in compat mode", async () => {
  const capturedBodies = [];
  await withPatchedGlobals(async () => {
    globalThis.fetch = async (_url, init) => {
      capturedBodies.push(JSON.parse(init.body));
      return new Response(
        'data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"YWJj","revised_prompt":"rev"}}\n',
        { status: 200, headers: { "content-type": "text/event-stream" } },
      );
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "generate",
          prompt: "cat",
          size: "1024x1024",
          quality: "low",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 123,
          negativePrompt: "avoid blur",
          baseURL: "https://upstream.example",
          textModelID: "gpt-5.5",
          imageModelID: "gpt-image-2",
          apiMode: "responses",
          requestPolicy: "openai",
          noPromptRevision: false,
        },
      },
      { signal: new AbortController().signal },
    );
    await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "generate",
          prompt: "cat",
          size: "1024x1024",
          quality: "low",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 123,
          negativePrompt: "avoid blur",
          baseURL: "https://upstream.example",
          textModelID: "gpt-5.5",
          imageModelID: "gpt-image-2",
          apiMode: "responses",
          requestPolicy: "compat",
          noPromptRevision: false,
        },
      },
      { signal: new AbortController().signal },
    );
    assert.equal(capturedBodies[0].tools[0].seed, undefined);
    assert.equal(capturedBodies[0].tools[0].negative_prompt, undefined);
    assert.ok(capturedBodies[0].instructions.includes("VERBATIM"));
    assert.equal(capturedBodies[1].tools[0].seed, 123);
    assert.equal(capturedBodies[1].tools[0].negative_prompt, "avoid blur");
    assert.ok(capturedBodies[1].instructions.includes("VERBATIM"));
  });
});

test("runRemoteImageJob sends input_fidelity for supported Responses edit models", async () => {
  const capturedBodies = [];
  await withPatchedGlobals(async () => {
    globalThis.fetch = async (_url, init) => {
      capturedBodies.push(JSON.parse(init.body));
      return new Response(
        'data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"YWJj","revised_prompt":"rev"}}\n',
        { status: 200, headers: { "content-type": "text/event-stream" } },
      );
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "edit",
          prompt: "cat",
          size: "1024x1024",
          quality: "low",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 0,
          negativePrompt: "",
          background: "auto",
          outputCompression: 100,
          inputFidelity: "high",
          imageStyle: "default",
          moderation: "low",
          baseURL: "https://upstream.example",
          textModelID: "gpt-5.5",
          imageModelID: "gpt-image-1.5",
          apiMode: "responses",
          requestPolicy: "openai",
          noPromptRevision: false,
        },
        sourceImages: [
          { imageB64: "iVBORw0KGgpzb3VyY2U=", name: "source.png", mimeType: "image/png" },
        ],
      },
      { signal: new AbortController().signal },
    );
    assert.equal(capturedBodies[0].tools[0].input_fidelity, "high");
  });
});

test("runRemoteImageJob sends style for dall-e-3 Images API generation", async () => {
  const capturedBodies = [];
  await withPatchedGlobals(async () => {
    globalThis.fetch = async (_url, init) => {
      capturedBodies.push(JSON.parse(init.body));
      return new Response(
        '{"data":[{"b64_json":"YWJj","revised_prompt":"rev"}]}',
        { status: 200, headers: { "content-type": "application/json" } },
      );
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "generate",
          prompt: "cat",
          size: "1024x1024",
          quality: "low",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 0,
          negativePrompt: "",
          background: "auto",
          outputCompression: 100,
          inputFidelity: "auto",
          imageStyle: "natural",
          moderation: "low",
          baseURL: "https://upstream.example",
          textModelID: "",
          imageModelID: "dall-e-3",
          apiMode: "images",
          requestPolicy: "openai",
          noPromptRevision: false,
        },
      },
      { signal: new AbortController().signal },
    );
    assert.equal(capturedBodies[0].style, "natural");
  });
});

test("optimizePromptRemote extracts output_text", async () => {
  await withPatchedGlobals(async () => {
    globalThis.fetch = async () => new Response('{"output_text":"optimized prompt"}', {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  }, async () => {
    const kernel = await loadRemoteKernel();
    const text = await kernel.optimizePromptRemote({
      apiKey: "key",
      prompt: "cat",
      mode: "generate",
      baseURL: "https://upstream.example",
      textModelID: "gpt-5.5",
      imagePaths: [],
      imagePath: "",
    }, new AbortController().signal);
    assert.equal(text, "optimized prompt");
  });
});

test("optimizePromptRemote sends canvas bytes for prompt inference", async () => {
  let capturedBody = null;
  await withPatchedGlobals(async () => {
    globalThis.fetch = async (_url, init) => {
      capturedBody = JSON.parse(init.body);
      return new Response('{"output_text":"一只橘猫，窗边逆光"}', {
        status: 200,
        headers: { "content-type": "application/json" },
      });
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    const text = await kernel.optimizePromptRemote({
      apiKey: "key",
      prompt: "",
      mode: "describe",
      baseURL: "https://upstream.example",
      textModelID: "gpt-5.5",
      sourceImages: [{ name: "canvas.png", imageB64: "YWJj" }],
    }, new AbortController().signal);
    assert.equal(text, "一只橘猫，窗边逆光");
    assert.equal(capturedBody.input[0].content[1].image_url, "data:image/png;base64,YWJj");
    assert.match(capturedBody.instructions, /attached image/);
  });
});

test("Android shell remote kernel can use native HTTP bridge to bypass browser fetch", async () => {
  const partials = [];
  const progressEvents = [];
  await withPatchedGlobals(async () => {
    globalThis.window.AndroidImageStudio = {
      invoke(requestId, method, payloadJson) {
        const args = JSON.parse(payloadJson);
        queueMicrotask(() => {
          if (method === "HttpRequestText") {
            const payload = args[0];
            if (payload.url.endsWith("/v1/responses")) {
              assert.equal(payload.streamLines, true);
              assert.equal(payload.keepAlive, true);
              window.__imageStudioNativeProgress?.(payload.requestKey, {
                event: {
                  type: "response.image_generation_call.partial_image",
                  partial_image_index: 0,
                  partial_image_b64: "cGFydGlhbA==",
                },
              });
              window.__imageStudioNativeResolve?.(requestId, {
                status: 200,
                body: "",
                contentType: "text/event-stream",
                rawPath: "/sdcard/Android/data/top.gptcodex.imagestudio/files/Pictures/ImageStudio/log/android-http-response.txt",
                resultImageB64: "YW5kcm9pZA==",
                revisedPrompt: "native bridge",
                sourceEvent: "final",
              });
              return;
            }
          }
          if (method === "CancelHttpRequest") {
            window.__imageStudioNativeResolve?.(requestId, null);
            return;
          }
          window.__imageStudioNativeReject?.(requestId, `unsupported ${method}`);
        });
      },
    };
    Object.defineProperty(globalThis, "navigator", {
      configurable: true,
      value: {
        userAgent: "Mozilla/5.0 (Linux; Android 16; Pixel)",
        platform: "Linux armv8l",
        userAgentData: { platform: "Android" },
      },
    });
    globalThis.fetch = async () => {
      throw new Error("browser fetch should not be used in Android native HTTP mode");
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    const result = await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "generate",
          prompt: "cat",
          size: "1024x1024",
          quality: "low",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 0,
          negativePrompt: "",
          baseURL: "https://upstream.example",
          textModelID: "gpt-5.5",
          imageModelID: "gpt-image-2",
          apiMode: "responses",
          noPromptRevision: false,
        },
      },
      {
        signal: new AbortController().signal,
        onPartialImage: (partial) => partials.push(partial),
        onProgress: (...args) => progressEvents.push(args),
      },
    );
    assert.equal(result.imageB64, "YW5kcm9pZA==");
    assert.equal(result.revisedPrompt, "native bridge");
    assert.equal(partials.length, 1);
    assert.equal(partials[0].imageB64, "cGFydGlhbA==");
    assert.equal(partials[0].partialImageIndex, 0);
    assert.ok(progressEvents.some(([stage]) => stage === "已收到图片数据片段"));
  });
});

test("Android shell downloads Images API URL results through the native binary host path", async () => {
  const calls = [];
  const imageB64 = "iVBORw0KGgoAAA==";
  await withPatchedGlobals(async () => {
    globalThis.window.AndroidImageStudio = {
      invoke(requestId, method, payloadJson) {
        const args = JSON.parse(payloadJson);
        queueMicrotask(() => {
          if (method === "HttpRequestText") {
            const payload = args[0];
            calls.push(payload);
            if (payload.method === "POST") {
              window.__imageStudioNativeResolve?.(requestId, {
                status: 200,
                body: '{"data":[{"url":"https://cdn.example.com/result.png"}]}',
                contentType: "application/json",
              });
              return;
            }
            assert.equal(payload.url, "https://cdn.example.com/result.png");
            assert.equal(payload.method, "GET");
            assert.equal(payload.responseBase64, true);
            assert.equal(payload.maxResponseBytes, 50 * 1024 * 1024);
            window.__imageStudioNativeResolve?.(requestId, {
              status: 200,
              body: "",
              contentType: "image/png",
              resultImageB64: imageB64,
              sourceEvent: "url_download",
            });
            return;
          }
          window.__imageStudioNativeReject?.(requestId, `unsupported ${method}`);
        });
      },
    };
    globalThis.fetch = async () => {
      throw new Error("Android URL download must not use WebView fetch");
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    const result = await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "generate",
          prompt: "cat",
          size: "1024x1024",
          quality: "low",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 0,
          negativePrompt: "",
          baseURL: "https://upstream.example",
          textModelID: "",
          imageModelID: "relay-image-model",
          apiMode: "images",
          requestPolicy: "openai",
          imagesNewAPICompat: true,
          noPromptRevision: false,
        },
      },
      { signal: new AbortController().signal },
    );
    assert.equal(calls.length, 2);
    assert.equal(result.imageB64, imageB64);
    assert.equal(result.sourceEvent, "images_api_url");
  });
});

test("Android shell keeps parsing NewAPI stream results after an empty keepalive", async () => {
  await withPatchedGlobals(async () => {
    globalThis.window.AndroidImageStudio = {
      invoke(requestId, method) {
        queueMicrotask(() => {
          if (method !== "HttpRequestText") {
            window.__imageStudioNativeReject?.(requestId, `unsupported ${method}`);
            return;
          }
          window.__imageStudioNativeProgress?.(requestId, { line: "data:" });
          window.__imageStudioNativeResolve?.(requestId, {
            status: 200,
            body: 'data:\n\ndata:{"data":[{"b64_json":"img-data"}]}\n',
            contentType: "text/event-stream",
          });
        });
      },
    };
    globalThis.fetch = async () => {
      throw new Error("Android images branch must use native HTTP");
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    const result = await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "generate",
          prompt: "cat",
          size: "1024x1024",
          quality: "low",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 0,
          negativePrompt: "",
          baseURL: "https://upstream.example",
          textModelID: "",
          imageModelID: "relay-image-model",
          apiMode: "images",
          requestPolicy: "openai",
          imagesNewAPICompat: true,
          noPromptRevision: false,
        },
      },
      { signal: new AbortController().signal },
    );
    assert.equal(result.imageB64, "img-data");
  });
});

test("Android shell remote kernel wraps Responses websocket requests as response.create payloads", async () => {
  let capturedInvoke = null;
  await withPatchedGlobals(async () => {
    globalThis.window.AndroidImageStudio = {
      invoke(requestId, method, payloadJson) {
        const args = JSON.parse(payloadJson);
        if (method === "ResponsesWebSocketRequest") {
          capturedInvoke = { method, args };
          queueMicrotask(() => {
            window.__imageStudioNativeResolve?.(requestId, {
              status: 200,
              body: "",
              contentType: "text/event-stream",
              resultImageB64: "ZmluYWw=",
              revisedPrompt: "rev",
              sourceEvent: "final",
            });
          });
          return;
        }
        window.__imageStudioNativeReject?.(requestId, `unsupported ${method}`);
      },
    };
    globalThis.fetch = async () => {
      throw new Error("websocket branch should not use browser fetch");
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    const result = await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "generate",
          prompt: "cat",
          size: "1024x1024",
          quality: "low",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 0,
          negativePrompt: "",
          baseURL: "https://upstream.example",
          textModelID: "gpt-5.5",
          imageModelID: "gpt-image-2",
          apiMode: "responses",
          responsesTransport: "websocket",
          requestPolicy: "openai",
          noPromptRevision: false,
        },
      },
      { signal: new AbortController().signal },
    );
    assert.equal(result.imageB64, "ZmluYWw=");
    assert.equal(capturedInvoke?.method, "ResponsesWebSocketRequest");
    const payload = JSON.parse(capturedInvoke.args[0].payload);
    assert.equal(payload.type, "response.create");
    assert.equal(payload.model, "gpt-5.5");
    assert.equal(payload.store, false);
    assert.equal(payload.stream, undefined);
  });
});

test("Android shell remote kernel falls back to native HTTP when Responses websocket handshake fails", async () => {
  const calls = [];
  await withPatchedGlobals(async () => {
    globalThis.window.AndroidImageStudio = {
      invoke(requestId, method, payloadJson) {
        const args = JSON.parse(payloadJson);
        calls.push(method);
        queueMicrotask(() => {
          if (method === "ResponsesWebSocketRequest") {
            window.__imageStudioNativeReject?.(requestId, "websocket handshake failed: HTTP 426: WebSocket upgrade required (Upgrade: websocket)");
            return;
          }
          if (method === "HttpRequestText") {
            const payload = args[0];
            assert.equal(payload.url, "https://upstream.example/v1/responses");
            assert.equal(payload.keepAlive, true);
            window.__imageStudioNativeResolve?.(requestId, {
              status: 200,
              body: "",
              contentType: "text/event-stream",
              resultImageB64: "ZmFsbGJhY2s=",
              revisedPrompt: "fallback",
              sourceEvent: "final",
            });
            return;
          }
          window.__imageStudioNativeReject?.(requestId, `unsupported ${method}`);
        });
      },
    };
    globalThis.fetch = async () => {
      throw new Error("fallback path should stay on Android native transport");
    };
  }, async () => {
    const kernel = await loadRemoteKernel();
    const result = await kernel.runRemoteImageJob(
      {
        payload: {
          apiKey: "key",
          mode: "generate",
          prompt: "cat",
          size: "1024x1024",
          quality: "low",
          outputFormat: "png",
          imagePaths: [],
          imagePath: "",
          maskB64: "",
          seed: 0,
          negativePrompt: "",
          baseURL: "https://upstream.example",
          textModelID: "gpt-5.5",
          imageModelID: "gpt-image-2",
          apiMode: "responses",
          responsesTransport: "websocket",
          requestPolicy: "openai",
          noPromptRevision: false,
        },
      },
      { signal: new AbortController().signal },
    );
    assert.equal(result.imageB64, "ZmFsbGJhY2s=");
    assert.deepEqual(calls.slice(0, 2), ["ResponsesWebSocketRequest", "HttpRequestText"]);
  });
});
