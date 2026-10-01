import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);

export default defineConfig(({ mode }) => ({
  base: mode === "demo" ? "./" : "/",
  plugins: [
    react(),
    ...(mode === "demo"
      ? [
          {
            name: "demo-api-docs",
            apply: "build" as const,
            generateBundle() {
              this.emitFile({
                type: "asset",
                fileName: "swagger/index.html",
                source: readFileSync(
                  new URL("./demo-api-docs.html", import.meta.url),
                ),
              });
              this.emitFile({
                type: "asset",
                fileName: "swagger/doc.json",
                source: readFileSync(
                  new URL("../backend/docs/swagger.json", import.meta.url),
                ),
              });
              for (const name of ["swagger-ui.css", "swagger-ui-bundle.js"]) {
                this.emitFile({
                  type: "asset",
                  fileName: `swagger/${name}`,
                  source: readFileSync(
                    require.resolve(`swagger-ui-dist/${name}`),
                  ),
                });
              }
            },
          },
        ]
      : []),
  ],
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
