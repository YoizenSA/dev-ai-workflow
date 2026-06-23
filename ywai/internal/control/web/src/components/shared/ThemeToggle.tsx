import { useThemeStore } from '../../stores/themeStore';

// Inline Lucide paths (sun + moon). lucide-react lands in slice G; the
// view-transition-name on the icon wrapper is what makes the morph work
// regardless of whether the SVG comes from lucide-react or inline.
const SunIcon = () => (
	<svg
		width="20"
		height="20"
		viewBox="0 0 24 24"
		fill="none"
		stroke="currentColor"
		strokeWidth="2"
		strokeLinecap="round"
		strokeLinejoin="round"
		aria-hidden="true"
	>
		<circle cx="12" cy="12" r="4" />
		<path d="M12 2v2" />
		<path d="M12 20v2" />
		<path d="m4.93 4.93 1.41 1.41" />
		<path d="m17.66 17.66 1.41 1.41" />
		<path d="M2 12h2" />
		<path d="M20 12h2" />
		<path d="m6.34 17.66-1.41 1.41" />
		<path d="m19.07 4.93-1.41 1.41" />
	</svg>
);

const MoonIcon = () => (
	<svg
		width="20"
		height="20"
		viewBox="0 0 24 24"
		fill="none"
		stroke="currentColor"
		strokeWidth="2"
		strokeLinecap="round"
		strokeLinejoin="round"
		aria-hidden="true"
	>
		<path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
	</svg>
);

export function ThemeToggle(): JSX.Element {
	const theme = useThemeStore((s) => s.theme);
	const toggle = useThemeStore((s) => s.toggle);
	const isLight = theme === 'light';
	const label = isLight ? 'Switch to dark theme' : 'Switch to light theme';
	return (
		<button
			type="button"
			className="theme-toggle"
			onClick={(e) => toggle(e.nativeEvent)}
			aria-label={label}
			title={label}
		>
			<span className="theme-toggle-icon" aria-hidden="true">
				{isLight ? <MoonIcon /> : <SunIcon />}
			</span>
		</button>
	);
}
