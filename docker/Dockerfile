# Modules caching
FROM golang:1.25-bookworm AS modules
WORKDIR /
COPY go.mod go.sum ./
RUN go mod download

# Builder
FROM golang:1.25-bookworm AS builder
COPY --from=modules /go/pkg /go/pkg
WORKDIR /
COPY . /
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o vibecheck .

# Final image
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /vibecheck /vibecheck
ENTRYPOINT ["/vibecheck"]
