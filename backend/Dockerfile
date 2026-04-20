FROM golang:1.22 AS builder

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/noryx-api ./cmd/noryx-api

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/noryx-api /usr/local/bin/noryx-api

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/noryx-api"]
