import path from "node:path";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react()],
  resolve: {
    // Mirrors tsconfig.json's "@/*" -> "./*" path mapping - Vite
    // doesn't read tsconfig paths on its own, so components/tests
    // importing "@/lib/..." (Tasks 7, 9) need this to resolve.
    alias: {
      "@": path.resolve(__dirname, "."),
    },
  },
  test: {
    environment: "jsdom",
    passWithNoTests: true,
    exclude: ["node_modules", "e2e"],
  },
});
