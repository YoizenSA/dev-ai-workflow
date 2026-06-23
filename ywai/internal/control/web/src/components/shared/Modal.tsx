import { type ReactNode, useEffect, useRef } from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";

interface ModalProps {
	open: boolean;
	onClose: () => void;
	title: string;
	subtitle?: string;
	children: ReactNode;
	footer?: ReactNode;
	width?: string;
}

export default function Modal({
	open,
	onClose,
	title,
	subtitle,
	children,
	footer,
	width,
}: ModalProps) {
	const overlayRef = useRef<HTMLDivElement>(null);

	useEffect(() => {
		const handleEsc = (e: KeyboardEvent) => {
			if (e.key === "Escape") onClose();
		};
		if (open) {
			document.addEventListener("keydown", handleEsc);
			document.body.style.overflow = "hidden";
		}
		return () => {
			document.removeEventListener("keydown", handleEsc);
			document.body.style.overflow = "";
		};
	}, [open, onClose]);

	if (!open) return null;

	return createPortal(
		<div
			className="overlay"
			ref={overlayRef}
			onClick={(e) => e.target === overlayRef.current && onClose()}
		>
			<div
				className="modal"
				style={
					width ? ({ "--modal-w": width } as React.CSSProperties) : undefined
				}
			>
				<div className="modal-head">
					<div>
						<h2 className="modal-title">{title}</h2>
						{subtitle && <p className="modal-subtitle">{subtitle}</p>}
					</div>
					<button className="modal-close" onClick={onClose} aria-label="Close">
						<X size={20} strokeWidth={2.5} />
					</button>
				</div>
				<div className="modal-body">{children}</div>
				{footer && <div className="modal-foot">{footer}</div>}
			</div>
		</div>,
		document.body,
	);
}
