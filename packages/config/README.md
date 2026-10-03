# @workspace/config

Shared TypeScript, ESLint and Prettier configuration. No runtime code.

- `tsconfig/base.json`, `nextjs.json`, `nestjs.json`, `react-library.json`, `node-library.json`
- `eslint` exports `createConfig()` for Node and library workspaces.
- `eslint/next` exports `createNextConfig()` for the Next.js app.
- `prettier` is referenced from the root `package.json`.

Changes here affect every workspace, so coordinate them through the integration owner.
