import { useEffect, useRef } from "react";

export interface AutocompleteItem {
  label: string;
  value: string;
  description?: string;
}

interface AutocompleteProps {
  items: AutocompleteItem[];
  selectedIndex: number;
  onSelect: (item: AutocompleteItem) => void;
  onClose: () => void;
  visible: boolean;
  anchorEl: HTMLTextAreaElement | null;
}

export default function Autocomplete({
  items,
  selectedIndex,
  onSelect,
  onClose,
  visible,
  anchorEl,
}: AutocompleteProps) {
  const listRef = useRef<HTMLUListElement>(null);

  useEffect(() => {
    if (visible && listRef.current && selectedIndex >= 0) {
      const el = listRef.current.children[selectedIndex] as HTMLElement | undefined;
      el?.scrollIntoView({ block: "nearest" });
    }
  }, [selectedIndex, visible]);

  if (!visible || items.length === 0) return null;

  const style: React.CSSProperties = {};
  if (anchorEl) {
    const rect = anchorEl.getBoundingClientRect();
    style.position = "fixed";
    style.left = rect.left + "px";
    style.top = rect.top - 8 + "px";
    style.width = Math.max(rect.width, 240) + "px";
    style.transform = "translateY(-100%)";
  }

  return (
    <>
      {/* backdrop to catch clicks outside */}
      <div
        className="autocomplete-backdrop"
        onClick={onClose}
        onKeyDown={() => {}}
        role="presentation"
      />
      <ul
        ref={listRef}
        className="autocomplete-dropdown"
        style={style}
        role="listbox"
      >
        {items.map((item, i) => (
          <li
            key={item.value}
            className={`autocomplete-item ${i === selectedIndex ? "selected" : ""}`}
            onClick={() => onSelect(item)}
            onMouseEnter={() => {}}
            role="option"
            aria-selected={i === selectedIndex}
          >
            <span className="autocomplete-label">{item.label}</span>
            {item.description && (
              <span className="autocomplete-desc">{item.description}</span>
            )}
          </li>
        ))}
      </ul>
    </>
  );
}
