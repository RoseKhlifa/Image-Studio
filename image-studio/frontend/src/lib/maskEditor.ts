export type MaskDimensions = {
  w: number;
  h: number;
};

export type MaskStrokeInput = {
  points: number[];
  size: number;
  erase?: boolean;
};

export type RenderMaskOptions = {
  dimensions: MaskDimensions;
  strokes: MaskStrokeInput[];
  baseMaskDataURL?: string | null;
  invert?: boolean;
  allowEmpty?: boolean;
};

export function normalizeMaskPixels(pixels: Uint8ClampedArray): boolean {
  let hasTransparency = false;
  for (let offset = 3; offset < pixels.length; offset += 4) {
    if (pixels[offset] < 255) {
      hasTransparency = true;
      break;
    }
  }

  let hasEditablePixels = false;
  for (let offset = 0; offset < pixels.length; offset += 4) {
    if (!hasTransparency) {
      const luma = Math.round(
        pixels[offset] * 0.299
        + pixels[offset + 1] * 0.587
        + pixels[offset + 2] * 0.114,
      );
      pixels[offset + 3] = 255 - luma;
    }
    pixels[offset] = 0;
    pixels[offset + 1] = 0;
    pixels[offset + 2] = 0;
    if (pixels[offset + 3] < 255) hasEditablePixels = true;
  }
  return hasEditablePixels;
}

export function invertMaskPixels(pixels: Uint8ClampedArray): boolean {
  let hasEditablePixels = false;
  for (let offset = 0; offset < pixels.length; offset += 4) {
    pixels[offset] = 0;
    pixels[offset + 1] = 0;
    pixels[offset + 2] = 0;
    pixels[offset + 3] = 255 - pixels[offset + 3];
    if (pixels[offset + 3] < 255) hasEditablePixels = true;
  }
  return hasEditablePixels;
}

export async function normalizeImportedMaskDataURL(dataURL: string): Promise<string | null> {
  const image = await loadImage(dataURL);
  return renderMaskPNGDataURL({
    dimensions: { w: image.naturalWidth, h: image.naturalHeight },
    strokes: [],
    baseMaskDataURL: dataURL,
  });
}

export async function createFullEditableMaskDataURL(dimensions: MaskDimensions): Promise<string> {
  const dataURL = await renderMaskPNGDataURL({ dimensions, strokes: [], invert: true });
  if (!dataURL) throw new Error("无法创建全选蒙版");
  return dataURL;
}

export async function renderMaskPNGDataURL({
  dimensions,
  strokes,
  baseMaskDataURL,
  invert = false,
  allowEmpty = false,
}: RenderMaskOptions): Promise<string | null> {
  const canvas = makeCanvas(dimensions);
  const context = requireContext(canvas);

  if (baseMaskDataURL) {
    const baseMask = await loadImage(baseMaskDataURL);
    context.clearRect(0, 0, canvas.width, canvas.height);
    context.drawImage(baseMask, 0, 0, canvas.width, canvas.height);
    const basePixels = context.getImageData(0, 0, canvas.width, canvas.height);
    normalizeMaskPixels(basePixels.data);
    context.putImageData(basePixels, 0, 0);
  } else {
    context.fillStyle = "#000";
    context.fillRect(0, 0, canvas.width, canvas.height);
  }

  context.lineCap = "round";
  context.lineJoin = "round";
  for (const stroke of strokes) drawStroke(context, stroke);
  context.globalCompositeOperation = "source-over";

  const finalPixels = context.getImageData(0, 0, canvas.width, canvas.height);
  const hasEditablePixels = invert
    ? invertMaskPixels(finalPixels.data)
    : hasTransparentPixel(finalPixels.data);
  if (!hasEditablePixels && !allowEmpty) return null;
  if (invert) context.putImageData(finalPixels, 0, 0);
  return canvas.toDataURL("image/png");
}

export async function loadImageDimensionsFromSource(source: string): Promise<MaskDimensions | null> {
  if (!source.trim()) return null;
  try {
    const image = await loadImage(source);
    if (image.naturalWidth <= 0 || image.naturalHeight <= 0) return null;
    return { w: image.naturalWidth, h: image.naturalHeight };
  } catch {
    return null;
  }
}

function drawStroke(context: CanvasRenderingContext2D, stroke: MaskStrokeInput) {
  const points = finitePointPairs(stroke.points);
  if (points.length === 0) return;
  const size = Number.isFinite(stroke.size) ? Math.max(1, stroke.size) : 1;
  context.globalCompositeOperation = stroke.erase ? "source-over" : "destination-out";
  context.fillStyle = "#000";
  context.strokeStyle = "#000";
  context.lineWidth = size;

  if (points.length === 1) {
    context.beginPath();
    context.arc(points[0].x, points[0].y, size / 2, 0, Math.PI * 2);
    context.fill();
    return;
  }

  context.beginPath();
  context.moveTo(points[0].x, points[0].y);
  for (let index = 1; index < points.length; index += 1) {
    context.lineTo(points[index].x, points[index].y);
  }
  context.stroke();
}

function finitePointPairs(points: number[]): Array<{ x: number; y: number }> {
  const result: Array<{ x: number; y: number }> = [];
  for (let index = 0; index + 1 < points.length; index += 2) {
    const x = points[index];
    const y = points[index + 1];
    if (Number.isFinite(x) && Number.isFinite(y)) result.push({ x, y });
  }
  return result;
}

function hasTransparentPixel(pixels: Uint8ClampedArray): boolean {
  for (let offset = 3; offset < pixels.length; offset += 4) {
    if (pixels[offset] < 255) return true;
  }
  return false;
}

function makeCanvas(dimensions: MaskDimensions): HTMLCanvasElement {
  const width = Math.round(dimensions.w);
  const height = Math.round(dimensions.h);
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
  if (!context) throw new Error("无法创建蒙版编辑画布");
  return context;
}

function loadImage(source: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error("无法解码蒙版图片"));
    image.src = source;
  });
}
