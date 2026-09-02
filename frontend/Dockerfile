# Build stage: compiles the application into hashed, minified assets.
#
# This replaces the previous model, where the whole UI was a single 504 KB
# index.html embedded verbatim in a Kubernetes ConfigMap. That shipped
# untranspiled source straight to the browser, which is how the 2026-05-04
# incident happened: a syntax error Safari could not parse froze the entire
# page, with no build step or smoke test in between to catch it.
FROM node:22-alpine AS build

WORKDIR /app

# Dependencies are installed from the lockfile alone first, so a source-only
# change reuses the cached layer.
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY tsconfig*.json vite.config.ts index.html ./
COPY public ./public
COPY src ./src

RUN npm run build

# Fonts are vendored into the image rather than pulled from a CDN, so the
# platform stays usable on air-gapped installations.
FROM node:22-alpine AS fonts
WORKDIR /fonts
RUN npm init -y >/dev/null \
    && npm install --no-audit --no-fund @fontsource-variable/plus-jakarta-sans@5.2.8 >/dev/null

FROM nginx:1.27-alpine

# `auto` forks one worker per host CPU. On high-density Kubernetes workers this
# can exceed the deliberately small frontend memory limit for a static site.
RUN sed -i -E 's/worker_processes[[:space:]]+auto;/worker_processes 1;/' /etc/nginx/nginx.conf

LABEL org.opencontainers.image.title="noryx-frontend"
LABEL org.opencontainers.image.description="Noryx CE frontend"

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
COPY --from=fonts \
     /fonts/node_modules/@fontsource-variable/plus-jakarta-sans/files/plus-jakarta-sans-latin-wght-normal.woff2 \
     /usr/share/nginx/html/fonts/plus-jakarta-sans.woff2

EXPOSE 80
