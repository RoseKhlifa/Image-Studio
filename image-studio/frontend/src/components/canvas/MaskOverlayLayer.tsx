import { useEffect, useRef } from "react";
import type Konva from "konva";
import { Circle, Image as KonvaImage, Layer, Line, Rect } from "react-konva";
import type { Stroke } from "../../state/studioStore.types";

type Point = { x: number; y: number };

type MaskOverlayLayerProps = {
  image: HTMLImageElement;
  baseMaskImage: HTMLImageElement | null;
  strokes: Stroke[];
  draft: Stroke | null;
  visible: boolean;
  opacity: number;
  cursor: Point | null;
  showCursor: boolean;
  brushSize: number;
  brushMode: "paint" | "erase";
  viewScale: number;
};

const MASK_COLOR = "#4d7cff";

export function MaskOverlayLayer({
  image,
  baseMaskImage,
  strokes,
  draft,
  visible,
  opacity,
  cursor,
  showCursor,
  brushSize,
  brushMode,
  viewScale,
}: MaskOverlayLayerProps) {
  const maskLayerRef = useRef<Konva.Layer | null>(null);
  const clampedOpacity = Math.max(0.1, Math.min(0.9, opacity));
  const clip = {
    clipX: 0,
    clipY: 0,
    clipWidth: image.width,
    clipHeight: image.height,
  };
  useEffect(() => {
    const canvas = maskLayerRef.current?.getCanvas()._canvas;
    if (!canvas) return;
    canvas.style.opacity = String(clampedOpacity);
    return () => {
      canvas.style.opacity = "";
    };
  }, [clampedOpacity]);
  return (
    <>
      <Layer ref={maskLayerRef} {...clip} visible={visible} listening={false}>
        {baseMaskImage ? (
          <>
            <Rect width={image.width} height={image.height} fill={MASK_COLOR} listening={false} />
            <KonvaImage
              image={baseMaskImage}
              width={image.width}
              height={image.height}
              globalCompositeOperation="destination-out"
              listening={false}
            />
          </>
        ) : null}
        {strokes.map((stroke, index) => (
          <MaskStrokeShape key={index} stroke={stroke} />
        ))}
        {draft ? <MaskStrokeShape stroke={draft} /> : null}
      </Layer>
      <Layer {...clip} listening={false}>
        {showCursor && cursor ? (
          <>
            <Circle
              x={cursor.x}
              y={cursor.y}
              radius={brushSize / 2}
              stroke="rgba(0,0,0,0.72)"
              strokeWidth={3 / Math.max(viewScale, 0.05)}
              listening={false}
            />
            <Circle
              x={cursor.x}
              y={cursor.y}
              radius={brushSize / 2}
              stroke={brushMode === "erase" ? "#ff7b7b" : "#ffffff"}
              strokeWidth={1.25 / Math.max(viewScale, 0.05)}
              dash={brushMode === "erase" ? [5 / Math.max(viewScale, 0.05), 4 / Math.max(viewScale, 0.05)] : undefined}
              listening={false}
            />
          </>
        ) : null}
      </Layer>
    </>
  );
}

function MaskStrokeShape({ stroke }: { stroke: Stroke }) {
  const operation = stroke.erase ? "destination-out" : "source-over";
  if (stroke.points.length === 2) {
    return (
      <Circle
        x={stroke.points[0]}
        y={stroke.points[1]}
        radius={stroke.size / 2}
        fill={MASK_COLOR}
        globalCompositeOperation={operation}
        listening={false}
      />
    );
  }
  return (
    <Line
      points={stroke.points}
      stroke={MASK_COLOR}
      strokeWidth={stroke.size}
      lineCap="round"
      lineJoin="round"
      globalCompositeOperation={operation}
      listening={false}
    />
  );
}
