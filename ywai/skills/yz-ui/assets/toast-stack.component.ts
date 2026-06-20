import { Component, ChangeDetectionStrategy, inject } from '@angular/core';
import { LucideAngularModule } from 'lucide-angular';
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
        <div class="toast" [class]="'toast ' + t.type" (click)="toast.dismiss(t.id)">
          <span class="toast-ico"><lucide-icon [name]="t.icon" [size]="17" /></span>
          <span class="toast-msg">{{ t.msg }}</span>
          <span class="toast-bar"></span>
        </div>
      }
    </div>
  `,
})
export class ToastStackComponent {
  readonly toast = inject(ToastService);
}
