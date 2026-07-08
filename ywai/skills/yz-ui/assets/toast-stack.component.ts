import { Component, ChangeDetectionStrategy, inject } from '@angular/core';
import { LucideAngularModule, X } from 'lucide-angular';
import { ToastService } from './toast.service';

/** Fixed bottom-right toast stack (uses global .toast classes). */
@Component({
  selector: 'yd-toast-stack',
  standalone: true,
  imports: [LucideAngularModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="toast-stack">
      @for (t of toast.toasts(); track t.id) {
        <div class="toast" [class]="'toast ' + t.type"
             [attr.role]="t.type === 'error' ? 'alert' : 'status'"
             [attr.aria-live]="t.type === 'error' ? 'assertive' : 'polite'"
             [style.--toast-dur]="t.duration + 'ms'"
             (mouseenter)="toast.pause(t.id)" (mouseleave)="toast.resume(t.id)"
             (focusin)="toast.pause(t.id)" (focusout)="toast.resume(t.id)">
          <span class="toast-ico"><lucide-icon [img]="t.icon" [size]="17" /></span>
          <span class="toast-msg">{{ t.msg }}</span>
          <button class="toast-close" type="button" aria-label="Cerrar notificación"
                  (click)="$event.stopPropagation(); toast.dismiss(t.id)"><lucide-icon [img]="XIcon" [size]="14" /></button>
          <span class="toast-bar"></span>
        </div>
      }
    </div>
  `,
})
export class ToastStackComponent {
  readonly toast = inject(ToastService);
  protected readonly XIcon = X;
}
