FROM golang:1.26.5-alpine AS builder
WORKDIR /src

# The monkeyd-crawler submodule is resolved through a `replace` directive, so
# its go.mod must be present before `go mod download` can read the build list.
# Only the module files are copied here, keeping this layer cached across
# ordinary source edits.
COPY go.mod go.sum ./
COPY third_party/monkeyd-crawler/go.mod third_party/monkeyd-crawler/go.sum ./third_party/monkeyd-crawler/
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
# No fonts are installed here: the monkeyd module's PDF renderer falls back to a
# font compiled into the binary when the host has none.
COPY --from=builder /out/server /server
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/server"]
