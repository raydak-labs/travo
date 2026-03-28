import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import path from 'path';

function manualChunks(id: string): string | undefined {
  if (!id.includes('node_modules')) return;
  if (
    id.includes('/react-dom/') ||
    id.includes('/react/') ||
    id.includes('/scheduler/')
  ) {
    return 'react-vendor';
  }
  if (id.includes('recharts')) return 'recharts-vendor';
  if (id.includes('lucide-react')) return 'icons-vendor';
  if (id.includes('@tanstack')) return 'tanstack-vendor';
  // Radix / floating-ui / etc. fall through to Rollup default chunking — isolating them caused circular chunk graphs.
  if (
    id.includes('node_modules/react-hook-form') ||
    id.includes('node_modules/@hookform') ||
    id.includes('node_modules/zod/')
  ) {
    return 'forms-vendor';
  }
  // Let Rollup assign other `node_modules` — a catch-all bucket can create circular chunks with react-vendor.
  return undefined;
}

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    rollupOptions: {
      output: {
        manualChunks,
      },
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      '@shared': path.resolve(__dirname, '../shared/src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:3000',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:3000',
        ws: true,
      },
    },
  },
});
