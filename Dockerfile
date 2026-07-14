FROM golang:1.25-bookworm AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/kode-stream ./cmd/kode-stream

FROM debian:bookworm-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl && rm -rf /var/lib/apt/lists/* && useradd --system --uid 10001 --home /app --shell /usr/sbin/nologin kode-stream
COPY --from=go-build /out/kode-stream /app/kode-stream
ENV KODE_STREAM_MODE=cloud
ENV KODE_STREAM_PORT=4317
ENV KODE_STREAM_BIND_ADDR=0.0.0.0
EXPOSE 4317
USER 10001:10001
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["curl", "-fsS", "http://127.0.0.1:4317/api/health"]
ENTRYPOINT ["/app/kode-stream", "serve"]
