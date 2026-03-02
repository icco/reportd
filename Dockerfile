# Build stage
FROM golang:1.26-alpine AS builder

ENV GOPROXY="https://proxy.golang.org"
ENV CGO_ENABLED=0

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build binary
COPY . .
RUN go build -ldflags="-s -w" -o /server .

# Final stage
FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata bash jq postgresql-client curl python3 && \
    curl -sSL https://sdk.cloud.google.com | bash -s -- --disable-prompts --install-dir=/opt && \
    ln -s /opt/google-cloud-sdk/bin/bq /usr/local/bin/bq && \
    ln -s /opt/google-cloud-sdk/bin/gcloud /usr/local/bin/gcloud

WORKDIR /app

COPY --from=builder /server .
COPY scripts/ ./scripts/

ENV NAT_ENV="production"
EXPOSE 8080

ENTRYPOINT ["/app/server"]
