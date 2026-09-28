import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  base: '/static/modern/',
  build: { chunkSizeWarningLimit: 1200 },
  plugins: [react()],
})
