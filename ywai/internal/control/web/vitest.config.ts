import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    // jsdom v25 leaves `window.localStorage` uninitialized for `about:blank`
    // URLs. The `environmentOptions: { jsdom: { url: 'http://localhost/' } }`
    // setting below was verified to NOT initialize the Storage backend in
    // Vitest 2.1.9 + jsdom 25.0.1 + Node 25, so the actual fix lives in
    // src/setupTests.ts (an in-memory localStorage polyfill).
    environmentOptions: { jsdom: { url: 'http://localhost/' } },
    globals: true,
    setupFiles: ['./src/setupTests.ts'],
  },
});
