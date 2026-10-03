import js from "@eslint/js";
import { defineConfig, globalIgnores } from "eslint/config";
import prettierConfig from "eslint-config-prettier/flat";
import globals from "globals";
import tseslint from "typescript-eslint";

// Folders that never contain hand-written source.
const DEFAULT_IGNORES = [
  "**/dist/**",
  "**/.next/**",
  "**/.turbo/**",
  "**/coverage/**",
  "**/bin/**",
];

/**
 * Flat-config factory for plain TypeScript workspaces (Node libraries, NestJS).
 * @param {object} options
 * @param {string} options.tsconfigRootDir  pass import.meta.dirname from the workspace
 * @param {"node" | "browser"} [options.runtime]
 * @param {boolean} [options.typeChecked]  enable type-aware rules (slower)
 * @param {string[]} [options.ignores]
 * @param {import("eslint").Linter.Config[]} [options.extraConfigs]
 */
export function createConfig({
  tsconfigRootDir,
  runtime = "node",
  typeChecked = false,
  ignores = [],
  extraConfigs = [],
} = {}) {
  return defineConfig(
    globalIgnores([...DEFAULT_IGNORES, ...ignores]),
    js.configs.recommended,
    typeChecked ? tseslint.configs.recommendedTypeChecked : tseslint.configs.recommended,
    {
      languageOptions: {
        globals: runtime === "browser" ? globals.browser : globals.node,
        // tsconfigRootDir is always needed: ESLint may be started from the repo root.
        parserOptions: typeChecked
          ? { projectService: true, tsconfigRootDir }
          : { tsconfigRootDir },
      },
    },
    typeChecked
      ? {
          // node:test's test()/it()/describe() return promises that are safe to ignore.
          files: ["**/*.test.ts", "**/*.spec.ts"],
          rules: {
            "@typescript-eslint/no-floating-promises": [
              "error",
              {
                allowForKnownSafeCalls: [
                  {
                    from: "package",
                    name: ["test", "it", "describe", "suite"],
                    package: "node:test",
                  },
                ],
              },
            ],
          },
        }
      : {},
    // Config files are plain JS and not part of any tsconfig.
    { files: ["**/*.{js,mjs,cjs}"], extends: [tseslint.configs.disableTypeChecked] },
    ...extraConfigs,
    // Must stay last so it can switch off formatting rules.
    prettierConfig,
  );
}

export default createConfig;
