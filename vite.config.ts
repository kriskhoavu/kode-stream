import { defineConfig } from 'vite';
import type { Plugin } from 'vite';
import react from '@vitejs/plugin-react';

function katexWoff2Only(): Plugin {
  return {
    name: 'katex-woff2-only',
    enforce: 'pre',
    transform(code, id) {
      if (!id.includes('/katex/dist/katex.min.css')) return null;
      return code.replace(/,url\(fonts\/[^)]*\.woff\) format\("woff"\),url\(fonts\/[^)]*\.ttf\) format\("truetype"\)/g, '');
    }
  };
}

export default defineConfig(({ mode }) => {
  const extensionBuild = mode === 'chrome-extension';
  return {
    root: 'web',
    base: extensionBuild ? './' : '/',
    publicDir: extensionBuild ? 'extension' : 'public',
    plugins: [react(), katexWoff2Only()],
    build: {
      outDir: extensionBuild ? '../dist/chrome-extension' : '../internal/server/frontend',
      emptyOutDir: true,
      chunkSizeWarningLimit: 650
    },
    server: {
      proxy: {
        '/api': 'http://127.0.0.1:4317'
      }
    }
  };
});
