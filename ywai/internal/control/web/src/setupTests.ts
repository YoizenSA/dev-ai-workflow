import '@testing-library/jest-dom';

// Workaround for Vitest 2.1.9 + jsdom 25.0.1 + Node 25 leaving
// `window.localStorage` as an empty object (no `getItem`/`setItem`/etc.).
// Verified that the `environmentOptions: { jsdom: { url: 'http://localhost/' } }`
// setting in vitest.config.ts does NOT initialize the Storage backend in
// this version combination — the polyfill below is required. Any component
// that reads localStorage during render (e.g. SessionSidebar reading
// `kanban-collapsed-groups`) would otherwise crash with
// "localStorage.getItem is not a function". Polyfill is in-memory and
// only activates when jsdom's Storage isn't properly installed.
if (typeof window.localStorage === 'undefined' || typeof window.localStorage.getItem !== 'function') {
  const store = new Map<string, string>();
  const memoryStorage: Storage = {
    get length() {
      return store.size;
    },
    clear() {
      store.clear();
    },
    getItem(key) {
      return store.has(key) ? (store.get(key) as string) : null;
    },
    key(index) {
      return Array.from(store.keys())[index] ?? null;
    },
    removeItem(key) {
      store.delete(key);
    },
    setItem(key, value) {
      store.set(key, String(value));
    },
  };
  Object.defineProperty(window, 'localStorage', { value: memoryStorage, configurable: true, writable: true });
  Object.defineProperty(globalThis, 'localStorage', { value: memoryStorage, configurable: true, writable: true });
}
