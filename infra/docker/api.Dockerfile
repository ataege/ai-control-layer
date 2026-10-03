# NestJS API. Build context: the repository root.
# The build needs no .env, no database and no running gateway.

FROM node:24-alpine AS base
# pnpm is installed inside the image with npm; corepack is being removed from Node.js.
RUN npm install --global pnpm@11.10.0
WORKDIR /repo

FROM base AS build
ENV CI=true
# Fill the pnpm store from the lockfile first so this layer is reused until dependencies change.
COPY pnpm-lock.yaml pnpm-workspace.yaml package.json ./
RUN pnpm fetch
COPY . .
RUN pnpm install --prefer-offline --frozen-lockfile --filter "api..."
# Build the workspace packages the API depends on (@workspace/contracts), then the API.
RUN pnpm --filter "api^..." run --if-present build
RUN pnpm --filter api run build
# Self-contained directory with production dependencies only
# (works because forceLegacyDeploy is set in pnpm-workspace.yaml).
RUN pnpm --filter api --prod deploy /prod/api

FROM node:24-alpine AS runtime
# Listen on all container interfaces; the host default stays 127.0.0.1.
ENV NODE_ENV=production API_HOST=0.0.0.0 API_PORT=3001
WORKDIR /app
COPY --from=build --chown=node:node /prod/api ./
# Unprivileged user that ships with the Node image.
USER node
EXPOSE 3001
# Started with node directly so the process receives stop signals as PID 1.
CMD ["node", "dist/main.js"]
