import { Component, ChangeDetectionStrategy, signal, computed, input, output, HostListener, ElementRef, inject, effect } from '@angular/core';
import { LucideAngularModule, CalendarClock, ChevronLeft, ChevronRight } from 'lucide-angular';
import { PopoverService } from './popover.service';
import { YdAnchoredDirective } from './yd-anchored.directive';

const MONTHS = ['enero', 'febrero', 'marzo', 'abril', 'mayo', 'junio', 'julio', 'agosto', 'septiembre', 'octubre', 'noviembre', 'diciembre'];
const DOW = ['L', 'M', 'X', 'J', 'V', 'S', 'D'];
const iso = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
const ARROW: Record<string, number> = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -7, ArrowDown: 7 };
/** Años hacia atrás/adelante que ofrece la lista del selector de año. */
const YEAR_SPAN = 12;

interface Cell { iso: string; day: number; muted: boolean; today: boolean; disabled: boolean; }

/**
 * Date picker themed estilo Google Calendar / shadcn (dropdown caption):
 * flechas ‹ › mueven el mes de a uno y Mes/Año son DROPDOWNS THEMED propios
 * (mismo lenguaje glass que el resto de la app, sin chrome nativo). El de año
 * es una lista scrolleable centrada en el año en vista. Emite ISO yyyy-mm-dd.
 *
 * Los dropdowns de mes/año NO pasan por PopoverService (eso cerraría al
 * calendario que los contiene): viven en su DOM con el signal local `picker`,
 * así el click-outside del propio calendario los mantiene/contiene. Todos los
 * clicks internos hacen stopPropagation; Esc cierra primero el dropdown abierto
 * y recién después el calendario.
 *
 * Usage: <yd-date [value]="from()" (valueChange)="from.set($event)" label="Desde" />
 */
@Component({
  selector: 'yd-date',
  standalone: true,
  imports: [LucideAngularModule, YdAnchoredDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="yd-date" [class.open]="open()">
      <button type="button" class="yd-date-trigger" (click)="toggle($event)">
        @if (label()) { <span class="yd-ctl-prefix">{{ label() }}:</span> }
        <span class="yd-date-label" [class.muted]="!value()">{{ display() }}</span>
        <lucide-icon [img]="CalendarClockIcon" [size]="15" />
      </button>
      @if (open()) {
        <div class="yd-pop yd-cal yd-menu-docked" ydAnchored [ydFixed]="true" (click)="picker.set(null)">
          <div class="yd-cal-head">
            <button type="button" class="yd-cal-arrow" aria-label="Mes anterior" (click)="nav(-1, $event)"><lucide-icon [img]="ChevronLeftIcon" [size]="16" /></button>

            <div class="yd-cal-jump">
              <!-- Selector de MES (dropdown themed propio) -->
              <div class="yd-cal-dd" [class.open]="picker() === 'month'">
                <button type="button" class="yd-cal-dd-trigger" aria-label="Elegir mes" (click)="togglePicker('month', $event)">
                  <span>{{ monthsFull[viewMonth()] }}</span>
                </button>
                @if (picker() === 'month') {
                  <div class="yd-cal-dd-menu yd-cal-dd-month" (click)="$event.stopPropagation()">
                    @for (m of monthsFull; track $index) {
                      <button type="button" class="yd-cal-dd-opt" [class.sel]="$index === viewMonth()"
                              (click)="setMonth($index, $event)">{{ m }}</button>
                    }
                  </div>
                }
              </div>

              <!-- Selector de AÑO (lista scrolleable themed) -->
              <div class="yd-cal-dd" [class.open]="picker() === 'year'">
                <button type="button" class="yd-cal-dd-trigger" aria-label="Elegir año" (click)="togglePicker('year', $event)">
                  <span>{{ viewYear() }}</span>
                </button>
                @if (picker() === 'year') {
                  <div class="yd-cal-dd-menu yd-cal-dd-year" (click)="$event.stopPropagation()">
                    @for (y of years(); track y) {
                      <button type="button" class="yd-cal-dd-opt" [class.sel]="y === viewYear()"
                              (click)="setYear(y, $event)">{{ y }}</button>
                    }
                  </div>
                }
              </div>
            </div>

            <button type="button" class="yd-cal-arrow" aria-label="Mes siguiente" (click)="nav(1, $event)"><lucide-icon [img]="ChevronRightIcon" [size]="16" /></button>
          </div>

          <div class="yd-cal-grid">
            @for (d of dow; track d) { <span class="yd-cal-dow">{{ d }}</span> }
            @for (c of cells(); track $index) {
              <button type="button" class="yd-cal-day" [class.muted]="c.muted" [class.today]="c.today"
                      [class.sel]="!c.muted && c.iso === value()" [class.kbd]="!c.muted && c.iso === focused()"
                      [disabled]="c.disabled" (click)="pick(c.iso, $event)">{{ c.day || '' }}</button>
            }
          </div>

          <div class="yd-cal-foot">
            <button type="button" class="yd-cal-today" (click)="today($event)">Hoy</button>
          </div>
        </div>
      }
    </div>
  `,
})
export class YdDateComponent {
  private readonly host = inject(ElementRef);
  private readonly popovers = inject(PopoverService);
  protected readonly CalendarClockIcon = CalendarClock;
  protected readonly ChevronLeftIcon = ChevronLeft;
  protected readonly ChevronRightIcon = ChevronRight;
  readonly value = input<string>('');
  /** Prefijo dentro del control (filtros): "Label: valor". */
  readonly label = input<string>('');
  /** Fecha mínima ISO (yyyy-mm-dd): los días anteriores quedan deshabilitados. */
  readonly min = input<string>('');
  readonly valueChange = output<string>();
  readonly dow = DOW;
  readonly monthsFull = MONTHS;

  readonly open = signal(false);
  /** Sub-dropdown abierto dentro del calendario (mes/año). Estado local: NO usa
   *  PopoverService para no cerrar al calendario que lo contiene. */
  readonly picker = signal<'month' | 'year' | null>(null);
  private readonly view = signal(new Date());
  /** Día con foco de teclado (no necesariamente el seleccionado). */
  readonly focused = signal('');
  readonly viewYear = computed(() => this.view().getFullYear());
  readonly viewMonth = computed(() => this.view().getMonth());

  /** Lista de años: año en vista ±YEAR_SPAN, descendente (recientes arriba). */
  readonly years = computed<number[]>(() => {
    const vy = this.viewYear();
    return Array.from({ length: YEAR_SPAN * 2 + 1 }, (_, i) => vy + YEAR_SPAN - i);
  });

  readonly display = computed(() => {
    const v = this.value();
    if (!v) return 'Seleccionar fecha';
    const [y, m, d] = v.split('-');
    return `${d}/${m}/${y}`;
  });

  readonly cells = computed<Cell[]>(() => {
    const y = this.view().getFullYear(), m = this.view().getMonth();
    const start = (new Date(y, m, 1).getDay() + 6) % 7;
    const days = new Date(y, m + 1, 0).getDate();
    const todayIso = iso(new Date());
    const min = this.min();
    const out: Cell[] = [];
    for (let i = 0; i < start; i++) out.push({ iso: '', day: 0, muted: true, today: false, disabled: true });
    for (let d = 1; d <= days; d++) {
      const di = iso(new Date(y, m, d));
      out.push({ iso: di, day: d, muted: false, today: di === todayIso, disabled: !!min && di < min });
    }
    return out;
  });

  constructor() {
    // Al abrir el selector de año, centra el año en vista en la lista scrolleable.
    effect(() => {
      if (this.picker() !== 'year') return;
      queueMicrotask(() => this.host.nativeElement
        .querySelector('.yd-cal-dd-year .yd-cal-dd-opt.sel')
        ?.scrollIntoView({ block: 'center' }));
    });
  }

  private readonly closeFn = (): void => { this.open.set(false); this.picker.set(null); };
  private close(): void { this.open.set(false); this.picker.set(null); this.popovers.closed(this.closeFn); }

  toggle(e: Event): void {
    e.stopPropagation();
    const next = !this.open();
    this.open.set(next);
    this.picker.set(null);
    if (next) {
      const base = this.value() || iso(new Date());
      this.focused.set(base);
      this.view.set(new Date(Number(base.slice(0, 4)), Number(base.slice(5, 7)) - 1, 1));
      this.popovers.opened(this.closeFn);
    } else this.popovers.closed(this.closeFn);
  }

  /** Abre/cierra el sub-dropdown de mes o año (uno a la vez). */
  togglePicker(kind: 'month' | 'year', e: Event): void {
    e.stopPropagation();
    this.picker.set(this.picker() === kind ? null : kind);
  }

  setMonth(m: number, e: Event): void {
    e.stopPropagation();
    const v = this.view();
    this.view.set(new Date(v.getFullYear(), m, 1));
    this.picker.set(null);
  }
  setYear(y: number, e: Event): void {
    e.stopPropagation();
    const v = this.view();
    this.view.set(new Date(y, v.getMonth(), 1));
    this.picker.set(null);
  }

  /** ‹ › — mueven el mes de a uno (y cierran cualquier dropdown abierto). */
  nav(delta: number, e: Event): void {
    e.stopPropagation();
    this.picker.set(null);
    const v = this.view();
    this.view.set(new Date(v.getFullYear(), v.getMonth() + delta, 1));
  }

  pick(di: string, e: Event): void { e.stopPropagation(); this.valueChange.emit(di); this.close(); }
  today(e: Event): void { e.stopPropagation(); const t = new Date(); this.view.set(t); this.valueChange.emit(iso(t)); this.close(); }

  @HostListener('document:keydown', ['$event'])
  onKey(e: KeyboardEvent): void {
    if (!this.open()) return;
    if (e.key === 'Escape') {
      if (this.picker()) this.picker.set(null); else this.close();
      return;
    }
    if (this.picker()) return; // con un dropdown abierto, las flechas no mueven el día
    if (e.key in ARROW) {
      e.preventDefault();
      const base = this.focused() || this.value() || iso(new Date());
      const [y, m, d] = base.split('-').map(Number);
      const nd = new Date(y, m - 1, d + ARROW[e.key]);
      this.focused.set(iso(nd));
      this.view.set(new Date(nd.getFullYear(), nd.getMonth(), 1));
    } else if (e.key === 'Enter' && this.focused()) {
      e.preventDefault();
      this.pick(this.focused(), e);
    }
  }

  @HostListener('document:click', ['$event'])
  onDocClick(e: Event): void {
    if (this.open() && !this.host.nativeElement.contains(e.target)) this.close();
  }
}
