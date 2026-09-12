import { type KeyboardEventHandler, type ReactNode, useLayoutEffect, useRef } from "react";

// Enter the top layer without moving or remounting the textarea, preserving native
// undo history and form ownership when expanding an editor inside a resource form.
export function CodeExpansion({ children, className = "", expanded, label, onCancel, onCollapse, onKeyDown }: {
  children: ReactNode;
  className?: string;
  expanded: boolean;
  label: string;
  onCancel?: () => void;
  onCollapse: () => void;
  onKeyDown?: KeyboardEventHandler<HTMLDialogElement>;
}) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const frameRef = useRef<HTMLDivElement>(null);
  const returnFocusRef = useRef<HTMLElement | null>(null);

  useLayoutEffect(() => {
    const dialog = dialogRef.current;
    const frame = frameRef.current;
    if (!dialog || !frame || !expanded) return;

    returnFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    frame.style.minHeight = `${frame.getBoundingClientRect().height}px`;
    const bodyOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    dialog.close();
    dialog.showModal();
    dialog.querySelector<HTMLTextAreaElement>("textarea")?.focus({ preventScroll: true });

    const viewport = window.visualViewport;
    const sizeToViewport = () => {
      dialog.style.height = `${viewport?.height ?? window.innerHeight}px`;
      dialog.style.top = `${viewport?.offsetTop ?? 0}px`;
    };
    sizeToViewport();
    viewport?.addEventListener("resize", sizeToViewport);
    viewport?.addEventListener("scroll", sizeToViewport);
    window.addEventListener("resize", sizeToViewport);

    return () => {
      viewport?.removeEventListener("resize", sizeToViewport);
      viewport?.removeEventListener("scroll", sizeToViewport);
      window.removeEventListener("resize", sizeToViewport);
      dialog.close();
      dialog.show();
      dialog.style.removeProperty("height");
      dialog.style.removeProperty("top");
      frame.style.removeProperty("min-height");
      document.body.style.overflow = bodyOverflow;
      const returnFocus = returnFocusRef.current;
      // Restore after React/native dialog focus restoration has completed.
      queueMicrotask(() => {
        if (returnFocus?.isConnected && !dialog.matches(":modal")) returnFocus.focus({ preventScroll: true });
      });
    };
  }, [expanded]);

  return (
    <div className={`min-w-0 ${className}`} data-code-expansion={expanded ? "expanded" : "inline"} ref={frameRef}>
      <dialog
        open
        aria-label={label}
        aria-modal={expanded || undefined}
        className="code-expansion-surface"
        data-expanded={expanded || undefined}
        ref={dialogRef}
        role={expanded ? "dialog" : "group"}
        onKeyDown={onKeyDown}
        onCancel={(event) => {
          event.preventDefault();
          (onCancel ?? onCollapse)();
        }}
      >
        {children}
      </dialog>
    </div>
  );
}
