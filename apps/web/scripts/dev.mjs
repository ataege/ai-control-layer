#!/usr/bin/env node
// Starts `next dev` on WEB_PORT (default 3000) and WEB_HOST (default 127.0.0.1) without relying
// on shell syntax.
import { createRequire } from "node:module";

import {
  resolveWebBindAddress,
  resolveWebPort,
  runNodeScript,
  webAppDirectory,
} from "./lib/launch.mjs";

// The Next.js CLI entry point, run with Node directly so no platform shim is needed.
const nextCliPath = createRequire(import.meta.url).resolve("next/dist/bin/next");

process.exit(
  await runNodeScript(
    nextCliPath,
    ["dev", "--port", String(resolveWebPort()), "--hostname", resolveWebBindAddress()],
    {
      cwd: webAppDirectory,
      env: process.env,
    },
  ),
);
