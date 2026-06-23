import { create } from 'zustand';

type Theme = 'dark' | 'light';
const STORAGE_KEY = 'yd-theme';

interface ViewTransition {
	ready: Promise<void>;
	finished: Promise<void>;
	skipTransition: () => void;
}

type VTDocument = Document & {
	startViewTransition?: (cb: () => void) => ViewTransition;
};

const DURATION = 450;
const EASING = 'ease-in-out';

/** Read the persisted theme from localStorage. Dark is the default. */
const readInitial = (): Theme => {
	try {
		return localStorage.getItem(STORAGE_KEY) === 'light' ? 'light' : 'dark';
	} catch {
		// localStorage access threw (Safari private mode) — default to dark.
		return 'dark';
	}
};

/**
 * Apply the persisted theme to <html data-theme> at startup, before the first paint.
 * Called from the inline IIFE in index.html so the first paint already has the right
 * data-theme (no flash of wrong theme). Mirrors readInitial() above — keep in sync.
 * A "// KEEP IN SYNC" comment in index.html points here.
 */
export const applyThemeFromStorage = (): void => {
	try {
		if (localStorage.getItem(STORAGE_KEY) === 'light') {
			document.documentElement.setAttribute('data-theme', 'light');
		}
	} catch {
		// localStorage access threw (Safari private mode) — keep default dark.
	}
};

interface ThemeState {
	theme: Theme;
	toggle: (event?: MouseEvent) => void;
}

export const useThemeStore = create<ThemeState>((set, get) => ({
	theme: readInitial(),
	toggle: (event) => {
		const next: Theme = get().theme === 'light' ? 'dark' : 'light';
		const apply = (): void => {
			set({ theme: next });
			if (next === 'light') {
				document.documentElement.setAttribute('data-theme', 'light');
			} else {
				document.documentElement.removeAttribute('data-theme');
			}
			try {
				localStorage.setItem(STORAGE_KEY, next);
			} catch {
				// localStorage write blocked (quota / Safari private) — non-fatal.
			}
		};

		// Degrade to instant cut when View Transitions is unsupported or
		// the user prefers reduced motion.
		const reducedMotion =
			typeof matchMedia === 'function' &&
			matchMedia('(prefers-reduced-motion: reduce)').matches;
		const startVT = (document as VTDocument).startViewTransition?.bind(document);
		if (!startVT || reducedMotion) {
			apply();
			return;
		}

		// Click-origin circle: WAAPI clip-path on ::view-transition-new(root).
		// Mirrors ywui theme.service.ts L46-59.
		const x = event?.clientX ?? innerWidth / 2;
		const y = event?.clientY ?? innerHeight / 2;
		const radius = Math.hypot(
			Math.max(x, innerWidth - x),
			Math.max(y, innerHeight - y),
		);
		const transition = startVT(apply);
		transition.ready
			.then(() => {
				document.documentElement.animate(
					{
						clipPath: [
							`circle(0px at ${x}px ${y}px)`,
							`circle(${radius}px at ${x}px ${y}px)`,
						],
					},
					{ duration: DURATION, easing: EASING, pseudoElement: '::view-transition-new(root)' },
				);
			})
			.catch(() => undefined);
	},
}));
