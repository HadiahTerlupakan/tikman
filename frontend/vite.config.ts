import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react-swc';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, './src'),
      '@/domain': path.resolve(import.meta.dirname, './src/domain'),
      '@/infrastructure': path.resolve(import.meta.dirname, './src/infrastructure'),
      '@/application': path.resolve(import.meta.dirname, './src/application'),
      '@/presentation': path.resolve(import.meta.dirname, './src/presentation'),
      '@/shared': path.resolve(import.meta.dirname, './src/shared'),
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
});
