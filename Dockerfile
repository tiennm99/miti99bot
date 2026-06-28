FROM golang:1.25-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# The deploy-notify commit SHA comes from the SOURCE_COMMIT runtime env that
# Coolify injects into the container (see docker-compose.yml), not from a build
# arg — Coolify does not pass build args here. The binary is built plain.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /out/server \
    ./cmd/server

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/server /server
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/server"]
