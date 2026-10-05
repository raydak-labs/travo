import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      '@shared': path.resolve(__dirname, '../shared/src'),
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['src/test/setup.ts'],
    globals: true,
    css: true,
    // Declared rather than defaulted. shared/vitest.config.ts pins include for the
    // same reason: the default glob is an unstated assumption that a rename can
    // break silently.
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
  },
});
