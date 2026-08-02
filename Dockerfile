# CGO build. ep uses confluent-kafka-go, which wraps librdkafka — so unlike the previous
# pure-Go segmentio/kafka-go build this needs a C toolchain. Statically linked anyway, so
# the runtime image stays distroless/static; this matches how anime-sync builds.
FROM golang:1.25 AS builder
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends build-essential \
    && rm -rf /var/lib/apt/lists/*
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags '-s -w -linkmode external -extldflags "-static"' \
    -tags musl -o main ./cmd

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
