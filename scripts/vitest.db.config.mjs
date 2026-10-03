// Vitest configuration for the API's database-backed tests, used only by `pnpm test:db`.
// The API's own config includes `src/**/*.spec.ts`; a `*.db-spec.ts` file does not match it,
// so these tests never run in `pnpm test` or `pnpm verify`, which need no database.
// A plain object, so this file does not need vitest to be resolvable from the root.
import { fromRepositoryRoot } from "./lib/repo-root.mjs";

export default {
  test: {
    root: fromRepositoryRoot("apps", "api"),
    include: ["src/**/*.db-spec.ts"],
    // Database tests share one PostgreSQL instance, so files run one after another.
    fileParallelism: false,
  },
};
