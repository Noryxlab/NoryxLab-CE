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

# nginx 1.29 rather than 1.27: the older tag sat on Alpine 3.21, whose openssl
# stopped receiving fixes on that branch, and the image scan went red on a CVE
# no change of ours had introduced. A base image is a dependency with a clock
# on it, and pinning one without moving it is how a build that passed in
# January fails in September having done nothing wrong.
FROM nginx:1.29-alpine

# Every security fix the distribution has published, at build time.
#
# The base image is rebuilt on its own schedule, which is not ours: on
# 2026-10-30 it carried brotli-libs 1.2.0-r0 and CVE-2026-111497 - an
# out-of-bounds write in the Brotli decoder, fixed in 1.2.1-r0 and published
# - so the gate that refuses a critical vulnerability refused an image whose
# Dockerfile had not changed. Upgrading here means the same Dockerfile can
# produce different packages on different days, which is the point: this
# image serves static files, nothing in it is pinned for a reason.
RUN apk upgrade --no-cache

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
