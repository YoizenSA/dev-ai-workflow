/**
 * Template de Componente Angular standalone con estilo Yoizen.
 *
 * Copiar este archivo para crear nuevos componentes consistentes
 * con el design system de Yoizen (Angular 19+, signals, standalone).
 */

import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';

type Variant = 'default' | 'primary' | 'secondary' | 'accent' | 'danger';
type Size = 'sm' | 'md' | 'lg';

/**
 * ComponentName - Breve descripción del componente.
 *
 * @example
 * ```html
 * <yz-component-name
 *   title="Título Ejemplo"
 *   description="Descripción opcional"
 *   variant="primary"
 *   size="md"
 *   (action)="onAction()"
 * />
 * ```
 */
@Component({
  selector: 'yz-component-name',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="card" [class]="variantClass()">
      <h3 class="card-title">{{ title() }}</h3>
      @if (description()) {
        <p class="card-description">{{ description() }}</p>
      }
      <button
        type="button"
        class="btn btn-{{ variant() }}"
        [disabled]="disabled()"
        (click)="action.emit()"
      >
        <ng-content />
      </button>
    </div>
  `,
  styles: `
    /* Consumir siempre tokens de palette.css — nunca hex directos. */
    .card {
      background: var(--surface);
      border: 1px solid var(--panel-border);
      border-radius: var(--radius-md);
      padding: var(--space-4);
      box-shadow: var(--shadow-glass);
    }

    .card-title {
      font-weight: 600;
      color: var(--text);
    }

    .card-description {
      margin-top: var(--space-2);
      color: var(--text-muted);
      font-size: 0.9rem;
    }
  `,
})
export class ComponentNameComponent {
  /** Título o contenido principal */
  readonly title = input.required<string>();
  /** Descripción opcional */
  readonly description = input<string>();
  /** Variante visual */
  readonly variant = input<Variant>('default');
  /** Tamaño del componente */
  readonly size = input<Size>('md');
  /** Estado deshabilitado */
  readonly disabled = input(false);
  /** Evento de acción principal */
  readonly action = output<void>();

  protected readonly variantClass = computed(() => `card--${this.variant()} card--${this.size()}`);
}
