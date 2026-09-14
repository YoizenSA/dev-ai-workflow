// Keeps centered [data-tip] tooltips inside the viewport.
//
// The tooltip is a ::after pseudo-element centered on its host
// (components.css: left 50% + translateX(-50%)), so a host near a viewport
// edge pushes half the tooltip off-screen (e.g. the workflow toolbar's early
// buttons). On hover this measures the real pseudo-element box and writes
// --tip-shift; components.css adds that offset to the centering transform of
// both the tooltip and its arrow. Directional variants (data-tip-pos="right",
// "left", "bottom") keep their own transforms and are untouched.

let active: Element | null = null;

const GUTTER = 8; // px kept free between tooltip and viewport edge

function clampTip(el: Element): void {
	const host = el instanceof HTMLElement ? el : null;
	if (!host) return;
	host.style.removeProperty("--tip-shift");
	const style = getComputedStyle(el, "::after");
	if (style.display === "none" || style.content === "none" || style.content === "") return;
	const width = Number.parseFloat(style.width);
	if (!Number.isFinite(width) || width <= 0) return;
	const box = el.getBoundingClientRect();
	const center = box.left + box.width / 2;
	let shift = 0;
	if (center - width / 2 < GUTTER) {
		shift = GUTTER - (center - width / 2); // tooltip spills past the left edge → nudge right
	} else if (center + width / 2 > window.innerWidth - GUTTER) {
		shift = window.innerWidth - GUTTER - (center + width / 2); // past the right edge → nudge left
	}
	if (shift !== 0) host.style.setProperty("--tip-shift", `${Math.round(shift)}px`);
}

document.addEventListener("mouseover", (e) => {
	const target = e.target instanceof Element ? e.target.closest("[data-tip]") : null;
	if (target === active) return; // moving within the same tooltip host
	active = target;
	if (target) clampTip(target);
});
