import { defineConfig } from 'vitest/config';
import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';

const apiTarget = process.env.VITE_API_TARGET || 'http://127.0.0.1:8080';

export default defineConfig({
  plugins: [tailwindcss(), sveltekit()],
  server: {
    proxy: {
      '/api': {
        target: apiTarget,
        changeOrigin: true
      },
      '/downloads': {
        target: apiTarget,
        changeOrigin: true
      }
    }
  },
  test: {
    include: ['src/**/*.{test,spec}.{js,ts}']
  }
});
