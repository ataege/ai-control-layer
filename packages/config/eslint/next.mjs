import { defineConfig, globalIgnores } from "eslint/config";
import nextCoreWebVitals from "eslint-config-next/core-web-vitals";
import nextTypescript from "eslint-config-next/typescript";
import prettierConfig from "eslint-config-prettier/flat";

/**
 * Flat-config factory for Next.js apps.
 * @param {object} options
 * @param {string} options.tsconfigRootDir  pass import.meta.dirname from the app
 * @param {string} [options.reactVersion]  pinned because version detection crashes on ESLint 10
 * @param {string[]} [options.ignores]
 * @param {import("eslint").Linter.Config[]} [options.extraConfigs]
 */
export function createNextConfig({
  tsconfigRootDir,
  reactVersion = "19.3",
  ignores = [],
  extraConfigs = [],
} = {}) {
  return defineConfig(
    globalIgnores([".next/**", "out/**", "build/**", "next-env.d.ts", ...ignores]),
    ...nextCoreWebVitals,
    ...nextTypescript,
    {
      languageOptions: { parserOptions: { tsconfigRootDir } },
      // eslint-plugin-react 7.37 calls context.getFilename() (removed in ESLint 10) when the
      // version is "detect". Pinning it skips that code path. Remove once the plugin supports 10.
      settings: { react: { version: reactVersion } },
    },
    ...extraConfigs,
    // Must stay last so it can switch off formatting rules.
    prettierConfig,
  );
}

export default createNextConfig;
