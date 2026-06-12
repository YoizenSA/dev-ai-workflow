import { Component, ChangeDetectionStrategy, signal, computed, input, output, HostListener, ElementRef, inject } from '@angular/core';
import { LucideAngularModule } from 'lucide-angular';
import { PopoverService } from './popover.service';

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
      <button type="button" class="yd-select-trigger" [disabled]="disabled()" (click)="toggle($event)">
        @if (tags() && hasSelection()) {
          <span class="yd-select-label"><span class="tag">{{ selectedLabel() }}</span></span>
        } @else {
          <span class="yd-select-label" [class.is-ph]="!hasSelection()">{{ selectedLabel() }}</span>
        }
        <lucide-icon name="chevron-down" [size]="15" />
      </button>
      @if (open()) {
        <div class="yd-pop yd-select-menu yd-menu-docked">
          @for (o of options(); track o.value) {
            <button type="button" class="yd-select-opt" [class.sel]="o.value === value()" (click)="pick(o.value)">
              @if (tags()) { <span class="tag">{{ o.label }}</span> } @else { {{ o.label }} }
            </button>
          }
        </div>
      }
    </div>
  `,
})
export class YdSelectComponent {
  private readonly host = inject(ElementRef);
  private readonly popovers = inject(PopoverService);
  readonly options = input<SelectOption[]>([]);
  readonly value = input<string>('');
  readonly placeholder = input<string>('Seleccionar');
  readonly disabled = input<boolean>(false);
  /** Renderiza las opciones y el valor como chips .tag (ej. versiones). */
  readonly tags = input<boolean>(false);
  readonly valueChange = output<string>();

  readonly open = signal(false);
  readonly hasSelection = computed(() => this.options().some((o) => o.value === this.value()));
  readonly selectedLabel = computed(() =>
    this.options().find((o) => o.value === this.value())?.label ?? this.placeholder(),
  );

  private readonly closeFn = (): void => this.open.set(false);

  toggle(e: Event): void {
    e.stopPropagation();
    const next = !this.open();
    this.open.set(next);
    if (next) this.popovers.opened(this.closeFn);
    else this.popovers.closed(this.closeFn);
  }
  pick(v: string): void { this.valueChange.emit(v); this.close(); }

  private close(): void { this.open.set(false); this.popovers.closed(this.closeFn); }

  @HostListener('document:click', ['$event'])
  onDocClick(e: Event): void {
    if (this.open() && !this.host.nativeElement.contains(e.target)) this.close();
  }
}
