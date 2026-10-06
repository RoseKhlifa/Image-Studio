import { useEffect } from "react";
import type { HistoryItem } from "../../types/domain";

type Tool = "pan" | "mask" | "annotate";

type UseCanvasShortcutsArgs = {
  brushSize: number;
  brushMode: "paint" | "erase";
  canNavigateBatchResults?: boolean;
  cancel: () => void;
  compareB: HistoryItem | null;
  copyCurrentImage: () => void;
  currentImage: HistoryItem | null;
  errorMessage: string | null;
  isMac: boolean;
  isRunning: boolean;
  navigateBatchResult?: (delta: -1 | 1) => void;
  redo: () => void;
  removeAnnotation: (id: string) => void;
  resetView: () => void;
  selectedAnnotationId: string | null;
  setBrushSize: (value: number) => void;
  setBrushMode: (value: "paint" | "erase") => void;
  setCompareB: (item: HistoryItem | null) => void;
  setErrorMessage: (value: string | null) => void;
  toggleFullscreen: () => void | Promise<void>;
  setSelectedAnnotationId: (value: string | null) => void;
  setTool: (value: Tool) => void;
  tool: Tool;
  undo: () => void;
};

export function useCanvasShortcuts({
  brushSize,
  brushMode,
  canNavigateBatchResults = false,
  cancel,
  compareB,
  copyCurrentImage,
  currentImage,
  errorMessage,
  isMac,
  isRunning,
  navigateBatchResult,
  redo,
  removeAnnotation,
  resetView,
  selectedAnnotationId,
  setBrushSize,
  setBrushMode,
  setCompareB,
  setErrorMessage,
  toggleFullscreen,
  setSelectedAnnotationId,
  setTool,
  tool,
  undo,
}: UseCanvasShortcutsArgs) {
  useEffect(() => {
    const isTypingInField = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement | null;
      return !!t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable);
    };

    const onKeyDown = (e: KeyboardEvent) => {
      if (e.defaultPrevented) return;
      if (isTypingInField(e)) return;
      const meta = e.ctrlKey || e.metaKey;
      const k = e.key.toLowerCase();

      if (meta && k === "z" && !e.shiftKey) {
        e.preventDefault();
        undo();
        return;
      }
      if (meta && ((k === "z" && e.shiftKey) || k === "y")) {
        e.preventDefault();
        redo();
        return;
      }
      if (k === "escape") {
        if (isRunning) cancel();
        else if (compareB) setCompareB(null);
        else if (selectedAnnotationId) setSelectedAnnotationId(null);
        else if (errorMessage) setErrorMessage(null);
        return;
      }
      if ((k === "delete" || k === "backspace") && selectedAnnotationId) {
        e.preventDefault();
        removeAnnotation(selectedAnnotationId);
        return;
      }
      if ((!isMac && k === "f11") || (isMac && e.ctrlKey && e.metaKey && k === "f")) {
        e.preventDefault();
        void toggleFullscreen();
        return;
      }
      if (meta && k === "c" && currentImage) {
        e.preventDefault();
        copyCurrentImage();
        return;
      }
      if ((e.key === "ArrowLeft" || e.key === "ArrowRight") && canNavigateBatchResults && navigateBatchResult) {
        e.preventDefault();
        navigateBatchResult(e.key === "ArrowLeft" ? -1 : 1);
        return;
      }
      if (k === "f") {
        e.preventDefault();
        resetView();
        return;
      }
      if (currentImage) {
        if (k === "1") {
          e.preventDefault();
          setTool("pan");
          return;
        }
        if (k === "2") {
          e.preventDefault();
          setTool("mask");
          return;
        }
        if (k === "3") {
          e.preventDefault();
          setTool("annotate");
          return;
        }
      }
      if (tool === "mask" && k === "x") {
        e.preventDefault();
        setBrushMode(brushMode === "paint" ? "erase" : "paint");
        return;
      }
      if (tool === "mask" && (k === "[" || k === "]")) {
        e.preventDefault();
        const delta = k === "[" ? -5 : 5;
        setBrushSize(Math.max(5, Math.min(120, brushSize + delta)));
      }
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [
    brushSize,
    brushMode,
    canNavigateBatchResults,
    cancel,
    compareB,
    copyCurrentImage,
    currentImage,
    errorMessage,
    isMac,
    isRunning,
    navigateBatchResult,
    redo,
    removeAnnotation,
    resetView,
    selectedAnnotationId,
    setBrushSize,
    setBrushMode,
    setCompareB,
    setErrorMessage,
    toggleFullscreen,
    setSelectedAnnotationId,
    setTool,
    tool,
    undo,
  ]);
}
