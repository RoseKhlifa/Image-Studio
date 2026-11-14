import { useEffect, useRef, type RefObject } from "react";
import type Konva from "konva";

// Canvas drawImage captures a single GIF frame. A native image underneath the
// transparent Konva layers lets the browser play every frame at its own timing.
export function AnimatedCanvasImage({ image, stageRef, view }: {
  image: HTMLImageElement;
  stageRef: RefObject<Konva.Stage | null>;
  view: { x: number; y: number; scale: number };
}) {
  const imageRef = useRef<HTMLImageElement | null>(null);

  useEffect(() => {
    const stage = stageRef.current;
    if (!stage) return;
    const syncPosition = () => {
      if (!imageRef.current) return;
      imageRef.current.style.transform = `translate(${stage.x()}px, ${stage.y()}px) scale(${stage.scaleX()})`;
    };
    // Follow native stage dragging without updating React state on every frame.
    stage.on("dragmove.gifPreview", syncPosition);
    return () => { stage.off("dragmove.gifPreview", syncPosition); };
  }, [image, stageRef]);

  return (
    <img
      ref={imageRef}
      src={image.src}
      alt="GIF 动画预览"
      draggable={false}
      data-testid="animated-canvas-image"
      style={{
        position: "absolute",
        top: 0,
        left: 0,
        width: image.naturalWidth || image.width,
        height: image.naturalHeight || image.height,
        maxWidth: "none",
        maxHeight: "none",
        transform: `translate(${view.x}px, ${view.y}px) scale(${view.scale})`,
        transformOrigin: "top left",
        pointerEvents: "none",
        userSelect: "none",
      }}
    />
  );
}
