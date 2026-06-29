import { useState, useRef, useEffect, useCallback, useMemo } from "react";
import { ChevronDown } from "lucide-react";
import { useAnchored } from "../../hooks/useAnchored";

export interface SelectOption {
  value: string;
  label: string;
}

interface YdSelectProps {
  options: SelectOption[];
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
  className?: string;
  /** Accessible label for the trigger (when no visible <label htmlFor> exists). */
  ariaLabel?: string;
  /** Reference to a visible label id that names this control. */
  ariaLabelledby?: string;
}

/**
 * Themed select dropdown (replaces native <select>, which can't be styled
 * to match the dark-glass theme). Uses yz-ui CSS classes from components.css.
 *
 * Port of Angular `yd-select.component.ts`.
 */
export default function YdSelect({
  options,
  value,
  onChange,
  placeholder = "Seleccionar",
  disabled = false,
  className,
  ariaLabel,
  ariaLabelledby,
}: YdSelectProps) {
  const [open, setOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const hostRef = useRef<HTMLDivElement>(null);

  const selectedLabel = useMemo(
    () => options.find((o) => o.value === value)?.label ?? placeholder,
    [options, value, placeholder],
  );

  const hasSelection = useMemo(
    () => options.some((o) => o.value === value),
    [options, value],
  );

  const toggle = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      if (!disabled) setOpen((v) => !v);
    },
    [disabled],
  );

  const pick = useCallback(
    (v: string) => {
      onChange(v);
      setOpen(false);
    },
    [onChange],
  );

  // Close on document click outside host.
  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (hostRef.current && !hostRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open]);

  // Position the popover (flip + clamp).
  useAnchored(menuRef, open);

  return (
    <div
      ref={hostRef}
      className={`yd-select${open ? " open" : ""}${className ? ` ${className}` : ""}`}
    >
      <button
        type="button"
        className="yd-select-trigger"
        disabled={disabled}
        onClick={toggle}
        aria-label={ariaLabel}
        aria-labelledby={ariaLabelledby}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className={`yd-select-label${!hasSelection ? " is-ph" : ""}`}>
          {selectedLabel}
        </span>
        <ChevronDown size={16} strokeWidth={2.5} />
      </button>
      {open && (
        <div ref={menuRef} className="yd-pop yd-select-menu yd-menu-docked">
          {options.map((o) => (
            <button
              key={o.value}
              type="button"
              className={`yd-select-opt${o.value === value ? " sel" : ""}`}
              onClick={() => pick(o.value)}
            >
              {o.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
