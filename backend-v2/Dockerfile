# --- build ------------------------------------------------------------------
# backend uses mattn/go-sqlite3 (cgo), so the build needs gcc — the default
# golang image ships with it; keep CGO_ENABLED=1.
FROM golang:1.26 AS builder
WORKDIR /src
# proxy.golang.org 国内直连被墙，换 goproxy.cn
ENV GOPROXY=https://goproxy.cn,direct

# cache module downloads
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/edith-server ./cmd/server

# --- run --------------------------------------------------------------------
FROM debian:bookworm-slim AS runner
WORKDIR /app
# debian slim 默认不带 ca-certificates，容器内所有 HTTPS 出站
# （LLM/E2B/COS）都会 TLS 校验失败。从 builder(golang) 复制根证书包，
# 避免 apt 联网安装。
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
RUN useradd --uid 10001 app \
    && mkdir -p /app/data \
    && chown -R app:app /app
COPY --from=builder /out/edith-server /app/edith-server
USER app
EXPOSE 8080
CMD ["/app/edith-server"]
