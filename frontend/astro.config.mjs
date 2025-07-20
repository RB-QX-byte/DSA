// @ts-check
import { defineConfig } from 'astro/config';

// https://astro.build/config
export default defineConfig({
  site: 'https://example.com', // Temporary site URL for build
  output: 'static',
  devToolbar: {
    enabled: false
  },
  vite: {
    css: {
      devSourcemap: true
    },
    optimizeDeps: {
      include: ['monaco-editor']
    },
    build: {
      rollupOptions: {
        external: id => id.includes('monaco-editor/esm/vs/editor/editor.worker'),
      }
    }
  },
  server: {
    port: 4321,
    host: true
  },
  build: {
    assets: 'assets'
  }
});
