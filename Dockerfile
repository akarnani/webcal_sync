FROM golang:1.19-alpine AS builder

# Build arguments for multi-arch support
ARG TARGETOS
ARG TARGETARCH

WORKDIR /build

# Copy everything including vendor directory
COPY . .

# Build the binary for the target platform
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o webcal_sync *.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/webcal_sync .

# Create directory for config files
RUN mkdir -p /app/config

# Set working directory where config files are expected
WORKDIR /app/config

ENTRYPOINT ["/app/webcal_sync"]
