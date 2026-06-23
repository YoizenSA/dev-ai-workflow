import { describe, it, expect, beforeEach, vi } from 'vitest';
import { useThemeStore, applyThemeFromStorage } from './themeStore';

describe('useThemeStore', () => {
	beforeEach(() => {
		localStorage.clear();
		document.documentElement.removeAttribute('data-theme');
		// Reset the module-level store between tests.
		useThemeStore.setState({ theme: 'dark' });
		// Drop any startViewTransition stub from a previous test.
		delete (document as { startViewTransition?: unknown }).startViewTransition;
	});

	it('theme defaults to "dark" when localStorage is empty', () => {
		expect(useThemeStore.getState().theme).toBe('dark');
	});

	it('theme reads from localStorage.getItem("yd-theme") on init', async () => {
		localStorage.clear();
		localStorage.setItem('yd-theme', 'light');
		// Force the module to re-evaluate so readInitial() runs with the new localStorage.
		vi.resetModules();
		const mod = await import('./themeStore');
		expect(mod.useThemeStore.getState().theme).toBe('light');
	});

	it('toggle() flips dark → light → dark', () => {
		const { toggle } = useThemeStore.getState();
		toggle();
		expect(useThemeStore.getState().theme).toBe('light');
		toggle();
		expect(useThemeStore.getState().theme).toBe('dark');
	});

	it('toggle() writes the new theme to localStorage after each flip', () => {
		useThemeStore.getState().toggle();
		expect(localStorage.getItem('yd-theme')).toBe('light');
		useThemeStore.getState().toggle();
		expect(localStorage.getItem('yd-theme')).toBe('dark');
	});

	it('toggle() sets document.documentElement[data-theme] for light, removes for dark', () => {
		useThemeStore.getState().toggle(); // → light
		expect(document.documentElement.getAttribute('data-theme')).toBe('light');
		useThemeStore.getState().toggle(); // → dark
		expect(document.documentElement.hasAttribute('data-theme')).toBe(false);
	});

	it('toggle() calls document.startViewTransition when available', () => {
		const startVT = vi.fn().mockReturnValue({
			ready: Promise.resolve(),
			finished: Promise.resolve(),
		});
		(document as { startViewTransition?: unknown }).startViewTransition = startVT;
		// Reduced motion is off by default in jsdom (no matchMedia stub → falls through to false).
		useThemeStore.getState().toggle();
		expect(startVT).toHaveBeenCalledTimes(1);
	});

	it('toggle() skips startViewTransition and applies synchronously when prefers-reduced-motion matches', () => {
		const startVT = vi.fn();
		(document as { startViewTransition?: unknown }).startViewTransition = startVT;
		vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: true } as MediaQueryList);
		useThemeStore.getState().toggle();
		expect(startVT).not.toHaveBeenCalled();
		expect(useThemeStore.getState().theme).toBe('light');
		vi.restoreAllMocks();
	});

	it('toggle() skips startViewTransition when startViewTransition is undefined', () => {
		// startViewTransition already deleted in beforeEach.
		useThemeStore.getState().toggle();
		expect(useThemeStore.getState().theme).toBe('light');
	});
});

describe('applyThemeFromStorage (FOUC bootstrap)', () => {
	beforeEach(() => {
		localStorage.clear();
		document.documentElement.removeAttribute('data-theme');
	});

	it('sets data-theme="light" when localStorage returns "light"', () => {
		localStorage.setItem('yd-theme', 'light');
		applyThemeFromStorage();
		expect(document.documentElement.getAttribute('data-theme')).toBe('light');
	});

	it('does not set data-theme when localStorage is empty (default dark)', () => {
		applyThemeFromStorage();
		expect(document.documentElement.hasAttribute('data-theme')).toBe(false);
	});

	it('does not throw and preserves dark default when localStorage.getItem throws (Safari private mode)', () => {
		vi.spyOn(window.localStorage, 'getItem').mockImplementation(() => {
			throw new Error('SecurityError: storage access denied');
		});
		expect(() => applyThemeFromStorage()).not.toThrow();
		expect(document.documentElement.hasAttribute('data-theme')).toBe(false);
		vi.restoreAllMocks();
	});
});
