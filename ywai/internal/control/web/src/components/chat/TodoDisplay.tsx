import { useState, useEffect } from "react";
import { ChevronDown, ChevronRight, CheckSquare, Square } from "lucide-react";
import { getTodo, type TodoItem } from "../../api/chat";

interface TodoDisplayProps {
  sessionId: string;
  refreshKey?: number;
}

export default function TodoDisplay({ sessionId, refreshKey }: TodoDisplayProps) {
  const [items, setItems] = useState<TodoItem[]>([]);
  const [collapsed, setCollapsed] = useState(false);

  useEffect(() => {
    getTodo(sessionId).then(setItems);
  }, [sessionId, refreshKey]);

  if (items.length === 0) return null;

  return (
    <div className="todo-display">
      <button className="todo-header" onClick={() => setCollapsed((c) => !c)}>
        {collapsed ? <ChevronRight size={16} /> : <ChevronDown size={16} />}
        <span>To-do ({items.filter((i) => !i.done).length})</span>
      </button>
      {!collapsed && (
        <div className="todo-list">
          {items.map((item, i) => (
            <label key={i} className={`todo-item ${item.done ? "done" : ""}`}>
              {item.done ? <CheckSquare size={16} /> : <Square size={16} />}
              <span>{item.text}</span>
            </label>
          ))}
        </div>
      )}
    </div>
  );
}
