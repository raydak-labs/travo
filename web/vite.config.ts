import { svelte } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';
import { loadEnv } from 'vite';
import { defineConfig } from 'vitest/config';

export default defineConfig(({ mode }) => {
  // Daemon for local dev: `travo-greenfield --mock` (default :8081); override with DAEMON_URL.
  const target = loadEnv(mode, '.', '').DAEMON_URL || 'http://127.0.0.1:8081';
  const proxy = { '/api': target };
  return {
    plugins: [tailwindcss(), svelte()],
    // pathname leaves %20 intact, so a checkout path with spaces misses $lib.
    resolve: {
      alias: { $lib: decodeURIComponent(new URL('./src/lib', import.meta.url).pathname) },
    },
    build: {
      // Flat output: the device server serves dist/ as-is, with the *.gz written by scripts/gzip-dist.mjs.
      assetsDir: '',
      cssCodeSplit: false,
      modulePreload: { polyfill: false },
    },
    server: { proxy },
    preview: { proxy },
    test: {
      include: ['src/**/*.test.ts', 'scripts/**/*.test.mjs'],
    },
  };
});
