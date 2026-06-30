import { useEffect, useState } from 'react'
import { driver, type Driver } from 'driver.js'
import 'driver.js/dist/driver.css'
import { workflowTourSteps, TOUR_SEEN_KEY } from './tour-steps'

// Tour wraps driver.js to run a one-time onboarding walkthrough of the Workflow
// Studio. It auto-runs the first time a user opens the editor (tracked in
// localStorage) and can be re-triggered via the <Tour/> `forceRun` prop or the
// Help button the editor mounts.
//
// The tour is scoped to the /workflows route; the data-tour attributes it
// targets are set on the toolbar, palette, canvas and detail panel.
export default function Tour({ forceRun = false }: { forceRun?: boolean }) {
	const [driverObj, setDriverObj] = useState<Driver | null>(null)

	const start = () => {
		const steps = workflowTourSteps()
		// Filter out steps whose target element isn't in the DOM yet (e.g. the
		// detail panel when nothing is selected) so the tour doesn't break.
		const present = steps.filter((s) => {
			if (!s.element) return true
			if (typeof s.element !== 'string') return true
			return document.querySelector(s.element) !== null
		})
		if (present.length === 0) return

		const d = driver({
			showProgress: true,
			steps: present,
			overlayColor: 'rgba(0,0,0,0.55)',
			// Apply our theme class to every popover in the tour.
			popoverClass: 'ywai-tour-popover',
			onDestroyed: () => {
				localStorage.setItem(TOUR_SEEN_KEY, '1')
			},
		})
		d.drive()
		setDriverObj(d)
	}

	useEffect(() => {
		if (forceRun) {
			start()
			return
		}
		// Auto-run only once per browser, and only after the DOM has the
		// toolbar targets (give the editor a tick to mount).
		if (localStorage.getItem(TOUR_SEEN_KEY) === '1') return
		const t = setTimeout(start, 400)
		return () => clearTimeout(t)
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [forceRun])

	// Expose a way to dismiss early.
	useEffect(() => {
		return () => driverObj?.destroy()
	}, [driverObj])

	return null
}
