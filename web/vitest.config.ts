import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import viteTsConfigPaths from "vite-tsconfig-paths";

export default defineConfig({
  plugins: [react(), viteTsConfigPaths()],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./test/setup.ts"],
    include: ["**/*.{test,spec}.{ts,tsx}"],
    exclude: ["node_modules", "e2e/**"],
    coverage: {
      reporter: ["text", "html"],
      exclude: ["node_modules/", "test/setup.ts"],
    },
  },
  resolve: {
    alias: {
      "~": "/app",
    },
  },
});
