# Stage 1: Build
FROM golang:1.25 AS builder

# Set working directory
WORKDIR /app

# Copy source code
COPY . .

# Build the binary
RUN go build -o wimp ./cmd/app/main.go

# Stage 2: Runtime
FROM ubuntu:22.04

# Copy the compiled binary
COPY --from=builder /app/wimp /usr/local/bin/

# Copy the data folder (even if empty)
COPY ./data /app/data

# Set working directory
WORKDIR /app

ENTRYPOINT ["wimp"]
