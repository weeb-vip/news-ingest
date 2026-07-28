# Pure-Go build (segmentio/kafka-go, no CGO/librdkafka) → tiny static image.
FROM golang:1.23 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o main ./cmd

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /app/main .
ARG VERSION
ENV VERSION=$VERSION
ENV PORT=3000
EXPOSE 3000
USER nonroot:nonroot
# Override per-deployment: the consumer runs `serve-consumer`.
CMD ["./main", "serve-api"]
