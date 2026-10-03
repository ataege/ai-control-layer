import { fileURLToPath } from "node:url";

// Tailwind v4 has no JS config, so the class sorter reads the shared stylesheet instead.
const tailwindStylesheetPath = fileURLToPath(
  new URL("../../ui/src/styles/globals.css", import.meta.url),
);

// Shared Prettier config. The plugin is referenced by absolute URL so it resolves
// from this package no matter which workspace Prettier is started in.
/** @type {import("prettier").Config} */
const config = {
  semi: true,
  singleQuote: false,
  trailingComma: "all",
  printWidth: 100,
  plugins: [import.meta.resolve("prettier-plugin-tailwindcss")],
  tailwindStylesheet: tailwindStylesheetPath,
  tailwindFunctions: ["cn", "cva"],
};

export default config;
