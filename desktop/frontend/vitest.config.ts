import { defineConfig } from "vitest/config";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { fileURLToPath } from "node:url";

// Unit and component tests (npm test): logic in .ts files, and components
// rendered in jsdom with the Go bindings mocked (vi.mock("./api")).
export default defineConfig({
  plugins: [svelte()],
  resolve: { conditions: ["browser"], alias: { $shared: fileURLToPath(new URL("./src/shared", import.meta.url)) } },
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.ts"],
    // CSS is emptied in tests, except tokens.css and base.css: tokens.test.ts
    // and bands.test.ts read them.
    css: { include: [/tokens\.css/, /base\.css/] },
  },
});
