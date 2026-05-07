FROM golang:1.23 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /semconv-proxy ./cmd/semconv-proxy

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /semconv-proxy /semconv-proxy
USER 65532:65532

EXPOSE 4317 4318 8080
ENTRYPOINT ["/semconv-proxy"]
