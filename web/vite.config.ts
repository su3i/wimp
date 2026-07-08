import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    rollupOptions: {
      output: {
        // Stable vendor chunks - these change far less often than app code, so
        // splitting them out means a returning user's cached copy survives most
        // deploys instead of being invalidated by every app-code change. Vite's
        // Rolldown-based bundler only accepts the function form of manualChunks.
        manualChunks(id) {
          if (!id.includes('node_modules')) return
          if (/[\\/](react|react-dom|react-router-dom)[\\/]/.test(id)) return 'vendor-react'
          if (id.includes('@tanstack/react-query')) return 'vendor-query'
        },
      },
    },
  },
})
