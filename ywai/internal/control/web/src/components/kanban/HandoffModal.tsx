import { useEffect, useRef, type RefObject } from "react";
import Modal from "../shared/Modal";

interface Props {
	open: boolean;
	onClose: () => void;
	// Element that triggered the modal — focus returns here on close for a11y.
	triggerRef: RefObject<HTMLElement>;
	// Short label for the modal title (e.g. the delegation's task summary).
	title: string;
	// Small subtitle under the title (e.g. "<agent> · <status>").
	subtitle?: string;
	// Full handoff text — rendered verbatim with whitespace preserved.
	handoff: string;
}

// HandoffModal is a focused "view the full handoff" modal. Lighter than
// DelegationDetailModal (no activity fetch) so clicking the truncated preview
// on a card opens the plan instantly.
export default function HandoffModal({
	open,
	onClose,
	triggerRef,
	title,
	subtitle,
	handoff,
}: Props) {
	const closeBtnRef = useRef<HTMLButtonElement>(null);

	// Focus management: on open move focus into the modal, on close return
	// it to the trigger. Without this keyboard users would have to tab back
	// through the whole board to find where they were.
	useEffect(() => {
		if (!open) return;
		const previouslyFocused = document.activeElement as HTMLElement | null;
		// Defer to the next frame so the portal-mounted close button exists.
		const id = requestAnimationFrame(() => {
			closeBtnRef.current?.focus();
		});
		return () => {
			cancelAnimationFrame(id);
			// Prefer the explicit trigger ref (e.g. the preview button the user
			// clicked), fall back to whatever was focused before.
			const target = triggerRef.current ?? previouslyFocused;
			if (target && typeof target.focus === "function") target.focus();
		};
	}, [open, triggerRef]);

	return (
		<Modal
			open={open}
			onClose={onClose}
			title={title}
			subtitle={subtitle}
			width="720px"
			closeButtonRef={closeBtnRef}
		>
			<div className="handoff-modal-body">
				<h4 className="activity-title">Handoff / Plan</h4>
				{/* white-space: pre-wrap preserves the markdown-style lists and
				    code blocks that architect/dev handoffs contain. */}
				<pre className="handoff-modal-text">{handoff}</pre>
			</div>
		</Modal>
	);
}
