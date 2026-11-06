# syntax=docker/dockerfile:1

# ---------- Build ----------
FROM golang:1.26-alpine AS builder
ENV GOTOOLCHAIN=auto \
    CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# ---------- Runtime ----------
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /out/api /app/api
RUN mkdir -p /app/uploads \
    && addgroup -S app -g 1001 \
    && adduser -S app -u 1001 -G app \
    && chown -R app:app /app
USER app

# PORT มีจาก env ตาม main.go (fallback 8084); ค่าตั้งจากรอบรัน: docker run -e PORT=...
ENV PORT=8084
# main.go serve static /uploads จาก ./uploads — mount volume ตรงนี้
VOLUME ["/app/uploads"]
EXPOSE 8084
ENTRYPOINT ["/app/api"]
