import { Component, ChangeDetectionStrategy, signal, computed, input, output, HostListener, ElementRef, inject } from '@angular/core';
import { LucideAngularModule } from 'lucide-angular';
import { PopoverService } from './popover.service';
import { YdAnchoredDirective } from './yd-anchored.directive';

const MONTHS = ['enero', 'febrero', 'marzo', 'abril', 'mayo', 'junio', 'julio', 'agosto', 'septiembre', 'octubre', 'noviembre', 'diciembre'];
const MONTHS_SHORT = ['ene', 'feb', 'mar', 'abr', 'may', 'jun', 'jul', 'ago', 'sep', 'oct', 'nov', 'dic'];
const DOW = ['L', 'M', 'X', 'J', 'V', 'S', 'D'];
const iso = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
const ARROW: Record<string, number> = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -7, ArrowDown: 7 };

interface Cell { iso: string; day: number; muted: boolean; today: boolean; }
interface Preset { label: string; iso: string; }

/**
 * Themed date picker (replaces native <input type=date>, whose calendar
 * popup can't be styled). Emits ISO yyyy-mm-dd via valueChange.
 *
 * - Click the title to jump months → years (no clicking ‹ › across a year).
 * - Keyboard: arrows move the day, Enter selects, Esc closes.
 * - `presetKind="past"` adds relative shortcuts (filters only — not a single
 *   future date like scheduling). Default 'none' = no presets.
 *
 * Usage: <yd-date [value]="from()" (valueChange)="from.set($event)" presetKind="past" />
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
        <lucide-icon name="calendar-clock" [size]="15" />
      </button>
      @if (open()) {
        <div class="yd-pop yd-cal yd-menu-docked" ydAnchored>
          @if (presets().length) {
            <div class="yd-cal-presets">
              @for (p of presets(); track p.label) {
                <button type="button" class="yd-preset" [class.sel]="p.iso === value()" (click)="applyPreset(p, $event)">{{ p.label }}</button>
              }
            </div>
          }
          <div class="yd-cal-head">
            <button type="button" class="yd-cal-title" (click)="cycleMode($event)">
              @if (mode() === 'days') { {{ monthName() }} {{ viewYear() }} }
              @else if (mode() === 'months') { {{ viewYear() }} }
              @else { {{ yearRangeLabel() }} }
              <lucide-icon name="chevron-down" [size]="13" />
            </button>
            <div class="yd-cal-nav">
              <button type="button" aria-label="Anterior" (click)="nav(-1, $event)"><lucide-icon name="chevron-left" [size]="16" /></button>
              <button type="button" aria-label="Siguiente" (click)="nav(1, $event)"><lucide-icon name="chevron-right" [size]="16" /></button>
            </div>
          </div>

          @if (mode() === 'days') {
            <div class="yd-cal-grid">
              @for (d of dow; track d) { <span class="yd-cal-dow">{{ d }}</span> }
              @for (c of cells(); track $index) {
                <button type="button" class="yd-cal-day" [class.muted]="c.muted" [class.today]="c.today"
                        [class.sel]="!c.muted && c.iso === value()" [class.kbd]="!c.muted && c.iso === focused()"
                        [disabled]="c.muted" (click)="pick(c.iso, $event)">{{ c.day || '' }}</button>
              }
            </div>
          } @else if (mode() === 'months') {
            <div class="yd-cal-mg">
              @for (m of monthsShort; track $index) {
                <button type="button" class="yd-cal-cell" [class.sel]="$index === viewMonth()" (click)="pickMonth($index, $event)">{{ m }}</button>
              }
            </div>
          } @else {
            <div class="yd-cal-mg">
              @for (y of yearGrid(); track y) {
                <button type="button" class="yd-cal-cell" [class.sel]="y === viewYear()" (click)="pickYear(y, $event)">{{ y }}</button>
              }
            </div>
          }

          <div class="yd-cal-foot">
            <button type="button" class="cal-clear" (click)="clear($event)">Borrar</button>
            <button type="button" class="cal-today" (click)="today($event)">Hoy</button>
          </div>
        </div>
      }
    </div>
  `,
})
export class YdDateComponent {
  private readonly host = inject(ElementRef);
  private readonly popovers = inject(PopoverService);
  readonly value = input<string>('');
  /** Prefijo dentro del control (filtros): "Label: valor". */
  readonly label = input<string>('');
  /** 'past' agrega atajos relativos (filtros); 'none' (default) no muestra ninguno. */
  readonly presetKind = input<'none' | 'past'>('none');
  readonly valueChange = output<string>();
  readonly dow = DOW;
  readonly monthsShort = MONTHS_SHORT;

  readonly open = signal(false);
  readonly mode = signal<'days' | 'months' | 'years'>('days');
  private readonly view = signal(new Date());
  /** Día con foco de teclado (no necesariamente el seleccionado). */
  readonly focused = signal('');
  readonly viewYear = computed(() => this.view().getFullYear());
  readonly viewMonth = computed(() => this.view().getMonth());
  readonly monthName = computed(() => MONTHS[this.view().getMonth()]);

  /** Bloque de 12 años alrededor del año en vista (4×3). */
  readonly yearGrid = computed<number[]>(() => {
    const base = this.viewYear() - (this.viewYear() % 12);
    return Array.from({ length: 12 }, (_, i) => base + i);
  });
  readonly yearRangeLabel = computed(() => { const ys = this.yearGrid(); return `${ys[0]} – ${ys[ys.length - 1]}`; });

  readonly display = computed(() => {
    const v = this.value();
    if (!v) return 'dd/mm/aaaa';
    const [y, m, d] = v.split('-');
    return `${d}/${m}/${y}`;
  });

  readonly presets = computed<Preset[]>(() => {
    if (this.presetKind() !== 'past') return [];
    const minus = (n: number) => { const d = new Date(); d.setDate(d.getDate() - n); return iso(d); };
    const t = new Date();
    return [
      { label: 'Hoy', iso: iso(t) },
      { label: 'Ayer', iso: minus(1) },
      { label: 'Hace 7 días', iso: minus(7) },
      { label: 'Hace 30 días', iso: minus(30) },
      { label: 'Inicio de mes', iso: iso(new Date(t.getFullYear(), t.getMonth(), 1)) },
    ];
  });

  readonly cells = computed<Cell[]>(() => {
    const y = this.view().getFullYear(), m = this.view().getMonth();
    const start = (new Date(y, m, 1).getDay() + 6) % 7;
    const days = new Date(y, m + 1, 0).getDate();
    const todayIso = iso(new Date());
    const out: Cell[] = [];
    for (let i = 0; i < start; i++) out.push({ iso: '', day: 0, muted: true, today: false });
    for (let d = 1; d <= days; d++) {
      const di = iso(new Date(y, m, d));
      out.push({ iso: di, day: d, muted: false, today: di === todayIso });
    }
    return out;
  });

  private readonly closeFn = (): void => this.open.set(false);
  private close(): void { this.open.set(false); this.popovers.closed(this.closeFn); }

  toggle(e: Event): void {
    e.stopPropagation();
    const next = !this.open();
    this.open.set(next);
    if (next) {
      this.mode.set('days');
      const base = this.value() || iso(new Date());
      this.focused.set(base);
      this.view.set(new Date(Number(base.slice(0, 4)), Number(base.slice(5, 7)) - 1, 1));
      this.popovers.opened(this.closeFn);
    } else this.popovers.closed(this.closeFn);
  }

  /** ‹ › — el paso depende del modo (mes / año / bloque de años). */
  nav(delta: number, e: Event): void {
    e.stopPropagation();
    const v = this.view();
    if (this.mode() === 'days') this.view.set(new Date(v.getFullYear(), v.getMonth() + delta, 1));
    else if (this.mode() === 'months') this.view.set(new Date(v.getFullYear() + delta, v.getMonth(), 1));
    else this.view.set(new Date(v.getFullYear() + delta * 12, v.getMonth(), 1));
  }

  cycleMode(e: Event): void {
    e.stopPropagation();
    this.mode.set(this.mode() === 'days' ? 'months' : this.mode() === 'months' ? 'years' : 'days');
  }
  pickMonth(m: number, e: Event): void { e.stopPropagation(); const v = this.view(); this.view.set(new Date(v.getFullYear(), m, 1)); this.mode.set('days'); }
  pickYear(y: number, e: Event): void { e.stopPropagation(); const v = this.view(); this.view.set(new Date(y, v.getMonth(), 1)); this.mode.set('months'); }

  pick(di: string, e: Event): void { e.stopPropagation(); this.valueChange.emit(di); this.close(); }
  applyPreset(p: Preset, e: Event): void { e.stopPropagation(); this.valueChange.emit(p.iso); this.close(); }
  clear(e: Event): void { e.stopPropagation(); this.valueChange.emit(''); this.close(); }
  today(e: Event): void { e.stopPropagation(); const t = new Date(); this.view.set(t); this.mode.set('days'); this.valueChange.emit(iso(t)); this.close(); }

  @HostListener('document:keydown', ['$event'])
  onKey(e: KeyboardEvent): void {
    if (!this.open()) return;
    if (e.key === 'Escape') { this.close(); return; }
    if (this.mode() !== 'days') return;
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
