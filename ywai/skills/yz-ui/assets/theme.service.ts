import { Injectable, signal } from '@angular/core';

/** View Transitions API (aún no está en todos los lib.dom de TS). */
interface ViewTransition { ready: Promise<void>; finished: Promise<void>; skipTransition: () => void; }
type VTDocument = Document & { startViewTransition?: (cb: () => void) => ViewTransition };

const DURATION = 560; // la onda expansiva con un poco más de presencia (no tan fugaz)
const EASING = 'cubic-bezier(0.16, 1, 0.3, 1)'; // = --ease-out (decelera: arranque ágil, llegada suave)

/**
 * Tema dark ⇄ light. El estado vive en `<html data-theme>` (aplicado antes del
 * primer paint en main.ts, sin flash) y se persiste en localStorage; por eso
 * sobrevive al login y a los reloads. Único dueño del tema → login y shell
 * comparten estado.
 *
 * `toggle()` hace un reveal circular con View Transitions desde el botón. El ícono
 * sol/luna gira + desvanece aparte (CSS, `view-transition-name`) — ese es el toque
 * de marca: un glow de color sobre el filo NO se puede (el clip-path recorta el
 * drop-shadow del snapshot y la capa de VT tapa cualquier overlay propio). Degrada
 * a corte seco sin soporte o con prefers-reduced-motion.
 */
@Injectable({ providedIn: 'root' })
export class ThemeService {
  readonly theme = signal<'dark' | 'light'>(
    document.documentElement.getAttribute('data-theme') === 'light' ? 'light' : 'dark',
  );
  private vt?: ViewTransition;

  toggle(event?: MouseEvent): void {
    const next = this.theme() === 'light' ? 'dark' : 'light';
    const apply = (): void => {
      this.theme.set(next);
      if (next === 'light') document.documentElement.setAttribute('data-theme', 'light');
      else document.documentElement.removeAttribute('data-theme');
    };
    localStorage.setItem('yd-theme', next);

    // Reveal circular desde el botón. Sin soporte o con reduced-motion, corte seco.
    const startVT = (document as VTDocument).startViewTransition?.bind(document);
    if (!startVT || matchMedia('(prefers-reduced-motion: reduce)').matches) { apply(); return; }

    // Doble-clic rápido: cerrar la transición en vuelo antes de iniciar otra (evita el snap a medias).
    this.vt?.skipTransition();

    // Origen del círculo: cursor del click. Por teclado (event.detail === 0) → centro del botón.
    const r = (event?.currentTarget as HTMLElement | null)?.getBoundingClientRect();
    const byKeyboard = !event || event.detail === 0;
    const x = byKeyboard ? (r ? r.left + r.width / 2 : innerWidth) : event.clientX;
    const y = byKeyboard ? (r ? r.top + r.height / 2 : innerHeight) : event.clientY;
    const end = Math.hypot(Math.max(x, innerWidth - x), Math.max(y, innerHeight - y));

    const vt = startVT(apply);
    this.vt = vt;
    vt.ready.then(() => {
      // Círculo que se expande desde el origen (decelera con --ease-out).
      document.documentElement.animate(
        { clipPath: [`circle(0px at ${x}px ${y}px)`, `circle(${end}px at ${x}px ${y}px)`] },
        { duration: DURATION, easing: EASING, pseudoElement: '::view-transition-new(root)' },
      );
    }).catch(() => undefined);
    vt.finished.catch(() => undefined).then(() => { if (this.vt === vt) this.vt = undefined; });
  }
}
