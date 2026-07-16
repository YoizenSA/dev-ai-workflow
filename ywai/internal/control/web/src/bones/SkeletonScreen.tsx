import type { ReactNode } from "react";
import { Skeleton } from "boneyard-js/react";

export type SkeletonScreenProps = {
	/** Unique capture name → name.bones.json after boneyard build */
	name: string;
	loading: boolean;
	children: ReactNode;
	/** Shown when loading and no bones registry entry yet */
	fallback: ReactNode;
	/** Mock tree for CLI / Vite plugin capture */
	fixture?: ReactNode;
	className?: string;
};

/**
 * Thin ywai wrapper around boneyard-js Skeleton with project defaults.
 */
export function SkeletonScreen({
	name,
	loading,
	children,
	fallback,
	fixture,
	className,
}: SkeletonScreenProps) {
	return (
		<Skeleton
			name={name}
			loading={loading}
			fallback={fallback}
			fixture={fixture}
			className={className}
			animate="shimmer"
			stagger
			transition
			select="viewport"
		>
			{children}
		</Skeleton>
	);
}
