import { type ReactNode, useEffect, useRef } from "react";

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

	return (
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
						<svg
							width="18"
							height="18"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							strokeWidth="2.5"
							strokeLinecap="round"
						>
							<line x1="18" y1="6" x2="6" y2="18" />
							<line x1="6" y1="6" x2="18" y2="18" />
						</svg>
					</button>
				</div>
				<div className="modal-body">{children}</div>
				{footer && <div className="modal-foot">{footer}</div>}
			</div>
		</div>
	);
}
