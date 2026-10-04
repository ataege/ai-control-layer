import path from "node:path";
import type { NextConfig } from "next";

const isDevelopment = process.env.NODE_ENV !== "production";

// Everything the app loads comes from its own origin. Next.js inlines its bootstrap and style
// tags, so those two directives allow inline content; `next dev` also needs eval for source maps
// and a websocket for hot reload. Nothing may frame the app (the approval screen).
const contentSecurityPolicy = [
  "default-src 'self'",
  `script-src 'self' 'unsafe-inline'${isDevelopment ? " 'unsafe-eval'" : ""}`,
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data: blob:",
  "font-src 'self' data:",
  `connect-src 'self'${isDevelopment ? " ws: wss:" : ""}`,
  "object-src 'none'",
  "base-uri 'self'",
  "form-action 'self'",
  "frame-ancestors 'none'",
].join("; ");

const securityHeaders = [
  { key: "Content-Security-Policy", value: contentSecurityPolicy },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "Referrer-Policy", value: "no-referrer" },
  { key: "X-Frame-Options", value: "DENY" },
];

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
  async headers() {
    return [{ source: "/:path*", headers: securityHeaders }];
  },
};

export default nextConfig;
