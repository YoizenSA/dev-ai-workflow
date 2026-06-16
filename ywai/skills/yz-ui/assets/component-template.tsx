/**
 * React component template · Yoizen UI (Dark Glass theme)
 *
 * Copy this when creating a new React component. It consumes the design-system
 * CLASSES and TOKENS shipped in assets/theme/ — NOT ad-hoc Tailwind utilities.
 *
 * The One Rule still holds in React: never type a hex / rgba / raw rem in JSX.
 * Use a theme class (.btn, .card, .pill, .field…) or a CSS var (var(--space-4)).
 * If a value isn't covered by a class, add a token to palette.css and read it
 * via inline `style` with a `var(--*)` — that's the only sanctioned exception.
 *
 * Requires assets/theme/index.css imported once in main.tsx:
 *   import './styles/index.css';
 */

import React from 'react';

interface ComponentNameProps {
  /** Card title (text hierarchy: --text). */
  title: string;
  /** Optional muted subtitle (--text-muted). */
  description?: string;
  /** Primary action label. */
  actionLabel?: string;
  /** Disabled state — drives --disabled-opacity, never a hardcoded 0.5. */
  disabled?: boolean;
  onAction?: () => void;
  children?: React.ReactNode;
}

/**
 * ComponentName — a glass card with a header and a primary action.
 *
 * @example
 * <ComponentName title="Resumen" description="Estado general" actionLabel="Nuevo"
 *                onAction={() => create()} />
 */
export const ComponentName: React.FC<ComponentNameProps> = ({
  title,
  description,
  actionLabel,
  disabled = false,
  onAction,
  children,
}) => {
  return (
    // .card = glass surface + token border/radius/blur. .card-pad = --space-6.
    <section className="card card-pad stack">
      <header className="page-header">
        <div className="page-heading">
          <h3 className="section-title">{title}</h3>
          {description && <p className="page-subtitle">{description}</p>}
        </div>

        {actionLabel && (
          <button
            type="button"
            className="btn btn-primary"
            disabled={disabled}
            onClick={onAction}
            // Icon-only controls would need aria-label; this one has a text label.
          >
            {actionLabel}
          </button>
        )}
      </header>

      {children && <div className="stack">{children}</div>}
    </section>
  );
};

export default ComponentName;
