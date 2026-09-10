/**
 * v1 `TuiThemeCurrent` → v2 `ResolvedTheme` adapter.
 *
 * Upstream v1 read `ctx.theme.current` with the fields the vendored sidebar
 * component needs: `success`, `error`, `warning`, `text`, `textMuted`,
 * `accent`, `backgroundPanel`, `backgroundElement`. The v2 host exposes a
 * flatter `ResolvedTheme`. This module is the only place that maps one onto
 * the other, so the ported components never touch the host theme directly.
 *
 * Fallbacks keep the UI readable on a partial theme:
 * - `accent` falls back to `primary`.
 * - `backgroundElement` falls back to `backgroundPanel`, then `background`.
 * - Every other field falls back to a neutral default from `FALLBACK_COLORS`.
 *
 * The module is pure and host-independent: it imports nothing, so it is
 * typechecked and unit-tested without the missing `@opencode/theme` package.
 */

/** The v1-shaped theme surface the ported components consume. */
export type TuiTheme = {
  success: string;
  error: string;
  warning: string;
  text: string;
  textMuted: string;
  accent: string;
  backgroundPanel?: string;
  backgroundElement?: string;
};

/** Neutral defaults used only when the host theme omits a field. */
export const FALLBACK_COLORS = {
  success: "green",
  error: "red",
  warning: "yellow",
  text: "white",
  textMuted: "gray",
  accent: "cyan",
} as const;

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === "object" && !Array.isArray(value);
}

function asColor(value: unknown): string | undefined {
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

/**
 * Map a v2 `ResolvedTheme` (or any partial theme object) onto the v1-shaped
 * `TuiTheme`. Unknown input yields the neutral defaults; a missing fallback
 * chain degrades field by field instead of collapsing the whole theme.
 */
export function resolveTuiTheme(theme: unknown): TuiTheme {
  const source = isRecord(theme) ? theme : {};

  const primary = asColor(source.primary);
  const backgroundPanel = asColor(source.backgroundPanel);
  const background = asColor(source.background);

  return {
    success: asColor(source.success) ?? FALLBACK_COLORS.success,
    error: asColor(source.error) ?? FALLBACK_COLORS.error,
    warning: asColor(source.warning) ?? FALLBACK_COLORS.warning,
    text: asColor(source.text) ?? FALLBACK_COLORS.text,
    textMuted: asColor(source.textMuted) ?? FALLBACK_COLORS.textMuted,
    accent:
      asColor(source.accent) ?? primary ?? FALLBACK_COLORS.accent,
    backgroundPanel: backgroundPanel ?? background,
    backgroundElement: asColor(source.backgroundElement) ?? backgroundPanel ?? background,
  };
}
