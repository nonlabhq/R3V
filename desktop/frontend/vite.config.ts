import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import wails from "@wailsio/runtime/plugins/vite";
import { fileURLToPath } from "node:url";

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [svelte(), wails("./bindings")],
  // What the App shares with R3V Cloud's web app (src/shared/README.md).
  resolve: { alias: { $shared: fileURLToPath(new URL("./src/shared", import.meta.url)) } },
  // three.js (the 3D model viewer) is one big chunk, loaded only when a
  // model is shown.
  build: { chunkSizeWarningLimit: 800 },
});
