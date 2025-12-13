# Build stage
FROM golang:1.25-alpine AS builder

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
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /server .
COPY templates/ templates/
COPY public/ public/

ENV NAT_ENV="production"
EXPOSE 8080

ENTRYPOINT ["/app/server"]
