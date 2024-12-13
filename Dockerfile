# Build stage
FROM golang:1.23-alpine3.19 AS build

WORKDIR /app

COPY . /app/

RUN go mod download && go build -o /go/bin/main

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install dependencies
RUN apk add --no-cache \
    ca-certificates \
    curl \
    bash \
    chromium \
    chromium-chromedriver

# Copy the binary from the build stage
COPY --from=build /go/bin/main /app/main

# Set PATH for Chrome
ENV PATH="/usr/lib/chromium/:$PATH"

# Run the application
CMD [ "/app/main" ]