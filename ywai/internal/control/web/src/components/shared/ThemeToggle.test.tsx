import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ThemeToggle } from './ThemeToggle';
import { useThemeStore } from '../../stores/themeStore';

describe('ThemeToggle', () => {
	beforeEach(() => {
		localStorage.clear();
		document.documentElement.removeAttribute('data-theme');
		useThemeStore.setState({ theme: 'dark' });
		delete (document as { startViewTransition?: unknown }).startViewTransition;
	});

	it('renders a <button> with aria-label "Switch to light theme" when current theme is dark', () => {
		render(<ThemeToggle />);
		expect(
			screen.getByRole('button', { name: /switch to light theme/i }),
		).toBeInTheDocument();
	});

	it('renders a <button> with aria-label "Switch to dark theme" when current theme is light', () => {
		useThemeStore.setState({ theme: 'light' });
		render(<ThemeToggle />);
		expect(
			screen.getByRole('button', { name: /switch to dark theme/i }),
		).toBeInTheDocument();
	});

	it('has the .theme-toggle className', () => {
		render(<ThemeToggle />);
		expect(screen.getByRole('button')).toHaveClass('theme-toggle');
	});

	it('clicking the button toggles the store and updates the aria-label', () => {
		render(<ThemeToggle />);
		const button = screen.getByRole('button', { name: /switch to light theme/i });
		fireEvent.click(button);
		expect(useThemeStore.getState().theme).toBe('light');
		expect(
			screen.getByRole('button', { name: /switch to dark theme/i }),
		).toBeInTheDocument();
	});

	it('pressing Space on a focused button activates the toggle', async () => {
		const user = userEvent.setup();
		render(<ThemeToggle />);
		const button = screen.getByRole('button', { name: /switch to light theme/i });
		button.focus();
		await user.keyboard(' ');
		expect(useThemeStore.getState().theme).toBe('light');
	});

	it('pressing Enter on a focused button activates the toggle', async () => {
		const user = userEvent.setup();
		render(<ThemeToggle />);
		const button = screen.getByRole('button', { name: /switch to light theme/i });
		button.focus();
		await user.keyboard('{Enter}');
		expect(useThemeStore.getState().theme).toBe('light');
	});
});
