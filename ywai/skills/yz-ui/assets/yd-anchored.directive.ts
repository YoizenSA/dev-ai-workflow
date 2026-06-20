import { Directive, ElementRef, DestroyRef, afterNextRender, inject, input } from '@angular/core';

/**
 * Posiciona un popover «docked» (yd-select / yd-date) respecto a su contenedor.
 *
 * El menú sigue siendo `position: absolute` anclado a su host (.yd-select /
 * .yd-date), pero al abrir —y en resize/scroll— ajustamos dos cosas para que
 * nunca quede pegado ni se salga del borde:
 *   • Flip: si debajo del disparador no entra el menú pero arriba sí, lo abre
 *     hacia arriba (clase `yd-menu-up`).
 *   • Clamp: deja siempre un margen contra el borde; si aun así no entra,
 *     limita el alto con `--yd-pop-maxh` y el menú scrollea por dentro.
 *
 * El borde de referencia es el `.modal` que contiene al host (si existe),
 * intersecado con el viewport; si el host no está en un modal, es el viewport.
 * Así el menú **nunca sobresale del modal**: con muchas opciones clampa su alto
 * al espacio disponible dentro del modal y scrollea por dentro. Para que un
 * popover alto (calendario) abra hacia abajo y no quede «mirando para arriba»,
 * el modal que lo contiene se ancla arriba por CSS (`.overlay:has(yd-date)`),
 * dejándole aire debajo.
 *
 * Cada popover decide si consume `--yd-pop-maxh`: el select sí (lista
 * scrolleable); el calendario la ignora (alto fijo) y sólo usa el flip.
 */
@Directive({ selector: '[ydAnchored]', standalone: true })
export class YdAnchoredDirective {
  private readonly el = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly destroyRef = inject(DestroyRef);

  /** Margen mínimo contra el borde del viewport (px). */
  readonly margin = input(14, { alias: 'ydMargin' });
  /** Alto mínimo al que puede encoger antes de dejar de clampar (px). */
  readonly minHeight = input(140, { alias: 'ydMinHeight' });
  /**
   * Confinar el popover al `.modal` que lo contiene (no sobresalir de él).
   * Sólo para popovers que pueden encoger y scrollear (la lista del select);
   * el calendario es de alto fijo y mayor que un modal corto, así que debe
   * seguir escapando al viewport (default false).
   */
  readonly confineToModal = input(false, { alias: 'ydConfineToModal' });

  /** Separación disparador↔menú; coincide con el `calc(100% + 6px)` del CSS. */
  private readonly gap = 6;
  private frame = 0;

  constructor() {
    afterNextRender(() => {
      this.reposition();
      window.addEventListener('resize', this.schedule, { passive: true });
      window.addEventListener('scroll', this.onScroll, { passive: true, capture: true });
      this.destroyRef.onDestroy(() => {
        cancelAnimationFrame(this.frame);
        window.removeEventListener('resize', this.schedule);
        window.removeEventListener('scroll', this.onScroll, true);
      });
    });
  }

  /** Recalcula como mucho una vez por frame ante scroll/resize. */
  private readonly schedule = (): void => {
    if (this.frame) return;
    this.frame = requestAnimationFrame(() => { this.frame = 0; this.reposition(); });
  };

  /**
   * El scroll externo (overlay/página) mueve el disparador y obliga a recalcular;
   * el scroll interno de la lista no, e ignorarlo evita un reposition que al
   * quitar el clamp por un frame reseteaba el scrollTop al tope (no se llegaba al final).
   */
  private readonly onScroll = (e: Event): void => {
    const t = e.target;
    if (t instanceof Node && this.el.nativeElement.contains(t)) return;
    this.schedule();
  };

  private reposition(): void {
    const pop = this.el.nativeElement;
    const host = pop.parentElement;
    if (!host) return;

    // Medimos el alto natural sin el clamp de una pasada anterior.
    pop.style.removeProperty('--yd-pop-maxh');
    const rect = host.getBoundingClientRect();
    const natural = pop.offsetHeight;

    // Con confineToModal, el popover no debe sobresalir del modal que lo
    // contiene: el borde de referencia es el `.modal` ancestro (intersecado con
    // el viewport). Sin modal —o sin confinar— cae al viewport.
    const modalRect = this.confineToModal() ? host.closest('.modal')?.getBoundingClientRect() : undefined;
    const topBound = modalRect ? Math.max(0, modalRect.top) : 0;
    const bottomBound = modalRect ? Math.min(window.innerHeight, modalRect.bottom) : window.innerHeight;

    const below = bottomBound - rect.bottom - this.gap - this.margin();
    const above = rect.top - topBound - this.gap - this.margin();

    const up = natural > below && above > below;
    const avail = Math.max(up ? above : below, this.minHeight());

    pop.classList.toggle('yd-menu-up', up);
    if (natural > avail) pop.style.setProperty('--yd-pop-maxh', `${Math.floor(avail)}px`);
  }
}
