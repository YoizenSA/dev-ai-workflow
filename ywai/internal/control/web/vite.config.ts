import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  base: '/ui/',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    target: 'es2015',
    cssCodeSplit: false,
    rollupOptions: {
      output: {
        format: 'iife',
        entryFileNames: 'app.js',
        chunkFileNames: 'app.js',
        assetFileNames: (assetInfo) => {
          if (assetInfo.name === 'index.html') return 'index.html';
          if (assetInfo.name === 'index.css') return 'index.css';
          return assetInfo.name || 'asset';
        }
      }
    }
  },
  server: {
    port: 3000,
    proxy: {
      '/api': 'http://localhost:5768',
      '/ws': {
        target: 'ws://localhost:5768',
        ws: true
      },
      '/missions': 'http://localhost:5768'
    }
  }
})
