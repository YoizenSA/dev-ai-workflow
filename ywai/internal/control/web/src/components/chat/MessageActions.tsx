import { Copy, Edit, Trash2, RotateCcw } from "lucide-react";

interface MessageActionsProps {
  text: string;
  messageId: string;
  isFirstUser: boolean;
  onEdit: (text: string) => void;
  onDelete: (messageId: string) => void;
  onRevert: (messageId: string) => void;
}

export default function MessageActions({
  text,
  messageId,
  isFirstUser,
  onEdit,
  onDelete,
  onRevert,
}: MessageActionsProps) {
  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(text);
    } catch { /* not critical */ }
  };

  return (
    <div className="message-actions">
      <button className="msg-action-btn" onClick={handleCopy} data-tip="Copy" aria-label="Copy message">
        <Copy size={14} />
      </button>
      <button className="msg-action-btn" onClick={() => onEdit(text)} data-tip="Edit" aria-label="Edit message">
        <Edit size={14} />
      </button>
      <button className="msg-action-btn" onClick={() => onDelete(messageId)} data-tip="Delete" aria-label="Delete message">
        <Trash2 size={14} />
      </button>
      {!isFirstUser && (
        <button className="msg-action-btn" onClick={() => onRevert(messageId)} data-tip="Revert to here" aria-label="Revert to here">
          <RotateCcw size={14} />
        </button>
      )}
    </div>
  );
}
