import { useEffect, useRef, type MouseEvent } from "react";
import { shouldUseNativeFileDrag } from "../../lib/dragExport";
import { usePlatform } from "../../platform/context";
import { BeginNativeFileDrag } from "../../platform/runtime/host";
import { trackNativeFileDragGesture } from "../../platform/runtime/nativeFileDragGesture";
import { useStudioStore } from "../../state/studioStore";

export function useNativeFileDrag(savedPath?: string | null) {
  const { targetPlatform } = usePlatform();
  const native = shouldUseNativeFileDrag(targetPlatform, savedPath);
  const cleanupRef = useRef<() => void>(() => {});
  const pendingRef = useRef(false);
  useEffect(() => () => cleanupRef.current(), []);

  const onMouseDown = (event: MouseEvent<HTMLElement>) => {
    if (!native || !savedPath || event.button !== 0) return;
    event.preventDefault();
    event.stopPropagation();
    if (pendingRef.current) return;
    cleanupRef.current();
    cleanupRef.current = trackNativeFileDragGesture(window, event, () => {
      pendingRef.current = true;
      void BeginNativeFileDrag(savedPath).catch((error: unknown) => {
        useStudioStore.getState().pushToast(
          `拖出复制失败:${error instanceof Error ? error.message : String(error)}`,
          "error",
          6000,
        );
      }).finally(() => { pendingRef.current = false; });
    });
  };
  return { native, onMouseDown };
}
