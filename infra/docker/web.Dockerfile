# Next.js web app (standalone output). Build context: the repository root.
# The build needs no .env, no database and no running API.

FROM node:24-alpine AS base
# pnpm is installed inside the image with npm; corepack is being removed from Node.js.
RUN npm install --global pnpm@11.10.0
WORKDIR /repo

FROM base AS build
ENV CI=true NEXT_TELEMETRY_DISABLED=1
# Fill the pnpm store from the lockfile first so this layer is reused until dependencies change.
COPY pnpm-lock.yaml pnpm-workspace.yaml package.json ./
RUN pnpm fetch
COPY . .
RUN pnpm install --prefer-offline --frozen-lockfile --filter "web..."
# Build the workspace packages the web app depends on (for example the contracts), then the app.
RUN pnpm --filter "web^..." run --if-present build
RUN pnpm --filter web run build

FROM node:24-alpine AS runtime
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000 HOSTNAME=0.0.0.0
WORKDIR /app
# The standalone tree mirrors the monorepo layout and relies on its symlinks: copy it as a whole.
COPY --from=build --chown=node:node /repo/apps/web/.next/standalone ./
# Static assets and public files are not part of the standalone output.
COPY --from=build --chown=node:node /repo/apps/web/.next/static ./apps/web/.next/static
COPY --from=build --chown=node:node /repo/apps/web/public ./apps/web/public
# Unprivileged user that ships with the Node image.
USER node
EXPOSE 3000
CMD ["node", "apps/web/server.js"]
