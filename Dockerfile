# Stage 1: Build the static binary
FROM golang:1.26.5-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o tf-http-backend ./cmd/tf-http-backend

# Create certs directory with correct ownership for nonroot user (UID 65532)
RUN mkdir -p /var/lib/tf-http-backend/certs && chown -R 65532:65532 /var/lib/tf-http-backend

# Stage 2: Minimal runtime image
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /app/tf-http-backend /tf-http-backend
COPY --from=builder /app/configs/config.yaml /configs/config.yaml
COPY --chown=nonroot:nonroot --from=builder /var/lib/tf-http-backend /var/lib/tf-http-backend

EXPOSE 443 80
ENTRYPOINT ["/tf-http-backend"]
CMD ["serve", "--config", "/configs/config.yaml"]
