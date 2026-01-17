# Stage 1: Download modules
FROM golang:1.25-bookworm AS modules
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# Stage 2: Install playwright and build binary
FROM golang:1.25-bookworm AS builder
WORKDIR /app

# Copy modules from cache
COPY --from=modules /go/pkg/mod /go/pkg/mod

# Copy source code
COPY . .

# Install playwright-go tool
RUN go install github.com/playwright-community/playwright-go/cmd/playwright@latest

# Install playwright browser binaries
# Use --platform=linux/amd64 to ensure compatibility
RUN /go/bin/playwright install chromium

# Build the Go binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o vibecheck .

# Stage 3: Final image with distroless base
FROM gcr.io/distroless/static:nonroot

WORKDIR /

# Copy Go binary
COPY --from=builder /app/vibecheck /vibecheck

# Copy Playwright browser binaries from builder stage
COPY --from=builder /root/.cache/ms-playwright /ms-playwright

# Set environment variable for playwright to find browsers
ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright
ENV PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1

ENTRYPOINT ["/vibecheck"]
