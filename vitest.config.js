import { defineConfig } from "vitest/config";

// Pure-logic tests only. No workerd pool — we stub KV and bot in-memory.
export default defineConfig({
  test: {
    environment: "node",
    include: ["tests/**/*.test.js"],
    passWithNoTests: true,
    coverage: {
      provider: "v8",
      reporter: ["text", "html"],
      include: ["src/**/*.js"],
    },
  },
});
