FROM golang:1.26.5-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# The deploy-notify commit SHA comes from Coolify's SOURCE_COMMIT runtime env,
# not from a build arg. Keep it out of the build so Docker cache survives across
# commits. The binary is built plain.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /out/server \
    ./cmd/server

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/server /server
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/server"]
