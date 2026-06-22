/** Utilidades de diff por líneas para la vista lado a lado (antes | después). */
export type DiffKind = 'ctx' | 'rm' | 'add' | 'empty';
export interface DiffCell { n: number | null; text: string; kind: DiffKind; }
export interface DiffRow { left: DiffCell; right: DiffCell; }
export interface UnifiedLine { type: 'added' | 'removed' | 'context'; line: string; }

const EMPTY: DiffCell = { n: null, text: '', kind: 'empty' };

/**
 * Convierte un diff unificado (added/removed/context) en filas lado a lado,
 * pareando cada bloque de líneas removidas con el de agregadas. Mantiene la
 * numeración de línea independiente por lado (vieja a la izquierda, nueva a la
 * derecha), como GitHub.
 */
export function unifiedToRows(lines: UnifiedLine[]): DiffRow[] {
  const rows: DiffRow[] = [];
  let oldN = 0, newN = 0;
  let rm: DiffCell[] = [], add: DiffCell[] = [];
  const flush = (): void => {
    const max = Math.max(rm.length, add.length);
    for (let k = 0; k < max; k++) rows.push({ left: rm[k] ?? EMPTY, right: add[k] ?? EMPTY });
    rm = []; add = [];
  };
  for (const l of lines) {
    if (l.type === 'removed') rm.push({ n: ++oldN, text: l.line, kind: 'rm' });
    else if (l.type === 'added') add.push({ n: ++newN, text: l.line, kind: 'add' });
    else {
      flush();
      rows.push({ left: { n: ++oldN, text: l.line, kind: 'ctx' }, right: { n: ++newN, text: l.line, kind: 'ctx' } });
    }
  }
  flush();
  return rows;
}

/**
 * Diff por líneas (LCS) entre dos textos → filas lado a lado. Para los cambios
 * de auditoría, donde sólo tenemos el valor "antes" y "después" crudos.
 */
export function diffTexts(oldText: string, newText: string): DiffRow[] {
  const a = (oldText ?? '').split('\n');
  const b = (newText ?? '').split('\n');
  const m = a.length, n = b.length;
  const dp: number[][] = Array.from({ length: m + 1 }, () => new Array<number>(n + 1).fill(0));
  for (let i = m - 1; i >= 0; i--) {
    for (let j = n - 1; j >= 0; j--) {
      dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
    }
  }
  const lines: UnifiedLine[] = [];
  let i = 0, j = 0;
  while (i < m && j < n) {
    if (a[i] === b[j]) { lines.push({ type: 'context', line: a[i] }); i++; j++; }
    else if (dp[i + 1][j] >= dp[i][j + 1]) { lines.push({ type: 'removed', line: a[i] }); i++; }
    else { lines.push({ type: 'added', line: b[j] }); j++; }
  }
  while (i < m) lines.push({ type: 'removed', line: a[i++] });
  while (j < n) lines.push({ type: 'added', line: b[j++] });
  return unifiedToRows(lines);
}
