import { useCallback } from "react";

interface LayoutSashProps {
  axis: "x" | "y";
  /** Which sibling to resize. "prev" = the element before this sash (default),
   *  "next" = the element after it. */
  resizeTarget?: "prev" | "next";
  /** Minimum size in px for the resized element (width for axis x, height for y). */
  min?: number;
  /** Flip the delta direction. Needed when the resizable element is to the
   *  RIGHT of the sash (e.g. the right panel): dragging left should grow it,
   *  which is the inverse of the default (drag right = grow). */
  invertDelta?: boolean;
  /** Called with the final size (px) when the drag ends. Use it to persist the
   *  size to React state / localStorage. */
  onResizeEnd?: (finalSize: number) => void;
}

/**
 * LayoutSash — the draggable divider between two siblings in a flex group.
 *
 * Mirrors VS Code's sash: a thin bar that thickens on hover, with the right
 * cursor for its axis (col-resize for row splits, row-resize for column
 * splits). Dragging adjusts the inline `flex` of the target sibling
 * (`resizeTarget`, default = previous sibling), measured against the shared
 * parent.
 *
 * Used in three places:
 *  - Between split-pane children (axis x/y, target prev)
 *  - Between the chat body and the right panel (axis x, target next, inverted)
 *  - Between the sidebar and the chat content (axis x, target prev)
 *
 * Sizes are not persisted to the URL (like VS Code, a reload resets the split
 * pane sizes to equal flex:1). The sidebar/right-panel widths ARE persisted
 * by their owning components via localStorage.
 */
export default function LayoutSash({
  axis,
  resizeTarget = "prev",
  min = 200,
  invertDelta = false,
  onResizeEnd,
}: LayoutSashProps) {
  const onMouseDown = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      const handle = e.currentTarget as HTMLElement;
      const parent = handle.parentElement;
      const target =
        resizeTarget === "next"
          ? (handle.nextElementSibling as HTMLElement | null)
          : (handle.previousElementSibling as HTMLElement | null);
      if (!parent || !target) return;

      const horizontal = axis === "x";
      const start = horizontal ? e.clientX : e.clientY;
      const startSize = horizontal
        ? target.getBoundingClientRect().width
        : target.getBoundingClientRect().height;

      const onMove = (ev: MouseEvent) => {
        let delta = (horizontal ? ev.clientX : ev.clientY) - start;
        if (invertDelta) delta = -delta;
        const next = Math.max(min, startSize + delta);
        target.style.flex = `0 0 ${next}px`;
        document.body.style.cursor = horizontal ? "col-resize" : "row-resize";
        document.body.style.userSelect = "none";
      };
      const onUp = () => {
        document.removeEventListener("mousemove", onMove);
        document.removeEventListener("mouseup", onUp);
        document.body.style.cursor = "";
        document.body.style.userSelect = "";
        // Report the final size so the caller can persist it. Read the live
        // DOM size (not the computed `next` var, which is only updated on move).
        if (onResizeEnd) {
          const finalSize = horizontal
            ? target.getBoundingClientRect().width
            : target.getBoundingClientRect().height;
          onResizeEnd(finalSize);
        }
      };
      document.addEventListener("mousemove", onMove);
      document.addEventListener("mouseup", onUp);
    },
    [axis, resizeTarget, min, invertDelta, onResizeEnd],
  );

  return (
    <div
      className={`layout-sash layout-sash-${axis}`}
      onMouseDown={onMouseDown}
      role="separator"
      aria-orientation={axis === "x" ? "vertical" : "horizontal"}
    />
  );
}
