# Build stage
FROM golang:1.23 as builder

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.mod

# Download dependencies
RUN go mod download && go mod tidy

# Copy source code
COPY main.go main.go

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o controller main.go

# Runtime stage
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/controller .
USER 65532:65532

ENTRYPOINT ["/controller"]
