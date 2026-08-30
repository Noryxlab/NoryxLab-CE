FROM golang:1.25 AS builder

# The version is stamped into the binary rather than supplied as an environment
# variable, so it cannot drift from the image that carries it.
ARG NORYX_VERSION=dev

# Edition selector. The Community image is built without the tag, so the
# Enterprise sources are not compiled into it at all: no environment variable
# can enable a feature whose machine code was never linked. See backend/ee.
ARG NORYX_EDITION_BUILD=community

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN go mod tidy
RUN BUILD_TAGS=""; \
    if [ "${NORYX_EDITION_BUILD}" = "enterprise" ]; then BUILD_TAGS="-tags enterprise"; fi; \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${BUILD_TAGS} \
      -ldflags "-X github.com/Noryxlab/NoryxLab-CE/backend/internal/buildinfo.Version=${NORYX_VERSION}" \
      -o /out/noryx-api ./cmd/noryx-api; \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${BUILD_TAGS} \
      -ldflags "-X github.com/Noryxlab/NoryxLab-CE/backend/internal/buildinfo.Version=${NORYX_VERSION}" \
      -o /out/noryx-project-files ./cmd/noryx-project-files

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/noryx-api /usr/local/bin/noryx-api
COPY --from=builder /out/noryx-project-files /usr/local/bin/noryx-project-files

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/noryx-api"]
