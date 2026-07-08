import { Component, ChangeDetectionStrategy, signal, computed, input, output, HostListener, ElementRef, inject, viewChild } from '@angular/core';
import { LucideAngularModule, ChevronDown, Search } from 'lucide-angular';
import { PopoverService } from './popover.service';
import { YdAnchoredDirective } from './yd-anchored.directive';

export interface SelectOption { value: string; label: string; }

/** A partir de esta cantidad de opciones, el menú muestra un buscador. */
const SEARCH_THRESHOLD = 7;

/**
 * Themed select dropdown (replaces native <select>, which can't be styled
 * to match the dark-glass theme). Signals + native control flow.
 *
 * Con muchas opciones agrega un buscador (filtra por label).
 * Usage: <yd-select [options]="opts" [value]="env()" (valueChange)="env.set($event)" />
 */
@Component({
  selector: 'yd-select',
  standalone: true,
  imports: [LucideAngularModule, YdAnchoredDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="yd-select" [class.open]="open()">
      <button type="button" class="yd-select-trigger" [disabled]="disabled()" (click)="toggle($event)">
        @if (label()) { <span class="yd-ctl-prefix">{{ label() }}:</span> }
        @if (tags() && hasSelection()) {
          <span class="yd-select-label"><span class="tag">{{ selectedLabel() }}</span></span>
        } @else {
          <span class="yd-select-label" [class.is-ph]="!hasSelection()">{{ selectedLabel() }}</span>
        }
        <lucide-icon [img]="ChevronDownIcon" [size]="15" />
      </button>
      @if (open()) {
        <div class="yd-pop yd-select-menu yd-menu-docked" ydAnchored [ydConfineToModal]="true">
          @if (showSearch()) {
            <div class="yd-select-search" (click)="$event.stopPropagation()">
              <lucide-icon [img]="SearchIcon" [size]="15" />
              <input #searchBox class="yd-select-search-input" [value]="query()" placeholder="Buscar…"
                     (input)="query.set($any($event.target).value)"
                     (keydown.enter)="pickFirst($event)" (keydown.escape)="close()" />
            </div>
          }
          <div class="yd-select-opts">
            @for (o of filtered(); track o.value) {
              <button type="button" class="yd-select-opt" [class.sel]="o.value === value()" (click)="pick(o.value)">
                @if (tags()) { <span class="tag">{{ o.label }}</span> } @else { {{ o.label }} }
              </button>
            } @empty {
              <div class="yd-select-empty">Sin coincidencias</div>
            }
          </div>
        </div>
      }
    </div>
  `,
})
export class YdSelectComponent {
  private readonly host = inject(ElementRef);
  private readonly popovers = inject(PopoverService);
  protected readonly ChevronDownIcon = ChevronDown;
  protected readonly SearchIcon = Search;
  readonly options = input<SelectOption[]>([]);
  readonly value = input<string>('');
  /** Prefijo dentro del control (filtros): "Label: valor". */
  readonly label = input<string>('');
  readonly placeholder = input<string>('Seleccionar');
  readonly disabled = input<boolean>(false);
  /** Renderiza las opciones y el valor como chips .tag (ej. versiones). */
  readonly tags = input<boolean>(false);
  readonly valueChange = output<string>();

  readonly open = signal(false);
  readonly query = signal('');
  private readonly searchBox = viewChild<ElementRef<HTMLInputElement>>('searchBox');

  readonly hasSelection = computed(() => this.options().some((o) => o.value === this.value()));
  readonly selectedLabel = computed(() =>
    this.options().find((o) => o.value === this.value())?.label ?? this.placeholder(),
  );
  readonly showSearch = computed(() => this.options().length > SEARCH_THRESHOLD);
  readonly filtered = computed(() => {
    const q = this.query().trim().toLowerCase();
    if (!q) return this.options();
    return this.options().filter((o) => o.label.toLowerCase().includes(q));
  });

  private readonly closeFn = (): void => this.close();

  toggle(e: Event): void {
    e.stopPropagation();
    const next = !this.open();
    this.open.set(next);
    if (next) {
      this.query.set('');
      this.popovers.opened(this.closeFn);
      // Foco en el buscador apenas se monta el menú.
      setTimeout(() => this.searchBox()?.nativeElement.focus(), 0);
    } else {
      this.popovers.closed(this.closeFn);
    }
  }
  pick(v: string): void { this.valueChange.emit(v); this.close(); }
  /** Enter en el buscador elige la primera coincidencia. */
  pickFirst(e: Event): void {
    e.preventDefault();
    const first = this.filtered()[0];
    if (first) this.pick(first.value);
  }

  close(): void {
    if (!this.open()) return;
    this.open.set(false);
    this.popovers.closed(this.closeFn);
  }

  @HostListener('document:click', ['$event'])
  onDocClick(e: Event): void {
    if (this.open() && !this.host.nativeElement.contains(e.target)) this.close();
  }
}
