import { defineConfig } from "@playwright/test";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

// Smoke tests (npm run smoke): the app's server build on the demo team
// (desktop/cmd/r3v-demo: storage in memory, no network, no keys), driven in
// Edge. The demo has its own folder and port, apart from the one used to
// look at the app.
const port = 8791;
const repo = fileURLToPath(new URL("../../..", import.meta.url));

export default defineConfig({
  testDir: ".",
  timeout: 60_000,
  // A server build just made starts slowly the first time (Windows scans a
  // new program): what the steps wait for gets longer than the default.
  expect: { timeout: 15_000 },
  workers: 1, // one demo team: the steps change it, in order
  reporter: [["list"]],
  use: {
    baseURL: `http://localhost:${port}`,
    channel: "msedge",
    locale: "en-US",
    viewport: { width: 1400, height: 900 },
  },
  webServer: {
    // (absolute paths: the demo moves between folders as it sets the team up)
    command: `go run ./desktop/cmd/r3v-demo -app ${join(repo, "desktop", "bin", "R3V-server.exe")} -sets ${join(repo, "testdata", "live")} ` +
      `-port ${port} -dir ${join(tmpdir(), "r3v-smoke")}`,
    cwd: repo,
    url: `http://localhost:${port}`,
    timeout: 180_000,
    reuseExistingServer: false,
  },
});
