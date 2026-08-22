import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";
import path from "path";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    // The antd vendor chunk alone is ~1.3 MB and is needed on first paint, so it
    // cannot be split or deferred any further.
    chunkSizeWarningLimit: 1300,
    rollupOptions: {
      output: {
        // Split the three heaviest dependency trees out of the entry chunk so a
        // change to app code does not invalidate all of them in the browser cache.
        manualChunks: {
          react: ["react", "react-dom", "react-router-dom"],
          antd: ["antd", "@ant-design/icons", "@ant-design/pro-components"],
          charts: ["recharts"],
        },
      },
    },
  },
  server: {
    port: 3000,
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      // Outside /api, so it needs its own entry or dev serves index.html for it.
      "/health": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
});
