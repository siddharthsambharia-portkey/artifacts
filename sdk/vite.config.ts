import { defineConfig } from 'vite';

export default defineConfig({
  build: {
    lib: {
      entry: 'src/artifact.ts',
      name: 'ArtifactSDK',
      formats: ['iife'],
      fileName: () => 'artifact.js',
    },
    outDir: 'dist',
    minify: 'esbuild',
    target: 'es2020',
  },
});
