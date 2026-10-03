#!/usr/bin/env node
// Serves the production build with the standalone server (`next start` does not support
// `output: "standalone"`). Run `build` first.
import { cpSync, existsSync, rmSync } from "node:fs";
import path from "node:path";

import {
  resolveWebBindAddress,
  resolveWebPort,
  runNodeScript,
  webAppDirectory,
} from "./lib/launch.mjs";

const buildDirectory = path.join(webAppDirectory, ".next");
// The standalone tree mirrors the monorepo layout, so the server sits under apps/web.
const standaloneAppDirectory = path.join(buildDirectory, "standalone", "apps", "web");
const standaloneServerPath = path.join(standaloneAppDirectory, "server.js");

if (!existsSync(standaloneServerPath)) {
  console.error("[web] No production build found. Run `pnpm --filter web run build` first.");
  process.exit(1);
}

// Static assets and public files are not part of the standalone output.
const assetDirectories = [
  {
    source: path.join(buildDirectory, "static"),
    destination: path.join(standaloneAppDirectory, ".next", "static"),
  },
  {
    source: path.join(webAppDirectory, "public"),
    destination: path.join(standaloneAppDirectory, "public"),
  },
];

for (const { source, destination } of assetDirectories) {
  // Replace the previous copy so files from an older build do not pile up.
  rmSync(destination, { recursive: true, force: true });
  if (existsSync(source)) {
    cpSync(source, destination, { recursive: true });
  }
}

process.exit(
  await runNodeScript(standaloneServerPath, [], {
    cwd: standaloneAppDirectory,
    env: {
      ...process.env,
      PORT: String(resolveWebPort()),
      HOSTNAME: resolveWebBindAddress(),
    },
  }),
);
