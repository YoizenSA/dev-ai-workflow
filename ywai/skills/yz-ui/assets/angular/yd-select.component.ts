import { Component, ChangeDetectionStrategy, signal, computed, input, output, HostListener, ElementRef, inject } from '@angular/core';
import { LucideAngularModule } from 'lucide-angular';

export interface SelectOption { value: string; label: string; }

/**
 * Themed select dropdown (replaces native <select>, which can't be styled
 * to match the dark-glass theme). Signals + native control flow.
 *
 * Usage: <yd-select [options]="opts" [value]="env()" (valueChange)="env.set($event)" />
 */
@Component({
  selector: 'yd-select',
  standalone: true,
  imports: [LucideAngularModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="yd-select" [class.open]="open()">
      <button type="button" class="yd-select-trigger" (click)="toggle($event)">
        <span class="yd-select-label">{{ selectedLabel() }}</span>
        <lucide-icon name="chevron-down" [size]="15" />
      </button>
      @if (open()) {
        <div class="yd-pop yd-select-menu yd-menu-docked">
          @for (o of options(); track o.value) {
            <button type="button" class="yd-select-opt" [class.sel]="o.value === value()" (click)="pick(o.value)">{{ o.label }}</button>
          }
        </div>
      }
    </div>
  `,
})
export class YdSelectComponent {
  private readonly host = inject(ElementRef);
  readonly options = input<SelectOption[]>([]);
  readonly value = input<string>('');
  readonly placeholder = input<string>('Seleccionar');
  readonly valueChange = output<string>();

  readonly open = signal(false);
  readonly selectedLabel = computed(() =>
    this.options().find((o) => o.value === this.value())?.label ?? this.placeholder(),
  );

  toggle(e: Event): void { e.stopPropagation(); this.open.update((v) => !v); }
  pick(v: string): void { this.valueChange.emit(v); this.open.set(false); }

  @HostListener('document:click', ['$event'])
  onDocClick(e: Event): void {
    if (!this.host.nativeElement.contains(e.target)) this.open.set(false);
  }
}
