import { useState, useEffect, useCallback, useRef } from "react";
import { getCommands, executeCommand, type CommandItem } from "../../api/chat";
import Autocomplete from "./Autocomplete";
import type { AutocompleteItem } from "./types";

interface CommandSuggestionsProps {
  visible: boolean;
  anchorEl: HTMLTextAreaElement | null;
  onClose: () => void;
  onCommand: (cmd: string) => void;
}

export default function CommandSuggestions({
  visible,
  anchorEl,
  onClose,
  onCommand,
}: CommandSuggestionsProps) {
  const [commands, setCommands] = useState<CommandItem[]>([]);
  const [filtered, setFiltered] = useState<AutocompleteItem[]>([]);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [query] = useState("");
  const mountedRef = useRef(false);

  useEffect(() => {
    getCommands().then(setCommands);
  }, []);

  useEffect(() => {
    if (!visible) return;
    if (!mountedRef.current) {
      mountedRef.current = true;
      return;
    }
    // Re-fetch commands when menu opens
    getCommands().then(setCommands);
  }, [visible]);

  useEffect(() => {
    if (!visible) return;
    const filtered: AutocompleteItem[] = commands
      .filter((c) => !query || c.label.includes(`/${query}`) || c.description.toLowerCase().includes(query.toLowerCase()))
      .map((c) => ({ label: c.label, description: c.description, value: c.value }));
    setFiltered(filtered);
    setSelectedIndex(0);
  }, [commands, query, visible]);

  const handleSelect = useCallback(
    (item: AutocompleteItem) => {
      onCommand(item.value);
      // Execute the command via API
      const sessionId = window.location.pathname.match(/\/chat\/([^/]+)/)?.[1];
      if (sessionId) executeCommand(sessionId, item.value);
      onClose();
    },
    [onCommand, onClose],
  );

  if (!visible || commands.length === 0) return null;

  return (
    <Autocomplete
      items={filtered}
      selectedIndex={selectedIndex}
      onSelect={handleSelect}
      onClose={onClose}
      visible={visible && filtered.length > 0}
      anchorEl={anchorEl}
    />
  );
}
