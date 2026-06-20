/**
 * Accesibilidad de modal en una sola directiva — el estándar yz-ui para
 * cualquier diálogo. Una card de vidrio NO es un diálogo accesible por sí sola.
 *
 * Aplicar al contenedor del diálogo (el `.modal`). Se encarga de:
 *   - role="dialog" + aria-modal (estáticos en el host)
 *   - focus-trap: lleva el foco al modal al abrir y cicla Tab/Shift+Tab dentro
 *   - restaurar el foco al elemento que lo abrió, al cerrar
 *   - bloquear el scroll del body mientras está abierto
 *   - emitir (close) cuando el usuario presiona Escape
 *
 * El abrir/cerrar lo controla el padre con un signal + control flow nativo
 * (nunca dejes un modal oculto en el DOM):
 *
 *   @if (open()) {
 *     <div class="overlay" (click)="open.set(false)">
 *       <div class="modal" yzModal aria-labelledby="dlgTitle"
 *            (close)="open.set(false)" (click)="$event.stopPropagation()">
 *         <div class="modal-head">
 *           <h3 class="modal-title" id="dlgTitle">Título</h3>
 *           <button class="modal-close" aria-label="Cerrar" (click)="open.set(false)">…</button>
 *         </div>
 *         <div class="modal-body">…</div>
 *         <div class="modal-foot">…</div>
 *       </div>
 *     </div>
 *   }
 */
import {
  Directive,
  ElementRef,
  type OnDestroy,
  type OnInit,
  inject,
  output,
} from '@angular/core';

const FOCUSABLE =
  'a[href],button:not([disabled]),input:not([disabled]),' +
  'select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';

@Directive({
  selector: '[yzModal]',
  standalone: true,
  host: {
    role: 'dialog',
    'aria-modal': 'true',
    tabindex: '-1',
    '(keydown.escape)': 'close.emit()',
    '(keydown.tab)': 'onTab($event)',
  },
})
export class YzModalDirective implements OnInit, OnDestroy {
  /** Emite cuando el usuario pide cerrar (Escape). El padre cierra el signal. */
  readonly close = output<void>();

  private readonly host: HTMLElement = inject(ElementRef).nativeElement;
  private restoreFocusTo: HTMLElement | null = null;

  ngOnInit(): void {
    this.restoreFocusTo = document.activeElement as HTMLElement | null;
    document.body.style.overflow = 'hidden'; // scroll-lock del fondo
    // Foco inicial al primer control del modal (o al modal mismo).
    queueMicrotask(() => (this.focusables()[0] ?? this.host).focus());
  }

  ngOnDestroy(): void {
    document.body.style.overflow = ''; // restaura el scroll
    this.restoreFocusTo?.focus(); // restaura el foco al disparador
  }

  protected onTab(e: KeyboardEvent): void {
    const items = this.focusables();
    if (items.length === 0) return;
    const first = items[0];
    const last = items[items.length - 1];
    const active = document.activeElement;
    if (e.shiftKey && active === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && active === last) {
      e.preventDefault();
      first.focus();
    }
  }

  private focusables(): HTMLElement[] {
    return Array.from(this.host.querySelectorAll<HTMLElement>(FOCUSABLE));
  }
}
