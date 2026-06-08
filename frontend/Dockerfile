FROM node:22-alpine AS keycloak-js

WORKDIR /build
RUN npm init -y >/dev/null \
    && npm install --no-audit --no-fund keycloak-js@26.2.4 esbuild@0.25.5 >/dev/null \
    && printf "import Keycloak from 'keycloak-js'; window.Keycloak = Keycloak;\n" > entry.js \
    && npx esbuild entry.js --bundle --format=iife --minify --outfile=keycloak.js

FROM nginx:1.27-alpine

LABEL org.opencontainers.image.title="noryx-frontend"
LABEL org.opencontainers.image.description="Noryx CE frontend runtime base image"

RUN mkdir -p /usr/share/nginx/html/vendor \
    && wget -q https://cdn.sheetjs.com/xlsx-0.20.3/package/dist/xlsx.full.min.js \
      -O /usr/share/nginx/html/vendor/xlsx.full.min.js

COPY --from=keycloak-js /build/keycloak.js /usr/share/nginx/html/vendor/keycloak.js
