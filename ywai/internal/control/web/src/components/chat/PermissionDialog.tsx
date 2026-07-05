import { useState } from "react";
import { ShieldCheck, ShieldX } from "lucide-react";
import { sendPermissionAction } from "../../api/chat";

export interface PermissionRequest {
  id: string;
  sessionID: string;
  tool: string;
  description: string;
}

interface PermissionDialogProps {
  request: PermissionRequest;
  onDone: () => void;
}

export default function PermissionDialog({ request, onDone }: PermissionDialogProps) {
  const [busy, setBusy] = useState(false);

  const act = async (action: "allow" | "deny", always: boolean) => {
    setBusy(true);
    const ok = await sendPermissionAction(request.sessionID, request.id, action, always);
    // If the API failed, still dismiss — the session will surface another permission event
    setBusy(false);
    if (ok) onDone();
    else onDone();
  };

  return (
    <div className="permission-dialog">
      <div className="permission-header">
        <ShieldCheck size={18} />
        <span>Permission Request</span>
      </div>
      <div className="permission-tool">
        <strong>{request.tool}</strong>
      </div>
      {request.description && (
        <div className="permission-desc">{request.description}</div>
      )}
      <div className="permission-actions">
        <button className="btn-permission-deny" disabled={busy} onClick={() => act("deny", false)}>
          <ShieldX size={16} /> Deny
        </button>
        <button className="btn-permission-allow" disabled={busy} onClick={() => act("allow", false)}>
          <ShieldCheck size={16} /> Allow
        </button>
        <button className="btn-permission-always" disabled={busy} onClick={() => act("allow", true)}>
          <ShieldCheck size={16} /> Allow always
        </button>
      </div>
    </div>
  );
}
