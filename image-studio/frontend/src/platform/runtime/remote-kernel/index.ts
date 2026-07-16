import {
  DEFAULT_AUTO_RETRY_COUNT,
  MAX_AUTO_RETRY_COUNT,
  MAX_ATTEMPTS,
  buildPromptOptimizePayload,
  extractInvalidSize,
  fileNameFromPath,
  isRetryableRaw,
  normalizeAPIMode,
  normalizeAutoRetryCount,
  normalizeBaseURL,
  repairSizeForOpenAI,
} from "../../../../../../shared/kernel/requestModel.js";
import {
  extractResponseErrorMessage,
  extractResponseText,
  isTransportishError,
  readRegisteredText,
  shouldUseAndroidNativeHTTP,
  sleepWithSignal,
  sourceToDataURL,
  validateRemoteBaseURL,
} from "./common.ts";
import { nativeHttpRequestText } from "./nativeHttp.ts";
import { requestImagesOnce } from "./images.ts";
import { requestResponsesOnce } from "./responses.ts";
import {
  RETRY_BACKOFF_MS,
  RemoteKernelError,
  type RemotePromptOptimizeInput,
  type RemoteJobCallbacks,
  type RemoteJobRequest,
  type RemoteJobResult,
} from "./types.ts";

export * from "./types.ts";

export async function runRemoteImageJob(
  request: RemoteJobRequest,
  callbacks: RemoteJobCallbacks,
): Promise<RemoteJobResult> {
  let lastError: RemoteKernelError | null = null;
  const autoRetryEnabled = request.payload.autoRetryEnabled !== false;
  const maxAttempts = (autoRetryEnabled ? normalizeAutoRetryCount(request.payload.autoRetryCount) : 0) + 1;
  const requestVariants: RemoteJobRequest[] = [request];
  let sizeRepairTried = false;
  if (request.payload.fallbackProfile?.baseURL?.trim() && request.payload.fallbackProfile?.apiKey?.trim()) {
    requestVariants.push({
      ...request,
      payload: {
        ...request.payload,
        baseURL: request.payload.fallbackProfile.baseURL,
        apiKey: request.payload.fallbackProfile.apiKey,
        textModelID: request.payload.fallbackProfile.textModelID || request.payload.textModelID,
        imageModelID: request.payload.fallbackProfile.imageModelID || request.payload.imageModelID,
        reasoningEffort: request.payload.fallbackProfile.reasoningEffort || request.payload.reasoningEffort,
        apiMode: request.payload.fallbackProfile.apiMode || request.payload.apiMode,
        responsesTransport: request.payload.fallbackProfile.responsesTransport || request.payload.responsesTransport,
        requestPolicy: request.payload.fallbackProfile.requestPolicy || request.payload.requestPolicy,
        imagesNewAPICompat: request.payload.fallbackProfile.imagesNewAPICompat === true,
        allowInsecureConnection: request.payload.fallbackProfile.allowInsecureConnection === true,
        autoRetryCount: request.payload.autoRetryCount,
      },
    });
  }
  for (let variantIndex = 0; variantIndex < requestVariants.length; variantIndex++) {
    const activeRequest = requestVariants[variantIndex];
    if (variantIndex > 0) {
      callbacks.onLog?.("主上游自动重试失败，切换到备用上游再试一次...");
    }
    for (let attempt = 1; attempt <= maxAttempts; attempt++) {
      try {
        validateRemoteBaseURL(
          activeRequest.payload.baseURL,
          activeRequest.payload.allowInsecureConnection === true,
        );
        if (activeRequest.payload.allowInsecureConnection && !shouldUseAndroidNativeHTTP()) {
          throw new RemoteKernelError("允许不安全连接需要桌面本地内核或 Android 原生内核；浏览器不能绕过 HTTPS 证书校验");
        }
        const apiMode = normalizeAPIMode(activeRequest.payload.apiMode);
        if (apiMode === "images") {
          return await requestImagesOnce(activeRequest, attempt, maxAttempts, callbacks);
        }
        return await requestResponsesOnce(activeRequest, attempt, maxAttempts, callbacks);
      } catch (error) {
        if (callbacks.signal.aborted) throw error;
        const typed = error instanceof RemoteKernelError
          ? error
          : new RemoteKernelError(String((error as any)?.message || error));
        lastError = typed;
        let retryableRaw = false;
        let workerAlreadyRetried = false;
        let invalidSizeRaw = null;
        if (typed.rawPath) {
          try {
            const rawText = readRegisteredText(typed.rawPath);
            retryableRaw = isRetryableRaw(rawText);
            invalidSizeRaw = extractInvalidSize(rawText);
            try {
              const parsed = JSON.parse(rawText);
              workerAlreadyRetried = parsed?.error?.type === "upstream_error"
                && Number.isFinite(Number(parsed?.error?.upstreamStatus));
            } catch {
              workerAlreadyRetried = false;
            }
          } catch {
            retryableRaw = false;
            workerAlreadyRetried = false;
            invalidSizeRaw = null;
          }
        }
        if (!sizeRepairTried && (invalidSizeRaw || extractInvalidSize(typed.message))) {
          const repairedPayload = repairSizeForOpenAI(activeRequest.payload);
          if (repairedPayload && repairedPayload.size !== activeRequest.payload.size) {
            sizeRepairTried = true;
            callbacks.onLog?.(`检测到上游拒绝当前尺寸 ${activeRequest.payload.size}，自动改为最近合法尺寸 ${repairedPayload.size} 后重试一次...`);
            const nextRequest = { ...activeRequest, payload: repairedPayload as typeof activeRequest.payload };
            requestVariants.splice(variantIndex, 1, nextRequest);
            activeRequest.payload = nextRequest.payload;
            attempt -= 1;
            continue;
          }
        }
        const retryable = retryableRaw || isTransportishError(typed);
        if (autoRetryEnabled && !workerAlreadyRetried && attempt < maxAttempts && retryable) {
          callbacks.onLog?.(typed.message);
          callbacks.onLog?.(`${Math.floor(RETRY_BACKOFF_MS / 1000)} 秒后自动重试...`);
          await sleepWithSignal(callbacks.signal, RETRY_BACKOFF_MS);
          continue;
        }
        lastError = typed;
        break;
      }
    }
  }
  if (lastError) {
    throw lastError;
  }
  throw new RemoteKernelError("多次请求后仍未成功");
}

export async function optimizePromptRemote(
  input: RemotePromptOptimizeInput,
  signal: AbortSignal,
): Promise<string> {
  validateRemoteBaseURL(input.baseURL, input.allowInsecureConnection === true);
  if (input.allowInsecureConnection && !shouldUseAndroidNativeHTTP()) {
    throw new RemoteKernelError("允许不安全连接需要桌面本地内核或 Android 原生内核；浏览器不能绕过 HTTPS 证书校验");
  }
  const mergedSources = input.sourceImages?.length
    ? input.sourceImages
    : [
        ...(input.imagePaths ?? []).map((path) => ({ path, name: fileNameFromPath(path) })),
        ...(input.imagePath ? [{ path: input.imagePath, name: fileNameFromPath(input.imagePath) }] : []),
      ];
  const sourceDataURLs: string[] = [];
  for (const source of mergedSources) {
    const dataURL = await sourceToDataURL(source);
    if (dataURL) sourceDataURLs.push(dataURL);
  }
  const url = `${normalizeBaseURL(input.baseURL)}/v1/responses`;
  const headers = {
    Authorization: `Bearer ${input.apiKey}`,
    "Content-Type": "application/json",
    Accept: "application/json",
  };
  const body = JSON.stringify(buildPromptOptimizePayload(input, sourceDataURLs));
  const proxyMode = input.proxyMode === "none" || input.proxyMode === "custom" ? input.proxyMode : "system";
  const response = shouldUseAndroidNativeHTTP()
    ? await nativeHttpRequestText(url, "POST", headers, body, signal, undefined, {
        proxyMode,
        proxyURL: input.proxyURL || "",
        allowInsecureConnection: input.allowInsecureConnection === true,
      })
    : {
        status: 0,
        body: "",
      };
  const raw = shouldUseAndroidNativeHTTP()
    ? response.body
    : await (async () => {
        if (proxyMode !== "system") {
          throw new RemoteKernelError("当前远程内核不能控制代理,请切回本地内核或使用 Android 原生运行");
        }
        const webResponse = await fetch(url, {
          method: "POST",
          headers,
          body,
          signal,
        });
        const text = await webResponse.text();
        response.status = webResponse.status;
        return text;
      })();
  if (response.status < 200 || response.status >= 300) {
    throw new RemoteKernelError(`上游返回 ${response.status}:${extractResponseErrorMessage(raw)}`);
  }
  const text = extractResponseText(raw);
  if (!text) {
    throw new RemoteKernelError("上游没有返回可用的优化结果");
  }
  return text;
}

export {
  DEFAULT_AUTO_RETRY_COUNT,
  MAX_ATTEMPTS,
  MAX_AUTO_RETRY_COUNT,
  RETRY_BACKOFF_MS,
};
