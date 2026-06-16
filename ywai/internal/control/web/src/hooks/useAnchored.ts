import { useEffect, useRef, useCallback } from "react";

/**
 * Positions a docked popover (yd-select / yd-date) relative to its container.
 *
 * The menu stays `position: absolute` anchored to its host, but on open — and
 * on resize/scroll — it:
 *   • Flips upward (adds `.yd-menu-up`) when there's no room below but there is above.
 *   • Clamps height via `--yd-pop-maxh` so the list scrolls inside.
 *
 * When the trigger lives inside a `.modal`, clamping is against the modal bounds
 * (not the viewport) so the menu doesn't collide with the modal's bottom edge.
 *
 * Port of Angular `yd-anchored.directive.ts`.
 */
export function useAnchored(
  menuRef: React.RefObject<HTMLElement | null>,
  open: boolean,
  opts?: { margin?: number; minHeight?: number },
) {
  const frameRef = useRef(0);
  const margin = opts?.margin ?? 14;
  const minHeight = opts?.minHeight ?? 140;
  const gap = 6;

  const bounds = useCallback(
    (host: HTMLElement): { top: number; bottom: number } => {
      const modal = host.closest(".modal");
      if (!modal) return { top: 0, bottom: window.innerHeight };
      const r = modal.getBoundingClientRect();
      return {
        top: Math.max(0, r.top),
        bottom: Math.min(window.innerHeight, r.bottom),
      };
    },
    [],
  );

  const reposition = useCallback(() => {
    const pop = menuRef.current;
    if (!pop) return;
    const host = pop.parentElement;
    if (!host) return;

    // Measure natural height without previous clamp.
    pop.style.removeProperty("--yd-pop-maxh");
    const rect = host.getBoundingClientRect();
    const natural = pop.offsetHeight;

    const { top, bottom } = bounds(host);
    const below = bottom - rect.bottom - gap - margin;
    const above = rect.top - top - gap - margin;

    const up = natural > below && above > below;
    const avail = Math.max(up ? above : below, minHeight);

    pop.classList.toggle("yd-menu-up", up);
    if (natural > avail)
      pop.style.setProperty("--yd-pop-maxh", `${Math.floor(avail)}px`);
  }, [menuRef, bounds, margin, minHeight]);

  const schedule = useCallback(() => {
    if (frameRef.current) return;
    frameRef.current = requestAnimationFrame(() => {
      frameRef.current = 0;
      reposition();
    });
  }, [reposition]);

  const onScroll = useCallback(
    (e: Event) => {
      const pop = menuRef.current;
      if (!pop) return;
      const t = e.target;
      // Ignore inner popover scroll — only react to outer scroll.
      if (t instanceof Node && pop.contains(t)) return;
      schedule();
    },
    [menuRef, schedule],
  );

  useEffect(() => {
    if (!open) return;
    reposition();
    window.addEventListener("resize", schedule, { passive: true });
    window.addEventListener("scroll", onScroll, { passive: true, capture: true });
    return () => {
      cancelAnimationFrame(frameRef.current);
      frameRef.current = 0;
      window.removeEventListener("resize", schedule);
      window.removeEventListener("scroll", onScroll, true);
    };
  }, [open, reposition, schedule, onScroll]);
}
