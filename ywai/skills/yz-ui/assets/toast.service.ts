import { Injectable, signal } from '@angular/core';

export type ToastType = 'success' | 'error' | 'warning' | 'info';
export interface ToastOpts { icon?: string; duration?: number; }
export interface ToastItem { id: number; msg: string; type: ToastType; icon: string; duration: number; }

const TOAST_ICON: Record<ToastType, string> = {
  success: 'check-circle-2',
  error: 'circle-alert',
  warning: 'triangle-alert',
  info: 'info',
};

/* Duración por severidad (ms): base + tiempo de lectura (~45ms/char), acotada.
   Los éxitos son los más cortos; los errores quedan en pantalla bastante más
   (requieren atención y suelen pedir una acción). Banda 3–10s alineada con los
   estándares (Material 3, Sonner/react-toastify/Radix rondan 4–5s). */
const DURATION: Record<ToastType, { base: number; min: number; max: number }> = {
  success: { base: 2600, min: 3000, max: 6000 },
  info:    { base: 3000, min: 3500, max: 6500 },
  warning: { base: 3500, min: 4500, max: 8000 },
  error:   { base: 4500, min: 6000, max: 10000 },
};

/** Global toast queue (signals). Rendered once by ToastStackComponent. */
@Injectable({ providedIn: 'root' })
export class ToastService {
  private seq = 0;
  private readonly timers = new Map<number, ReturnType<typeof setTimeout>>();
  private readonly ends = new Map<number, number>(); // id → timestamp en que vence (para pausar/reanudar)
  readonly toasts = signal<ToastItem[]>([]);

  show(msg: string, type: ToastType = 'success', opts: ToastOpts = {}): void {
    const id = ++this.seq;
    const duration = opts.duration ?? this.durationFor(type, msg);
    const icon = opts.icon ?? TOAST_ICON[type];
    this.toasts.update((list) => [...list, { id, msg, type, icon, duration }]);
    this.schedule(id, duration);
  }

  /** Confirmación de copia al portapapeles: estilo éxito, ícono de portapapeles. */
  copied(msg: string): void {
    this.show(msg, 'success', { icon: 'clipboard-check' });
  }

  /** Pausa el contador mientras el cursor/foco está sobre el toast. */
  pause(id: number): void {
    const t = this.timers.get(id);
    if (t) { clearTimeout(t); this.timers.delete(id); }
  }

  /** Reanuda con el tiempo que quedaba al pausar. */
  resume(id: number): void {
    if (this.timers.has(id)) return;
    const end = this.ends.get(id);
    if (end == null) return;
    this.schedule(id, Math.max(600, end - Date.now()));
  }

  dismiss(id: number): void {
    const t = this.timers.get(id);
    if (t) { clearTimeout(t); this.timers.delete(id); }
    this.ends.delete(id);
    this.toasts.update((list) => list.filter((x) => x.id !== id));
  }

  private durationFor(type: ToastType, msg: string): number {
    const d = DURATION[type];
    return Math.min(d.max, Math.max(d.min, d.base + msg.length * 45));
  }

  private schedule(id: number, ms: number): void {
    this.ends.set(id, Date.now() + ms);
    this.timers.set(id, setTimeout(() => this.dismiss(id), ms));
  }
}
