import type {
  BatchProcessConfig,
  HistoryItem,
  LoopGenerationConfig,
  Preset,
  PromptTemplate,
  SourceImage,
  UpstreamProfile,
  Workspace,
} from "../../types/domain";

export type PreviewScenario = "mac-workspace" | "windows-right-rail";

export interface WorkspacePreviewData {
  profile: UpstreamProfile;
  history: HistoryItem[];
  currentImage: HistoryItem;
  sources: SourceImage[];
  workspace: Workspace;
  promptTemplates?: PromptTemplate[];
  presets?: Preset[];
}

const PREVIEW_PNG_B64 =
  "iVBORw0KGgoAAAANSUhEUgAAAEAAAABACAIAAAAlC+aJAAAAbUlEQVR4nO3PQQ3AIADAQMD2/hdwwZE8SBR0ztn3jJ9Zd7wD8E1gTWBNYE1gTWBNYE1gTWBNYE1gTWBNYE1gTWBNYE1gTWBNYE1gTWBNYE1gTWBNYE1gTWBNYE1gTWBNYE1gTWBNYE1gTWBNYF0X2AGCb5Q0aAAAAAElFTkSuQmCC";

function previewImageUrl(label: string, hue: number): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="480" height="480" viewBox="0 0 480 480"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0%" stop-color="hsl(${hue} 78% 58%)"/><stop offset="55%" stop-color="hsl(${(hue + 58) % 360} 74% 44%)"/><stop offset="100%" stop-color="hsl(${(hue + 128) % 360} 72% 26%)"/></linearGradient></defs><rect width="480" height="480" fill="url(#g)"/><circle cx="360" cy="116" r="132" fill="rgba(255,255,255,.18)"/><circle cx="116" cy="348" r="154" fill="rgba(0,0,0,.2)"/><rect x="56" y="306" width="368" height="90" rx="28" fill="rgba(0,0,0,.34)"/><text x="82" y="365" font-family="Inter,Arial,sans-serif" font-size="44" font-weight="800" fill="white">${label}</text></svg>`;
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
}

export function readPreviewScenario(): PreviewScenario | null {
  if (typeof window === "undefined") return null;
  try {
    const params = new URLSearchParams(window.location.search);
    const preview = (params.get("preview") ?? "").trim().toLowerCase();
    if (preview === "mac-workspace") return "mac-workspace";
    if (preview === "windows-right-rail") return "windows-right-rail";
    return null;
  } catch {
    return null;
  }
}


function buildHistory(now: number): HistoryItem[] {
  const batchPrompt = "赛博雨夜角色海报，湿地街道反光，红青霓虹边缘光，35mm，电影感，超细节";
  const batchDefs = Array.from({ length: 9 }, (_, index) => ({
    id: `preview-batch-${index + 1}`,
    prompt: batchPrompt,
    revisedPrompt: `同一提示词批量结果 ${index + 1}，强化雨滴高光、轮廓光与街面反射`,
    mode: index % 3 === 0 ? "edit" as const : "generate" as const,
    size: index < 3 ? "2880x2880" as const : index < 6 ? "2048x2048" as const : "1024x1024" as const,
    quality: index < 3 ? "high" as const : "medium" as const,
    negativePrompt: "模糊, 低清晰度, 脏污噪点",
    styleTag: index % 2 === 0 ? "电影海报" : "胶片人像",
    previewUrl: previewImageUrl(`B${index + 1}`, 196 + index * 17),
    batchIndex: index,
  }));

  const defs = [
    ...batchDefs,
    {
      id: "preview-history-1",
      prompt: "复古未来主义列车站台，雨雾、钠灯、金属结构、低角度广角",
      revisedPrompt: "高对比、冷暖霓虹、边缘轮廓光、海报构图、主体占中",
      mode: "edit" as const,
      size: "2880x2880" as const,
      quality: "high" as const,
      negativePrompt: "模糊, 低清晰度, 脏污噪点",
      styleTag: "电影海报",
      previewUrl: previewImageUrl("R1", 312),
    },
    {
      id: "preview-history-2",
      prompt: "产品棚拍样张，银色耳机置于磨砂台面，柔光箱高光干净，商业摄影",
      revisedPrompt: "极简背景、金属反射控制、轻微俯拍、留白构图",
      mode: "generate" as const,
      size: "2048x2048" as const,
      quality: "medium" as const,
      negativePrompt: "畸变, 文字, 水印",
      styleTag: "商业摄影",
      previewUrl: previewImageUrl("P1", 42),
    },
    {
      id: "preview-history-3",
      prompt: "复古未来主义列车站台，雨雾、钠灯、金属结构、低角度广角",
      revisedPrompt: "同一提示词的第二版，强化雨滴高光与面部轮廓",
      mode: "edit" as const,
      size: "2048x2048" as const,
      quality: "medium" as const,
      negativePrompt: "过曝, 手部畸形",
      styleTag: "胶片人像",
      previewUrl: previewImageUrl("R2", 162),
    },
    {
      id: "preview-history-4",
      prompt: "建筑外观黄昏蓝调时刻，玻璃幕墙反射天光，广角透视校正",
      revisedPrompt: "蓝金时刻、垂直线控制、通透玻璃、干净天空",
      mode: "generate" as const,
      size: "1024x1024" as const,
      quality: "high" as const,
      negativePrompt: "低清, 透视变形",
      styleTag: "建筑表现",
      previewUrl: previewImageUrl("A1", 224),
    },
    {
      id: "preview-history-5",
      prompt: "产品棚拍样张，银色耳机置于磨砂台面，柔光箱高光干净，商业摄影",
      revisedPrompt: "同一提示词的第二版，增加侧后方轮廓光",
      mode: "edit" as const,
      size: "1024x1024" as const,
      quality: "medium" as const,
      negativePrompt: "重影, 杂乱背景",
      styleTag: "静物生活",
      previewUrl: previewImageUrl("P2", 62),
    },
    {
      id: "preview-history-6",
      prompt: "复古未来主义列车站台，雨雾、钠灯、金属结构、低角度广角",
      revisedPrompt: "同一提示词的第三版，改成更强对比的半身构图",
      mode: "generate" as const,
      size: "1024x1024" as const,
      quality: "medium" as const,
      negativePrompt: "糊边, 文字乱码",
      styleTag: "工业设计",
      previewUrl: previewImageUrl("R3", 182),
    },
  ];

  return defs.map((item, index) => ({
    ...item,
    imageB64: item.previewUrl ? undefined : PREVIEW_PNG_B64,
    outputFormat: "png",
    background: index % 3 === 0 ? "opaque" : "auto",
    outputCompression: 100,
    inputFidelity: index % 2 === 0 ? "auto" : "high",
    imageStyle: index % 3 === 0 ? "default" : "natural",
    moderation: index % 2 === 0 ? "low" : "auto",
    createdAt: now - index * 55 * 60 * 1000,
    savedPath: `/tmp/${item.id}.png`,
    rawPath: `/tmp/${item.id}.json`,
    seed: 3200 + index,
    elapsedSec: 7 + index,
  }));
}

function buildSources(): SourceImage[] {
  return [
    {
      path: "/tmp/preview-source-a.png",
      name: "原图-A.png",
      size: 16384,
      imageB64: PREVIEW_PNG_B64,
    },
    {
      path: "/tmp/preview-source-b.png",
      name: "构图参考-B.png",
      size: 16384,
      imageB64: PREVIEW_PNG_B64,
    },
  ];
}

function buildPreviewProfile(now: number): UpstreamProfile {
  return {
    id: "preview-profile",
    name: "Preview Responses",
    provider: "openai",
    apiMode: "responses",
    responsesTransport: "sse",
    requestPolicy: "openai",
    baseURL: "https://code1.linzefeng.top",
    textModelID: "gpt-4.1-mini",
    imageModelID: "gpt-image-1",
    reasoningEffort: "xhigh",
    concurrencyLimit: 1,
    createdAt: now,
    lastUsedAt: now,
  };
}

function buildWorkspace(
  workspaceId: string,
  currentImage: HistoryItem,
  sources: SourceImage[],
): Workspace {
  const loopGeneration: LoopGenerationConfig = {
    enabled: false,
    totalCount: 10,
    concurrency: 2,
    autoSave: false,
    autoSaveDir: "",
    livePreview: true,
  };
  const batchProcess: BatchProcessConfig = {
    enabled: false,
    inputDir: "",
    outputMode: "source_dir",
    outputDir: "",
    concurrency: 2,
    retryOnFailure: false,
    fileNamePrefix: "processed-",
    autoAspectResolution: "",
    discoveredSources: [],
  };
  return {
    id: workspaceId,
    name: "联调样例",
    prompt: currentImage.prompt,
    negativePrompt: currentImage.negativePrompt ?? "",
    mode: "edit",
    size: "2880x2880",
    quality: "high",
    outputFormat: "png",
    seed: 3200,
    background: currentImage.background ?? "auto",
    outputCompression: currentImage.outputCompression ?? 100,
    inputFidelity: currentImage.inputFidelity ?? "auto",
    imageStyle: currentImage.imageStyle ?? "default",
    moderation: currentImage.moderation ?? "low",
    userIdentifier: "",
    partialImages: 1,
    batchCount: 1,
    editSourceMode: "manual",
    batchProcess,
    loopGeneration,
    sources,
    currentImageId: currentImage.id,
    batchResultIds: [],
    resultGridOpen: false,
    runningJobIds: [],
    jobsTotal: 0,
    jobsCompleted: 0,
    progress: null,
    streamPreview: null,
    streamPreviews: {},
    lastLogLine: "",
    errorMessage: null,
    errorRawPath: null,
    lastPayload: null,
  };
}

export function buildMacWorkspacePreview(workspaceId: string): WorkspacePreviewData {
  const now = Date.now();
  const history = buildHistory(now);
  const currentImage = history[0];
  const sources = buildSources();
  const profile = buildPreviewProfile(now);
  const workspace = buildWorkspace(workspaceId, currentImage, sources);

  return {
    profile,
    history,
    currentImage,
    sources,
    workspace,
  };
}

function buildWindowsRightRailHistory(now: number): HistoryItem[] {
  const prompt = "画一只小猫";
  const defs = [
    {
      id: "win-preview-history-1",
      prompt,
      revisedPrompt: "奶油色英短幼猫，坐在木地板边缘，室内自然光，干净背景，照片感",
      mode: "generate" as const,
      size: "3456x2304" as const,
      quality: "auto" as const,
      negativePrompt: "模糊, 双影, 多余肢体",
      styleTag: "",
      previewUrl: previewImageUrl("文生图", 36),
      createdAt: now - 8 * 60 * 1000,
      savedPath: "/tmp/win-preview-history-1.png",
      rawPath: "/tmp/win-preview-history-1.json",
      outputFormat: "png" as const,
      background: "auto" as const,
      outputCompression: 100,
      inputFidelity: "auto" as const,
      imageStyle: "default" as const,
      moderation: "low" as const,
      seed: 1201,
      elapsedSec: 16,
    },
    {
      id: "win-preview-history-2",
      prompt,
      revisedPrompt: "同一提示词第二版，拉近景别，保留木色背景与柔和窗边光",
      mode: "generate" as const,
      size: "3456x2304" as const,
      quality: "auto" as const,
      negativePrompt: "模糊, 双影, 多余肢体",
      styleTag: "",
      previewUrl: previewImageUrl("2", 28),
      createdAt: now - 18 * 60 * 1000,
      savedPath: "/tmp/win-preview-history-2.png",
      rawPath: "/tmp/win-preview-history-2.json",
      outputFormat: "png" as const,
      background: "auto" as const,
      outputCompression: 100,
      inputFidelity: "auto" as const,
      imageStyle: "default" as const,
      moderation: "low" as const,
      seed: 1202,
      elapsedSec: 19,
    },
    {
      id: "win-preview-history-3",
      prompt: "奶白色小猫窝在亚麻靠垫上，近景，柔光，干净背景",
      revisedPrompt: "更近的半身取景，毛发细节清楚，保持暖白色调",
      mode: "edit" as const,
      size: "1024x1024" as const,
      quality: "medium" as const,
      negativePrompt: "锐化过度, 假眼神光",
      styleTag: "静物生活",
      previewUrl: previewImageUrl("图生图", 58),
      createdAt: now - 44 * 60 * 1000,
      savedPath: "/tmp/win-preview-history-3.png",
      rawPath: "/tmp/win-preview-history-3.json",
      outputFormat: "png" as const,
      background: "auto" as const,
      outputCompression: 100,
      inputFidelity: "high" as const,
      imageStyle: "natural" as const,
      moderation: "low" as const,
      seed: 1203,
      elapsedSec: 22,
    },
  ];

  return defs;
}

function buildWindowsWorkspace(
  workspaceId: string,
  currentImage: HistoryItem,
  sources: SourceImage[],
): Workspace {
  const loopGeneration: LoopGenerationConfig = {
    enabled: false,
    totalCount: 10,
    concurrency: 2,
    autoSave: false,
    autoSaveDir: "",
    livePreview: true,
  };
  const batchProcess: BatchProcessConfig = {
    enabled: false,
    inputDir: "",
    outputMode: "source_dir",
    outputDir: "",
    concurrency: 2,
    retryOnFailure: false,
    fileNamePrefix: "processed-",
    autoAspectResolution: "",
    discoveredSources: [],
  };
  return {
    id: workspaceId,
    name: "Windows 右栏预览",
    prompt: "",
    negativePrompt: "",
    mode: "generate",
    size: "1024x1024",
    quality: "medium",
    outputFormat: "png",
    seed: 0,
    background: "auto",
    outputCompression: 100,
    inputFidelity: "auto",
    imageStyle: "default",
    moderation: "low",
    userIdentifier: "",
    partialImages: 0,
    batchCount: 1,
    editSourceMode: "manual",
    batchProcess,
    loopGeneration,
    sources,
    currentImageId: currentImage.id,
    batchResultIds: [],
    resultGridOpen: false,
    runningJobIds: [],
    jobsTotal: 0,
    jobsCompleted: 0,
    progress: null,
    streamPreview: null,
    streamPreviews: {},
    lastLogLine: "",
    errorMessage: null,
    errorRawPath: null,
    lastPayload: null,
  };
}

export function buildWindowsRightRailPreview(workspaceId: string): WorkspacePreviewData {
  const now = Date.now();
  const history = buildWindowsRightRailHistory(now);
  const currentImage = history[0];
  const sources: SourceImage[] = [];
  const profile = buildPreviewProfile(now);
  profile.name = "Windows Preview";
  const workspace = buildWindowsWorkspace(workspaceId, currentImage, sources);

  return {
    profile,
    history,
    currentImage,
    sources,
    workspace,
    promptTemplates: [],
    presets: [],
  };
}
