import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    // jsdom v25 leaves `window.localStorage` uninitialized for `about:blank`
    // URLs, so any code that reads localStorage (e.g. SessionSidebar) crashes.
    // Forcing an http:// origin tells jsdom to construct the Storage backend.
    environmentOptions: { jsdom: { url: 'http://localhost/' } },
    globals: true,
    setupFiles: ['./src/setupTests.ts'],
  },
});
