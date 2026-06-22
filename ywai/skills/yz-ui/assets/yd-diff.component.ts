import { Component, ChangeDetectionStrategy, input } from '@angular/core';
import { DiffRow } from './diff';

/**
 * Diff lado a lado (antes | después) estilo GitHub: dos columnas con número de
 * línea, fondo rojo en lo removido (izquierda) y verde en lo agregado (derecha),
 * líneas sin contraparte como hueco atenuado. Sólo presenta `rows`; el cálculo
 * vive en `shared/diff.ts` (unifiedToRows / diffTexts).
 */
@Component({
  selector: 'yd-diff',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="diff-split">
      @for (r of rows(); track $index) {
        <span class="diff-ln" [class.rm]="r.left.kind === 'rm'" [class.empty]="r.left.kind === 'empty'">{{ r.left.n }}</span>
        <code class="diff-cell" [class.rm]="r.left.kind === 'rm'" [class.empty]="r.left.kind === 'empty'">{{ r.left.text }}</code>
        <span class="diff-ln" [class.add]="r.right.kind === 'add'" [class.empty]="r.right.kind === 'empty'">{{ r.right.n }}</span>
        <code class="diff-cell" [class.add]="r.right.kind === 'add'" [class.empty]="r.right.kind === 'empty'">{{ r.right.text }}</code>
      }
    </div>
  `,
})
export class YdDiffComponent {
  readonly rows = input.required<DiffRow[]>();
}
