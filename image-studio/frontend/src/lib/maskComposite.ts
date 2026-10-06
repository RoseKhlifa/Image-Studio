import { dataURLFromBase64 } from "./images.ts";

export type PreparedMaskComposite = {
  sourceDataURL: string;
  sourceUploadDataURL: string;
  maskB64: string;
};

export const MAX_MASK_INPUT_BYTES = 50 * 1024 * 1024;

export async function normalizeMaskForSource(
  sourceDataURL: string,
  maskB64: string,
): Promise<PreparedMaskComposite> {
  assertImageDataURLWithinLimit(sourceDataURL, "蒙版源图");
  const normalizedInputMaskB64 = assertBase64WithinLimit(maskB64, "蒙版图片");
  const [source, mask] = await Promise.all([
    loadImage(sourceDataURL),
    loadImage(dataURLFromBase64(normalizedInputMaskB64)),
  ]);
  const sourceCanvas = makeCanvas(source.naturalWidth, source.naturalHeight);
  const sourceContext = requireContext(sourceCanvas);
  sourceContext.drawImage(source, 0, 0, sourceCanvas.width, sourceCanvas.height);
  const sourceUploadDataURL = sourceCanvas.toDataURL("image/png");
  assertImageDataURLWithinLimit(sourceUploadDataURL, "蒙版源图 PNG");

  const canvas = makeCanvas(source.naturalWidth, source.naturalHeight);
  const context = requireContext(canvas);
  context.drawImage(mask, 0, 0, canvas.width, canvas.height);
  const pixels = context.getImageData(0, 0, canvas.width, canvas.height);
  let hasTransparency = false;
  for (let offset = 3; offset < pixels.data.length; offset += 4) {
    const alpha = pixels.data[offset];
    if (alpha < 255) hasTransparency = true;
  }
  if (!hasTransparency) {
    for (let offset = 0; offset < pixels.data.length; offset += 4) {
      const luma = Math.round(
        pixels.data[offset] * 0.299
        + pixels.data[offset + 1] * 0.587
        + pixels.data[offset + 2] * 0.114,
      );
      pixels.data[offset] = 0;
      pixels.data[offset + 1] = 0;
      pixels.data[offset + 2] = 0;
      pixels.data[offset + 3] = 255 - luma;
    }
    context.putImageData(pixels, 0, 0);
  }
  const normalizedMaskB64 = stripDataURL(canvas.toDataURL("image/png"));
  assertBase64WithinLimit(normalizedMaskB64, "蒙版 PNG");
  return { sourceDataURL, sourceUploadDataURL, maskB64: normalizedMaskB64 };
}

export async function compositeMaskedEdit(
  prepared: PreparedMaskComposite,
  generatedB64: string,
): Promise<string> {
  const [source, generated, mask] = await Promise.all([
    loadImage(prepared.sourceDataURL),
    loadImage(dataURLFromBase64(generatedB64)),
    loadImage(dataURLFromBase64(prepared.maskB64)),
  ]);
  const width = source.naturalWidth;
  const height = source.naturalHeight;
  const maskCanvas = makeCanvas(width, height);
  const maskContext = requireContext(maskCanvas);
  maskContext.drawImage(mask, 0, 0, width, height);
  const maskPixels = maskContext.getImageData(0, 0, width, height).data;

  const sourceCanvas = makeCanvas(width, height);
  const sourceContext = requireContext(sourceCanvas);
  sourceContext.drawImage(source, 0, 0, width, height);
  const sourcePixels = sourceContext.getImageData(0, 0, width, height).data;

  const generatedCanvas = makeCanvas(width, height);
  const generatedContext = requireContext(generatedCanvas);
  generatedContext.drawImage(generated, 0, 0, width, height);
  const generatedPixels = generatedContext.getImageData(0, 0, width, height).data;

  const result = generatedContext.createImageData(width, height);
  for (let offset = 0; offset < result.data.length; offset += 4) {
    const keep = maskPixels[offset + 3];
    const edit = 255 - keep;
    for (let channel = 0; channel < 4; channel++) {
      result.data[offset + channel] = Math.round(
        (sourcePixels[offset + channel] * keep + generatedPixels[offset + channel] * edit) / 255,
      );
    }
  }
  generatedContext.putImageData(result, 0, 0);
  return stripDataURL(generatedCanvas.toDataURL("image/png"));
}

function makeCanvas(width: number, height: number): HTMLCanvasElement {
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
    throw new Error("蒙版源图尺寸无效");
  }
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  return canvas;
}

function requireContext(canvas: HTMLCanvasElement): CanvasRenderingContext2D {
  const context = canvas.getContext("2d", { willReadFrequently: true });
  if (!context) throw new Error("无法创建蒙版合成画布");
  return context;
}

function loadImage(dataURL: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error("无法解码蒙版或源图片"));
    image.src = dataURL;
  });
}

function stripDataURL(value: string): string {
  const comma = value.indexOf(",");
  return comma >= 0 ? value.slice(comma + 1) : value;
}

export function assertBase64WithinLimit(value: string, label: string): string {
  const encoded = String(value || "").replace(/\s+/g, "");
  if (!encoded) throw new Error(`${label}为空`);
  if (!/^[A-Za-z0-9+/]*={0,2}$/.test(encoded) || encoded.length % 4 === 1) {
    throw new Error(`${label} base64 无效`);
  }
  const padding = encoded.endsWith("==") ? 2 : encoded.endsWith("=") ? 1 : 0;
  const decodedBytes = Math.floor((encoded.length * 3) / 4) - padding;
  if (decodedBytes <= 0) throw new Error(`${label}为空`);
  if (decodedBytes > MAX_MASK_INPUT_BYTES) {
    throw new Error(`${label}过大(${decodedBytes}B > ${MAX_MASK_INPUT_BYTES}B 上限)`);
  }
  return encoded;
}

export function assertImageDataURLWithinLimit(value: string, label: string): { mimeType: string; b64: string } {
  const match = /^data:(image\/(?:png|jpeg|webp));base64,(.+)$/is.exec(String(value || "").trim());
  if (!match) throw new Error(`${label}不是支持的 PNG/JPEG/WebP base64 data URL`);
  return { mimeType: match[1].toLowerCase(), b64: assertBase64WithinLimit(match[2], label) };
}
