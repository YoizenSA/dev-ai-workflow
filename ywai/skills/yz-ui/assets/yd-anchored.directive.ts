import { Directive, ElementRef, DestroyRef, afterNextRender, inject, input } from '@angular/core';

/**
 * Posiciona un popover «docked» (yd-select / yd-date) respecto a su contenedor.
 *
 * El menú sigue siendo `position: absolute` anclado a su host (.yd-select /
 * .yd-date), pero al abrir —y en resize/scroll— ajustamos dos cosas para que
 * nunca quede pegado ni se salga del borde:
 *   • Preferencia por abajo: el menú abre SIEMPRE hacia abajo mientras haya un
 *     alto usable bajo el disparador (medido contra el viewport, no el modal).
 *     Sólo voltea hacia arriba (`yd-menu-up`) si abajo no alcanza el mínimo y
 *     arriba ofrece más espacio.
 *   • Clamp: deja siempre un margen contra el borde; si aun así no entra,
 *     limita el alto con `--yd-pop-maxh` y el menú scrollea por dentro.
 *
 * El límite inferior es el viewport: el menú puede caer por debajo del borde del
 * modal (incluso sobre su footer) con tal de no voltear hacia arriba y tapar
 * contenido. Con `ydConfineToModal` sólo se acota el límite superior al modal
 * (para el caso raro de tener que voltear). El modal va centrado por CSS.
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

    // Preferencia fuerte por abrir HACIA ABAJO: el límite inferior es siempre el
    // viewport (no el modal), así el menú cae por debajo —incluso sobre el footer
    // del modal— en vez de voltear hacia arriba y tapar contenido. Con confineToModal
    // mantenemos el límite superior en el modal para el caso (raro) de tener que
    // voltear. Si abajo no entra, clampa el alto y scrollea por dentro.
    const modalRect = this.confineToModal() ? host.closest('.modal')?.getBoundingClientRect() : undefined;
    const topBound = modalRect ? Math.max(0, modalRect.top) : 0;
    const bottomBound = window.innerHeight;

    const below = bottomBound - rect.bottom - this.gap - this.margin();
    const above = rect.top - topBound - this.gap - this.margin();

    // Sólo voltea hacia arriba si abajo no alcanza un alto usable y arriba ofrece más.
    const up = below < this.minHeight() && above > below;
    const avail = Math.max(up ? above : below, this.minHeight());

    pop.classList.toggle('yd-menu-up', up);
    if (natural > avail) pop.style.setProperty('--yd-pop-maxh', `${Math.floor(avail)}px`);
  }
}
