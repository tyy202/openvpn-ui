import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  base: '/static/modern/',
  build: {
    chunkSizeWarningLimit: 1200,
    rollupOptions: {
      output: {
        entryFileNames: 'assets/access-console.js',
        chunkFileNames: 'assets/[name].js',
        assetFileNames: assetInfo => assetInfo.names.some(name => name.endsWith('.css'))
          ? 'assets/access-console.css'
          : 'assets/[name][extname]',
      },
    },
  },
  plugins: [react()],
})
