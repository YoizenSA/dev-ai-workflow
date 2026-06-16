import { Injectable, signal } from '@angular/core';

/** View Transitions API (aún no está en todos los lib.dom de TS). */
type VTDocument = Document & { startViewTransition?: (cb: () => void) => { ready: Promise<void> } };

/**
 * Tema dark ⇄ light. El estado vive en `<html data-theme>` (aplicado antes del
 * primer paint en main.ts, sin flash) y se persiste en localStorage; por eso
 * sobrevive al login y a los reloads. Único dueño del tema → login y shell
 * comparten estado.
 *
 * `toggle()` hace un reveal circular con View Transitions desde el botón;
 * degrada a corte seco sin soporte o con prefers-reduced-motion.
 */
@Injectable({ providedIn: 'root' })
export class ThemeService {
  readonly theme = signal<'dark' | 'light'>(
    document.documentElement.getAttribute('data-theme') === 'light' ? 'light' : 'dark',
  );

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

    // Origen del círculo: cursor del click, o el centro del botón (teclado).
    const r = (event?.currentTarget as HTMLElement | null)?.getBoundingClientRect();
    const x = event?.clientX || (r ? r.left + r.width / 2 : innerWidth);
    const y = event?.clientY || (r ? r.top + r.height / 2 : innerHeight);
    const end = Math.hypot(Math.max(x, innerWidth - x), Math.max(y, innerHeight - y));

    void startVT(apply).ready.then(() => {
      document.documentElement.animate(
        { clipPath: [`circle(0px at ${x}px ${y}px)`, `circle(${end}px at ${x}px ${y}px)`] },
        { duration: 450, easing: 'ease-in-out', pseudoElement: '::view-transition-new(root)' },
      );
    }).catch(() => undefined);
  }
}
