type MouseEventTarget = Pick<Window, "addEventListener" | "removeEventListener">;

// Start OLE/NSDragging directly from a mouse gesture. Starting it from HTML
// dragstart can overlap the WebView's own native drag loop and mouse capture.
export function trackNativeFileDragGesture(
  target: MouseEventTarget,
  origin: { clientX: number; clientY: number },
  start: () => void,
): () => void {
  let timer: ReturnType<typeof setTimeout> | null = null;
  let active = true;
  const cleanup = () => {
    active = false;
    if (timer !== null) clearTimeout(timer);
    target.removeEventListener("mousemove", onMove);
    target.removeEventListener("mouseup", cleanup);
    target.removeEventListener("blur", cleanup);
    target.removeEventListener("keydown", onKeyDown);
  };
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === "Escape") cleanup();
  };
  const onMove = (event: MouseEvent) => {
    if (!active) return;
    if ((event.buttons & 1) === 0) { cleanup(); return; }
    if (Math.hypot(event.clientX - origin.clientX, event.clientY - origin.clientY) < 5) return;
    if (timer !== null) return;
    // Let the current WebView input event finish before acquiring native capture.
    timer = setTimeout(() => {
      if (!active) return;
      cleanup();
      start();
    }, 0);
  };
  target.addEventListener("mousemove", onMove);
  target.addEventListener("mouseup", cleanup);
  target.addEventListener("blur", cleanup);
  target.addEventListener("keydown", onKeyDown);
  return cleanup;
}
