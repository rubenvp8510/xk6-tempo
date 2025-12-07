# Build stage
FROM golang:1.21-alpine AS builder

# Install xk6 and build dependencies
RUN apk add --no-cache git
RUN go install go.k6.io/xk6/cmd/xk6@latest

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build custom k6 binary with xk6-tempo extension
RUN xk6 build --with github.com/rvargasp/xk6-tempo=.

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates && \
    addgroup -S k6 && \
    adduser -S -G k6 k6

USER k6

COPY --from=builder /build/k6 /usr/bin/k6

WORKDIR /scripts

ENTRYPOINT ["k6"]

