import path from "node:path";
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Self-contained server bundle for the container image.
  output: "standalone",
  // Monorepo root, so file tracing includes workspace packages.
  outputFileTracingRoot: path.join(__dirname, "../.."),
  // The UI package ships raw TSX.
  transpilePackages: ["@workspace/ui"],
  // Do not let `next dev` write AGENTS.md / CLAUDE.md into the app.
  agentRules: false,
  poweredByHeader: false,
};

export default nextConfig;
