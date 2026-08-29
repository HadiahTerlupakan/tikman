import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react-swc';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
    // Driving an antd Modal through userEvent takes seconds in jsdom, and CI
    // runners are slower than a dev machine: at the 5s default those tests pass
    // locally and time out there.
    testTimeout: 15000,
  },
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
});
