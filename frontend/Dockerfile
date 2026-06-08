FROM nginx:1.27-alpine

LABEL org.opencontainers.image.title="noryx-frontend"
LABEL org.opencontainers.image.description="Noryx CE frontend runtime base image"

RUN mkdir -p /usr/share/nginx/html/vendor \
    && wget -q https://cdn.sheetjs.com/xlsx-0.20.3/package/dist/xlsx.full.min.js \
      -O /usr/share/nginx/html/vendor/xlsx.full.min.js
