FROM oven/bun:1.4.2 AS frontend
WORKDIR /src
COPY frontend/package.json frontend/bun.lock ./
RUN bun install --frozen-lockfile
COPY frontend/ ./
RUN bun run build

FROM golang:1.27.1-alpine AS backend
RUN apk add --no-cache gcc musl-dev ca-certificates
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=1 go build -trimpath -tags "netgo osusergo" -ldflags '-s -w -linkmode external -extldflags "-static"' -o /deezmails ./cmd/deezmails && mkdir /data

FROM scratch
WORKDIR /app
COPY --from=backend /deezmails /deezmails
COPY --from=backend /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=backend --chown=65532:65532 /data /data
COPY --from=frontend /src/dist ./frontend/dist
ENV SQLITE_PATH=/data/deezmails.db
USER 65532:65532
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/deezmails"]
