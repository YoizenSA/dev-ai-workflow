import { Component, ChangeDetectionStrategy, signal, computed, input, output, HostListener, ElementRef, inject } from '@angular/core';
import { LucideAngularModule } from 'lucide-angular';
import { PopoverService } from './popover.service';
import { YdAnchoredDirective } from './yd-anchored.directive';

const MONTHS = ['enero', 'febrero', 'marzo', 'abril', 'mayo', 'junio', 'julio', 'agosto', 'septiembre', 'octubre', 'noviembre', 'diciembre'];
const DOW = ['L', 'M', 'X', 'J', 'V', 'S', 'D'];
const iso = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;

interface Cell { iso: string; day: number; muted: boolean; today: boolean; }

/**
 * Themed date picker (replaces native <input type=date>, whose calendar
 * popup can't be styled). Emits ISO yyyy-mm-dd via valueChange.
 *
 * Usage: <yd-date [value]="from()" (valueChange)="from.set($event)" />
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
          <div class="yd-cal-head">
            <span class="yd-cal-title">{{ monthName() }} {{ viewYear() }}</span>
            <div class="yd-cal-nav">
              <button type="button" (click)="move(-1, $event)"><lucide-icon name="chevron-left" [size]="16" /></button>
              <button type="button" (click)="move(1, $event)"><lucide-icon name="chevron-right" [size]="16" /></button>
            </div>
          </div>
          <div class="yd-cal-grid">
            @for (d of dow; track d) { <span class="yd-cal-dow">{{ d }}</span> }
            @for (c of cells(); track $index) {
              <button type="button" class="yd-cal-day" [class.muted]="c.muted" [class.today]="c.today"
                      [class.sel]="!c.muted && c.iso === value()" [disabled]="c.muted" (click)="pick(c.iso, $event)">{{ c.day || '' }}</button>
            }
          </div>
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
  readonly valueChange = output<string>();
  readonly dow = DOW;

  readonly open = signal(false);
  private readonly view = signal(new Date());
  readonly viewYear = computed(() => this.view().getFullYear());
  readonly monthName = computed(() => MONTHS[this.view().getMonth()]);

  readonly display = computed(() => {
    const v = this.value();
    if (!v) return 'dd/mm/aaaa';
    const [y, m, d] = v.split('-');
    return `${d}/${m}/${y}`;
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
    if (next) this.popovers.opened(this.closeFn);
    else this.popovers.closed(this.closeFn);
  }
  move(delta: number, e: Event): void { e.stopPropagation(); const v = this.view(); this.view.set(new Date(v.getFullYear(), v.getMonth() + delta, 1)); }
  pick(di: string, e: Event): void { e.stopPropagation(); this.valueChange.emit(di); this.close(); }
  clear(e: Event): void { e.stopPropagation(); this.valueChange.emit(''); this.close(); }
  today(e: Event): void { e.stopPropagation(); const t = new Date(); this.view.set(t); this.valueChange.emit(iso(t)); this.close(); }

  @HostListener('document:click', ['$event'])
  onDocClick(e: Event): void {
    if (this.open() && !this.host.nativeElement.contains(e.target)) this.close();
  }
}
