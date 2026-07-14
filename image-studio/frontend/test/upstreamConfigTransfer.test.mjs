import assert from "node:assert/strict";
import test from "node:test";

test("buildUpstreamConfigExportFile includes profile metadata and api keys", async () => {
  const mod = await import(`../src/lib/upstreamConfigTransfer.ts?upstream-config-transfer=${Date.now()}-${Math.random().toString(36).slice(2)}`);
  const payload = mod.buildUpstreamConfigExportFile([
    {
      id: "p-1",
      name: "主配置",
      apiMode: "responses",
      requestPolicy: "openai",
      imagesNewAPICompat: false,
      baseURL: "https://relay.example.com",
      textModelID: "gpt-5.5",
      imageModelID: "gpt-image-2",
      reasoningEffort: "xhigh",
      concurrencyLimit: 3,
      createdAt: 1,
      lastUsedAt: 2,
    },
  ], "p-1", { "p-1": "sk-live" }, "p-1");

  assert.equal(payload.version, 1);
  assert.equal(payload.activeProfileId, "p-1");
  assert.equal(payload.aiProfileId, "p-1");
  assert.equal(payload.profiles[0].apiKey, "sk-live");
});

test("parseUpstreamConfigImportFile normalizes profiles and preserves embedded api keys", async () => {
  const mod = await import(`../src/lib/upstreamConfigTransfer.ts?upstream-config-transfer=${Date.now()}-${Math.random().toString(36).slice(2)}`);
  const parsed = mod.parseUpstreamConfigImportFile(JSON.stringify({
    version: 1,
    activeProfileId: "p-2",
    aiProfileId: "p-2",
    profiles: [
      {
        id: "p-2",
        name: "备用",
        apiMode: "images",
        responsesTransport: "sse",
        requestPolicy: "compat",
        imagesNewAPICompat: true,
        baseURL: " https://img.example.com/ ",
        textModelID: "ignored",
        imageModelID: "gpt-image-2",
        reasoningEffort: "medium",
        concurrencyLimit: 5.9,
        fallbackProfileId: "backup-1",
        createdAt: 10,
        lastUsedAt: 11,
        apiKey: " sk-backup ",
      },
    ],
  }));

  assert.deepEqual(parsed, {
    activeProfileId: "p-2",
    aiProfileId: "p-2",
    profiles: [
      {
        id: "p-2",
        name: "备用",
        apiMode: "images",
        responsesTransport: "sse",
        requestPolicy: "compat",
        imagesNewAPICompat: true,
        baseURL: "https://img.example.com",
        textModelID: "ignored",
        imageModelID: "gpt-image-2",
        reasoningEffort: "medium",
        concurrencyLimit: 5,
        fallbackProfileId: "backup-1",
        createdAt: 10,
        lastUsedAt: 11,
        apiKey: "sk-backup",
      },
    ],
  });
});

test("parseUpstreamConfigImportFile rejects empty profile payloads", async () => {
  const mod = await import(`../src/lib/upstreamConfigTransfer.ts?upstream-config-transfer=${Date.now()}-${Math.random().toString(36).slice(2)}`);
  assert.throws(
    () => mod.parseUpstreamConfigImportFile(JSON.stringify({ version: 1, profiles: [] })),
    /暂不支持这类 JSON/,
  );
});

test("parseUpstreamConfigImportFile adapts newapi_channel_conn template", async () => {
  const mod = await import(`../src/lib/upstreamConfigTransfer.ts?upstream-config-transfer=${Date.now()}-${Math.random().toString(36).slice(2)}`);
  const parsed = mod.parseUpstreamConfigImportFile(JSON.stringify({
    _type: "newapi_channel_conn",
    key: "sk-newapi",
    url: "https://api.linzefeng.top",
  }));

  assert.equal(parsed.activeProfileId, "template-1");
  assert.deepEqual(parsed.profiles, [
    {
      id: "template-1",
      name: "NewAPI · api.linzefeng.top",
      apiMode: "responses",
      responsesTransport: "sse",
      requestPolicy: "openai",
      imagesNewAPICompat: false,
      baseURL: "https://api.linzefeng.top",
      textModelID: "",
      imageModelID: "",
      reasoningEffort: "xhigh",
      concurrencyLimit: 0,
      createdAt: parsed.profiles[0].createdAt,
      apiKey: "sk-newapi",
    },
  ]);
});

test("parseUpstreamConfigImportFile adapts opencode provider template", async () => {
  const mod = await import(`../src/lib/upstreamConfigTransfer.ts?upstream-config-transfer=${Date.now()}-${Math.random().toString(36).slice(2)}`);
  const parsed = mod.parseUpstreamConfigImportFile(JSON.stringify({
    provider: {
      openai: {
        options: {
          baseURL: "https://gptcodex.top/v1",
          apiKey: "sk-opencode",
        },
        models: {
          "gpt-5.2": {
            name: "GPT-5.2",
            variants: {
              low: {},
              medium: {},
              high: {},
              xhigh: {},
            },
          },
          "gpt-5.5": {
            name: "GPT-5.5",
            variants: {
              low: {},
              medium: {},
              high: {},
              xhigh: {},
            },
          },
        },
      },
    },
  }));

  assert.equal(parsed.activeProfileId, "template-1");
  assert.deepEqual(parsed.profiles, [
    {
      id: "template-1",
      name: "OpenCode · openai · gptcodex.top",
      apiMode: "responses",
      responsesTransport: "sse",
      requestPolicy: "openai",
      imagesNewAPICompat: false,
      baseURL: "https://gptcodex.top",
      textModelID: "gpt-5.5",
      imageModelID: "",
      reasoningEffort: "xhigh",
      concurrencyLimit: 0,
      createdAt: parsed.profiles[0].createdAt,
      apiKey: "sk-opencode",
    },
  ]);
});

test("applyParsedUpstreamConfigImport re-links fallback ids and active profile", async () => {
  const mod = await import(`../src/lib/upstreamConfigTransfer.ts?upstream-config-transfer=${Date.now()}-${Math.random().toString(36).slice(2)}`);
  const store = {
    profiles: [],
    activeProfileId: "",
    aiProfileId: "",
  };
  let nextId = 1;

  const result = await mod.applyParsedUpstreamConfigImport({
    activeProfileId: "b",
    aiProfileId: "b",
    profiles: [
      {
        id: "a",
        name: "主配置",
        apiMode: "responses",
        responsesTransport: "sse",
        requestPolicy: "openai",
        imagesNewAPICompat: false,
        baseURL: "https://primary.example.com",
        textModelID: "gpt-5.5",
        imageModelID: "",
        reasoningEffort: "xhigh",
        concurrencyLimit: 0,
        createdAt: 1,
      },
      {
        id: "b",
        name: "备用配置",
        apiMode: "responses",
        responsesTransport: "sse",
        requestPolicy: "openai",
        imagesNewAPICompat: false,
        baseURL: "https://backup.example.com",
        textModelID: "gpt-5.2",
        imageModelID: "",
        reasoningEffort: "high",
        concurrencyLimit: 0,
        fallbackProfileId: "a",
        createdAt: 2,
      },
    ],
  }, {
    getProfiles: () => store.profiles,
    createProfile: async (input) => {
      const id = `p-${nextId++}`;
      store.profiles.push({
        id,
        name: input.name ?? "",
        apiMode: input.apiMode,
        responsesTransport: input.responsesTransport ?? "sse",
        requestPolicy: input.requestPolicy ?? "openai",
        imagesNewAPICompat: input.imagesNewAPICompat === true,
        baseURL: input.baseURL ?? "",
        textModelID: input.textModelID ?? "",
        imageModelID: input.imageModelID ?? "",
        reasoningEffort: input.reasoningEffort ?? "xhigh",
        concurrencyLimit: input.concurrencyLimit ?? 0,
        createdAt: nextId,
      });
      return id;
    },
    updateProfile: async (id, patch) => {
      const profile = store.profiles.find((item) => item.id === id);
      if (!profile) return false;
      Object.assign(profile, patch);
      return true;
    },
    setActiveProfile: async (id) => {
      store.activeProfileId = id;
    },
    setAIProfile: async (id) => {
      store.aiProfileId = id;
      return true;
    },
  });

  assert.equal(result.importedCount, 2);
  assert.equal(store.activeProfileId, "p-2");
  assert.equal(store.aiProfileId, "p-2");
  assert.equal(result.activeProfileId, "p-2");
  assert.equal(result.aiProfileId, "p-2");
  assert.deepEqual(
    store.profiles.map((item) => ({ id: item.id, name: item.name, fallbackProfileId: item.fallbackProfileId })),
    [
      { id: "p-1", name: "主配置", fallbackProfileId: undefined },
      { id: "p-2", name: "备用配置", fallbackProfileId: "p-1" },
    ],
  );
});
