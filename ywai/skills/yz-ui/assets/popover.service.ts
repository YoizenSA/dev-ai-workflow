import { Injectable } from '@angular/core';

/**
 * Coordina los popovers «docked» (yd-select / yd-date): al abrir uno, cierra
 * los demás. Cada popover registra su `close` al abrirse y lo desregistra al
 * cerrarse.
 *
 * Los sub-dropdowns internos de un popover (p. ej. los selectores de mes/año del
 * calendario de yd-date) NO se registran acá a propósito: hacerlo cerraría al
 * popover que los contiene. Ver el comentario en `yd-date.component.ts`.
 *
 * Es el complemento del `@HostListener('document:click')` de cada componente:
 * ese cierra al clickear afuera; éste garantiza el «uno abierto a la vez» aunque
 * la apertura no venga de un click que llegue a `document` (p. ej. por teclado).
 */
@Injectable({ providedIn: 'root' })
export class PopoverService {
  private readonly openClosers = new Set<() => void>();

  opened(close: () => void): void {
    this.openClosers.forEach((fn) => {
      if (fn !== close) fn();
    });
    this.openClosers.add(close);
  }

  closed(close: () => void): void {
    this.openClosers.delete(close);
  }
}
