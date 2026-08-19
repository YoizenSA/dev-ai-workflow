import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { boneyardPlugin } from 'boneyard-js/vite'

export default defineConfig({
  plugins: [
    react(),
    // Captures <Skeleton name="..."> layouts in dev and writes src/bones/*.bones.json
    boneyardPlugin({ out: './src/bones' }),
  ],
  base: '/',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    target: 'es2020',
    cssCodeSplit: false,
    rollupOptions: {
      output: {
        // ESM so Vite's preload helper can keep import.meta (IIFE empties it).
        // The Go SPA server already serves hashed /assets/* as static files.
        format: 'es',
        entryFileNames: 'assets/app-[hash].js',
        chunkFileNames: 'assets/app-[hash].js',
        assetFileNames: 'assets/[name]-[hash][extname]',
      },
    },
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
