import { Injectable, signal } from '@angular/core';

export type ToastType = 'success' | 'error' | 'info';
export interface ToastItem { id: number; msg: string; type: ToastType; icon: string; }

const TOAST_ICON: Record<ToastType, string> = {
  success: 'check-circle-2',
  error: 'circle-alert',
  info: 'activity',
};

/** Global toast queue (signals). Rendered once by ToastStackComponent. */
@Injectable({ providedIn: 'root' })
export class ToastService {
  private seq = 0;
  private readonly timers = new Map<number, ReturnType<typeof setTimeout>>();
  readonly toasts = signal<ToastItem[]>([]);

  show(msg: string, type: ToastType = 'success'): void {
    const id = ++this.seq;
    this.toasts.update((list) => [...list, { id, msg, type, icon: TOAST_ICON[type] }]);
    this.timers.set(id, setTimeout(() => this.dismiss(id), 3200));
  }

  dismiss(id: number): void {
    const t = this.timers.get(id);
    if (t) { clearTimeout(t); this.timers.delete(id); }
    this.toasts.update((list) => list.filter((x) => x.id !== id));
  }
}
