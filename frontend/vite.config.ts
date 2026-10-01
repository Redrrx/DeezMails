import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig(({ mode }) => ({
  base: mode === "demo" ? "./" : "/",
  plugins: [react()],
  build: {
    outDir: mode === "demo" ? "dist-demo" : "dist",
    rollupOptions: {
      output: {
        manualChunks: {
          "fluent-vendor": [
            "@fluentui/react-components",
            "@fluentui/react-icons",
          ],
          "data-vendor": [
            "@hookform/resolvers/zod",
            "@tanstack/react-query",
            "react-hook-form",
            "zod",
            "zustand",
          ],
        },
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": "http://localhost:8080",
      "/health": "http://localhost:8080",
      "/oauth": "http://localhost:8080",
      "/swagger": "http://localhost:8080",
    },
  },
}));
