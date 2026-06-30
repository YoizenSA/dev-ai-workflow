import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  base: '/',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    target: 'es2015',
    cssCodeSplit: false,
    rollupOptions: {
      output: {
        format: 'iife',
        // Content-hashed filenames for cache busting. index.html (served by the
        // Go control server) references these by hash, so every rebuild
        // invalidates the browser cache automatically — no hard refresh needed.
        entryFileNames: 'assets/app-[hash].js',
        chunkFileNames: 'assets/app-[hash].js',
        assetFileNames: 'assets/[name]-[hash][extname]'
      }
    }
  },
  server: {
    port: 3000,
    proxy: {
      // The workflows WebSocket lives at /api/workflows/ws, so the /api proxy
      // must also handle the ws:// upgrade (not just HTTP). Without ws:true
      // here, the socket connects but never receives frames.
      '/api': {
        target: 'http://localhost:5768',
        ws: true,
      },
      '/ws': {
        target: 'ws://localhost:5768',
        ws: true
      },
      '/missions': 'http://localhost:5768'
    }
  }
})
