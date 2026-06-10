import { ReactNode, Ref, useEffect } from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";
import { usePlatform } from "../../platform/context";

// 居中 modal:点击背景 / Esc 关闭。
export function Modal({
  open, onClose, title, children, width = 480, backdropClassName = "", cardClassName = "", headerClassName = "", bodyClassName = "", bodyRef,
}: {
  open: boolean;
  onClose: () => void;
  title?: string;
  children: ReactNode;
  width?: number;
  backdropClassName?: string;
  cardClassName?: string;
  headerClassName?: string;
  bodyClassName?: string;
  bodyRef?: Ref<HTMLDivElement>;
}) {
  const { isAndroidPhone, usesFluentUI, usesAppleUI } = usePlatform();
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;
  const modal = (
    <div
      className={`app-modal-backdrop ${isAndroidPhone ? "app-modal-backdrop-phone" : "app-modal-backdrop-desktop"} ${backdropClassName}`}
      onClick={onClose}
    >
      <div
        style={{ width }}
        role="dialog"
        aria-modal="true"
        aria-label={title ?? "弹窗"}
        onClick={(e) => e.stopPropagation()}
        className={`app-modal-card ${usesAppleUI ? "liquid-glass-panel" : ""} ${isAndroidPhone ? "app-modal-card-phone" : "app-modal-card-desktop"} ${usesFluentUI ? "app-modal-card-windows" : ""} ${cardClassName}`}
      >
        {title && (
          <div className={`app-modal-header ${isAndroidPhone ? "app-modal-header-phone" : "app-modal-header-desktop"} ${headerClassName}`}>
            <h3 className="m-0 text-[15px] font-semibold tracking-[-0.01em] text-zinc-900 dark:text-zinc-100">{title}</h3>
            <button
              onClick={onClose}
              title="关闭 (Esc)"
              className={`-mr-1 p-1.5 text-zinc-500 hover:bg-black/[0.05] hover:text-zinc-900 dark:hover:bg-white/[0.06] dark:hover:text-zinc-100 ${usesFluentUI ? "rounded-[8px]" : "rounded-full"}`}
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        )}
        <div ref={bodyRef} className={`modal-scroll-body app-modal-body ${isAndroidPhone ? "app-modal-body-phone" : "app-modal-body-desktop"} ${bodyClassName}`}>{children}</div>
      </div>
    </div>
  );

  if (typeof document === "undefined") return modal;
  return createPortal(modal, document.body);
}
