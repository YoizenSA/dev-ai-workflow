import { Injectable } from '@angular/core';

/**
 * Coordina los popovers de la app (yd-select, yd-date, menú de usuario…):
 * sólo uno puede estar abierto a la vez. Cada dueño registra su callback de
 * cierre al abrirse; abrir otro cierra al anterior.
 */
@Injectable({ providedIn: 'root' })
export class PopoverService {
  private current: (() => void) | null = null;

  /** Llamar al abrir un popover, pasando cómo cerrarlo. Cierra el que estuviera abierto. */
  opened(close: () => void): void {
    if (this.current && this.current !== close) this.current();
    this.current = close;
  }

  /** Llamar al cerrar (por selección, click afuera o Escape). */
  closed(close: () => void): void {
    if (this.current === close) this.current = null;
  }
}
