# Stage 1: Build the Go application using your Makefile
FROM golang:1.24.1 AS builder 

WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker's build cache
COPY go.mod .

# Download dependencies
RUN go mod download

# Copy your Makefile and the rest of the application source code
COPY Makefile .
COPY . .

# IMPORTANT: Ensure your Makefile's 'build' target includes CGO_ENABLED=0
# build:
#         CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/atlas
RUN CGO_ENABLED=0 make build 

# Stage 2: Create the final lean image
FROM alpine:latest

# Set working directory inside the container
WORKDIR /app

# Copy the built executable from the builder stage
# It's located at /app/bin/atlas in the builder stage
COPY --from=builder /app/bin/atlas ./bin/atlas

# Ensure the executable has correct permissions as root, before switching user
RUN chmod +x ./bin/atlas

# Create a non-root user and switch to it for security
RUN adduser -D appuser
USER appuser

# Expose the port(s) your Atlas load balancer listens on.
# REPLACE '8000' with the actual port Atlas listens on (e.g., 80, 8080).
EXPOSE 8000

# Define the primary executable for the container (Atlas itself)
ENTRYPOINT ["./bin/atlas"]

# Define the default command-line arguments for Atlas.
# Based on your 'make run' which is just './bin/atlas',
# we'll assume it runs directly without further subcommands.
# If Atlas takes specific arguments on startup (e.g., --config, --port), add them here.
CMD [] # No default arguments needed if ./bin/atlas runs directly