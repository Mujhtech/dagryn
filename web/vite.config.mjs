import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import viteTsConfigPaths from "vite-tsconfig-paths";
import { tanstackRouter } from "@tanstack/router-plugin/vite";

export default defineConfig({
  plugins: [
    tanstackRouter({
      routesDirectory: "./app/routes",
      generatedRouteTree: "./app/routeTree.gen.ts",
      // prerender: {
      //   enabled: true,
      //   crawlLinks: true, // Discovers all linkable pages
      // },
      // sitemap: {
      //   enabled: true,
      //   host: 'https://myapp.com',
      // },
    }),
    react(),
    tailwindcss(),
    viteTsConfigPaths(),
  ],
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
  resolve: {
    alias: {
      "~": "/app",
    },
  },
});

