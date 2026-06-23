# syntax=docker/dockerfile:1.7

FROM node:24-bookworm-slim AS web-deps
WORKDIR /src
RUN corepack enable
COPY pnpm-lock.yaml pnpm-workspace.yaml ./
COPY web/package.json web/package.json
RUN pnpm --dir web install --frozen-lockfile

FROM web-deps AS web-build
COPY web web
RUN pnpm --dir web build

FROM golang:1.26-bookworm AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd cmd
COPY internal internal
COPY migrations migrations
ARG VERSION=0.1.0
ARG BUILD_COMMIT=development
ARG BUILD_TIMESTAMP=development
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags "-s -w -X main.version=${VERSION} -X main.buildCommit=${BUILD_COMMIT} -X main.buildTimestamp=${BUILD_TIMESTAMP}" \
    -o /out/nostos ./cmd/app
RUN mkdir -p /out/data

FROM gcr.io/distroless/base-debian12:nonroot AS runtime
WORKDIR /
COPY --from=go-build /out/nostos /nostos
COPY --from=go-build --chown=nonroot:nonroot /out/data /data
COPY --from=go-build --chown=nonroot:nonroot /src/migrations /migrations
COPY --from=web-build --chown=nonroot:nonroot /src/web/dist /web/dist
ENV APP_ENV=production \
    APP_HOST=0.0.0.0 \
    APP_PORT=7000 \
    DATA_DIR=/data \
    MIGRATIONS_DIR=/migrations \
    WEB_DIST_DIR=/web/dist
EXPOSE 7000
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD ["/nostos", "doctor"]
ENTRYPOINT ["/nostos"]
CMD ["server"]
