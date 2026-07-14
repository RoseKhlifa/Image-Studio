import { targetPlatform } from "../index.ts";
import {
  RemoteKernelError,
  runRemoteImageJob,
  optimizePromptRemote,
} from "./remoteKernel.ts";
import { normalizeRequestPolicy } from "../../../../../shared/kernel/requestModel.js";
import {
  canUseWebGLImageTransforms,
  cropVirtualImage,
  flipVirtualImage,
  isVirtualPath,
  openImageDialogFallback,
  openVirtualPath,
  readVirtualImageAsBase64,
  readVirtualText,
  registerVirtualImage,
  registerVirtualText,
  releaseVirtualPath,
  rotateVirtualImage,
} from "../../lib/virtualHostStore.ts";
import {
  browserStoredAPIKey,
  fileNameFromPath,
  saveByDownload,
  setBrowserStoredAPIKey,
} from "./hostBrowser.ts";
import {
  clearLocalEvents,
  emitLocalEvent,
  getForcedKernelRuntimeMode,
  onLocalEvent,
  setForcedKernelRuntimeMode,
} from "./hostEvents.ts";
import {
  canInvokeAndroidMethod,
  getRuntime,
  hasServiceMethod,
  invokeAndroid,
  invokeService,
} from "./hostBindings.ts";
import type {
  BatchInputDirectoryLike,
  CodexAPIConfigLike,
  GenerateOptionsLike,
  HostCapabilities,
  HostKind,
  CompatibilityStateLike,
  AppUpdateInfoLike,
  AppUpdateProbeResultLike,
  ImageTransformResultLike,
  ImportedImageLike,
  JobStartedLike,
  KernelRuntimeMode,
  MediaAssetRefLike,
  ProbeUpstreamOptionsLike,
  ProbeUpstreamResultLike,
  PromptImportActivationLike,
  PromptImportPayloadLike,
  PromptOptimizeOptionsLike,
  SelectFileResponseLike,
  SelectFilesResponseLike,
} from "./hostTypes.ts";

const remoteJobControllers = new Map<string, AbortController>();

function unsupportedMessage(method: string): string {
  const kind = detectHostKind();
  if (kind === "android-shell") {
    return `当前 Android shell 未提供 ${method} 对应的本地内核能力`;
  }
  if (kind === "browser") {
    return `当前浏览器预览环境未注入 ${method} 宿主能力`;
  }
  return `宿主未暴露 ${method} 能力`;
}

function makeJobID(): string {
  try {
    if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
      return `job-${crypto.randomUUID()}`;
    }
  } catch {
    // ignore
  }
  return `job-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}

function supportsDesktopNativeGPUTransforms(): boolean {
  return detectHostKind() === "wails-desktop" && targetPlatform === "macos";
}

async function persistVirtualTransformResult(
  result: { path: string; imageB64?: string; mimeType?: string; name?: string; acceleration?: string },
  fallbackName: string,
): Promise<ImageTransformResultLike> {
  const imageB64 = result.imageB64 || readVirtualImageAsBase64(result.path);
  const suggested = result.name || fallbackName;
  const imported = await ImportImageFromB64(imageB64, suggested);
  return {
    path: imported.path,
    acceleration: result.acceleration,
  };
}

async function materializeReadablePathAsVirtual(path: string): Promise<ImportedImageLike> {
  const imageB64 = await ReadImageAsBase64(path);
  return registerVirtualImage({
    imageB64,
    suggestedName: fileNameFromPath(path),
  });
}

async function importHistoryFallback(): Promise<string> {
  if (typeof document === "undefined") return "";
  return new Promise((resolve, reject) => {
    const input = document.createElement("input");
    input.type = "file";
    input.accept = "application/json,.json";
    input.style.position = "fixed";
    input.style.left = "-9999px";
    document.body.appendChild(input);
    const cleanup = () => input.remove();
    input.addEventListener("change", async () => {
      try {
        const file = input.files?.[0];
        if (!file) {
          cleanup();
          resolve("");
          return;
        }
        cleanup();
        resolve(await file.text());
      } catch (error) {
        cleanup();
        reject(error);
      }
    }, { once: true });
    input.click();
  });
}

async function startRemoteJob(options: GenerateOptionsLike): Promise<JobStartedLike> {
  const jobId = options.requestedJobId?.trim() || makeJobID();
  const controller = new AbortController();
  remoteJobControllers.set(jobId, controller);
  void (async () => {
    try {
      const result = await runRemoteImageJob({ payload: {
        ...options,
        requestPolicy: normalizeRequestPolicy(options.requestPolicy),
        imagesNewAPICompat: options.imagesNewAPICompat === true,
      }, sourceImages: options.sourceImages }, {
        signal: controller.signal,
        onLog: (line) => emitLocalEvent(`log:${jobId}`, line),
        onProgress: (stage, elapsed, bytes) => emitLocalEvent(`progress:${jobId}`, { stage, elapsed, bytes }),
        onPartialImage: (partial) => emitLocalEvent(`preview:${jobId}`, {
          imageB64: partial.imageB64,
          revisedPrompt: partial.revisedPrompt || "",
          partialImageIndex: partial.partialImageIndex ?? -1,
          mode: options.mode || "generate",
          prompt: options.prompt,
        }),
      });
      if (controller.signal.aborted) return;
      const saved = registerVirtualImage({
        imageB64: result.imageB64,
        suggestedName: `image-${options.mode || "generate"}.${options.outputFormat || "png"}`,
      });
      emitLocalEvent(`result:${jobId}`, {
        imageB64: result.imageB64,
        revisedPrompt: result.revisedPrompt,
        sourceEvent: result.sourceEvent,
        savedPath: saved.path,
        rawPath: result.rawPath,
        mode: result.mode,
        prompt: result.prompt,
      });
    } catch (error) {
      if (controller.signal.aborted) return;
      const typed = error instanceof RemoteKernelError
        ? error
        : new RemoteKernelError(String((error as any)?.message || error));
      emitLocalEvent(`error:${jobId}`, {
        message: typed.message,
        rawPath: typed.rawPath || null,
      });
    } finally {
      remoteJobControllers.delete(jobId);
    }
  })();
  return { jobId };
}

function withoutRuntimeSourceImages(options: GenerateOptionsLike): GenerateOptionsLike {
  const { sourceImages: _sourceImages, ...payload } = options;
  return payload;
}

async function runPersistedVirtualTransform(
  path: string,
  transform: (virtualPath: string) => Promise<{ path: string; imageB64?: string; mimeType?: string; name?: string; acceleration?: string }>,
): Promise<ImageTransformResultLike> {
  const imported = await materializeReadablePathAsVirtual(path);
  let resultPath: string | null = null;
  try {
    const result = await transform(imported.path);
    resultPath = result.path;
    return await persistVirtualTransformResult(result, fileNameFromPath(path));
  } finally {
    releaseVirtualPath(imported.path);
    if (resultPath && resultPath !== imported.path) releaseVirtualPath(resultPath);
  }
}

export function detectHostKind(): HostKind {
  if (targetPlatform === "android" || targetPlatform === "android-pad") {
    return canInvokeAndroidMethod("GetOutputDir") || hasServiceMethod("GetOutputDir") ? "android-shell" : "browser";
  }
  if (hasServiceMethod("Generate") && getRuntime()) return "wails-desktop";
  return "browser";
}

export function setKernelRuntimeMode(mode: KernelRuntimeMode) {
  setForcedKernelRuntimeMode(mode);
}

export function getKernelRuntimeMode(): KernelRuntimeMode {
  return getForcedKernelRuntimeMode();
}

export function getHostCapabilities(): HostCapabilities {
  const kind = detectHostKind();
  const localGenerationCapable = kind === "wails-desktop" && hasServiceMethod("Generate") && hasServiceMethod("Edit");
  const localPromptOptimizeCapable = kind === "wails-desktop" && hasServiceMethod("OptimizePrompt");
  const localModeEnabled = getForcedKernelRuntimeMode() !== "remote";
  const hasDesktopNativeTransforms = kind === "wails-desktop"
    && hasServiceMethod("RotateImage")
    && hasServiceMethod("FlipImage")
    && hasServiceMethod("CropImage");
  const webGLTransforms = canUseWebGLImageTransforms();
  const imageTransformAcceleration = supportsDesktopNativeGPUTransforms()
    ? "gpu-metal"
    : webGLTransforms
      ? "gpu-webgl"
      : hasDesktopNativeTransforms
        ? "native"
        : kind === "android-shell" || kind === "browser"
          ? "cpu-canvas"
          : "none";
  return {
    localGeneration: localGenerationCapable && localModeEnabled,
    promptOptimization: localPromptOptimizeCapable && localModeEnabled,
    nativeFileDialogs: kind === "wails-desktop" || canInvokeAndroidMethod("OpenImageDialog"),
    nativeImageTransforms:
      hasDesktopNativeTransforms
      || kind === "android-shell"
      || kind === "browser",
    imageTransformAcceleration,
    nativeHistoryFileIO: kind === "wails-desktop" || canInvokeAndroidMethod("ImportHistoryFromFile"),
    nativeOutputDirectoryPicker: hasServiceMethod("ChooseOutputDir") && kind !== "android-shell",
    secureCredentialStore: kind === "wails-desktop",
  };
}

export function EventsOn(eventName: string, callback: (...args: any[]) => void) {
  const offLocal = onLocalEvent(eventName, callback);
  const runtime = getRuntime();
  const offRuntime = runtime?.EventsOnMultiple
    ? runtime.EventsOnMultiple(eventName, callback, -1)
    : () => undefined;
  return () => {
    offLocal();
    offRuntime();
  };
}

export function EventsOff(eventName: string, ...additionalEventNames: string[]) {
  clearLocalEvents(eventName, ...additionalEventNames);
  getRuntime()?.EventsOff?.(eventName, ...additionalEventNames);
}

export function WindowSetSystemDefaultTheme() {
  getRuntime()?.WindowSetSystemDefaultTheme?.();
}

export function WindowSetLightTheme() {
  getRuntime()?.WindowSetLightTheme?.();
}

export function WindowSetDarkTheme() {
  getRuntime()?.WindowSetDarkTheme?.();
}

export function Generate(options: GenerateOptionsLike): Promise<JobStartedLike> {
  if (getForcedKernelRuntimeMode() === "local" && detectHostKind() !== "wails-desktop") {
    return Promise.reject(new Error("当前宿主不支持强制本地内核"));
  }
  if (getHostCapabilities().localGeneration) {
    return invokeService<JobStartedLike>(unsupportedMessage, "Generate", withoutRuntimeSourceImages(options));
  }
  return startRemoteJob({ ...options, mode: "generate" });
}

export function Edit(options: GenerateOptionsLike): Promise<JobStartedLike> {
  if (getForcedKernelRuntimeMode() === "local" && detectHostKind() !== "wails-desktop") {
    return Promise.reject(new Error("当前宿主不支持强制本地内核"));
  }
  if (getHostCapabilities().localGeneration) {
    return invokeService<JobStartedLike>(unsupportedMessage, "Edit", withoutRuntimeSourceImages(options));
  }
  return startRemoteJob({ ...options, mode: "edit" });
}

export function OptimizePrompt(options: PromptOptimizeOptionsLike): Promise<string> {
  if (getForcedKernelRuntimeMode() === "local" && detectHostKind() !== "wails-desktop") {
    return Promise.reject(new Error("当前宿主不支持强制本地内核"));
  }
  if (getHostCapabilities().promptOptimization) {
    const { sourceImages: _sourceImages, ...nativeOptions } = options;
    return invokeService<string>(unsupportedMessage, "OptimizePrompt", nativeOptions);
  }
  const controller = new AbortController();
  return optimizePromptRemote({
    apiKey: options.apiKey,
    prompt: options.prompt,
    mode: options.mode,
    baseURL: options.baseURL,
    textModelID: options.textModelID,
    proxyMode: options.proxyMode,
    proxyURL: options.proxyURL,
    imagePaths: options.imagePaths,
    imagePath: options.imagePath,
    sourceImages: options.sourceImages,
  }, controller.signal);
}

export function Cancel(jobId: string): Promise<void> {
  const remote = remoteJobControllers.get(jobId);
  if (remote) {
    remote.abort();
    remoteJobControllers.delete(jobId);
    return Promise.resolve();
  }
  if (hasServiceMethod("Cancel")) {
    return invokeService<void>(unsupportedMessage, "Cancel", jobId);
  }
  if (canInvokeAndroidMethod("Cancel")) {
    return invokeAndroid<void>(unsupportedMessage, "Cancel", jobId).catch(() => undefined);
  }
  return Promise.resolve();
}

export function OpenImageDialog(): Promise<SelectFileResponseLike> {
  if (hasServiceMethod("OpenImageDialog")) {
    return invokeService<SelectFileResponseLike>(unsupportedMessage, "OpenImageDialog").catch(() => openImageDialogFallback());
  }
  if (canInvokeAndroidMethod("OpenImageDialog")) {
    return invokeAndroid<SelectFileResponseLike>(unsupportedMessage, "OpenImageDialog").catch(() => openImageDialogFallback());
  }
  return openImageDialogFallback();
}

export function OpenMaskImageDialog(): Promise<SelectFileResponseLike> {
  if (hasServiceMethod("OpenMaskImageDialog")) {
    return invokeService<SelectFileResponseLike>(unsupportedMessage, "OpenMaskImageDialog").catch(() => OpenImageDialog());
  }
  if (canInvokeAndroidMethod("OpenMaskImageDialog")) {
    return invokeAndroid<SelectFileResponseLike>(unsupportedMessage, "OpenMaskImageDialog").catch(() => OpenImageDialog());
  }
  return openImageDialogFallback();
}

export function OpenImagesDialog(): Promise<SelectFilesResponseLike> {
  if (hasServiceMethod("OpenImagesDialog")) {
    return invokeService<SelectFilesResponseLike>(unsupportedMessage, "OpenImagesDialog");
  }
  return Promise.reject(new Error(unsupportedMessage("OpenImagesDialog")));
}

export function ChooseBatchInputDir(): Promise<BatchInputDirectoryLike> {
  if (hasServiceMethod("ChooseBatchInputDir")) {
    return invokeService<BatchInputDirectoryLike>(unsupportedMessage, "ChooseBatchInputDir");
  }
  return Promise.reject(new Error(unsupportedMessage("ChooseBatchInputDir")));
}

export function ListBatchInputImages(directory: string): Promise<BatchInputDirectoryLike> {
  if (hasServiceMethod("ListBatchInputImages")) {
    return invokeService<BatchInputDirectoryLike>(unsupportedMessage, "ListBatchInputImages", directory);
  }
  return Promise.reject(new Error(unsupportedMessage("ListBatchInputImages")));
}

export function GetOutputDir(): Promise<string> {
  if (hasServiceMethod("GetOutputDir")) {
    return invokeService<string>(unsupportedMessage, "GetOutputDir");
  }
  if (canInvokeAndroidMethod("GetOutputDir")) {
    return invokeAndroid<string>(unsupportedMessage, "GetOutputDir");
  }
  return Promise.resolve("");
}

export function canLoadCodexAPIConfig(): boolean {
  return detectHostKind() === "wails-desktop" && hasServiceMethod("LoadCodexAPIConfig");
}

export function LoadCodexAPIConfig(): Promise<CodexAPIConfigLike> {
  if (hasServiceMethod("LoadCodexAPIConfig")) {
    return invokeService<CodexAPIConfigLike>(unsupportedMessage, "LoadCodexAPIConfig");
  }
  return Promise.reject(new Error(unsupportedMessage("LoadCodexAPIConfig")));
}

export function canImportPromptByToken(): boolean {
  return detectHostKind() === "wails-desktop" && hasServiceMethod("ImportPromptByToken");
}

export function ActivatePromptImportListener(): Promise<PromptImportActivationLike> {
  if (hasServiceMethod("ActivatePromptImportListener")) {
    return invokeService<PromptImportActivationLike>(unsupportedMessage, "ActivatePromptImportListener");
  }
  return Promise.resolve({ tokens: [], invalidCount: 0 });
}

export function ImportPromptByToken(token: string): Promise<PromptImportPayloadLike> {
  if (hasServiceMethod("ImportPromptByToken")) {
    return invokeService<PromptImportPayloadLike>(unsupportedMessage, "ImportPromptByToken", token);
  }
  return Promise.reject(new Error(unsupportedMessage("ImportPromptByToken")));
}

export function DeleteStoredAPIKey(user: string): Promise<void> {
  if (hasServiceMethod("DeleteStoredAPIKey")) {
    return invokeService<void>(unsupportedMessage, "DeleteStoredAPIKey", user);
  }
  if (canInvokeAndroidMethod("DeleteStoredAPIKey")) {
    return invokeAndroid<void>(unsupportedMessage, "DeleteStoredAPIKey", user);
  }
  setBrowserStoredAPIKey(user, "");
  return Promise.resolve();
}

export function GetStoredAPIKey(user: string): Promise<string> {
  if (hasServiceMethod("GetStoredAPIKey")) {
    return invokeService<string>(unsupportedMessage, "GetStoredAPIKey", user);
  }
  if (canInvokeAndroidMethod("GetStoredAPIKey")) {
    return invokeAndroid<string>(unsupportedMessage, "GetStoredAPIKey", user);
  }
  return Promise.resolve(browserStoredAPIKey(user));
}

export function SetStoredAPIKey(user: string, value: string): Promise<void> {
  if (hasServiceMethod("SetStoredAPIKey")) {
    return invokeService<void>(unsupportedMessage, "SetStoredAPIKey", user, value);
  }
  if (canInvokeAndroidMethod("SetStoredAPIKey")) {
    return invokeAndroid<void>(unsupportedMessage, "SetStoredAPIKey", user, value);
  }
  setBrowserStoredAPIKey(user, value);
  return Promise.resolve();
}

export function SaveImageAs(imageB64: string, suggestedName: string): Promise<string> {
  if (hasServiceMethod("SaveImageAs")) {
    return invokeService<string>(unsupportedMessage, "SaveImageAs", imageB64, suggestedName);
  }
  if (canInvokeAndroidMethod("SaveImageAs")) {
    return invokeAndroid<string>(unsupportedMessage, "SaveImageAs", imageB64, suggestedName);
  }
  const mimeType = suggestedName.toLowerCase().endsWith(".jpg") || suggestedName.toLowerCase().endsWith(".jpeg")
    ? "image/jpeg"
    : suggestedName.toLowerCase().endsWith(".webp")
      ? "image/webp"
      : "image/png";
  return Promise.resolve(saveByDownload(new Blob([Uint8Array.from(atob(imageB64), (ch) => ch.charCodeAt(0))], { type: mimeType }), suggestedName));
}

export function SaveImagePathAs(path: string, suggestedName: string): Promise<string> {
  if (hasServiceMethod("SaveImagePathAs")) {
    return invokeService<string>(unsupportedMessage, "SaveImagePathAs", path, suggestedName);
  }
  if (isVirtualPath(path)) {
    return ReadImageAsBase64(path).then((b64) => SaveImageAs(b64, suggestedName));
  }
  if (canInvokeAndroidMethod("SaveImagePathAs")) {
    return invokeAndroid<string>(unsupportedMessage, "SaveImagePathAs", path, suggestedName)
      .catch(() => ReadImageAsBase64(path).then((b64) => SaveImageAs(b64, suggestedName)));
  }
  return ReadImageAsBase64(path).then((b64) => SaveImageAs(b64, suggestedName));
}

export function BeginNativeFileDrag(path: string): Promise<void> {
  if (hasServiceMethod("BeginNativeFileDrag")) {
    return invokeService<void>(unsupportedMessage, "BeginNativeFileDrag", path);
  }
  return Promise.reject(new Error(unsupportedMessage("BeginNativeFileDrag")));
}

export function SaveImageToDir(imageB64: string, directory: string, suggestedName: string): Promise<string> {
  if (hasServiceMethod("SaveImageToDir")) {
    return invokeService<string>(unsupportedMessage, "SaveImageToDir", imageB64, directory, suggestedName);
  }
  if (canInvokeAndroidMethod("SaveImageToDir")) {
    return invokeAndroid<string>(unsupportedMessage, "SaveImageToDir", imageB64, directory, suggestedName);
  }
  return Promise.reject(new Error(unsupportedMessage("SaveImageToDir")));
}

export function SaveImagePathToDir(path: string, directory: string, suggestedName: string): Promise<string> {
  if (hasServiceMethod("SaveImagePathToDir")) {
    return invokeService<string>(unsupportedMessage, "SaveImagePathToDir", path, directory, suggestedName);
  }
  if (isVirtualPath(path)) {
    return ReadImageAsBase64(path).then((b64) => SaveImageToDir(b64, directory, suggestedName));
  }
  if (canInvokeAndroidMethod("SaveImagePathToDir")) {
    return invokeAndroid<string>(unsupportedMessage, "SaveImagePathToDir", path, directory, suggestedName)
      .catch(() => ReadImageAsBase64(path).then((b64) => SaveImageToDir(b64, directory, suggestedName)));
  }
  return ReadImageAsBase64(path).then((b64) => SaveImageToDir(b64, directory, suggestedName));
}

export function RegisterMediaAsset(savedPath: string, thumbPath: string): Promise<MediaAssetRefLike> {
  if (hasServiceMethod("RegisterMediaAsset")) {
    return invokeService<MediaAssetRefLike>(unsupportedMessage, "RegisterMediaAsset", savedPath, thumbPath);
  }
  return Promise.resolve({ savedPath, thumbPath });
}

export function RegisterImportedImageAsset(path: string): Promise<MediaAssetRefLike> {
  if (hasServiceMethod("RegisterImportedImageAsset")) {
    return invokeService<MediaAssetRefLike>(unsupportedMessage, "RegisterImportedImageAsset", path);
  }
  return Promise.resolve({ savedPath: path });
}

export function ImportImageFromB64(imageB64: string, suggestedName: string): Promise<ImportedImageLike> {
  if (hasServiceMethod("ImportImageFromB64")) {
    return invokeService<ImportedImageLike>(unsupportedMessage, "ImportImageFromB64", imageB64, suggestedName)
      .catch(() => registerVirtualImage({ imageB64, suggestedName }));
  }
  if (canInvokeAndroidMethod("ImportImageFromB64")) {
    return invokeAndroid<ImportedImageLike>(unsupportedMessage, "ImportImageFromB64", imageB64, suggestedName)
      .catch(() => registerVirtualImage({ imageB64, suggestedName }));
  }
  return Promise.resolve(registerVirtualImage({ imageB64, suggestedName }));
}

export function RotateImage(path: string, degrees: number): Promise<ImageTransformResultLike> {
  if (isVirtualPath(path)) {
    return rotateVirtualImage(path, degrees).then((result) => ({ path: result.path, acceleration: result.acceleration || "cpu-canvas" }));
  }
  if (supportsDesktopNativeGPUTransforms()) {
    return invokeService<ImageTransformResultLike>(unsupportedMessage, "RotateImage", path, degrees);
  }
  return runPersistedVirtualTransform(path, (virtualPath) => rotateVirtualImage(virtualPath, degrees));
}

export function FlipImage(path: string, horizontal: boolean): Promise<ImageTransformResultLike> {
  if (isVirtualPath(path)) {
    return flipVirtualImage(path, horizontal).then((result) => ({ path: result.path, acceleration: result.acceleration || "cpu-canvas" }));
  }
  if (supportsDesktopNativeGPUTransforms()) {
    return invokeService<ImageTransformResultLike>(unsupportedMessage, "FlipImage", path, horizontal);
  }
  return runPersistedVirtualTransform(path, (virtualPath) => flipVirtualImage(virtualPath, horizontal));
}

export function CropImage(path: string, x: number, y: number, width: number, height: number): Promise<ImageTransformResultLike> {
  if (isVirtualPath(path)) {
    return cropVirtualImage(path, x, y, width, height).then((result) => ({ path: result.path, acceleration: result.acceleration || "cpu-canvas" }));
  }
  if (supportsDesktopNativeGPUTransforms()) {
    return invokeService<ImageTransformResultLike>(unsupportedMessage, "CropImage", path, x, y, width, height);
  }
  return runPersistedVirtualTransform(path, (virtualPath) => cropVirtualImage(virtualPath, x, y, width, height));
}

export function ReadImageAsBase64(path: string): Promise<string> {
  if (isVirtualPath(path)) {
    return Promise.resolve(readVirtualImageAsBase64(path));
  }
  if (canInvokeAndroidMethod("ReadImageAsBase64")) {
    return invokeAndroid<string>(unsupportedMessage, "ReadImageAsBase64", path);
  }
  return invokeService<string>(unsupportedMessage, "ReadImageAsBase64", path);
}

export function ExportHistoryToFile(jsonContent: string): Promise<string> {
  if (hasServiceMethod("ExportHistoryToFile")) {
    return invokeService<string>(unsupportedMessage, "ExportHistoryToFile", jsonContent);
  }
  return Promise.resolve(saveByDownload(new Blob([jsonContent], { type: "application/json" }), `image-studio-history-${Date.now()}.json`));
}

export function ImportHistoryFromFile(): Promise<string> {
  if (hasServiceMethod("ImportHistoryFromFile")) {
    return invokeService<string>(unsupportedMessage, "ImportHistoryFromFile");
  }
  if (canInvokeAndroidMethod("ImportHistoryFromFile")) {
    return invokeAndroid<string>(unsupportedMessage, "ImportHistoryFromFile");
  }
  return importHistoryFallback();
}

export function ExportUpstreamConfigToFile(jsonContent: string): Promise<string> {
  if (hasServiceMethod("ExportUpstreamConfigToFile")) {
    return invokeService<string>(unsupportedMessage, "ExportUpstreamConfigToFile", jsonContent);
  }
  return Promise.resolve(saveByDownload(new Blob([jsonContent], { type: "application/json" }), `image-studio-upstream-config-${Date.now()}.json`));
}

export function ImportUpstreamConfigFromFile(): Promise<string> {
  if (hasServiceMethod("ImportUpstreamConfigFromFile")) {
    return invokeService<string>(unsupportedMessage, "ImportUpstreamConfigFromFile");
  }
  return importHistoryFallback();
}

export function LoadCompatibilityState(): Promise<CompatibilityStateLike | null> {
  if (hasServiceMethod("LoadCompatibilityState")) {
    return invokeService<CompatibilityStateLike>(unsupportedMessage, "LoadCompatibilityState")
      .catch(() => null);
  }
  return Promise.resolve(null);
}

export function SaveCompatibilityState(state: CompatibilityStateLike): Promise<void> {
  if (hasServiceMethod("SaveCompatibilityState")) {
    return invokeService<void>(unsupportedMessage, "SaveCompatibilityState", state);
  }
  return Promise.resolve();
}

export function CheckForAppUpdate(): Promise<AppUpdateInfoLike | null> {
  if (hasServiceMethod("CheckForAppUpdate")) {
    return invokeService<AppUpdateInfoLike>(unsupportedMessage, "CheckForAppUpdate")
      .catch(() => null);
  }
  return Promise.resolve(null);
}

export function WriteAppUpdateProbe(result: AppUpdateProbeResultLike): Promise<void> {
  if (hasServiceMethod("WriteAppUpdateProbe")) {
    return invokeService<void>(unsupportedMessage, "WriteAppUpdateProbe", result);
  }
  return Promise.resolve();
}

export function RegisterTrustedOutputDir(root: string): Promise<void> {
  if (hasServiceMethod("RegisterTrustedOutputDir")) {
    return invokeService<void>(unsupportedMessage, "RegisterTrustedOutputDir", root);
  }
  return Promise.resolve();
}

export function SetKeepLogsEnabled(enabled: boolean): Promise<void> {
  if (hasServiceMethod("SetKeepLogsEnabled")) {
    return invokeService<void>(unsupportedMessage, "SetKeepLogsEnabled", enabled);
  }
  return Promise.resolve();
}

export function SetCleanupPreviewCacheOnExitEnabled(enabled: boolean): Promise<void> {
  if (hasServiceMethod("SetCleanupPreviewCacheOnExitEnabled")) {
    return invokeService<void>(unsupportedMessage, "SetCleanupPreviewCacheOnExitEnabled", enabled);
  }
  return Promise.resolve();
}

export function SetOutputDir(path: string): Promise<void> {
  if (hasServiceMethod("SetOutputDir")) {
    return invokeService<void>(unsupportedMessage, "SetOutputDir", path);
  }
  if (canInvokeAndroidMethod("SetOutputDir")) {
    return invokeAndroid<void>(unsupportedMessage, "SetOutputDir", path);
  }
  return Promise.resolve();
}

export function ChooseOutputDir(): Promise<string> {
  if (hasServiceMethod("ChooseOutputDir")) {
    return invokeService<string>(unsupportedMessage, "ChooseOutputDir");
  }
  if (canInvokeAndroidMethod("ChooseOutputDir")) {
    return invokeAndroid<string>(unsupportedMessage, "ChooseOutputDir");
  }
  return GetOutputDir();
}

export function ChooseDirectory(title: string): Promise<string> {
  if (hasServiceMethod("ChooseDirectory")) {
    return invokeService<string>(unsupportedMessage, "ChooseDirectory", title);
  }
  return Promise.reject(new Error(unsupportedMessage("ChooseDirectory")));
}

export function BuildBatchOutputPath(sourcePath: string, outputDir: string, prefix: string): Promise<string> {
  if (hasServiceMethod("BuildBatchOutputPath")) {
    return invokeService<string>(unsupportedMessage, "BuildBatchOutputPath", sourcePath, outputDir, prefix);
  }
  return Promise.reject(new Error(unsupportedMessage("BuildBatchOutputPath")));
}

export function OpenOutputDir(): Promise<void> {
  if (hasServiceMethod("OpenOutputDir")) {
    return invokeService<void>(unsupportedMessage, "OpenOutputDir");
  }
  if (canInvokeAndroidMethod("OpenOutputDir")) {
    return invokeAndroid<void>(unsupportedMessage, "OpenOutputDir");
  }
  return Promise.reject(new Error(unsupportedMessage("OpenOutputDir")));
}

export function OpenExternalURL(url: string): Promise<void> {
  if (canInvokeAndroidMethod("OpenExternalURL")) {
    return invokeAndroid<void>(unsupportedMessage, "OpenExternalURL", url).catch(() => {
      const opened = typeof window !== "undefined" ? window.open(url, "_blank", "noopener,noreferrer") : null;
      if (!opened && typeof window !== "undefined") window.location.href = url;
    });
  }
  if (!hasServiceMethod("OpenExternalURL")) {
    if (typeof window !== "undefined") {
      const opened = window.open(url, "_blank", "noopener,noreferrer");
      if (!opened) window.location.href = url;
      return Promise.resolve();
    }
    return Promise.reject(new Error(unsupportedMessage("OpenExternalURL")));
  }
  return invokeService<void>(unsupportedMessage, "OpenExternalURL", url);
}

export function OpenFile(path: string): Promise<void> {
  if (isVirtualPath(path)) {
    return openVirtualPath(path);
  }
  if (canInvokeAndroidMethod("OpenFile")) {
    return invokeAndroid<void>(unsupportedMessage, "OpenFile", path);
  }
  return invokeService<void>(unsupportedMessage, "OpenFile", path);
}

export function ReadTextFile(path: string): Promise<string> {
  if (isVirtualPath(path)) {
    return Promise.resolve(readVirtualText(path));
  }
  if (canInvokeAndroidMethod("ReadTextFile")) {
    return invokeAndroid<string>(unsupportedMessage, "ReadTextFile", path);
  }
  return invokeService<string>(unsupportedMessage, "ReadTextFile", path);
}

export async function probeCurrentUpstream(
  baseURL: string,
  apiKey: string,
  proxyMode = "system",
  proxyURL = "",
  apiMode = "responses",
  responsesTransport = "sse",
  signal?: AbortSignal,
): Promise<ProbeUpstreamResultLike> {
  if (signal?.aborted) throw new DOMException("Aborted", "AbortError");
  const options: ProbeUpstreamOptionsLike = {
    baseURL,
    apiKey,
    proxyMode,
    proxyURL,
    apiMode,
    responsesTransport,
  };
  if (hasServiceMethod("ProbeUpstream")) {
    return invokeService<ProbeUpstreamResultLike>(unsupportedMessage, "ProbeUpstream", options);
  }
  if (canInvokeAndroidMethod("ProbeUpstream")) {
    return invokeAndroid<ProbeUpstreamResultLike>(unsupportedMessage, "ProbeUpstream", options);
  }
  throw new Error(unsupportedMessage("ProbeUpstream"));
}

export function registerEphemeralLog(text: string, suggestedName = "raw-response.txt"): string {
  return registerVirtualText(text, suggestedName);
}
