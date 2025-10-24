# Multi-stage Dockerfile for Go Lambda with Chrome
# Stage 1: Build Go binary
FROM golang:1.23-alpine AS builder

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the Go binary with size optimization for linux/amd64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o main ./cmd/lambda

# Stage 2: Use AWS Lambda base image
FROM public.ecr.aws/lambda/provided:al2-x86_64

# Install Chrome dependencies
RUN yum update -y && \
    yum install -y \
    wget unzip \
    nss atk at-spi2-atk cups-libs \
    libdrm libXcomposite libXdamage libXrandr libgbm \
    alsa-lib \
    && yum clean all

# Install Chrome
RUN wget -q https://dl.google.com/linux/direct/google-chrome-stable_current_x86_64.rpm \
    && rpm -ivh --nodeps google-chrome-stable_current_x86_64.rpm \
    && rm google-chrome-stable_current_x86_64.rpm

# Copy the Go binary from builder stage
COPY --from=builder /app/main /var/runtime/bootstrap

# Set permissions
RUN chmod +x /var/runtime/bootstrap

# Set Chrome environment variables
ENV CHROME_BIN=/usr/bin/google-chrome-stable \
    CHROME_PATH=/usr/bin/google-chrome-stable

# Set the CMD to your handler
CMD ["bootstrap"]